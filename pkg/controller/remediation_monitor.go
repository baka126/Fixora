package controller

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"reflect"
	"strings"
	"time"

	"fixora/pkg/gitops"
	"fixora/pkg/notifications"
	"fixora/pkg/telemetry"
	"fixora/pkg/vcs"
	"gopkg.in/yaml.v3"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const remediationObservationDelay = 5 * time.Minute

func (c *Controller) monitorRemediationOutcomes() {
	if c.history == nil || !c.history.HasDB() {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	for _, rec := range c.history.RemediationsForMonitoring(ctx, 50) {
		if rec.Status != RemediationObserving && rec.Status != RemediationAwaitingApply {
			status, ok := c.pollRemediationPR(ctx, rec)
			if !ok {
				continue
			}

			if status.Merged {
				c.markRemediationStatus(ctx, rec.ID, RemediationAwaitingApply, firstNonEmpty(status.URL, rec.PRURL), "PR merged; waiting for the remediation changes to be applied in the cluster")
				continue
			}
			if status.State == "open" && rec.Status != RemediationPROpened {
				c.markRemediationStatus(ctx, rec.ID, RemediationPROpened, firstNonEmpty(status.URL, rec.PRURL), "PR detected; waiting for merge")
			}
			continue
		}

		if rec.Status == RemediationAwaitingApply {
			applied, failure := c.remediationAppliedForObservation(ctx, rec)
			if failure != "" {
				c.markProductionRemediationFailure(ctx, rec, failure)
				continue
			}
			if !applied {
				continue
			}
			c.saveRemediationWorkloadSnapshot(ctx, rec.ID, "post_merge", rec.Namespace, firstNonEmpty(rec.WorkloadKind, "Pod"), firstNonEmpty(rec.WorkloadName, rec.PodName))
			c.markRemediationStatus(ctx, rec.ID, RemediationObserving, rec.PRURL, "Remediation changes detected in cluster; observing workload health")
			continue
		}

		if !rec.UpdatedAt.IsZero() && time.Since(rec.UpdatedAt) < remediationObservationDelay {
			continue
		}

		ready, gitOpsFailure := c.gitOpsReadyForObservation(ctx, rec)
		if gitOpsFailure != "" {
			c.markProductionRemediationFailure(ctx, rec, gitOpsFailure)
			continue
		}
		if !ready {
			continue
		}

		if failure := c.workloadRegressionReason(ctx, rec); failure != "" {
			c.markProductionRemediationFailure(ctx, rec, failure)
			continue
		}
		c.markRemediationStatus(ctx, rec.ID, RemediationSucceeded, rec.PRURL, "Post-merge observation completed without regression")
		telemetry.IncRemediation(string(RemediationSucceeded), rec.PatchStrategy)
	}

	// Reverts are intentionally operator-triggered from the UI once production_failed
	// is visible. This keeps rollback creation deliberate after the failed fix is
	// confirmed applied and observed.
}

func (c *Controller) pollRemediationPR(ctx context.Context, rec RemediationRecord) (vcsStatus, bool) {
	provider, _ := c.getVCSProvider(ctx, rec.Namespace, rec.VCSType)
	if provider == nil {
		slog.Warn("Skipping remediation monitor: no VCS provider", "id", rec.ID, "vcs_type", rec.VCSType)
		return vcsStatus{}, false
	}

	status, err := remediationPRStatus(ctx, provider, rec)
	if err != nil {
		slog.Error("Failed to poll remediation PR status", "id", rec.ID, "repo", rec.Options.RepoName, "head", rec.Options.Head, "pr_url", rec.PRURL, "error", err)
		return vcsStatus{}, false
	}
	status.State = strings.ToLower(status.State)
	if status.State == "opened" {
		status.State = "open"
	}
	if status.State == "merged" {
		status.Merged = true
	}
	if status.State == "" || status.State == "not_found" {
		return vcsStatus{}, false
	}
	if status.State == "closed" && !status.Merged {
		c.markRemediationStatus(ctx, rec.ID, RemediationPRFailed, status.URL, "PR closed without merge")
		return vcsStatus{}, false
	}
	return vcsStatus{URL: status.URL, State: status.State, Merged: status.Merged}, true
}

type vcsStatus struct {
	URL    string
	State  string
	Merged bool
}

type pullRequestURLStatusProvider interface {
	GetPullRequestStatusByURL(ctx context.Context, prURL string) (vcs.PullRequestStatus, error)
}

func remediationPRStatus(ctx context.Context, provider vcs.Provider, rec RemediationRecord) (vcs.PullRequestStatus, error) {
	if rec.PRURL != "" {
		if urlProvider, ok := provider.(pullRequestURLStatusProvider); ok {
			status, err := urlProvider.GetPullRequestStatusByURL(ctx, rec.PRURL)
			if err == nil && status.State != "" && status.State != "not_found" {
				return status, nil
			}
			if err != nil {
				slog.Debug("Failed to poll remediation PR by URL; falling back to branch", "id", rec.ID, "pr_url", rec.PRURL, "error", err)
			}
		}
	}
	if rec.Options.RepoOwner == "" || rec.Options.RepoName == "" || rec.Options.Head == "" {
		return vcs.PullRequestStatus{State: "not_found"}, nil
	}
	return provider.GetPullRequestStatus(ctx, rec.Options.RepoOwner, rec.Options.RepoName, rec.Options.Head)
}

func (c *Controller) remediationAppliedForObservation(ctx context.Context, rec RemediationRecord) (bool, string) {
	ready, gitOpsFailure := c.gitOpsReadyForObservation(ctx, rec)
	if !ready {
		return false, ""
	}

	expectations := remediationContainerExpectations(rec)
	if len(expectations) == 0 {
		if gitOpsFailure != "" {
			return true, gitOpsFailure
		}
		if rec.Source.Controller == gitops.ControllerArgoCD || rec.Source.Controller == gitops.ControllerFlux {
			return true, ""
		}
		slog.Debug("Waiting for manual apply before observing remediation", "id", rec.ID, "namespace", rec.Namespace, "workload_kind", rec.WorkloadKind, "workload", rec.WorkloadName)
		return false, ""
	}

	live, ok := c.liveContainerStates(ctx, rec)
	if !ok {
		return false, ""
	}
	for _, expected := range expectations {
		if expected.Name == "" {
			if !expected.matchesAny(live) {
				return false, ""
			}
			continue
		}
		actual, ok := live[expected.Name]
		if !ok || !expected.matches(actual) {
			return false, ""
		}
	}
	if gitOpsFailure != "" {
		return true, gitOpsFailure
	}
	return true, ""
}

type remediationContainerExpectation struct {
	Name      string
	Image     string
	EnvHashes map[string]string
	Requests  map[string]string
	Limits    map[string]string
}

type liveContainerState struct {
	Image     string
	EnvHashes map[string]string
	Requests  map[string]string
	Limits    map[string]string
}

func (e remediationContainerExpectation) matches(actual liveContainerState) bool {
	if e.Image != "" && e.Image != actual.Image {
		return false
	}
	for key, value := range e.EnvHashes {
		if actual.EnvHashes[key] != value {
			return false
		}
	}
	for key, value := range e.Requests {
		if actual.Requests[key] != value {
			return false
		}
	}
	for key, value := range e.Limits {
		if actual.Limits[key] != value {
			return false
		}
	}
	return true
}

func (e remediationContainerExpectation) matchesAny(live map[string]liveContainerState) bool {
	for _, actual := range live {
		if e.matches(actual) {
			return true
		}
	}
	return false
}

func remediationContainerExpectations(rec RemediationRecord) []remediationContainerExpectation {
	merged := map[string]remediationContainerExpectation{}
	for _, file := range rec.ChangedFiles {
		for _, expected := range containerExpectationsFromApplyHints(file.ApplyHints) {
			mergeContainerExpectation(merged, expected)
		}
	}
	out := make([]remediationContainerExpectation, 0, len(merged))
	for _, expected := range merged {
		if expected.Image == "" && len(expected.EnvHashes) == 0 && len(expected.Requests) == 0 && len(expected.Limits) == 0 {
			continue
		}
		out = append(out, expected)
	}
	return out
}

func remediationApplyHintsFromContent(content []byte) []remediationApplyHint {
	merged := map[string]remediationContainerExpectation{}
	for _, manifest := range yamlDocuments(content) {
		expectations := manifestContainerExpectations(manifest)
		if len(expectations) == 0 {
			expectations = helmValueExpectations(manifest)
		}
		for _, expected := range expectations {
			mergeContainerExpectation(merged, expected)
		}
	}
	out := make([]remediationApplyHint, 0, len(merged))
	for _, expected := range merged {
		if expected.Image == "" && len(expected.EnvHashes) == 0 && len(expected.Requests) == 0 && len(expected.Limits) == 0 {
			continue
		}
		out = append(out, remediationApplyHint{
			ContainerName:  expected.Name,
			Image:          expected.Image,
			EnvValueHashes: expected.EnvHashes,
			Requests:       expected.Requests,
			Limits:         expected.Limits,
		})
	}
	return out
}

func containerExpectationsFromApplyHints(hints []remediationApplyHint) []remediationContainerExpectation {
	out := make([]remediationContainerExpectation, 0, len(hints))
	for _, hint := range hints {
		out = append(out, remediationContainerExpectation{
			Name:      hint.ContainerName,
			Image:     hint.Image,
			EnvHashes: hint.EnvValueHashes,
			Requests:  hint.Requests,
			Limits:    hint.Limits,
		})
	}
	return out
}

func mergeContainerExpectation(merged map[string]remediationContainerExpectation, expected remediationContainerExpectation) {
	current := merged[expected.Name]
	current.Name = expected.Name
	if expected.Image != "" {
		current.Image = expected.Image
	}
	if len(expected.EnvHashes) > 0 {
		if current.EnvHashes == nil {
			current.EnvHashes = map[string]string{}
		}
		for k, v := range expected.EnvHashes {
			current.EnvHashes[k] = v
		}
	}
	if len(expected.Requests) > 0 {
		if current.Requests == nil {
			current.Requests = map[string]string{}
		}
		for k, v := range expected.Requests {
			current.Requests[k] = v
		}
	}
	if len(expected.Limits) > 0 {
		if current.Limits == nil {
			current.Limits = map[string]string{}
		}
		for k, v := range expected.Limits {
			current.Limits[k] = v
		}
	}
	merged[expected.Name] = current
}

func helmValueExpectations(values map[string]interface{}) []remediationContainerExpectation {
	if image := helmImageValue(values["image"]); image != "" {
		return []remediationContainerExpectation{{Image: image}}
	}
	return nil
}

func helmImageValue(raw interface{}) string {
	switch value := raw.(type) {
	case string:
		return strings.TrimSpace(value)
	case map[string]interface{}:
		repository := stringValue(firstMapValue(value, "repository", "repo", "name"))
		tag := stringValue(value["tag"])
		digest := stringValue(value["digest"])
		if repository == "" {
			return ""
		}
		if digest != "" {
			return repository + "@" + digest
		}
		if tag != "" {
			return repository + ":" + tag
		}
		return repository
	default:
		return ""
	}
}

func firstMapValue(values map[string]interface{}, keys ...string) interface{} {
	for _, key := range keys {
		if value, ok := values[key]; ok {
			return value
		}
	}
	return nil
}

func yamlDocuments(content []byte) []map[string]interface{} {
	decoder := yaml.NewDecoder(bytes.NewReader(content))
	var docs []map[string]interface{}
	for {
		var doc map[string]interface{}
		err := decoder.Decode(&doc)
		if err != nil {
			break
		}
		if len(doc) > 0 {
			docs = append(docs, doc)
		}
	}
	return docs
}

func manifestMatchesRemediation(manifest map[string]interface{}, rec RemediationRecord) bool {
	kind := stringValue(manifest["kind"])
	if kind == "" {
		return false
	}
	meta, _ := manifest["metadata"].(map[string]interface{})
	name := stringValue(meta["name"])
	namespace := firstNonEmpty(stringValue(meta["namespace"]), rec.Namespace)
	if namespace != "" && rec.Namespace != "" && namespace != rec.Namespace {
		return false
	}
	targetKind := firstNonEmpty(rec.WorkloadKind, "Pod")
	targetName := firstNonEmpty(rec.WorkloadName, rec.PodName)
	return strings.EqualFold(kind, targetKind) && (name == "" || name == targetName)
}

func manifestContainerExpectations(manifest map[string]interface{}) []remediationContainerExpectation {
	spec, _ := manifest["spec"].(map[string]interface{})
	switch strings.ToLower(stringValue(manifest["kind"])) {
	case "deployment", "statefulset", "daemonset":
		template := mapValue(spec["template"])
		podSpec := mapValue(template["spec"])
		return containerExpectationsFromPodSpec(podSpec)
	case "job":
		template := mapValue(spec["template"])
		podSpec := mapValue(template["spec"])
		return containerExpectationsFromPodSpec(podSpec)
	case "cronjob":
		jobTemplate := mapValue(spec["jobTemplate"])
		jobSpec := mapValue(jobTemplate["spec"])
		template := mapValue(jobSpec["template"])
		podSpec := mapValue(template["spec"])
		return containerExpectationsFromPodSpec(podSpec)
	case "pod":
		return containerExpectationsFromPodSpec(spec)
	default:
		return nil
	}
}

func containerExpectationsFromPodSpec(podSpec map[string]interface{}) []remediationContainerExpectation {
	var out []remediationContainerExpectation
	for _, key := range []string{"initContainers", "containers"} {
		for _, raw := range sliceValue(podSpec[key]) {
			container := mapValue(raw)
			name := stringValue(container["name"])
			if name == "" {
				continue
			}
			expected := remediationContainerExpectation{
				Name:      name,
				Image:     stringValue(container["image"]),
				EnvHashes: envExpectations(container["env"]),
				Requests:  resourceExpectations(container, "requests"),
				Limits:    resourceExpectations(container, "limits"),
			}
			out = append(out, expected)
		}
	}
	return out
}

func envExpectations(raw interface{}) map[string]string {
	env := map[string]string{}
	for _, item := range sliceValue(raw) {
		entry := mapValue(item)
		name := stringValue(entry["name"])
		value := stringValue(entry["value"])
		if name != "" && value != "" {
			env[name] = hashApplyValue(value)
		}
	}
	return env
}

func resourceExpectations(container map[string]interface{}, section string) map[string]string {
	resources := mapValue(container["resources"])
	values := mapValue(resources[section])
	out := map[string]string{}
	for name, raw := range values {
		if value := stringValue(raw); value != "" {
			out[name] = value
		}
	}
	return out
}

func (c *Controller) liveContainerStates(ctx context.Context, rec RemediationRecord) (map[string]liveContainerState, bool) {
	switch firstNonEmpty(rec.WorkloadKind, "Pod") {
	case "Deployment":
		workload, err := c.clientset.AppsV1().Deployments(rec.Namespace).Get(ctx, rec.WorkloadName, metav1.GetOptions{})
		if err != nil {
			return nil, false
		}
		return liveContainerStatesFromPodSpec(workload.Spec.Template.Spec), true
	case "StatefulSet":
		workload, err := c.clientset.AppsV1().StatefulSets(rec.Namespace).Get(ctx, rec.WorkloadName, metav1.GetOptions{})
		if err != nil {
			return nil, false
		}
		return liveContainerStatesFromPodSpec(workload.Spec.Template.Spec), true
	case "DaemonSet":
		workload, err := c.clientset.AppsV1().DaemonSets(rec.Namespace).Get(ctx, rec.WorkloadName, metav1.GetOptions{})
		if err != nil {
			return nil, false
		}
		return liveContainerStatesFromPodSpec(workload.Spec.Template.Spec), true
	case "Job":
		workload, err := c.clientset.BatchV1().Jobs(rec.Namespace).Get(ctx, rec.WorkloadName, metav1.GetOptions{})
		if err != nil {
			return nil, false
		}
		return liveContainerStatesFromPodSpec(workload.Spec.Template.Spec), true
	case "CronJob":
		workload, err := c.clientset.BatchV1().CronJobs(rec.Namespace).Get(ctx, rec.WorkloadName, metav1.GetOptions{})
		if err != nil {
			return nil, false
		}
		return liveContainerStatesFromPodSpec(workload.Spec.JobTemplate.Spec.Template.Spec), true
	default:
		podName := firstNonEmpty(rec.WorkloadName, rec.PodName)
		pod, err := c.clientset.CoreV1().Pods(rec.Namespace).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			return nil, false
		}
		return liveContainerStatesFromPodSpec(pod.Spec), true
	}
}

