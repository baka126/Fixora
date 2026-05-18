package analyzer

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type JobAnalyzer struct{}

func (j *JobAnalyzer) Analyze(ctx Context) ([]Result, error) {
	list, err := ctx.Client.BatchV1().Jobs(ctx.Namespace).List(ctx.Context, metav1.ListOptions{
		LabelSelector: ctx.LabelSelector,
	})
	if err != nil {
		return nil, err
	}

	var results []Result
	for _, job := range list.Items {
		var failures []string
		related := []string{}
		if ownerKind, ownerName := GetRootOwner(ctx.Context, ctx.Client, job.Namespace, job.ObjectMeta); ownerKind != "" {
			related = append(related, fmt.Sprintf("%s/%s", ownerKind, ownerName))
		}

		if job.Spec.Suspend != nil && *job.Spec.Suspend {
			failures = append(failures, fmt.Sprintf("Job %s is suspended.", job.Name))
		}

		if job.Status.Failed > 0 {
			failures = append(failures, fmt.Sprintf("Job %s has %d failed pods.", job.Name, job.Status.Failed))

			// Check for backoff limit events
			events, _ := ctx.Client.CoreV1().Events(job.Namespace).List(ctx.Context, metav1.ListOptions{
				FieldSelector: fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=Job", job.Name),
			})
			for _, evt := range events.Items {
				if evt.Reason == "BackoffLimitExceeded" {
					failures = append(failures, fmt.Sprintf("Backoff limit exceeded: %s", evt.Message))
					break
				}
			}
		}

		if len(failures) > 0 {
			results = append(results, Result{
				Kind:          "Job",
				Name:          job.Name,
				Namespace:     job.Namespace,
				Symptom:       "Batch job failure detected",
				Category:      CategoryRuntime,
				LikelyCause:   "The job has reached its backoff limit or pods are failing to complete successfully.",
				Confidence:    85,
				PatchStrategy: PatchNone,
				Evidence:      failures,
				Related:       related,
			})
		}
	}

	return results, nil
}
