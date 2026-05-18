package analyzer

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type InstallPlanAnalyzer struct{}

func (i *InstallPlanAnalyzer) Analyze(ctx Context) ([]Result, error) {
	if ctx.DynamicClient == nil {
		return nil, nil
	}

	gvr := schema.GroupVersionResource{
		Group:    "operators.coreos.com",
		Version:  "v1alpha1",
		Resource: "installplans",
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
		
		if phase != "" && phase != "Complete" {
			results = append(results, Result{
				Kind:          "InstallPlan",
				Name:          item.GetName(),
				Namespace:     item.GetNamespace(),
				Symptom:       "Operator install plan stalled",
				Category:      CategoryRollout,
				LikelyCause:   "The InstallPlan is not complete, possibly awaiting manual approval or failing to resolve requirements.",
				Confidence:    80,
				PatchStrategy: PatchNone,
				Evidence:      []string{fmt.Sprintf("InstallPlan phase is %s", phase)},
			})
		}
	}

	return results, nil
}
