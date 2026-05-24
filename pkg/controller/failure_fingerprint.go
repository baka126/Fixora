package controller

import (
	"crypto/sha256"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
)

var (
	hexRegex       = regexp.MustCompile(`0x[a-f0-9]+`)
	uuidRegex      = regexp.MustCompile(`[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}`)
	timestampRegex = regexp.MustCompile(`\d{4}-\d{2}-\d{2}[t ]\d{2}:\d{2}:\d{2}(\.\d+)?(z|[+-]\d{2}:?\d{2})?`)
	ipRegex        = regexp.MustCompile(`\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}`)
	// Pod names in error messages often look like "pod-name-7f6c5b4a-12345"
	podSuffixRegex = regexp.MustCompile(`-[a-z0-9]{8,10}-[a-z0-9]{5}`)
)

type failureFingerprint struct {
	Hash    string
	Summary string
	Parts   []string
}

func (c *Controller) failureFingerprintForPod(pod *v1.Pod, identity workloadIdentity, reason string) failureFingerprint {
	if pod == nil {
		return failureFingerprint{}
	}
	scenario := diagnosticLockReason(reason)
	containerName := failingContainerNameForPod(pod)
	container := fingerprintContainerSpec(pod, containerName)
	status := fingerprintContainerStatus(pod, containerName)

	clusterName := "cluster"
	if c != nil && c.config != nil {
		clusterName = firstNonEmpty(c.config.ClusterName, "cluster")
	}
	parts := []string{
		"cluster=" + clusterName,
		"namespace=" + pod.Namespace,
		"workload_kind=" + identity.Kind,
		"workload_name=" + identity.Name,
		"scenario=" + scenario,
	}
	if containerName != "" {
		parts = append(parts, "container="+containerName)
	}
	if container != nil {
		parts = append(parts,
			"image="+container.Image,
			"command="+strings.Join(container.Command, " "),
			"args="+strings.Join(container.Args, " "),
		)
		if container.Resources.Requests != nil {
			if mem := container.Resources.Requests.Memory(); mem != nil && !mem.IsZero() {
				parts = append(parts, "memory_request="+mem.String())
			}
			if cpu := container.Resources.Requests.Cpu(); cpu != nil && !cpu.IsZero() {
				parts = append(parts, "cpu_request="+cpu.String())
			}
		}
		if container.Resources.Limits != nil {
			if mem := container.Resources.Limits.Memory(); mem != nil && !mem.IsZero() {
				parts = append(parts, "memory_limit="+mem.String())
			}
			if cpu := container.Resources.Limits.Cpu(); cpu != nil && !cpu.IsZero() {
				parts = append(parts, "cpu_limit="+cpu.String())
			}
		}
		parts = append(parts, fingerprintEnvRefs(container)...)
		parts = append(parts, fingerprintProbe("readiness", container.ReadinessProbe)...)
		parts = append(parts, fingerprintProbe("liveness", container.LivenessProbe)...)
		parts = append(parts, fingerprintProbe("startup", container.StartupProbe)...)
	}
	parts = append(parts, fingerprintStatusParts(status)...)
	parts = append(parts, fingerprintPodConditions(pod)...)
	parts = append(parts, fingerprintVolumes(pod)...)
	parts = compactFingerprintParts(parts)

	hashInput := strings.Join(parts, "\n")
	sum := sha256.Sum256([]byte(hashInput))
	return failureFingerprint{
		Hash:    fmt.Sprintf("%x", sum[:])[:16],
		Summary: fingerprintSummary(identity, scenario, containerName, container, status),
		Parts:   parts,
	}
}

func diagnosticLockNameWithFingerprint(base string, fp failureFingerprint) string {
	if fp.Hash == "" {
		return base
	}
	return base + "/" + fp.Hash
}

func repeatedNotificationLockName(base string, fp failureFingerprint) string {
	return "repeat/" + diagnosticLockNameWithFingerprint(base, fp)
}

func compactFingerprintParts(parts []string) []string {
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || strings.HasSuffix(part, "=") {
			continue
		}
		out = append(out, part)
	}
	sort.Strings(out)
	return out
}

