package controller

import (
	"context"
	"fmt"
	"sort"
	"strings"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type IssueCategory string

const (
	CategoryUnknown    IssueCategory = "unknown"
	CategoryScheduling IssueCategory = "scheduling-capacity"
	CategoryRuntime    IssueCategory = "runtime"
	CategoryConfig     IssueCategory = "configuration"
	CategoryRollout    IssueCategory = "rollout"
	CategoryNetwork    IssueCategory = "network"
	CategoryStorage    IssueCategory = "storage"
)

type PatchStrategy string

const (
	PatchNone             PatchStrategy = "none"
	PatchResources        PatchStrategy = "resources"
	PatchImage            PatchStrategy = "image"
	PatchEnvOrVolumeRef   PatchStrategy = "env-or-volume-ref"
	PatchSchedulingPolicy PatchStrategy = "scheduling-policy"
	PatchProbe            PatchStrategy = "probe"
	PatchServiceSelector  PatchStrategy = "service-selector"
	PatchPVC              PatchStrategy = "pvc-or-volume"
)

type Diagnosis struct {
	Symptom       string
	Category      IssueCategory
	LikelyCause   string
	Confidence    int
	PatchStrategy PatchStrategy
	Evidence      []string
	Related       []string
}

func (d Diagnosis) Summary() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Diagnosis: %s\nCategory: %s\nLikely Cause: %s\nConfidence: %d%%\nRecommended Patch Strategy: %s",
		d.Symptom, d.Category, d.LikelyCause, d.Confidence, d.PatchStrategy))

	if len(d.Evidence) > 0 {
		sb.WriteString("\nEvidence:")
		for _, item := range d.Evidence {
			sb.WriteString("\n- " + item)
		}
	}
	if len(d.Related) > 0 {
		sb.WriteString("\nRelated Resources:")
		for _, item := range d.Related {
			sb.WriteString("\n- " + item)
		}
	}
	return sb.String()
}

func (c *Controller) classifyPodIssue(ctx context.Context, pod *v1.Pod, triggerReason string) Diagnosis {
	events, _ := c.listPodEvents(ctx, pod)
	eventText := eventsToText(events)
	containerSummary := summarizeContainerStates(pod)

	base := Diagnosis{
		Symptom:       firstNonEmpty(triggerReason, string(pod.Status.Phase)),
		Category:      CategoryUnknown,
		LikelyCause:   "No deterministic Kubernetes classifier matched; use logs, events, and metrics for deeper analysis.",
		Confidence:    35,
		PatchStrategy: PatchNone,
		Evidence:      []string{fmt.Sprintf("Pod phase is %s", pod.Status.Phase)},
	}
	if containerSummary != "" {
		base.Evidence = append(base.Evidence, containerSummary)
	}

	if d, ok := c.classifyScheduling(ctx, pod, events, eventText, triggerReason); ok {
		return d
	}
	if d, ok := classifyImagePull(pod, events, eventText, triggerReason); ok {
		return d
	}
	if d, ok := classifyConfigError(pod, events, eventText, triggerReason); ok {
		return d
	}
	if d, ok := c.classifyStorage(ctx, pod, events, eventText); ok {
		return d
	}
	if d, ok := classifyOOM(pod, triggerReason); ok {
		return d
	}
	if d, ok := classifyProbeFailure(pod, events, eventText, triggerReason); ok {
		return d
	}
	if d, ok := c.classifyNetwork(ctx, pod); ok {
		return d
	}
	if d, ok := classifyCrashLoop(pod, triggerReason); ok {
		return d
	}

	if len(events) > 0 {
		base.Evidence = append(base.Evidence, lastEventEvidence(events))
	}
	return base
}

func (c *Controller) classifyScheduling(ctx context.Context, pod *v1.Pod, events []v1.Event, eventText, triggerReason string) (Diagnosis, bool) {
	hasUnschedulable := strings.Contains(strings.ToLower(triggerReason+" "+eventText), "unschedulable")
	for _, cond := range pod.Status.Conditions {
		if cond.Type == v1.PodScheduled && cond.Status == v1.ConditionFalse && cond.Reason == "Unschedulable" {
			hasUnschedulable = true
			break
		}
	}
	if !hasUnschedulable {
		return Diagnosis{}, false
	}

	evidence := []string{"Pod is pending or has FailedScheduling/Unschedulable evidence."}
	related := []string{}
	for _, event := range events {
		if event.Reason == "FailedScheduling" || strings.Contains(strings.ToLower(event.Message), "unschedulable") {
			evidence = append(evidence, fmt.Sprintf("Scheduling event: %s", event.Message))
			break
		}
	}
	if len(pod.Spec.NodeSelector) > 0 {
		evidence = append(evidence, fmt.Sprintf("Node selector: %v", pod.Spec.NodeSelector))
	}
	if len(pod.Spec.Tolerations) > 0 {
		evidence = append(evidence, fmt.Sprintf("Tolerations configured: %d", len(pod.Spec.Tolerations)))
	}
	for _, claim := range c.pendingPVCs(ctx, pod) {
		evidence = append(evidence, fmt.Sprintf("PVC %s is %s", claim.Name, claim.Status.Phase))
		related = append(related, "PersistentVolumeClaim/"+claim.Name)
	}

	return Diagnosis{
		Symptom:       "Pod cannot be scheduled",
		Category:      CategoryScheduling,
		LikelyCause:   "The scheduler cannot place the pod because of capacity, affinity, taints, selectors, or pending storage.",
		Confidence:    82,
		PatchStrategy: PatchSchedulingPolicy,
		Evidence:      evidence,
		Related:       related,
	}, true
}

