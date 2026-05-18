package analyzer

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type NetworkPolicyAnalyzer struct{}

func (n *NetworkPolicyAnalyzer) Analyze(ctx Context) ([]Result, error) {
	policies, err := ctx.Client.NetworkingV1().NetworkPolicies(ctx.Namespace).List(ctx.Context, metav1.ListOptions{
		LabelSelector: ctx.LabelSelector,
	})
	if err != nil {
		return nil, err
	}

	var results []Result
	for _, policy := range policies.Items {
		var failures []string

		// Check for overly permissive policy
		if len(policy.Spec.PodSelector.MatchLabels) == 0 && len(policy.Spec.PodSelector.MatchExpressions) == 0 {
			failures = append(failures, fmt.Sprintf("NetworkPolicy %s applies to all pods in the namespace (empty selector).", policy.Name))
		}

		// Check if it applies to any pods
		pods, err := ctx.Client.CoreV1().Pods(policy.Namespace).List(ctx.Context, metav1.ListOptions{
			LabelSelector: metav1.FormatLabelSelector(&policy.Spec.PodSelector),
		})
		if err == nil && len(pods.Items) == 0 {
			failures = append(failures, fmt.Sprintf("NetworkPolicy %s does not match any pods in the namespace.", policy.Name))
		}

		if len(failures) > 0 {
			results = append(results, Result{
				Kind:          "NetworkPolicy",
				Name:          policy.Name,
				Namespace:     policy.Namespace,
				Symptom:       "Network isolation potentially misconfigured",
				Category:      CategoryNetwork,
				LikelyCause:   "The NetworkPolicy might be too permissive or applies to no workloads, potentially causing security gaps or unintended isolation.",
				Confidence:    70,
				PatchStrategy: PatchNone,
				Evidence:      failures,
			})
		}
	}

	return results, nil
}