func fingerprintContainerSpec(pod *v1.Pod, name string) *v1.Container {
	if pod == nil {
		return nil
	}
	for i := range pod.Spec.InitContainers {
		if pod.Spec.InitContainers[i].Name == name {
			return &pod.Spec.InitContainers[i]
		}
	}
	for i := range pod.Spec.Containers {
		if pod.Spec.Containers[i].Name == name {
			return &pod.Spec.Containers[i]
		}
	}
	if len(pod.Spec.Containers) > 0 {
		return &pod.Spec.Containers[0]
	}
	if len(pod.Spec.InitContainers) > 0 {
		return &pod.Spec.InitContainers[0]
	}
	return nil
}

func fingerprintContainerStatus(pod *v1.Pod, name string) *v1.ContainerStatus {
	if pod == nil {
		return nil
	}
	for i := range pod.Status.InitContainerStatuses {
		if pod.Status.InitContainerStatuses[i].Name == name {
			return &pod.Status.InitContainerStatuses[i]
		}
	}
	for i := range pod.Status.ContainerStatuses {
		if pod.Status.ContainerStatuses[i].Name == name {
			return &pod.Status.ContainerStatuses[i]
		}
	}
	for i := range pod.Status.InitContainerStatuses {
		if isFailingContainerStatus(pod.Status.InitContainerStatuses[i]) {
			return &pod.Status.InitContainerStatuses[i]
		}
	}
	for i := range pod.Status.ContainerStatuses {
		if isFailingContainerStatus(pod.Status.ContainerStatuses[i]) {
			return &pod.Status.ContainerStatuses[i]
		}
	}
	return nil
}

func fingerprintStatusParts(status *v1.ContainerStatus) []string {
	if status == nil {
		return nil
	}
	var parts []string
	if status.State.Waiting != nil {
		waitingClass := diagnosticLockReason(status.State.Waiting.Reason)
		parts = append(parts, "waiting_class="+waitingClass)
		switch waitingClass {
		case "container-config", "container-create", "crashloop", "not-ready":
			parts = append(parts, "waiting_message="+stableFailureMessage(status.State.Waiting.Message))
		}
	}
	if status.State.Terminated != nil {
		parts = append(parts,
			"terminated_reason="+status.State.Terminated.Reason,
			fmt.Sprintf("exit_code=%d", status.State.Terminated.ExitCode),
			"terminated_message="+stableFailureMessage(status.State.Terminated.Message),
		)
	}
	if status.LastTerminationState.Terminated != nil {
		parts = append(parts,
			"last_terminated_reason="+status.LastTerminationState.Terminated.Reason,
			fmt.Sprintf("last_exit_code=%d", status.LastTerminationState.Terminated.ExitCode),
			"last_terminated_message="+stableFailureMessage(status.LastTerminationState.Terminated.Message),
		)
	}
	return parts
}

func fingerprintPodConditions(pod *v1.Pod) []string {
	var parts []string
	for _, cond := range pod.Status.Conditions {
		if cond.Status == v1.ConditionTrue {
			continue
		}
		switch cond.Type {
		case v1.PodScheduled, v1.PodReady, v1.ContainersReady:
			parts = append(parts,
				"condition="+string(cond.Type),
				"condition_reason="+cond.Reason,
				"condition_message="+stableFailureMessage(cond.Message),
			)
		}
	}
	return parts
}

