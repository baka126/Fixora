package analyzer

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type CatalogSourceAnalyzer struct{}

func (c *CatalogSourceAnalyzer) Analyze(ctx Context) ([]Result, error) {
	if ctx.DynamicClient == nil {
		return nil, nil
	}

	gvr := schema.GroupVersionResource{
		Group:    "operators.coreos.com",
		Version:  "v1alpha1",
		Resource: "catalogsources",
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
		state, _, _ := unstructured.NestedString(item.Object, "status", "connectionState", "lastObservedState")
		
		if state != "" && strings.ToUpper(state) != "READY" {
			results = append(results, Result{
				Kind:          "CatalogSource",
				Name:          item.GetName(),
				Namespace:     item.GetNamespace(),
				Symptom:       "Operator catalog connection error",
				Category:      CategoryScheduling,
				LikelyCause:   "The CatalogSource is not ready, preventing the operator manager from finding new operator versions.",
				Confidence:    90,
				PatchStrategy: PatchNone,
				Evidence:      []string{fmt.Sprintf("CatalogSource state is %s", state)},
			})
		}
	}

	return results, nil
}