func liveContainerStatesFromPodSpec(podSpec v1.PodSpec) map[string]liveContainerState {
	out := map[string]liveContainerState{}
	for _, container := range append(append([]v1.Container{}, podSpec.InitContainers...), podSpec.Containers...) {
		state := liveContainerState{
			Image:     container.Image,
			EnvHashes: map[string]string{},
			Requests:  map[string]string{},
			Limits:    map[string]string{},
		}
		for _, env := range container.Env {
			if env.Value != "" {
				state.EnvHashes[env.Name] = hashApplyValue(env.Value)
			}
		}
		for name, quantity := range container.Resources.Requests {
			state.Requests[string(name)] = quantity.String()
		}
		for name, quantity := range container.Resources.Limits {
			state.Limits[string(name)] = quantity.String()
		}
		out[container.Name] = state
	}
	return out
}

func hashApplyValue(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func mapValue(raw interface{}) map[string]interface{} {
	if value, ok := raw.(map[string]interface{}); ok {
		return value
	}
	return nil
}

func sliceValue(raw interface{}) []interface{} {
	if value, ok := raw.([]interface{}); ok {
		return value
	}
	return nil
}

func stringValue(raw interface{}) string {
	switch value := raw.(type) {
	case string:
		return strings.TrimSpace(value)
	case fmt.Stringer:
		return strings.TrimSpace(value.String())
	case int, int64, float64, bool:
		return strings.TrimSpace(fmt.Sprint(value))
	default:
		if reflect.ValueOf(raw).IsValid() {
			return strings.TrimSpace(fmt.Sprint(raw))
		}
		return ""
	}
}

func (c *Controller) gitOpsReadyForObservation(ctx context.Context, rec RemediationRecord) (bool, string) {
	if rec.Source.Controller == gitops.ControllerFlux {
		return c.fluxReadyForObservation(ctx, rec)
	}
	if rec.Source.Controller != gitops.ControllerArgoCD || rec.Source.AppName == "" || c.dynamicClient == nil {
		return true, ""
	}

	app, err := c.dynamicClient.Resource(schema.GroupVersionResource{
		Group:    "argoproj.io",
		Version:  "v1alpha1",
		Resource: "applications",
	}).Namespace(firstNonEmpty(c.config.ArgoCDNamespace, "argocd")).Get(ctx, rec.Source.AppName, metav1.GetOptions{})
	if err != nil {
		slog.Debug("ArgoCD application not ready for remediation observation", "id", rec.ID, "app", rec.Source.AppName, "error", err)
		return false, ""
	}

	syncStatus, _, _ := unstructured.NestedString(app.Object, "status", "sync", "status")
	healthStatus, _, _ := unstructured.NestedString(app.Object, "status", "health", "status")
	if syncStatus != "" && syncStatus != "Synced" {
		return false, ""
	}
	if healthStatus == "Degraded" {
		return true, fmt.Sprintf("ArgoCD application %s is Degraded after sync", rec.Source.AppName)
	}
	return true, ""
}

func (c *Controller) fluxReadyForObservation(ctx context.Context, rec RemediationRecord) (bool, string) {
	if c.dynamicClient == nil || rec.Source.AppName == "" {
		return true, ""
	}
	namespace := firstNonEmpty(rec.Source.AppNamespace, rec.Namespace)
	var gvrs []schema.GroupVersionResource
	switch rec.Source.ManifestType {
	case gitops.ManifestFluxHelmRelease:
		gvrs = []schema.GroupVersionResource{
			{Group: "helm.toolkit.fluxcd.io", Version: "v2", Resource: "helmreleases"},
			{Group: "helm.toolkit.fluxcd.io", Version: "v2beta2", Resource: "helmreleases"},
		}
	default:
		gvrs = []schema.GroupVersionResource{
			{Group: "kustomize.toolkit.fluxcd.io", Version: "v1", Resource: "kustomizations"},
			{Group: "kustomize.toolkit.fluxcd.io", Version: "v1beta2", Resource: "kustomizations"},
		}
	}

	for _, gvr := range gvrs {
		obj, err := c.dynamicClient.Resource(gvr).Namespace(namespace).Get(ctx, rec.Source.AppName, metav1.GetOptions{})
		if err != nil {
			continue
		}
		return fluxObjectReady(obj, rec.Source.AppName)
	}
	slog.Debug("Flux object not ready for remediation observation", "id", rec.ID, "namespace", namespace, "app", rec.Source.AppName)
	return false, ""
}

func fluxObjectReady(obj *unstructured.Unstructured, name string) (bool, string) {
	conditions, ok, _ := unstructured.NestedSlice(obj.Object, "status", "conditions")
	if !ok {
		return false, ""
	}
	for _, raw := range conditions {
		condition, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		condType, _ := condition["type"].(string)
		if condType != "Ready" {
			continue
		}
		status, _ := condition["status"].(string)
		reason, _ := condition["reason"].(string)
		message, _ := condition["message"].(string)
		switch status {
		case "True":
			return true, ""
		case "False":
			return true, fmt.Sprintf("Flux object %s Ready=False reason=%s message=%s", name, reason, message)
		default:
			return false, ""
		}
	}
	return false, ""
}

func (c *Controller) workloadRegressionReason(ctx context.Context, rec RemediationRecord) string {
	if reason := c.workloadSnapshotRegressionReason(ctx, rec); reason != "" {
		return reason
	}
	if reason := c.workloadRolloutRegressionReason(ctx, rec); reason != "" {
		return reason
	}

	if rec.WorkloadSelector != "" {
		selector, err := labels.Parse(rec.WorkloadSelector)
		if err == nil && !selector.Empty() {
			pods, err := c.clientset.CoreV1().Pods(rec.Namespace).List(ctx, metav1.ListOptions{LabelSelector: selector.String()})
			if err == nil {
				if len(pods.Items) == 0 {
					return fmt.Sprintf("no pods found for remediated %s %s after rollout", firstNonEmpty(rec.WorkloadKind, "workload"), firstNonEmpty(rec.WorkloadName, rec.PodName))
				}
				for i := range pods.Items {
					if reason := podFailureReason(&pods.Items[i]); reason != "" {
						return reason
					}
				}
				return ""
			}
		}
	}

	pod, err := c.clientset.CoreV1().Pods(rec.Namespace).Get(ctx, rec.PodName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return ""
	}
	if err != nil {
		slog.Debug("Unable to inspect remediated pod", "id", rec.ID, "ns", rec.Namespace, "pod", rec.PodName, "error", err)
		return ""
	}

	if reason := podFailureReason(pod); reason != "" {
		return reason
	}

	if c.promClient != nil && c.config.PrometheusHighErrorRateThreshold > 0 {
		errRate, err := c.promClient.GetHTTPErrorRate(rec.Namespace, rec.PodName)
		if err == nil && errRate > c.config.PrometheusHighErrorRateThreshold {
			return fmt.Sprintf("HTTP error rate %.2f%% exceeded threshold %.2f%% after remediation", errRate*100, c.config.PrometheusHighErrorRateThreshold*100)
		}
	}
	return ""
}

func (c *Controller) workloadSnapshotRegressionReason(ctx context.Context, rec RemediationRecord) string {
	if c == nil || c.history == nil || !c.history.HasDB() {
		return ""
	}
	before, ok := c.history.LatestRemediationSnapshot(ctx, rec.ID, "pre_pr")
	if !ok {
		return ""
	}
	after, ok := c.captureWorkloadSnapshot(ctx, rec.Namespace, firstNonEmpty(rec.WorkloadKind, "Pod"), firstNonEmpty(rec.WorkloadName, rec.PodName))
	if !ok {
		return ""
	}
	c.history.SaveRemediationSnapshot(ctx, rec.ID, "observation", after)
	return workloadSnapshotRegressionReason(before, after)
}

func (c *Controller) workloadRolloutRegressionReason(ctx context.Context, rec RemediationRecord) string {
	switch rec.WorkloadKind {
	case "Deployment":
		deploy, err := c.clientset.AppsV1().Deployments(rec.Namespace).Get(ctx, rec.WorkloadName, metav1.GetOptions{})
		if err != nil {
			return ""
		}
		desired := int32(1)
		if deploy.Spec.Replicas != nil {
			desired = *deploy.Spec.Replicas
		}
		if desired > 0 && deploy.Status.AvailableReplicas == 0 {
			return fmt.Sprintf("Deployment %s has no available replicas after remediation", deploy.Name)
		}
		if deploy.Status.UnavailableReplicas > 0 && deploy.Status.UpdatedReplicas < desired {
			return fmt.Sprintf("Deployment %s rollout unavailable=%d updated=%d desired=%d", deploy.Name, deploy.Status.UnavailableReplicas, deploy.Status.UpdatedReplicas, desired)
		}
	case "StatefulSet":
		sts, err := c.clientset.AppsV1().StatefulSets(rec.Namespace).Get(ctx, rec.WorkloadName, metav1.GetOptions{})
		if err != nil {
			return ""
		}
		if sts.Status.Replicas > 0 && sts.Status.ReadyReplicas == 0 {
			return fmt.Sprintf("StatefulSet %s has no ready replicas after remediation", sts.Name)
		}
	case "DaemonSet":
		ds, err := c.clientset.AppsV1().DaemonSets(rec.Namespace).Get(ctx, rec.WorkloadName, metav1.GetOptions{})
		if err != nil {
			return ""
		}
		if ds.Status.DesiredNumberScheduled > 0 && ds.Status.NumberReady == 0 {
			return fmt.Sprintf("DaemonSet %s has no ready pods after remediation", ds.Name)
		}
	}
	return ""
}

func podFailureReason(pod *v1.Pod) string {
	if pod.Status.Phase == v1.PodFailed {
		return firstNonEmpty(pod.Status.Reason, "pod entered Failed phase")
	}
	for _, status := range pod.Status.ContainerStatuses {
		if status.State.Waiting != nil {
			switch status.State.Waiting.Reason {
			case "CrashLoopBackOff", "ImagePullBackOff", "ErrImagePull", "CreateContainerConfigError", "CreateContainerError":
				return fmt.Sprintf("container %s entered %s after remediation", status.Name, status.State.Waiting.Reason)
			}
		}
		if status.State.Terminated != nil {
			switch status.State.Terminated.Reason {
			case "OOMKilled", "Error":
				return fmt.Sprintf("container %s terminated with %s after remediation", status.Name, status.State.Terminated.Reason)
			}
		}
	}
	return ""
}

func (c *Controller) markProductionRemediationFailure(ctx context.Context, rec RemediationRecord, reason string) {
	c.markRemediationStatus(ctx, rec.ID, RemediationProductionFailed, rec.PRURL, reason)
	telemetry.IncRemediation(string(RemediationProductionFailed), rec.PatchStrategy)
	notifications.SendNotification(c.config, fmt.Sprintf("❌ Fixora remediation failed after merge for %s/%s: %s\nPR: %s", rec.Namespace, rec.PodName, reason, firstNonEmpty(rec.PRURL, rec.Options.Head)))
}

func (c *Controller) openRevertPR(ctx context.Context, rec RemediationRecord) {
	provider, _ := c.getVCSProvider(ctx, rec.Namespace, rec.VCSType)
	if provider == nil {
		slog.Warn("Skipping remediation revert: no VCS provider", "id", rec.ID, "vcs_type", rec.VCSType)
		return
	}

	revertFiles, err := buildRevertFileChanges(rec.ChangedFiles)
	if err != nil {
		c.markRemediationStatus(ctx, rec.ID, RemediationRevertFailed, rec.PRURL, "cannot safely generate revert PR: "+err.Error())
		notifications.SendNotification(c.config, fmt.Sprintf("⚠️ Fixora could not safely generate a revert PR for %s/%s: %v", rec.Namespace, rec.PodName, err))
		return
	}

	head := fmt.Sprintf("fixora/revert-%s-%d-%d", slugify(rec.PodName), rec.ID, time.Now().Unix())
	opts := vcs.PullRequestOptions{
		Title:         fmt.Sprintf("Fixora: revert failed remediation for %s/%s", rec.Namespace, rec.PodName),
		Body:          revertPRBody(rec),
		Head:          head,
		Base:          rec.Options.Base,
		RepoOwner:     rec.Options.RepoOwner,
		RepoName:      rec.Options.RepoName,
		Files:         revertFiles,
		CommitMessage: fmt.Sprintf("revert: failed Fixora remediation for %s/%s", rec.Namespace, rec.PodName),
	}

	prURL, err := provider.CreatePullRequest(ctx, opts)
	if err != nil {
		slog.Error("Failed to create remediation revert PR", "id", rec.ID, "repo", rec.Options.RepoName, "error", err)
		c.markRemediationStatus(ctx, rec.ID, RemediationRevertFailed, rec.PRURL, "failed to create revert PR: "+err.Error())
		telemetry.IncRemediation(string(RemediationRevertFailed), rec.PatchStrategy)
		return
	}
	if prURL == "" {
		return
	}

	c.history.MarkRemediationRevertOpened(ctx, rec.ID, prURL, head)
	telemetry.IncRemediation(string(RemediationRevertOpened), rec.PatchStrategy)
	notifications.SendNotification(c.config, fmt.Sprintf("↩️ Opened revert PR for failed Fixora remediation on %s/%s:\n%s\nOriginal PR: %s", rec.Namespace, rec.PodName, prURL, firstNonEmpty(rec.PRURL, rec.Options.Head)))
}

func buildRevertFileChanges(changed []remediationChangedFile) ([]vcs.FileChange, error) {
	if len(changed) == 0 {
		return nil, fmt.Errorf("no changed files recorded")
	}
	revertFiles := make([]vcs.FileChange, 0, len(changed))
	for _, file := range changed {
		if file.FilePath == "" {
			return nil, fmt.Errorf("changed file has no path")
		}
		if file.Create {
			revertFiles = append(revertFiles, vcs.FileChange{FilePath: file.FilePath, Delete: true})
			continue
		}
		if !file.HasPrevious {
			return nil, fmt.Errorf("missing previous content for %s", file.FilePath)
		}
		revertFiles = append(revertFiles, vcs.FileChange{
			FilePath:   file.FilePath,
			NewContent: append([]byte(nil), file.PreviousContent...),
		})
	}
	return revertFiles, nil
}

func revertPRBody(rec RemediationRecord) string {
	return fmt.Sprintf(`### Revert Failed Fixora Remediation

Fixora observed a production regression after the original remediation was merged.

* **Workload:** %s/%s
* **Original PR:** %s
* **Patch Strategy:** %s
* **Failure:** %s
* **GitOps Source:** %s

This PR only restores files changed by the failed Fixora remediation.`,
		rec.Namespace,
		rec.PodName,
		firstNonEmpty(rec.PRURL, rec.Options.Head),
		rec.PatchStrategy,
		firstNonEmpty(rec.FailureReason, "production failure recorded"),
		rec.Source.Summary(),
	)
}
