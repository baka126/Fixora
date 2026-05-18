package analyzer

import (
	"fmt"

	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type EndpointSliceAnalyzer struct{}

func (e *EndpointSliceAnalyzer) Analyze(ctx Context) ([]Result, error) {
	slices, err := ctx.Client.DiscoveryV1().EndpointSlices(ctx.Namespace).List(ctx.Context, metav1.ListOptions{
		LabelSelector: ctx.LabelSelector,
	})
	if err != nil {
		return nil, err
	}
	var results []Result
	for _, slice := range slices.Items {
		serviceName := slice.Labels[discoveryv1.LabelServiceName]
		if serviceName == "" {
			continue
		}
		total := len(slice.Endpoints)
		ready := 0
		for _, ep := range slice.Endpoints {
			if ep.Conditions.Ready != nil && *ep.Conditions.Ready {
				ready++
			}
		}
		if total == 0 || ready > 0 {
			continue
		}
		results = append(results, Result{
			Kind:          "Service",
			Name:          serviceName,
			Namespace:     slice.Namespace,
			Symptom:       "Service has no ready endpoints",
			Category:      CategoryNetwork,
			LikelyCause:   "The Service selector matches endpoints that are not ready, so traffic may fail even though the Service exists.",
			Confidence:    85,
			PatchStrategy: PatchServiceSelector,
			Evidence:      []string{fmt.Sprintf("EndpointSlice %s has %d endpoints and 0 ready endpoints", slice.Name, total)},
			Related:       []string{"EndpointSlice/" + slice.Name},
		})
	}
	return results, nil
}
