package analyzer

import (
	"context"
	"fmt"
	"sort"
	"strings"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type PodAnalyzer struct{}

func (p *PodAnalyzer) Analyze(ctx Context) ([]Result, error) {
	// If Namespace is specified, we might only want to analyze that.
	// But usually, in Fixora, we are triggered for a specific pod.
	// Let's assume for now this can list pods if none specified, or just handle what's in Context.
	// Actually, the current classifyPodIssue takes a *v1.Pod.
	// We might need to change how this is called.

	// If we want to follow k8sgpt style, we list and analyze.
	listOptions := metav1.ListOptions{}
	targetPodName := ""
	if strings.HasPrefix(ctx.LabelSelector, "metadata.name=") {
		listOptions.FieldSelector = ctx.LabelSelector
		targetPodName = strings.TrimPrefix(ctx.LabelSelector, "metadata.name=")
	} else {
		listOptions.LabelSelector = ctx.LabelSelector
	}
	targetedPod := listOptions.FieldSelector != ""
	list, err := ctx.Client.CoreV1().Pods(ctx.Namespace).List(ctx.Context, listOptions)
	if err != nil {
		return nil, err
	}

	var results []Result
	for _, pod := range list.Items {
		if targetedPod && targetPodName != "" && pod.Name != targetPodName {
			continue
		}
		// Only analyze pods that are not in a healthy state
		if !targetedPod && (pod.Status.Phase == v1.PodRunning || pod.Status.Phase == v1.PodSucceeded) {
			// Even if running, it might have issues (e.g. failing probes)
			// But let's check for readiness
			allReady := true
			for _, cs := range pod.Status.ContainerStatuses {
				if !cs.Ready {
					allReady = false
					break
				}
			}
			if allReady {
				continue
			}
		}

		res := p.analyzePod(ctx, &pod)
		results = append(results, res)
	}

	return results, nil
}

func (p *PodAnalyzer) analyzePod(ctx Context, pod *v1.Pod) Result {
	events, _ := p.listPodEvents(ctx.Context, ctx.Client, pod)
	eventText := eventsToText(events)
	containerSummary := summarizeContainerStates(pod)

	related := []string{}
	if ownerKind, ownerName := GetRootOwner(ctx.Context, ctx.Client, pod.Namespace, pod.ObjectMeta); ownerKind != "" {
		related = append(related, fmt.Sprintf("%s/%s", ownerKind, ownerName))
	}

	base := Result{
		Kind:          "Pod",
		Name:          pod.Name,
		Namespace:     pod.Namespace,
		Symptom:       string(pod.Status.Phase),
		Category:      CategoryUnknown,
		LikelyCause:   "No deterministic Kubernetes classifier matched; use logs, events, and metrics for deeper analysis.",
		Confidence:    35,
		PatchStrategy: PatchNone,
		Evidence:      []string{fmt.Sprintf("Pod phase is %s", pod.Status.Phase)},
		Related:       related,
	}
	if containerSummary != "" {
		base.Evidence = append(base.Evidence, containerSummary)
	}

	if d, ok := p.classifyScheduling(ctx, pod, events, eventText); ok {
		return d
	}
	if d, ok := classifyImagePull(pod, events, eventText); ok {
		return d
	}
	if d, ok := classifyConfigError(pod, events, eventText); ok {
		return d
	}
	if d, ok := p.classifyStorage(ctx, pod, events, eventText); ok {
		return d
	}
	if d, ok := classifyOOM(pod); ok {
		return d
	}
	if d, ok := classifyProbeFailure(pod, events, eventText); ok {
		return d
	}
	if d, ok := p.classifyNetwork(ctx, pod); ok {
		return d
	}
	if d, ok := classifyCrashLoop(pod); ok {
		return d
	}

	if len(events) > 0 {
		base.Evidence = append(base.Evidence, lastEventEvidence(events))
	}
	return base
}

func (p *PodAnalyzer) listPodEvents(ctx context.Context, client kubernetes.Interface, pod *v1.Pod) ([]v1.Event, error) {
	eventsTimeline, err := client.CoreV1().Events(pod.Namespace).List(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("involvedObject.name=%s,involvedObject.namespace=%s", pod.Name, pod.Namespace),
	})

	if err != nil {
		return nil, err
	}
	return eventsTimeline.Items, nil
}