func classifyImagePull(pod *v1.Pod, events []v1.Event, eventText, triggerReason string) (Diagnosis, bool) {
	haystack := strings.ToLower(triggerReason + " " + eventText + " " + summarizeContainerStates(pod))
	if !containsAny(haystack, "imagepullbackoff", "errimagepull", "pull image", "failed to pull image") {
		return Diagnosis{}, false
	}

	cause := "The image reference, registry credentials, or imagePullPolicy is preventing Kubernetes from pulling the container image."
	if containsAny(haystack, "not found", "manifest unknown", "name unknown") {
		cause = "The image tag or repository appears to be missing."
	} else if containsAny(haystack, "unauthorized", "authentication required", "denied") {
		cause = "The image registry rejected the pull because credentials or permissions are missing."
	}

	return Diagnosis{
		Symptom:       "Container image cannot be pulled",
		Category:      CategoryRollout,
		LikelyCause:   cause,
		Confidence:    88,
		PatchStrategy: PatchImage,
		Evidence:      append([]string{imageRefs(pod)}, eventEvidence(events, "Failed", "BackOff")...),
	}, true
}

func classifyConfigError(pod *v1.Pod, events []v1.Event, eventText, triggerReason string) (Diagnosis, bool) {
	haystack := strings.ToLower(triggerReason + " " + eventText + " " + summarizeContainerStates(pod))
	if !containsAny(haystack, "createcontainerconfigerror", "secret", "configmap", "couldn't find key", "not found") {
		return Diagnosis{}, false
	}
	if containsAny(haystack, "imagepull", "errimagepull") {
		return Diagnosis{}, false
	}

	return Diagnosis{
		Symptom:       "Container configuration cannot be materialized",
		Category:      CategoryConfig,
		LikelyCause:   "The pod references a missing Secret, ConfigMap, key, environment source, command, argument, or volume mount.",
		Confidence:    84,
		PatchStrategy: PatchEnvOrVolumeRef,
		Evidence:      append(referenceEvidence(pod), eventEvidence(events, "Failed", "FailedMount")...),
		Related:       referencedConfigResources(pod),
	}, true
}

func (c *Controller) classifyStorage(ctx context.Context, pod *v1.Pod, events []v1.Event, eventText string) (Diagnosis, bool) {
	pendingClaims := c.pendingPVCs(ctx, pod)
	haystack := strings.ToLower(eventText)
	if len(pendingClaims) == 0 && !containsAny(haystack, "failedmount", "persistentvolumeclaim", "timed out waiting for the condition") {
		return Diagnosis{}, false
	}

	evidence := eventEvidence(events, "FailedMount", "FailedAttachVolume")
	for _, claim := range pendingClaims {
		evidence = append(evidence, fmt.Sprintf("PVC %s is %s", claim.Name, claim.Status.Phase))
	}

	return Diagnosis{
		Symptom:       "Pod storage cannot be mounted or bound",
		Category:      CategoryStorage,
		LikelyCause:   "A volume, Secret, ConfigMap, or PersistentVolumeClaim dependency is missing, pending, or not attachable.",
		Confidence:    82,
		PatchStrategy: PatchPVC,
		Evidence:      evidence,
		Related:       referencedVolumeResources(pod),
	}, true
}

func classifyOOM(pod *v1.Pod, triggerReason string) (Diagnosis, bool) {
	if !strings.Contains(strings.ToLower(triggerReason+" "+summarizeContainerStates(pod)), "oomkilled") {
		return Diagnosis{}, false
	}
	return Diagnosis{
		Symptom:       "Container was killed by the kernel OOM killer",
		Category:      CategoryRuntime,
		LikelyCause:   "The workload exceeded its memory limit or the node was under memory pressure.",
		Confidence:    90,
		PatchStrategy: PatchResources,
		Evidence:      append([]string{"Container state includes OOMKilled."}, resourceEvidence(pod)...),
	}, true
}

