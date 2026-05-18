package analyzer

import (
	"fmt"

	autoscalingv2 "k8s.io/api/autoscaling/v2"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type HpaAnalyzer struct{}

func (h *HpaAnalyzer) Analyze(ctx Context) ([]Result, error) {
	list, err := ctx.Client.AutoscalingV2().HorizontalPodAutoscalers(ctx.Namespace).List(ctx.Context, metav1.ListOptions{
		LabelSelector: ctx.LabelSelector,
	})
	if err != nil {
		return nil, err
	}

	var results []Result
	for _, hpa := range list.Items {
		var failures []string

		// Check Conditions
		for _, condition := range hpa.Status.Conditions {
			if condition.Type == autoscalingv2.ScalingLimited && condition.Status == v1.ConditionTrue {
				failures = append(failures, fmt.Sprintf("Scaling is limited: %s", condition.Message))
			}
			if (condition.Type == autoscalingv2.AbleToScale || condition.Type == autoscalingv2.ScalingActive) && condition.Status == v1.ConditionFalse {
				failures = append(failures, fmt.Sprintf("HPA condition %s is False: %s", condition.Type, condition.Message))
			}
		}

		// Check Target Reference
		target := hpa.Spec.ScaleTargetRef
		if !h.targetExists(ctx, hpa.Namespace, target) {
			failures = append(failures, fmt.Sprintf("Scale target %s/%s does not exist.", target.Kind, target.Name))
		}

		if len(failures) > 0 {
			results = append(results, Result{
				Kind:          "HorizontalPodAutoscaler",
				Name:          hpa.Name,
				Namespace:     hpa.Namespace,
				Symptom:       "Autoscaling issues detected",
				Category:      CategoryScheduling,
				LikelyCause:   "The HPA is unable to scale due to missing metrics, missing target, or resource limits.",
				Confidence:    80,
				PatchStrategy: PatchResources,
				Evidence:      failures,
				Related:       []string{fmt.Sprintf("%s/%s", target.Kind, target.Name)},
			})
		}
	}

	return results, nil
}

func (h *HpaAnalyzer) targetExists(ctx Context, namespace string, target autoscalingv2.CrossVersionObjectReference) bool {
	switch target.Kind {
	case "Deployment":
		_, err := ctx.Client.AppsV1().Deployments(namespace).Get(ctx.Context, target.Name, metav1.GetOptions{})
		return err == nil
	case "StatefulSet":
		_, err := ctx.Client.AppsV1().StatefulSets(namespace).Get(ctx.Context, target.Name, metav1.GetOptions{})
		return err == nil
	case "ReplicaSet":
		_, err := ctx.Client.AppsV1().ReplicaSets(namespace).Get(ctx.Context, target.Name, metav1.GetOptions{})
		return err == nil
	}
	return true // Default to true for unknown kinds to avoid false positives
}
