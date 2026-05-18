package analyzer

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type PDBAnalyzer struct{}

func (p *PDBAnalyzer) Analyze(ctx Context) ([]Result, error) {
	list, err := ctx.Client.PolicyV1().PodDisruptionBudgets(ctx.Namespace).List(ctx.Context, metav1.ListOptions{
		LabelSelector: ctx.LabelSelector,
	})
	if err != nil {
		return nil, err
	}

	var results []Result
	for _, pdb := range list.Items {
		var failures []string

		// Check Conditions
		for _, condition := range pdb.Status.Conditions {
			if condition.Type == "DisruptionAllowed" && condition.Status == metav1.ConditionFalse {
				failures = append(failures, fmt.Sprintf("Disruption is not allowed for PDB %s: %s (Reason: %s)", pdb.Name, condition.Message, condition.Reason))
			}
		}

		// Check if it matches any pods
		if pdb.Spec.Selector != nil {
			pods, err := ctx.Client.CoreV1().Pods(pdb.Namespace).List(ctx.Context, metav1.ListOptions{
				LabelSelector: metav1.FormatLabelSelector(pdb.Spec.Selector),
			})
			if err == nil && len(pods.Items) == 0 {
				failures = append(failures, fmt.Sprintf("PDB %s selector does not match any pods in the namespace.", pdb.Name))
			}
		}

		if len(failures) > 0 {
			results = append(results, Result{
				Kind:          "PodDisruptionBudget",
				Name:          pdb.Name,
				Namespace:     pdb.Namespace,
				Symptom:       "Pod disruption budget constraint failure",
				Category:      CategoryScheduling,
				LikelyCause:   "The PDB is preventing disruptions (e.g. node drains) because the current number of pods is at or below the minimum available threshold.",
				Confidence:    85,
				PatchStrategy: PatchNone,
				Evidence:      failures,
			})
		}
	}

	return results, nil
}
