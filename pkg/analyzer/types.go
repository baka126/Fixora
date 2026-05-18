package analyzer

import (
	"context"
	"fmt"
	"strings"

	"fixora/pkg/ai"
	"fixora/pkg/metrics"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
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

type Result struct {
	Symptom       string
	Category      IssueCategory
	LikelyCause   string
	Confidence    int
	PatchStrategy PatchStrategy
	Evidence      []string
	Related       []string
	Kind          string
	Name          string
	Namespace     string
}

func (r Result) Summary() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Diagnosis: %s\nCategory: %s\nLikely Cause: %s\nConfidence: %d%%\nRecommended Patch Strategy: %s",
		r.Symptom, r.Category, r.LikelyCause, r.Confidence, r.PatchStrategy))

	if len(r.Evidence) > 0 {
		sb.WriteString("\nEvidence:")
		for _, item := range r.Evidence {
			sb.WriteString("\n- " + item)
		}
	}
	if len(r.Related) > 0 {
		sb.WriteString("\nRelated Resources:")
		for _, item := range r.Related {
			sb.WriteString("\n- " + item)
		}
	}
	return sb.String()
}

type IAnalyzer interface {
	Analyze(ctx Context) ([]Result, error)
}

type Context struct {
	Client        kubernetes.Interface
	DynamicClient dynamic.Interface
	Context       context.Context
	Namespace     string
	LabelSelector string
	AIClient      ai.Provider
	MetricsClient metrics.MetricsProvider
}
