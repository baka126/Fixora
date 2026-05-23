package analyzer

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type SecurityAnalyzer struct{}

func (s *SecurityAnalyzer) Analyze(ctx Context) ([]Result, error) {
	listOptions := metav1.ListOptions{LabelSelector: ctx.LabelSelector}
	targetPodName := ""
	if strings.HasPrefix(ctx.LabelSelector, "metadata.name=") {
		listOptions.LabelSelector = ""
		listOptions.FieldSelector = ctx.LabelSelector
		targetPodName = strings.TrimPrefix(ctx.LabelSelector, "metadata.name=")
	}
	
	// Ensure we only list within the requested namespace to avoid performance issues
	namespace := firstNonEmpty(ctx.Namespace, metav1.NamespaceAll)
	
	pods, err := ctx.Client.CoreV1().Pods(namespace).List(ctx.Context, listOptions)
	if err != nil {
		return nil, err
	}

	var results []Result
	for _, pod := range pods.Items {
		if targetPodName != "" && pod.Name != targetPodName {
			continue
		}
		if result, ok := classifyRuntimeSecurityFailure(&pod, ctx.Logs); ok {
			results = append(results, result)
			continue
		}

		var failures []string

		for _, container := range pod.Spec.Containers {
			if container.SecurityContext != nil {
				if container.SecurityContext.Privileged != nil && *container.SecurityContext.Privileged {
					failures = append(failures, fmt.Sprintf("Container %s is running in privileged mode.", container.Name))
				}
				if container.SecurityContext.RunAsNonRoot != nil && !*container.SecurityContext.RunAsNonRoot {
					failures = append(failures, fmt.Sprintf("Container %s is not configured to run as a non-root user.", container.Name))
				}
			}
		}

		if len(failures) > 0 {
			results = append(results, Result{
				Kind:          "Pod",
				Name:          pod.Name,
				Namespace:     pod.Namespace,
				Symptom:       "Security risk detected",
				Category:      CategoryConfig,
				LikelyCause:   "The pod configuration violates zero-trust security principles by allowing elevated privileges or root access.",
				Confidence:    70,
				PatchStrategy: PatchNone,
				Evidence:      failures,
			})
		}
	}

	return results, nil
}

func classifyRuntimeSecurityFailure(pod *v1.Pod, logs string) (Result, bool) {
	haystack := strings.ToLower(logs + "\n" + summarizePodSecurityContext(pod))
	hasPermissionFailure := containsAny(haystack, "permission denied", "operation not permitted", "read-only file system", "readonly file system", "cannot write")
	hasNonRootFailure := containsAny(haystack, "run as non-root", "runasnonroot", "refusing to run as root", "must not run as root", "container has runasnonroot")
	if !hasPermissionFailure && !hasNonRootFailure {
		return Result{}, false
	}

	paths := writablePathCandidates(logs)
	if len(paths) > 0 {
		return Result{
			Kind:          "Pod",
			Name:          pod.Name,
			Namespace:     pod.Namespace,
			Symptom:       "Container cannot write to a required filesystem path",
			Category:      CategoryConfig,
			LikelyCause:   "The container is trying to write to a path that is not writable under the current securityContext or read-only root filesystem policy. Mount a scoped emptyDir at the exact writable path instead of loosening the whole container security posture.",
			Confidence:    86,
			PatchStrategy: PatchSecurityContext,
			Evidence: []string{
				"Permission-denied or read-only-filesystem log evidence was found.",
				"Writable path candidates: " + strings.Join(paths, ", "),
				summarizePodSecurityContext(pod),
			},
			Related: podOwnerRefs(pod),
		}, true
	}

	if hasNonRootFailure {
		return Result{
			Kind:          "Pod",
			Name:          pod.Name,
			Namespace:     pod.Namespace,
			Symptom:       "Container securityContext does not match non-root runtime expectations",
			Category:      CategoryConfig,
			LikelyCause:   "The process or cluster policy expects the container to run as a non-root user. Set runAsNonRoot and a non-zero runAsUser/group when the image supports it instead of granting elevated privileges.",
			Confidence:    80,
			PatchStrategy: PatchSecurityContext,
			Evidence: []string{
				"Log evidence indicates non-root or privilege-drop mismatch.",
				summarizePodSecurityContext(pod),
			},
			Related: podOwnerRefs(pod),
		}, true
	}

	return Result{
		Kind:          "Pod",
		Name:          pod.Name,
		Namespace:     pod.Namespace,
		Symptom:       "Container hit a security policy or filesystem permission failure",
		Category:      CategoryConfig,
		LikelyCause:   "The container is blocked by filesystem permissions or securityContext settings, but Fixora could not identify a specific safe field to patch automatically.",
		Confidence:    62,
		PatchStrategy: PatchNone,
		Evidence: []string{
			"Permission-denied or operation-not-permitted log evidence was found.",
			summarizePodSecurityContext(pod),
		},
		Related: podOwnerRefs(pod),
	}, true
}

