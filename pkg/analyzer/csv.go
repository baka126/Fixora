package analyzer

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type CSVAnalyzer struct{}

func (c *CSVAnalyzer) Analyze(ctx Context) ([]Result, error) {
	if ctx.DynamicClient == nil {
		return nil, nil
	}

	gvr := schema.GroupVersionResource{
		Group:    "operators.coreos.com",
		Version:  "v1alpha1",
		Resource: "clusterserviceversions",
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
		phase, _, _ := unstructured.NestedString(item.Object, "status", "phase")
		
		if phase != "" && phase != "Succeeded" {
			results = append(results, Result{
				Kind:          "ClusterServiceVersion",
				Name:          item.GetName(),
				Namespace:     item.GetNamespace(),
				Symptom:       "Operator installation failure",
				Category:      CategoryRollout,
				LikelyCause:   "The ClusterServiceVersion (CSV) failed to install or reach a successful phase, potentially due to missing dependencies or permissions.",
				Confidence:    85,
				PatchStrategy: PatchNone,
				Evidence:      []string{fmt.Sprintf("CSV phase is %s", phase)},
			})
		}
	}

	return results, nil
}
