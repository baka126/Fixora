package analyzer

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type StatefulSetAnalyzer struct{}

func (s *StatefulSetAnalyzer) Analyze(ctx Context) ([]Result, error) {
	list, err := ctx.Client.AppsV1().StatefulSets(ctx.Namespace).List(ctx.Context, metav1.ListOptions{
		LabelSelector: ctx.LabelSelector,
	})
	if err != nil {
		return nil, err
	}

	var results []Result
	for _, ss := range list.Items {
		var failures []string

		if ss.Status.Replicas != ss.Status.ReadyReplicas {
			failures = append(failures, fmt.Sprintf("StatefulSet has %d/%d ready replicas.", ss.Status.ReadyReplicas, ss.Status.Replicas))
		}

		if ss.Status.UpdatedReplicas < ss.Status.Replicas {
			failures = append(failures, fmt.Sprintf("StatefulSet update in progress: %d/%d updated.", ss.Status.UpdatedReplicas, ss.Status.Replicas))
		}

		if len(failures) > 0 {
			results = append(results, Result{
				Kind:          "StatefulSet",
				Name:          ss.Name,
				Namespace:     ss.Namespace,
				Symptom:       "StatefulSet rollout or scaling failure",
				Category:      CategoryRollout,
				LikelyCause:   "The stateful workload is failing to scale or update correctly, potentially due to resource constraints or volume binding issues.",
				Confidence:    85,
				PatchStrategy: PatchResources,
				Evidence:      failures,
			})
		}
	}

	return results, nil
}