func (p *PodAnalyzer) classifyScheduling(ctx Context, pod *v1.Pod, events []v1.Event, eventText string) (Result, bool) {
	hasUnschedulable := strings.Contains(strings.ToLower(eventText), "unschedulable")
	for _, cond := range pod.Status.Conditions {
		if cond.Type == v1.PodScheduled && cond.Status == v1.ConditionFalse && cond.Reason == "Unschedulable" {
			hasUnschedulable = true
			break
		}
	}
	if !hasUnschedulable {
		return Result{}, false
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
	for _, claim := range p.pendingPVCs(ctx, pod) {
		evidence = append(evidence, fmt.Sprintf("PVC %s is %s", claim.Name, claim.Status.Phase))
		related = append(related, "PersistentVolumeClaim/"+claim.Name)
	}

	return Result{
		Kind:          "Pod",
		Name:          pod.Name,
		Namespace:     pod.Namespace,
		Symptom:       "Pod cannot be scheduled",
		Category:      CategoryScheduling,
		LikelyCause:   "The scheduler cannot place the pod because of capacity, affinity, taints, selectors, or pending storage.",
		Confidence:    82,
		PatchStrategy: PatchSchedulingPolicy,
		Evidence:      evidence,
		Related:       related,
	}, true
}

func classifyImagePull(pod *v1.Pod, events []v1.Event, eventText string) (Result, bool) {
	haystack := strings.ToLower(eventText + " " + summarizeContainerStates(pod))
	if !containsAny(haystack, "imagepullbackoff", "errimagepull", "pull image", "failed to pull image") {
		return Result{}, false
	}

	cause := "The image reference, registry credentials, or imagePullPolicy is preventing Kubernetes from pulling the container image."
	if containsAny(haystack, "not found", "manifest unknown", "name unknown") {
		cause = "The image tag or repository appears to be missing."
	} else if containsAny(haystack, "unauthorized", "authentication required", "denied") {
		cause = "The image registry rejected the pull because credentials or permissions are missing."
	}

	return Result{
		Kind:          "Pod",
		Name:          pod.Name,
		Namespace:     pod.Namespace,
		Symptom:       "Container image cannot be pulled",
		Category:      CategoryRollout,
		LikelyCause:   cause,
		Confidence:    88,
		PatchStrategy: PatchImage,
		Evidence:      append([]string{imageRefs(pod)}, eventEvidence(events, "Failed", "BackOff")...),
	}, true
}

func classifyConfigError(pod *v1.Pod, events []v1.Event, eventText string) (Result, bool) {
	haystack := strings.ToLower(eventText + " " + summarizeContainerStates(pod))
	if !containsAny(haystack, "createcontainerconfigerror", "secret", "configmap", "couldn't find key", "not found") {
		return Result{}, false
	}
	if containsAny(haystack, "imagepull", "errimagepull") {
		return Result{}, false
	}

	return Result{
		Kind:          "Pod",
		Name:          pod.Name,
		Namespace:     pod.Namespace,
		Symptom:       "Container configuration cannot be materialized",
		Category:      CategoryConfig,
		LikelyCause:   "The pod references a missing Secret, ConfigMap, key, environment source, command, argument, or volume mount.",
		Confidence:    84,
		PatchStrategy: PatchEnvOrVolumeRef,
		Evidence:      append(referenceEvidence(pod), eventEvidence(events, "Failed", "FailedMount")...),
		Related:       referencedConfigResources(pod),
	}, true
}

func (p *PodAnalyzer) classifyStorage(ctx Context, pod *v1.Pod, events []v1.Event, eventText string) (Result, bool) {
	pendingClaims := p.pendingPVCs(ctx, pod)
	haystack := strings.ToLower(eventText)
	if len(pendingClaims) == 0 && !containsAny(haystack, "failedmount", "persistentvolumeclaim", "timed out waiting for the condition") {
		return Result{}, false
	}

	evidence := eventEvidence(events, "FailedMount", "FailedAttachVolume")
	for _, claim := range pendingClaims {
		evidence = append(evidence, fmt.Sprintf("PVC %s is %s", claim.Name, claim.Status.Phase))
	}

	return Result{
		Kind:          "Pod",
		Name:          pod.Name,
		Namespace:     pod.Namespace,
		Symptom:       "Pod storage cannot be mounted or bound",
		Category:      CategoryStorage,
		LikelyCause:   "A volume, Secret, ConfigMap, or PersistentVolumeClaim dependency is missing, pending, or not attachable.",
		Confidence:    82,
		PatchStrategy: PatchPVC,
		Evidence:      evidence,
		Related:       referencedVolumeResources(pod),
	}, true
}

func classifyOOM(pod *v1.Pod) (Result, bool) {
	if !strings.Contains(strings.ToLower(summarizeContainerStates(pod)), "oomkilled") {
		return Result{}, false
	}
	return Result{
		Kind:          "Pod",
		Name:          pod.Name,
		Namespace:     pod.Namespace,
		Symptom:       "Container was killed by the kernel OOM killer",
		Category:      CategoryRuntime,
		LikelyCause:   "The workload exceeded its memory limit or the node was under memory pressure.",
		Confidence:    90,
		PatchStrategy: PatchResources,
		Evidence:      append([]string{"Container state includes OOMKilled."}, resourceEvidence(pod)...),
	}, true
}

func classifyProbeFailure(pod *v1.Pod, events []v1.Event, eventText string) (Result, bool) {
	haystack := strings.ToLower(eventText)
	if !containsAny(haystack, "readiness probe failed", "liveness probe failed", "startup probe failed", "unhealthy", "health check") {
		return Result{}, false
	}
	return Result{
		Kind:          "Pod",
		Name:          pod.Name,
		Namespace:     pod.Namespace,
		Symptom:       "Health probe is failing",
		Category:      CategoryRuntime,
		LikelyCause:   "The configured readiness, liveness, or startup probe does not match the application behavior or the application is not healthy.",
		Confidence:    78,
		PatchStrategy: PatchProbe,
		Evidence:      append(probeEvidence(pod), eventEvidence(events, "Unhealthy")...),
	}, true
}

func (p *PodAnalyzer) classifyNetwork(ctx Context, pod *v1.Pod) (Result, bool) {
	if pod.Status.Phase != v1.PodRunning {
		return Result{}, false
	}

	services, err := ctx.Client.CoreV1().Services(pod.Namespace).List(ctx.Context, metav1.ListOptions{})
	if err != nil {
		return Result{}, false
	}

	var evidence []string
	var related []string
	for _, svc := range services.Items {
		if !selectorMatches(svc.Spec.Selector, pod.Labels) {
			continue
		}
		related = append(related, "Service/"+svc.Name)
		endpoints, err := ctx.Client.CoreV1().Endpoints(pod.Namespace).Get(ctx.Context, svc.Name, metav1.GetOptions{})
		if err != nil || !endpointsIncludePod(endpoints, pod.Name) {
			evidence = append(evidence, fmt.Sprintf("Service %s selects this pod, but endpoints do not include it.", svc.Name))
		}
	}
	if len(evidence) == 0 {
		return Result{}, false
	}

	return Result{
		Kind:          "Pod",
		Name:          pod.Name,
		Namespace:     pod.Namespace,
		Symptom:       "Service endpoint routing does not include the pod",
		Category:      CategoryNetwork,
		LikelyCause:   "A Service selector, pod labels, readiness gate, or endpoint publishing behavior is preventing traffic from reaching this pod.",
		Confidence:    68,
		PatchStrategy: PatchServiceSelector,
		Evidence:      evidence,
		Related:       related,
	}, true
}

func classifyCrashLoop(pod *v1.Pod) (Result, bool) {
	haystack := strings.ToLower(summarizeContainerStates(pod))
	if !containsAny(haystack, "crashloopbackoff", "exitcode:", "containercannotrun") {
		return Result{}, false
	}
	return Result{
		Kind:          "Pod",
		Name:          pod.Name,
		Namespace:     pod.Namespace,
		Symptom:       "Container is repeatedly crashing",
		Category:      CategoryRuntime,
		LikelyCause:   "The container process exits unsuccessfully; logs and events should identify the application or runtime failure.",
		Confidence:    70,
		PatchStrategy: PatchNone,
		Evidence:      []string{summarizeContainerStates(pod)},
	}, true
}

func (p *PodAnalyzer) pendingPVCs(ctx Context, pod *v1.Pod) []v1.PersistentVolumeClaim {
	var claims []v1.PersistentVolumeClaim
	for _, vol := range pod.Spec.Volumes {
		if vol.PersistentVolumeClaim == nil {
			continue
		}
		claim, err := ctx.Client.CoreV1().PersistentVolumeClaims(pod.Namespace).Get(ctx.Context, vol.PersistentVolumeClaim.ClaimName, metav1.GetOptions{})
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
	if len(events) == 0 {
		return ""
	}
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
