package analyzer

import (
	"fmt"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ResourceQuotaAnalyzer struct{}

func (r *ResourceQuotaAnalyzer) Analyze(ctx Context) ([]Result, error) {
	quotas, err := ctx.Client.CoreV1().ResourceQuotas(ctx.Namespace).List(ctx.Context, metav1.ListOptions{
		LabelSelector: ctx.LabelSelector,
	})
	if err != nil {
		return nil, err
	}
	var results []Result
	for _, quota := range quotas.Items {
		var evidence []string
		for resource, hard := range quota.Status.Hard {
			used, ok := quota.Status.Used[resource]
			if !ok || hard.IsZero() {
				continue
			}
			usedMilli := used.MilliValue()
			hardMilli := hard.MilliValue()
			if hardMilli <= 0 {
				continue
			}
			if float64(usedMilli)/float64(hardMilli) >= 0.95 {
				evidence = append(evidence, fmt.Sprintf("%s used=%s hard=%s", resource, used.String(), hard.String()))
			}
		}
		if len(evidence) == 0 {
			continue
		}
		results = append(results, Result{
			Kind:          "ResourceQuota",
			Name:          quota.Name,
			Namespace:     quota.Namespace,
			Symptom:       "Namespace resource quota nearly exhausted",
			Category:      CategoryScheduling,
			LikelyCause:   "New pods or reschedules may be blocked because namespace quota usage is at or above 95% of hard limits.",
			Confidence:    80,
			PatchStrategy: PatchSchedulingPolicy,
			Evidence:      evidence,
			Related:       quotaRelatedResources(quota),
		})
	}
	return results, nil
}

func quotaRelatedResources(quota v1.ResourceQuota) []string {
	return []string{"ResourceQuota/" + quota.Name}
}
