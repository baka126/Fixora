package controller

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type probeCorrectionSuggestion struct {
	Path           string
	Port           string
	CurrentProbes  []string
	ImpactedRoutes []string
	Reason         string
	Confidence     int
}

func (s probeCorrectionSuggestion) Summary() string {
	var parts []string
	if s.Path != "" {
		parts = append(parts, "path="+s.Path)
	}
	if s.Port != "" {
		parts = append(parts, "port="+s.Port)
	}
	if len(parts) == 0 {
		return ""
	}
	return fmt.Sprintf("Probe correction candidate: update failing readiness/liveness probe to %s. Current probes: %s. User-facing impact: %s. Reason: %s",
		strings.Join(parts, ", "),
		strings.Join(s.CurrentProbes, "; "),
		strings.Join(s.ImpactedRoutes, ", "),
		s.Reason,
	)
}

func (s probeCorrectionSuggestion) Evidence() []string {
	var out []string
	if s.Path != "" || s.Port != "" {
		out = append(out, fmt.Sprintf("Application listener hints suggest probe path=%s port=%s.", firstNonEmpty(s.Path, "<unchanged>"), firstNonEmpty(s.Port, "<unchanged>")))
	}
	if len(s.CurrentProbes) > 0 {
		out = append(out, "Current probe config: "+strings.Join(s.CurrentProbes, "; "))
	}
	if len(s.ImpactedRoutes) > 0 {
		out = append(out, "Service/Ingress/Gateway impact: "+strings.Join(s.ImpactedRoutes, ", "))
	}
	return out
}

func (c *Controller) probeCorrectionSuggestion(ctx context.Context, pod *v1.Pod, diagnosis Diagnosis, collected CollectedEvidence) (probeCorrectionSuggestion, bool) {
	if c == nil || c.clientset == nil || pod == nil {
		return probeCorrectionSuggestion{}, false
	}
	haystack := strings.Join([]string{
		diagnosis.Symptom,
		diagnosis.LikelyCause,
		strings.Join(diagnosis.Evidence, "\n"),
		collected.EventTimeline,
		collected.Logs,
		collected.StackTrace,
	}, "\n")
	if !isProbeFailureEvidence(haystack) && diagnosis.PatchStrategy != PatchProbe {
		return probeCorrectionSuggestion{}, false
	}

	container := targetContainerSpec(pod, failingContainerNameForPod(pod))
	current := currentHTTPProbeSummaries(container)
	if len(current) == 0 {
		return probeCorrectionSuggestion{}, false
	}
	hint := inferProbeHint(haystack)
	if hint.Path == "" && hint.Port == "" {
		return probeCorrectionSuggestion{}, false
	}
	if !probeHintDiffers(container, hint) {
		return probeCorrectionSuggestion{}, false
	}

	impact := c.probeRouteImpact(ctx, pod)
	if len(impact) == 0 && !containsAny(haystack, "no endpoints", "no ready endpoints", "service unavailable", "503") {
		return probeCorrectionSuggestion{}, false
	}
	suggestion := probeCorrectionSuggestion{
		Path:           hint.Path,
		Port:           hint.Port,
		CurrentProbes:  current,
		ImpactedRoutes: impact,
		Reason:         "probe target differs from application listener evidence",
		Confidence:     86,
	}
	return suggestion, true
}

func applyProbeCorrectionSuggestion(diagnosis *Diagnosis, corr *ResourceCorrelation, suggestion probeCorrectionSuggestion) {
	if diagnosis == nil || suggestion.Summary() == "" {
		return
	}
	diagnosis.Category = CategoryRuntime
	diagnosis.PatchStrategy = PatchProbe
	if diagnosis.Confidence < suggestion.Confidence {
		diagnosis.Confidence = suggestion.Confidence
	}
	diagnosis.Symptom = "Health probe does not match application listener"
	diagnosis.LikelyCause = "The workload appears healthy on a different path or port than the configured readiness/liveness probe. Because Services, Ingresses, or HTTPRoutes depend on ready endpoints, update the probe target instead of changing routing first."
	diagnosis.Evidence = append(diagnosis.Evidence, suggestion.Evidence()...)
	if corr != nil {
		corr.add(suggestion.Summary())
		for _, ref := range suggestion.ImpactedRoutes {
			if kind, name, ok := splitKindName(ref); ok {
				corr.relate(kind, name)
				corr.score(kind, name, 55, "traffic depends on probe-ready endpoints")
			}
		}
	}
}

