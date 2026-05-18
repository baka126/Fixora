package controller

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"fixora/pkg/analyzer"
	v1 "k8s.io/api/core/v1"
)

type analyzerOrchestration struct {
	Findings []Diagnosis
	Related  []string
}

func (o analyzerOrchestration) Summary() string {
	if len(o.Findings) == 0 {
		return "Analyzer Findings: no additional related analyzer findings."
	}
	lines := make([]string, 0, len(o.Findings))
	for _, finding := range o.Findings {
		ref := analyzerRef(finding)
		if ref == "" {
			ref = "related resource"
		}
		evidence := ""
		if len(finding.Evidence) > 0 {
			evidence = " Evidence: " + finding.Evidence[0]
		}
		lines = append(lines, fmt.Sprintf("%s: %s. Cause: %s. Strategy: %s.%s",
			ref,
			finding.Symptom,
			finding.LikelyCause,
			finding.PatchStrategy,
			evidence,
		))
	}
	return "Analyzer Findings:\n- " + strings.Join(lines, "\n- ")
}

func (c *Controller) runIncidentAnalyzers(ctx context.Context, pod *v1.Pod, triggerReason string, primary Diagnosis, corr ResourceCorrelation) analyzerOrchestration {
	identity := c.workloadIdentityForPod(ctx, pod)
	relevant := newAnalyzerRelevantSet()
	relevant.add("Pod", pod.Name)
	relevant.add(identity.Kind, identity.Name)
	relevant.addRefs(corr.Related...)
	relevant.addRefs(primary.Related...)

	selected := selectedIncidentAnalyzers(primary, triggerReason, identity)
	if len(selected) == 0 {
		return analyzerOrchestration{Related: relevant.refs()}
	}

	analyzers := analyzer.GetAnalyzerMap()
	var findings []Diagnosis
	for _, name := range selected {
		ana, ok := analyzers[name]
		if !ok {
			continue
		}
		results, err := ana.Analyze(analyzer.Context{
			Client:        c.clientset,
			DynamicClient: c.dynamicClient,
			Context:       ctx,
			Namespace:     pod.Namespace,
		})
		if err != nil {
			slog.Debug("Incident analyzer failed", "analyzer", name, "ns", pod.Namespace, "pod", pod.Name, "error", err)
			continue
		}
		for _, result := range results {
			if !relevant.matches(result) {
				continue
			}
			findings = append(findings, result)
			relevant.add(result.Kind, result.Name)
			relevant.addRefs(result.Related...)
		}
	}

	findings = dedupeAnalyzerFindings(findings)
	if len(findings) > 6 {
		findings = findings[:6]
	}
	return analyzerOrchestration{
		Findings: findings,
		Related:  relevant.refs(),
	}
}

