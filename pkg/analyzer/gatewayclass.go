package analyzer

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type GatewayClassAnalyzer struct{}

func (g *GatewayClassAnalyzer) Analyze(ctx Context) ([]Result, error) {
	if ctx.DynamicClient == nil {
		return nil, nil
	}

	gvr := schema.GroupVersionResource{
		Group:    "gateway.networking.k8s.io",
		Version:  "v1",
		Resource: "gatewayclasses",
	}

	list, err := ctx.DynamicClient.Resource(gvr).List(ctx.Context, metav1.ListOptions{
		LabelSelector: ctx.LabelSelector,
	})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	var results []Result
	for _, item := range list.Items {
		var failures []string

		if conditions, ok, _ := unstructured.NestedSlice(item.Object, "status", "conditions"); ok && !hasAcceptedCondition(conditions) {
			failures = append(failures, fmt.Sprintf("GatewayClass %s is not accepted by its controller.", item.GetName()))
		}

		if len(failures) > 0 {
			results = append(results, Result{
				Kind:          "GatewayClass",
				Name:          item.GetName(),
				Namespace:     "",
				Symptom:       "GatewayClass rejection",
				Category:      CategoryNetwork,
				LikelyCause:   "The Gateway controller has rejected this GatewayClass, likely due to unsupported parameters or internal errors.",
				Confidence:    90,
				PatchStrategy: PatchNone,
				Evidence:      failures,
			})
		}
	}

	return results, nil
}