type probeHint struct {
	Path string
	Port string
}

func inferProbeHint(text string) probeHint {
	var hint probeHint
	hint.Path = inferProbePath(text)
	hint.Port = inferProbePort(text)
	return hint
}

func inferProbePath(text string) string {
	if text == "" {
		return ""
	}
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)(?:health|readiness|ready|live|liveness|startup|status)[^\n]*?(\/[A-Za-z0-9._~!$&'()*+,;=:@%/\-]*[A-Za-z0-9_\-/])`),
		regexp.MustCompile(`(?i)(?:GET|HEAD)\s+(\/[A-Za-z0-9._~!$&'()*+,;=:@%/\-]*[A-Za-z0-9_\-/])\s+(?:404|405|500)`),
		regexp.MustCompile(`(?i)(?:registered|mounted|serving|route|endpoint|path)[^\n]*?(\/(?:healthz|health|readyz|ready|livez|live|actuator\/health|api\/health|ping|status)[A-Za-z0-9._~!$&'()*+,;=:@%/\-]*)`),
	}
	for _, re := range patterns {
		for _, match := range re.FindAllStringSubmatch(text, -1) {
			if len(match) < 2 {
				continue
			}
			path := normalizeProbePath(match[1])
			if path != "" {
				return path
			}
		}
	}
	return ""
}