func selectedIncidentAnalyzers(primary Diagnosis, triggerReason string, identity workloadIdentity) []string {
	selected := map[string]bool{}
	add := func(names ...string) {
		for _, name := range names {
			if name != "" {
				selected[name] = true
			}
		}
	}

	switch identity.Kind {
	case "Deployment":
		add("deployment", "replicaset")
	case "StatefulSet":
		add("statefulset")
	case "DaemonSet":
		add("daemonset")
	case "Job":
		add("job")
	case "ReplicaSet":
		add("replicaset")
	}

	haystack := strings.ToLower(triggerReason + " " + primary.Symptom + " " + primary.LikelyCause + " " + strings.Join(primary.Evidence, " "))
	switch primary.Category {
	case CategoryNetwork:
		add("service", "endpoints", "ingress", "networkpolicy", "gateway", "httproute")
	case CategoryStorage:
		add("storage", "quota")
	case CategoryScheduling:
		add("storage", "quota", "hpa", "node")
	case CategoryConfig:
		add("policy", "webhook")
	case CategoryRollout:
		add("hpa")
	case CategoryRuntime:
		add("hpa")
	}

	if containsAny(haystack, "service", "endpoint", "ingress", "network", "gateway", "httproute", "dns") {
		add("service", "endpoints", "ingress", "networkpolicy", "gateway", "httproute")
	}
	if containsAny(haystack, "pvc", "persistentvolume", "volume", "mount", "storage") {
		add("storage")
	}
	if containsAny(haystack, "hpa", "autoscal", "metrics") {
		add("hpa")
	}
	if containsAny(haystack, "node", "taint", "affinity", "selector", "unschedulable") {
		add("node", "quota")
	}

	delete(selected, "pod")
	names := make([]string, 0, len(selected))
	for name := range selected {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func mergeAnalyzerFindings(primary Diagnosis, findings []Diagnosis) Diagnosis {
	if len(findings) == 0 {
		return primary
	}
	if primary.Category == CategoryUnknown || (primary.PatchStrategy == PatchNone && primary.Confidence < 70) {
		best := findings[0]
		for _, finding := range findings[1:] {
			if finding.Confidence > best.Confidence && finding.PatchStrategy != PatchNone {
				best = finding
			}
		}
		if best.Confidence > primary.Confidence && best.PatchStrategy != PatchNone {
			best.Related = uniqueSorted(append(best.Related, primary.Related...))
			return best
		}
	}

	var evidence []string
	var related []string
	related = append(related, primary.Related...)
	for _, finding := range findings {
		related = append(related, analyzerRef(finding))
		related = append(related, finding.Related...)
		if len(evidence) >= 4 {
			continue
		}
		evidence = append(evidence, fmt.Sprintf("%s: %s", analyzerRef(finding), finding.Symptom))
	}
	primary.Evidence = uniqueSorted(append(primary.Evidence, evidence...))
	primary.Related = uniqueSorted(related)
	return primary
}

func dedupeAnalyzerFindings(findings []Diagnosis) []Diagnosis {
	best := map[string]Diagnosis{}
	for _, finding := range findings {
		key := strings.ToLower(strings.Join([]string{
			finding.Namespace,
			finding.Kind,
			finding.Name,
			finding.Symptom,
			string(finding.Category),
		}, "/"))
		if key == "////" {
			key = strings.ToLower(finding.Summary())
		}
		if existing, ok := best[key]; !ok || finding.Confidence > existing.Confidence {
			best[key] = finding
		}
	}
	values := make([]Diagnosis, 0, len(best))
	for _, finding := range best {
		values = append(values, finding)
	}
	sort.Slice(values, func(i, j int) bool {
		if values[i].Confidence == values[j].Confidence {
			return analyzerRef(values[i]) < analyzerRef(values[j])
		}
		return values[i].Confidence > values[j].Confidence
	})
	return values
}

func analyzerRef(result Diagnosis) string {
	if strings.TrimSpace(result.Kind) == "" || strings.TrimSpace(result.Name) == "" {
		return ""
	}
	return strings.TrimSpace(result.Kind) + "/" + strings.TrimSpace(result.Name)
}

type analyzerRelevantSet struct {
	items map[string]string
}

func newAnalyzerRelevantSet() analyzerRelevantSet {
	return analyzerRelevantSet{items: map[string]string{}}
}

func (s analyzerRelevantSet) add(kind, name string) {
	kind = strings.TrimSpace(kind)
	name = strings.TrimSpace(name)
	if kind == "" || name == "" {
		return
	}
	s.items[strings.ToLower(kind+"/"+name)] = kind + "/" + name
}

func (s analyzerRelevantSet) addRefs(refs ...string) {
	for _, ref := range refs {
		if kind, name, ok := splitKindName(ref); ok {
			s.add(kind, name)
		}
	}
}

func (s analyzerRelevantSet) matches(result Diagnosis) bool {
	if s.contains(result.Kind, result.Name) {
		return true
	}
	for _, ref := range result.Related {
		if kind, name, ok := splitKindName(ref); ok && s.contains(kind, name) {
			return true
		}
	}
	for _, item := range result.Evidence {
		lower := strings.ToLower(item)
		for key := range s.items {
			parts := strings.SplitN(key, "/", 2)
			if len(parts) == 2 && strings.Contains(lower, parts[1]) {
				return true
			}
		}
	}
	return false
}

func (s analyzerRelevantSet) contains(kind, name string) bool {
	if kind == "" || name == "" {
		return false
	}
	return s.items[strings.ToLower(strings.TrimSpace(kind)+"/"+strings.TrimSpace(name))] != ""
}

func (s analyzerRelevantSet) refs() []string {
	refs := make([]string, 0, len(s.items))
	for _, ref := range s.items {
		refs = append(refs, ref)
	}
	sort.Strings(refs)
	return refs
}