func writablePathCandidates(logs string) []string {
	if logs == "" {
		return nil
	}
	seen := map[string]bool{}
	re := regexp.MustCompile(`(?i)(?:permission denied|read-only file system|readonly file system|cannot write|open|mkdir|touch|write)[^\n'"]*?(/(?:tmp|var/tmp|cache|var/cache|var/run|run|app|opt|data|var/lib)[A-Za-z0-9._/\-]*)`)
	for _, match := range re.FindAllStringSubmatch(logs, -1) {
		if len(match) < 2 {
			continue
		}
		path := normalizeWritablePath(match[1])
		if path != "" {
			seen[path] = true
		}
	}
	paths := make([]string, 0, len(seen))
	for path := range seen {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	if len(paths) > 3 {
		paths = paths[:3]
	}
	return paths
}

func normalizeWritablePath(path string) string {
	path = strings.TrimRight(strings.TrimSpace(path), ".,:;)")
	if path == "" || path == "/" {
		return ""
	}
	for _, prefix := range []string{"/tmp", "/var/tmp", "/cache", "/var/cache", "/var/run", "/run"} {
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			return prefix
		}
	}
	return path
}

func summarizePodSecurityContext(pod *v1.Pod) string {
	if pod == nil {
		return "SecurityContext: unavailable"
	}
	var parts []string
	if pod.Spec.SecurityContext != nil {
		if pod.Spec.SecurityContext.RunAsNonRoot != nil {
			parts = append(parts, fmt.Sprintf("pod.runAsNonRoot=%t", *pod.Spec.SecurityContext.RunAsNonRoot))
		}
		if pod.Spec.SecurityContext.RunAsUser != nil {
			parts = append(parts, fmt.Sprintf("pod.runAsUser=%d", *pod.Spec.SecurityContext.RunAsUser))
		}
	}
	for _, container := range pod.Spec.Containers {
		if container.SecurityContext == nil {
			continue
		}
		if container.SecurityContext.ReadOnlyRootFilesystem != nil {
			parts = append(parts, fmt.Sprintf("%s.readOnlyRootFilesystem=%t", container.Name, *container.SecurityContext.ReadOnlyRootFilesystem))
		}
		if container.SecurityContext.RunAsNonRoot != nil {
			parts = append(parts, fmt.Sprintf("%s.runAsNonRoot=%t", container.Name, *container.SecurityContext.RunAsNonRoot))
		}
		if container.SecurityContext.AllowPrivilegeEscalation != nil {
			parts = append(parts, fmt.Sprintf("%s.allowPrivilegeEscalation=%t", container.Name, *container.SecurityContext.AllowPrivilegeEscalation))
		}
	}
	if len(parts) == 0 {
		return "SecurityContext: no explicit pod/container securityContext fields found"
	}
	return "SecurityContext: " + strings.Join(parts, ", ")
}
