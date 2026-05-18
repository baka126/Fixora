package analyzer

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type CronJobAnalyzer struct{}

func (c *CronJobAnalyzer) Analyze(ctx Context) ([]Result, error) {
	list, err := ctx.Client.BatchV1().CronJobs(ctx.Namespace).List(ctx.Context, metav1.ListOptions{
		LabelSelector: ctx.LabelSelector,
	})
	if err != nil {
		return nil, err
	}

	var results []Result
	for _, cj := range list.Items {
		var failures []string

		if cj.Spec.Suspend != nil && *cj.Spec.Suspend {
			failures = append(failures, fmt.Sprintf("CronJob %s is suspended.", cj.Name))
		}

		// Check last schedule time
		if cj.Status.LastScheduleTime == nil {
			failures = append(failures, fmt.Sprintf("CronJob %s has never been scheduled.", cj.Name))
		}

		if len(failures) > 0 {
			results = append(results, Result{
				Kind:          "CronJob",
				Name:          cj.Name,
				Namespace:     cj.Namespace,
				Symptom:       "CronJob scheduling issues",
				Category:      CategoryScheduling,
				LikelyCause:   "The CronJob is suspended or has failed to trigger its scheduled runs.",
				Confidence:    75,
				PatchStrategy: PatchSchedulingPolicy,
				Evidence:      failures,
			})
		}
	}

	return results, nil
}
