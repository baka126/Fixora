package analyzer

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type SecurityAnalyzer struct{}

func (s *SecurityAnalyzer) Analyze(ctx Context) ([]Result, error) {
	pods, err := ctx.Client.CoreV1().Pods(ctx.Namespace).List(ctx.Context, metav1.ListOptions{
		LabelSelector: ctx.LabelSelector,
	})
	if err != nil {
		return nil, err
	}

	var results []Result
	for _, pod := range pods.Items {
		var failures []string

		for _, container := range pod.Spec.Containers {
			if container.SecurityContext != nil {
				if container.SecurityContext.Privileged != nil && *container.SecurityContext.Privileged {
					failures = append(failures, fmt.Sprintf("Container %s is running in privileged mode.", container.Name))
				}
				if container.SecurityContext.RunAsNonRoot != nil && !*container.SecurityContext.RunAsNonRoot {
					failures = append(failures, fmt.Sprintf("Container %s is not configured to run as a non-root user.", container.Name))
				}
			}
		}

		if len(failures) > 0 {
			results = append(results, Result{
				Kind:          "Pod",
				Name:          pod.Name,
				Namespace:     pod.Namespace,
				Symptom:       "Security risk detected",
				Category:      CategoryConfig,
				LikelyCause:   "The pod configuration violates zero-trust security principles by allowing elevated privileges or root access.",
				Confidence:    70,
				PatchStrategy: PatchNone,
				Evidence:      failures,
			})
		}
	}

	return results, nil
}
