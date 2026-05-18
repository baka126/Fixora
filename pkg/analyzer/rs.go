package analyzer

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ReplicaSetAnalyzer struct{}

func (r *ReplicaSetAnalyzer) Analyze(ctx Context) ([]Result, error) {
	list, err := ctx.Client.AppsV1().ReplicaSets(ctx.Namespace).List(ctx.Context, metav1.ListOptions{
		LabelSelector: ctx.LabelSelector,
	})
	if err != nil {
		return nil, err
	}

	var results []Result
	for _, rs := range list.Items {
		var failures []string
		related := []string{}
		if ownerKind, ownerName := GetRootOwner(ctx.Context, ctx.Client, rs.Namespace, rs.ObjectMeta); ownerKind != "" {
			related = append(related, fmt.Sprintf("%s/%s", ownerKind, ownerName))
		}

		// ReplicaSet is usually managed by Deployment, but direct failures here are useful
		if rs.Status.Replicas != rs.Status.ReadyReplicas && rs.Status.Replicas > 0 {
			// DeploymentAnalyzer might catch this, but let's check for specific RS conditions
			for _, cond := range rs.Status.Conditions {
				if cond.Type == "ReplicaFailure" {
					failures = append(failures, fmt.Sprintf("ReplicaSet failure: %s (Reason: %s)", cond.Message, cond.Reason))
				}
			}
		}

		if len(failures) > 0 {
			results = append(results, Result{
				Kind:          "ReplicaSet",
				Name:          rs.Name,
				Namespace:     rs.Namespace,
				Symptom:       "ReplicaSet pod creation failure",
				Category:      CategoryRollout,
				LikelyCause:   "The ReplicaSet is unable to create or maintain the desired number of pods, likely due to quota, permissions, or image issues.",
				Confidence:    85,
				PatchStrategy: PatchResources,
				Evidence:      failures,
				Related:       related,
			})
		}
	}

	return results, nil
}
