package analyzer

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type DaemonSetAnalyzer struct{}

func (d *DaemonSetAnalyzer) Analyze(ctx Context) ([]Result, error) {
	list, err := ctx.Client.AppsV1().DaemonSets(ctx.Namespace).List(ctx.Context, metav1.ListOptions{
		LabelSelector: ctx.LabelSelector,
	})
	if err != nil {
		return nil, err
	}

	var results []Result
	for _, ds := range list.Items {
		var failures []string

		if ds.Status.NumberReady < ds.Status.DesiredNumberScheduled {
			failures = append(failures, fmt.Sprintf("DaemonSet has %d ready replicas out of %d desired.", ds.Status.NumberReady, ds.Status.DesiredNumberScheduled))
		}

		if ds.Status.NumberUnavailable > 0 {
			failures = append(failures, fmt.Sprintf("DaemonSet has %d unavailable replicas.", ds.Status.NumberUnavailable))
		}

		if len(failures) > 0 {
			results = append(results, Result{
				Kind:          "DaemonSet",
				Name:          ds.Name,
				Namespace:     ds.Namespace,
				Symptom:       "DaemonSet health issues",
				Category:      CategoryRollout,
				LikelyCause:   "One or more nodes are failing to host the DaemonSet pod, potentially due to resource pressure, taints, or selector mismatches.",
				Confidence:    85,
				PatchStrategy: PatchResources,
				Evidence:      failures,
			})
		}
	}

	return results, nil
}