func inferProbePort(text string) string {
	if text == "" {
		return ""
	}
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)(?:listening|listen|serving|started|bound|http server|address)[^\n]*(?::|port\s+)([0-9]{2,5})`),
		regexp.MustCompile(`(?i)0\.0\.0\.0:([0-9]{2,5})`),
		regexp.MustCompile(`(?i)127\.0\.0\.1:([0-9]{2,5})`),
		regexp.MustCompile(`(?i)localhost:([0-9]{2,5})`),
	}
	for _, re := range patterns {
		for _, match := range re.FindAllStringSubmatch(text, -1) {
			if len(match) < 2 {
				continue
			}
			port, err := strconv.Atoi(match[1])
			if err == nil && port > 0 && port <= 65535 {
				// Avoid common peer/admin ports if they look like they might not be the primary service port
				if port == 2379 || port == 2380 || port == 7001 { // etcd, weblogic etc.
					continue
				}
				return strconv.Itoa(port)
			}
		}
	}
	return ""
}

func normalizeProbePath(path string) string {
	path = strings.TrimSpace(path)
	path = strings.TrimRight(path, ".,:;)'\"")
	if path == "" || !strings.HasPrefix(path, "/") {
		return ""
	}
	return path
}

func isProbeFailureEvidence(text string) bool {
	return containsAny(text, "readiness probe failed", "liveness probe failed", "startup probe failed", "health check failed", "unhealthy", "no ready endpoints", "no endpoints")
}

func currentHTTPProbeSummaries(container *v1.Container) []string {
	if container == nil {
		return nil
	}
	var out []string
	add := func(name string, probe *v1.Probe) {
		if probe == nil || probe.HTTPGet == nil {
			return
		}
		out = append(out, fmt.Sprintf("%s path=%s port=%s", name, probe.HTTPGet.Path, intstrValue(probe.HTTPGet.Port)))
	}
	add("readiness", container.ReadinessProbe)
	add("liveness", container.LivenessProbe)
	add("startup", container.StartupProbe)
	sort.Strings(out)
	return out
}

func probeHintDiffers(container *v1.Container, hint probeHint) bool {
	if container == nil {
		return false
	}
	for _, probe := range []*v1.Probe{container.ReadinessProbe, container.LivenessProbe, container.StartupProbe} {
		if probe == nil || probe.HTTPGet == nil {
			continue
		}
		if hint.Path != "" && normalizeProbePath(probe.HTTPGet.Path) != hint.Path {
			return true
		}
		if hint.Port != "" && intstrValue(probe.HTTPGet.Port) != hint.Port {
			return true
		}
	}
	return false
}

func intstrValue(value intstr.IntOrString) string {
	if value.Type == intstr.String {
		return value.StrVal
	}
	return strconv.Itoa(int(value.IntVal))
}

func (c *Controller) probeRouteImpact(ctx context.Context, pod *v1.Pod) []string {
	if c == nil || c.clientset == nil || pod == nil {
		return nil
	}
	var impact []string
	services, err := c.clientset.CoreV1().Services(pod.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil
	}
	for _, svc := range services.Items {
		if !selectorMatches(svc.Spec.Selector, pod.Labels) {
			continue
		}
		if !c.serviceEndpointsMissingPod(ctx, pod, svc.Name) {
			continue
		}
		impact = append(impact, "Service/"+svc.Name)
		for _, ingress := range c.ingressesForService(ctx, pod.Namespace, svc.Name) {
			impact = append(impact, "Ingress/"+ingress.Name)
		}
		for _, route := range c.httpRoutesForService(ctx, pod.Namespace, svc.Name) {
			impact = append(impact, "HTTPRoute/"+route)
		}
	}
	return uniqueSorted(impact)
}

func (c *Controller) serviceEndpointsMissingPod(ctx context.Context, pod *v1.Pod, serviceName string) bool {
	endpoints, err := c.clientset.CoreV1().Endpoints(pod.Namespace).Get(ctx, serviceName, metav1.GetOptions{})
	if err != nil {
		return true
	}
	return !endpointsIncludePod(endpoints, pod.Name)
}

func (c *Controller) ingressesForService(ctx context.Context, namespace, serviceName string) []networkingv1.Ingress {
	ingresses, err := c.clientset.NetworkingV1().Ingresses(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil
	}
	var out []networkingv1.Ingress
	for _, ingress := range ingresses.Items {
		if ingressReferencesService(ingress, serviceName) {
			out = append(out, ingress)
		}
	}
	return out
}

func (c *Controller) httpRoutesForService(ctx context.Context, namespace, serviceName string) []string {
	if c.dynamicClient == nil {
		return nil
	}
	gvr := schema.GroupVersionResource{Group: "gateway.networking.k8s.io", Version: "v1", Resource: "httproutes"}
	list, err := c.dynamicClient.Resource(gvr).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil
	}
	var out []string
	for _, item := range list.Items {
		if httpRouteReferencesService(item, namespace, serviceName) {
			out = append(out, item.GetName())
		}
	}
	sort.Strings(out)
	return out
}

func httpRouteReferencesService(item unstructured.Unstructured, defaultNamespace, serviceName string) bool {
	rules, ok, _ := unstructured.NestedSlice(item.Object, "spec", "rules")
	if !ok {
		return false
	}
	for _, rawRule := range rules {
		rule, ok := rawRule.(map[string]interface{})
		if !ok {
			continue
		}
		backendRefs, ok := rule["backendRefs"].([]interface{})
		if !ok {
			continue
		}
		for _, rawBackend := range backendRefs {
			backend, ok := rawBackend.(map[string]interface{})
			if !ok {
				continue
			}
			kind, _ := backend["kind"].(string)
			if kind != "" && kind != "Service" {
				continue
			}
			name, _ := backend["name"].(string)
			if name != serviceName {
				continue
			}
			namespace := defaultNamespace
			if ns, ok := backend["namespace"].(string); ok && ns != "" {
				namespace = ns
			}
			if namespace == defaultNamespace {
				return true
			}
		}
	}
	return false
}
