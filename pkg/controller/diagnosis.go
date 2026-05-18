package controller

import (
	"context"

	"fixora/pkg/analyzer"
	"fixora/pkg/notifications"
	v1 "k8s.io/api/core/v1"
)

// Bridge types for backward compatibility in this file
type IssueCategory = analyzer.IssueCategory
type PatchStrategy = analyzer.PatchStrategy
type Diagnosis = analyzer.Result

const (
	CategoryUnknown    = analyzer.CategoryUnknown
	CategoryScheduling = analyzer.CategoryScheduling
	CategoryRuntime    = analyzer.CategoryRuntime
	CategoryConfig     = analyzer.CategoryConfig
	CategoryRollout    = analyzer.CategoryRollout
	CategoryNetwork    = analyzer.CategoryNetwork
	CategoryStorage    = analyzer.CategoryStorage

	PatchNone             = analyzer.PatchNone
	PatchResources        = analyzer.PatchResources
	PatchImage            = analyzer.PatchImage
	PatchEnvOrVolumeRef   = analyzer.PatchEnvOrVolumeRef
	PatchSchedulingPolicy = analyzer.PatchSchedulingPolicy
	PatchProbe            = analyzer.PatchProbe
	PatchServiceSelector  = analyzer.PatchServiceSelector
	PatchPVC              = analyzer.PatchPVC
)

func refineDiagnosisFromEvidence(d Diagnosis, evidence notifications.EvidenceChain) Diagnosis {
	haystack := notifications.Haystack(evidence, d.Evidence, d.LikelyCause)
	if notifications.ContainsAny(haystack, "exec format error", "architecture mismatch", "cpu architecture", "wrong architecture") {
		d.Category = CategoryRuntime
		d.PatchStrategy = PatchImage
		d.Confidence = 85
		d.LikelyCause = "The container image is incompatible with the node CPU architecture; use a multi-architecture image or rebuild the image for the target platform."
		if d.Symptom == "" || d.Symptom == "Container is repeatedly crashing" {
			d.Symptom = "Container image cannot run on the node architecture"
		}
	}
	return d
}

func (c *Controller) classifyPodIssue(ctx context.Context, pod *v1.Pod, triggerReason string) Diagnosis {
	ana := &analyzer.PodAnalyzer{}
	anaCtx := analyzer.Context{
		Client:        c.clientset,
		DynamicClient: c.dynamicClient,
		Context:       ctx,
		Namespace:     pod.Namespace,
		LabelSelector: "metadata.name=" + pod.Name, // Surgical selection
	}

	results, err := ana.Analyze(anaCtx)
	if err != nil || len(results) == 0 {
		return Diagnosis{
			Symptom:     triggerReason,
			Category:    CategoryUnknown,
			LikelyCause: "Analyzer failed or found no issues; manual investigation required.",
			Confidence:  10,
		}
	}

	// For Fixora's current workflow, we expect one result for the triggered pod
	return results[0]
}
