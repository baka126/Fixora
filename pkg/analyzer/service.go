package analyzer

import (
	"fmt"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ServiceAnalyzer struct{}

func (s *ServiceAnalyzer) Analyze(ctx Context) ([]Result, error) {
	list, err := ctx.Client.CoreV1().Services(ctx.Namespace).List(ctx.Context, metav1.ListOptions{
		LabelSelector: ctx.LabelSelector,
	})
	if err != nil {
		return nil, err
	}

	var results []Result
	for _, svc := range list.Items {
		var failures []string
		var related []string

		// Check for selector
		if len(svc.Spec.Selector) == 0 && svc.Spec.Type != v1.ServiceTypeExternalName {
			// Some services (like those for endpoints manually managed) might not have selectors
			// but usually it's a configuration error for standard services.
			// Let's check endpoints.
		}

		// Check Endpoints
		endpoints, err := ctx.Client.CoreV1().Endpoints(svc.Namespace).Get(ctx.Context, svc.Name, metav1.GetOptions{})
		if err != nil {
			failures = append(failures, fmt.Sprintf("Service has no corresponding endpoints resource: %v", err))
		} else {
			hasEndpoints := false
			for _, subset := range endpoints.Subsets {
				if len(subset.Addresses) > 0 {
					hasEndpoints = true
					break
				}
			}

			if !hasEndpoints {
				failures = append(failures, "Service has no active endpoints. This usually means no Pods match the selector or matching Pods are not Ready.")
				for k, v := range svc.Spec.Selector {
					failures = append(failures, fmt.Sprintf("Missing selector: %s=%s", k, v))
				}
			}
		}

		if len(failures) > 0 {
			results = append(results, Result{
				Kind:          "Service",
				Name:          svc.Name,
				Namespace:     svc.Namespace,
				Symptom:       "Service has no traffic targets",
				Category:      CategoryNetwork,
				LikelyCause:   "No pods are currently matching the service selector or pods are failing readiness probes.",
				Confidence:    90,
				PatchStrategy: PatchServiceSelector,
				Evidence:      failures,
				Related:       related,
			})
		}
	}

	return results, nil
}
