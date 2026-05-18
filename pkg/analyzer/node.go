package analyzer

import (
	"fmt"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type NodeAnalyzer struct{}

func (n *NodeAnalyzer) Analyze(ctx Context) ([]Result, error) {
	list, err := ctx.Client.CoreV1().Nodes().List(ctx.Context, metav1.ListOptions{
		LabelSelector: ctx.LabelSelector,
	})
	if err != nil {
		return nil, err
	}

	var results []Result
	for _, node := range list.Items {
		var failures []string

		for _, condition := range node.Status.Conditions {
			switch condition.Type {
			case v1.NodeReady:
				if condition.Status != v1.ConditionTrue {
					failures = append(failures, fmt.Sprintf("Node is not Ready: %s (Reason: %s)", condition.Message, condition.Reason))
				}
			case v1.NodeMemoryPressure, v1.NodeDiskPressure, v1.NodePIDPressure, v1.NodeNetworkUnavailable:
				if condition.Status == v1.ConditionTrue {
					failures = append(failures, fmt.Sprintf("Node has %s: %s (Reason: %s)", condition.Type, condition.Message, condition.Reason))
				}
			}
		}

		if len(failures) > 0 {
			results = append(results, Result{
				Kind:          "Node",
				Name:          node.Name,
				Namespace:     "", // Nodes are cluster-scoped
				Symptom:       "Node health issues detected",
				Category:      CategoryScheduling,
				LikelyCause:   "The node is under resource pressure or has a network/internal failure, preventing it from hosting workloads reliably.",
				Confidence:    95,
				PatchStrategy: PatchSchedulingPolicy,
				Evidence:      failures,
			})
		}
	}

	return results, nil
}
