package analyzer

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type DeploymentAnalyzer struct{}

func (d *DeploymentAnalyzer) Analyze(ctx Context) ([]Result, error) {
	list, err := ctx.Client.AppsV1().Deployments(ctx.Namespace).List(ctx.Context, metav1.ListOptions{
		LabelSelector: ctx.LabelSelector,
	})
	if err != nil {
		return nil, err
	}

	var results []Result
	for _, deploy := range list.Items {
		var failures []string

		if deploy.Status.Replicas != deploy.Status.ReadyReplicas {
			failures = append(failures, fmt.Sprintf("Deployment has %d/%d ready replicas.", deploy.Status.ReadyReplicas, deploy.Status.Replicas))
		}

		if deploy.Status.UnavailableReplicas > 0 {
			failures = append(failures, fmt.Sprintf("Deployment has %d unavailable replicas.", deploy.Status.UnavailableReplicas))
		}

		for _, cond := range deploy.Status.Conditions {
			if cond.Type == "Progressing" && cond.Status == "False" {
				failures = append(failures, fmt.Sprintf("Deployment rollout stalled: %s", cond.Message))
			}
		}

		if len(failures) > 0 {
			results = append(results, Result{
				Kind:          "Deployment",
				Name:          deploy.Name,
				Namespace:     deploy.Namespace,
				Symptom:       "Deployment rollout or scaling failure",
				Category:      CategoryRollout,
				LikelyCause:   "The deployment rollout is failing or replicas are unable to reach a ready state.",
				Confidence:    85,
				PatchStrategy: PatchResources,
				Evidence:      failures,
			})
		}
	}

	return results, nil
}
