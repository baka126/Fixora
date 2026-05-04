package controller

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"fixora/pkg/gitops"
	"fixora/pkg/notifications"
	"fixora/pkg/telemetry"
	"fixora/pkg/vcs"
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
		if rec.Status == RemediationPROpened {
			status, ok := c.pollRemediationPR(ctx, rec)
			if !ok || !status.Merged {
				continue
			}
			c.markRemediationStatus(ctx, rec.ID, RemediationObserving, firstNonEmpty(status.URL, rec.PRURL), "PR merged; observing GitOps sync and workload health")
			continue
		}

		if rec.Status != RemediationObserving {
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
		}
	}

	for _, rec := range c.history.RemediationsNeedingRevert(ctx, 20) {
		c.openRevertPR(ctx, rec)
	}
}

func (c *Controller) pollRemediationPR(ctx context.Context, rec RemediationRecord) (vcsStatus, bool) {
	provider, _ := c.getVCSProvider(ctx, rec.Namespace, rec.VCSType)
	if provider == nil {
		slog.Warn("Skipping remediation monitor: no VCS provider", "id", rec.ID, "vcs_type", rec.VCSType)
		return vcsStatus{}, false
	}

	status, err := provider.GetPullRequestStatus(ctx, rec.Options.RepoOwner, rec.Options.RepoName, rec.Options.Head)
	if err != nil {
		slog.Error("Failed to poll remediation PR status", "id", rec.ID, "repo", rec.Options.RepoName, "head", rec.Options.Head, "error", err)
		return vcsStatus{}, false
	}
	if status.State == "closed" && !status.Merged {
		c.markRemediationStatus(ctx, rec.ID, RemediationPRFailed, status.URL, "PR closed without merge")
		return vcsStatus{}, false
	}
	return vcsStatus{URL: status.URL, Merged: status.Merged}, true
}

type vcsStatus struct {
	URL    string
	Merged bool
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
