package analyzer

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type GatewayAnalyzer struct{}

func (g *GatewayAnalyzer) Analyze(ctx Context) ([]Result, error) {
	if ctx.DynamicClient == nil {
		return nil, nil
	}

	gvr := schema.GroupVersionResource{
		Group:    "gateway.networking.k8s.io",
		Version:  "v1",
		Resource: "gateways",
	}

	list, err := ctx.DynamicClient.Resource(gvr).Namespace(ctx.Namespace).List(ctx.Context, metav1.ListOptions{
		LabelSelector: ctx.LabelSelector,
	})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, nil // Gateway API not installed
		}
		return nil, err
	}

	var results []Result
	for _, item := range list.Items {
		var failures []string

		// Check GatewayClass
		gcName, ok, _ := unstructured.NestedString(item.Object, "spec", "gatewayClassName")
		if !ok || gcName == "" {
			failures = append(failures, "Gateway is missing spec.gatewayClassName.")
		}

		gcGVR := schema.GroupVersionResource{
			Group:    "gateway.networking.k8s.io",
			Version:  "v1",
			Resource: "gatewayclasses",
		}
		if gcName != "" {
			_, err := ctx.DynamicClient.Resource(gcGVR).Get(ctx.Context, gcName, metav1.GetOptions{})
			if err != nil {
				failures = append(failures, fmt.Sprintf("Gateway references non-existent GatewayClass: %s", gcName))
			}
		}

		// Check Status
		if conditions, ok, _ := unstructured.NestedSlice(item.Object, "status", "conditions"); ok && !hasAcceptedCondition(conditions) {
			failures = append(failures, "Gateway is not accepted by the controller.")
		}

		if len(failures) > 0 {
			results = append(results, Result{
				Kind:          "Gateway",
				Name:          item.GetName(),
				Namespace:     item.GetNamespace(),
				Symptom:       "Gateway configuration issue",
				Category:      CategoryNetwork,
				LikelyCause:   "The Gateway is misconfigured or its GatewayClass is missing/invalid.",
				Confidence:    85,
				PatchStrategy: PatchNone,
				Evidence:      failures,
			})
		}
	}

	return results, nil
}