func classifyProbeFailure(pod *v1.Pod, events []v1.Event, eventText, triggerReason string) (Diagnosis, bool) {
	haystack := strings.ToLower(triggerReason + " " + eventText)
	if !containsAny(haystack, "readiness probe failed", "liveness probe failed", "startup probe failed", "unhealthy", "health check") {
		return Diagnosis{}, false
	}
	return Diagnosis{
		Symptom:       "Health probe is failing",
		Category:      CategoryRuntime,
		LikelyCause:   "The configured readiness, liveness, or startup probe does not match the application behavior or the application is not healthy.",
		Confidence:    78,
		PatchStrategy: PatchProbe,
		Evidence:      append(probeEvidence(pod), eventEvidence(events, "Unhealthy")...),
	}, true
}

func (c *Controller) classifyNetwork(ctx context.Context, pod *v1.Pod) (Diagnosis, bool) {
	if pod.Status.Phase != v1.PodRunning {
		return Diagnosis{}, false
	}

	services, err := c.clientset.CoreV1().Services(pod.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return Diagnosis{}, false
	}

	var evidence []string
	var related []string
	for _, svc := range services.Items {
		if !selectorMatches(svc.Spec.Selector, pod.Labels) {
			continue
		}
		related = append(related, "Service/"+svc.Name)
		endpoints, err := c.clientset.CoreV1().Endpoints(pod.Namespace).Get(ctx, svc.Name, metav1.GetOptions{})
		if err != nil || !endpointsIncludePod(endpoints, pod.Name) {
			evidence = append(evidence, fmt.Sprintf("Service %s selects this pod, but endpoints do not include it.", svc.Name))
		}
	}
	if len(evidence) == 0 {
		return Diagnosis{}, false
	}

	return Diagnosis{
		Symptom:       "Service endpoint routing does not include the pod",
		Category:      CategoryNetwork,
		LikelyCause:   "A Service selector, pod labels, readiness gate, or endpoint publishing behavior is preventing traffic from reaching this pod.",
		Confidence:    68,
		PatchStrategy: PatchServiceSelector,
		Evidence:      evidence,
		Related:       related,
	}, true
}

func classifyCrashLoop(pod *v1.Pod, triggerReason string) (Diagnosis, bool) {
	haystack := strings.ToLower(triggerReason + " " + summarizeContainerStates(pod))
	if !containsAny(haystack, "crashloopbackoff", "exitcode:", "containercannotrun") {
		return Diagnosis{}, false
	}
	return Diagnosis{
		Symptom:       "Container is repeatedly crashing",
		Category:      CategoryRuntime,
		LikelyCause:   "The container process exits unsuccessfully; logs and events should identify the application or runtime failure.",
		Confidence:    70,
		PatchStrategy: PatchNone,
		Evidence:      []string{summarizeContainerStates(pod)},
	}, true
}

func (c *Controller) pendingPVCs(ctx context.Context, pod *v1.Pod) []v1.PersistentVolumeClaim {
	var claims []v1.PersistentVolumeClaim
	for _, vol := range pod.Spec.Volumes {
		if vol.PersistentVolumeClaim == nil {
			continue
		}
		claim, err := c.clientset.CoreV1().PersistentVolumeClaims(pod.Namespace).Get(ctx, vol.PersistentVolumeClaim.ClaimName, metav1.GetOptions{})
		if err == nil && claim.Status.Phase != v1.ClaimBound {
			claims = append(claims, *claim)
		}
	}
	return claims
}

func summarizeContainerStates(pod *v1.Pod) string {
	var parts []string
	for _, status := range append(pod.Status.InitContainerStatuses, pod.Status.ContainerStatuses...) {
		if status.State.Waiting != nil {
			parts = append(parts, fmt.Sprintf("%s waiting: %s %s", status.Name, status.State.Waiting.Reason, status.State.Waiting.Message))
		}
		if status.State.Terminated != nil {
			parts = append(parts, fmt.Sprintf("%s terminated: %s exit=%d", status.Name, status.State.Terminated.Reason, status.State.Terminated.ExitCode))
		}
	}
	return strings.Join(parts, "; ")
}

func eventsToText(events []v1.Event) string {
	var sb strings.Builder
	for _, event := range events {
		sb.WriteString(event.Reason)
		sb.WriteString(" ")
		sb.WriteString(event.Message)
		sb.WriteString("\n")
	}
	return sb.String()
}