func fingerprintEnvRefs(container *v1.Container) []string {
	if container == nil {
		return nil
	}
	var parts []string
	for _, env := range container.Env {
		if env.ValueFrom == nil {
			if env.Value != "" {
				parts = append(parts, "env="+env.Name)
			}
			continue
		}
		if ref := env.ValueFrom.ConfigMapKeyRef; ref != nil {
			parts = append(parts, "env_configmap="+env.Name+":"+ref.Name+":"+ref.Key)
		}
		if ref := env.ValueFrom.SecretKeyRef; ref != nil {
			parts = append(parts, "env_secret="+env.Name+":"+ref.Name+":"+ref.Key)
		}
		if ref := env.ValueFrom.FieldRef; ref != nil {
			parts = append(parts, "env_field="+env.Name+":"+ref.FieldPath)
		}
	}
	for _, envFrom := range container.EnvFrom {
		if envFrom.ConfigMapRef != nil {
			parts = append(parts, "envfrom_configmap="+envFrom.ConfigMapRef.Name)
		}
		if envFrom.SecretRef != nil {
			parts = append(parts, "envfrom_secret="+envFrom.SecretRef.Name)
		}
	}
	return parts
}

func fingerprintProbe(prefix string, probe *v1.Probe) []string {
	if probe == nil {
		return nil
	}
	var parts []string
	if probe.HTTPGet != nil {
		parts = append(parts, fmt.Sprintf("%s_probe=http:%s:%s", prefix, probe.HTTPGet.Port.String(), probe.HTTPGet.Path))
	}
	if probe.TCPSocket != nil {
		parts = append(parts, fmt.Sprintf("%s_probe=tcp:%s", prefix, probe.TCPSocket.Port.String()))
	}
	if probe.GRPC != nil {
		service := ""
		if probe.GRPC.Service != nil {
			service = *probe.GRPC.Service
		}
		parts = append(parts, fmt.Sprintf("%s_probe=grpc:%d:%s", prefix, probe.GRPC.Port, service))
	}
	if probe.Exec != nil {
		parts = append(parts, fmt.Sprintf("%s_probe=exec:%s", prefix, strings.Join(probe.Exec.Command, " ")))
	}
	return parts
}

func fingerprintVolumes(pod *v1.Pod) []string {
	var parts []string
	for _, volume := range pod.Spec.Volumes {
		if volume.ConfigMap != nil {
			parts = append(parts, "volume_configmap="+volume.Name+":"+volume.ConfigMap.Name)
		}
		if volume.Secret != nil {
			parts = append(parts, "volume_secret="+volume.Name+":"+volume.Secret.SecretName)
		}
		if volume.PersistentVolumeClaim != nil {
			parts = append(parts, "volume_pvc="+volume.Name+":"+volume.PersistentVolumeClaim.ClaimName)
		}
	}
	return parts
}

func stableFailureMessage(message string) string {
	message = strings.ToLower(strings.TrimSpace(message))
	// Apply regex replacements to strip dynamic content
	message = hexRegex.ReplaceAllString(message, "0xhex")
	message = uuidRegex.ReplaceAllString(message, "uuid")
	message = timestampRegex.ReplaceAllString(message, "timestamp")
	message = ipRegex.ReplaceAllString(message, "ip")
	message = podSuffixRegex.ReplaceAllString(message, "-suffix")

	message = strings.Join(strings.Fields(message), " ")
	if len(message) > 240 {
		message = message[:240]
	}
	return message
}

func fingerprintSummary(identity workloadIdentity, scenario, containerName string, container *v1.Container, status *v1.ContainerStatus) string {
	var parts []string
	if identity.Kind != "" && identity.Name != "" {
		parts = append(parts, identity.Kind+"/"+identity.Name)
	}
	if scenario != "" {
		parts = append(parts, scenario)
	}
	if containerName != "" {
		parts = append(parts, "container "+containerName)
	}
	if container != nil && container.Image != "" {
		parts = append(parts, "image "+container.Image)
	}
	if status != nil && status.State.Waiting != nil && status.State.Waiting.Reason != "" {
		parts = append(parts, status.State.Waiting.Reason)
	}
	if status != nil && status.State.Terminated != nil && status.State.Terminated.Reason != "" {
		parts = append(parts, status.State.Terminated.Reason)
	}
	return strings.Join(parts, " · ")
}

func repeatNotificationWindow(cooldown time.Duration) time.Duration {
	if cooldown <= 0 {
		return 10 * time.Minute
	}
	if cooldown < 10*time.Minute {
		return cooldown
	}
	return 10 * time.Minute
}
