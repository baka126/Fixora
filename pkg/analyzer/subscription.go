package analyzer

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type SubscriptionAnalyzer struct{}

func (s *SubscriptionAnalyzer) Analyze(ctx Context) ([]Result, error) {
	if ctx.DynamicClient == nil {
		return nil, nil
	}

	gvr := schema.GroupVersionResource{
		Group:    "operators.coreos.com",
		Version:  "v1alpha1",
		Resource: "subscriptions",
	}

	list, err := ctx.DynamicClient.Resource(gvr).Namespace(ctx.Namespace).List(ctx.Context, metav1.ListOptions{
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
		state, _, _ := unstructured.NestedString(item.Object, "status", "state")
		
		if state != "" && state != "AtLatestKnown" {
			results = append(results, Result{
				Kind:          "Subscription",
				Name:          item.GetName(),
				Namespace:     item.GetNamespace(),
				Symptom:       "Operator subscription issue",
				Category:      CategoryScheduling,
				LikelyCause:   "The operator subscription is failing to reach the latest version, possibly due to catalog or network issues.",
				Confidence:    80,
				PatchStrategy: PatchNone,
				Evidence:      []string{fmt.Sprintf("Subscription state is %s", state)},
			})
		}
	}

	return results, nil
}