func eventEvidence(events []v1.Event, reasons ...string) []string {
	reasonSet := map[string]bool{}
	for _, reason := range reasons {
		reasonSet[reason] = true
	}
	var evidence []string
	for _, event := range events {
		if len(reasonSet) == 0 || reasonSet[event.Reason] {
			evidence = append(evidence, fmt.Sprintf("Event %s: %s", event.Reason, event.Message))
		}
		if len(evidence) >= 3 {
			break
		}
	}
	if len(evidence) == 0 && len(events) > 0 {
		evidence = append(evidence, lastEventEvidence(events))
	}
	return evidence
}

func lastEventEvidence(events []v1.Event) string {
	event := events[len(events)-1]
	return fmt.Sprintf("Latest event %s: %s", event.Reason, event.Message)
}

func imageRefs(pod *v1.Pod) string {
	var refs []string
	for _, container := range pod.Spec.Containers {
		refs = append(refs, fmt.Sprintf("%s=%s", container.Name, container.Image))
	}
	sort.Strings(refs)
	return "Images: " + strings.Join(refs, ", ")
}

func referenceEvidence(pod *v1.Pod) []string {
	var evidence []string
	refs := referencedConfigResources(pod)
	if len(refs) > 0 {
		evidence = append(evidence, "Referenced config resources: "+strings.Join(refs, ", "))
	}
	volRefs := referencedVolumeResources(pod)
	if len(volRefs) > 0 {
		evidence = append(evidence, "Referenced volume resources: "+strings.Join(volRefs, ", "))
	}
	if len(evidence) == 0 {
		evidence = append(evidence, "Pod spec includes container env, envFrom, command, args, or volume fields that can cause config errors.")
	}
	return evidence
}

func referencedConfigResources(pod *v1.Pod) []string {
	seen := map[string]bool{}
	for _, container := range pod.Spec.Containers {
		for _, envFrom := range container.EnvFrom {
			if envFrom.SecretRef != nil {
				seen["Secret/"+envFrom.SecretRef.Name] = true
			}
			if envFrom.ConfigMapRef != nil {
				seen["ConfigMap/"+envFrom.ConfigMapRef.Name] = true
			}
		}
		for _, env := range container.Env {
			if env.ValueFrom == nil {
				continue
			}
			if env.ValueFrom.SecretKeyRef != nil {
				seen["Secret/"+env.ValueFrom.SecretKeyRef.Name] = true
			}
			if env.ValueFrom.ConfigMapKeyRef != nil {
				seen["ConfigMap/"+env.ValueFrom.ConfigMapKeyRef.Name] = true
			}
		}
	}
	return sortedKeys(seen)
}

func referencedVolumeResources(pod *v1.Pod) []string {
	seen := map[string]bool{}
	for _, vol := range pod.Spec.Volumes {
		if vol.Secret != nil {
			seen["Secret/"+vol.Secret.SecretName] = true
		}
		if vol.ConfigMap != nil {
			seen["ConfigMap/"+vol.ConfigMap.Name] = true
		}
		if vol.PersistentVolumeClaim != nil {
			seen["PersistentVolumeClaim/"+vol.PersistentVolumeClaim.ClaimName] = true
		}
	}
	return sortedKeys(seen)
}

func resourceEvidence(pod *v1.Pod) []string {
	var evidence []string
	for _, container := range pod.Spec.Containers {
		evidence = append(evidence, fmt.Sprintf("%s requests=%v limits=%v", container.Name, container.Resources.Requests, container.Resources.Limits))
	}
	return evidence
}

func probeEvidence(pod *v1.Pod) []string {
	var evidence []string
	for _, container := range pod.Spec.Containers {
		var probes []string
		if container.ReadinessProbe != nil {
			probes = append(probes, "readiness")
		}
		if container.LivenessProbe != nil {
			probes = append(probes, "liveness")
		}
		if container.StartupProbe != nil {
			probes = append(probes, "startup")
		}
		if len(probes) > 0 {
			evidence = append(evidence, fmt.Sprintf("%s has probes: %s", container.Name, strings.Join(probes, ", ")))
		}
	}
	return evidence
}

func selectorMatches(selector, labels map[string]string) bool {
	if len(selector) == 0 {
		return false
	}
	for key, value := range selector {
		if labels[key] != value {
			return false
		}
	}
	return true
}

func endpointsIncludePod(endpoints *v1.Endpoints, podName string) bool {
	if endpoints == nil {
		return false
	}
	for _, subset := range endpoints.Subsets {
		for _, address := range subset.Addresses {
			if address.TargetRef != nil && address.TargetRef.Kind == "Pod" && address.TargetRef.Name == podName {
				return true
			}
		}
	}
	return false
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func containsAny(s string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(s, needle) {
			return true
		}
	}
	return false
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return "Unknown"
}
