package analyzer

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type PolicyAnalyzer struct{}

func (p *PolicyAnalyzer) Analyze(ctx Context) ([]Result, error) {
	if ctx.DynamicClient == nil {
		return nil, nil
	}

	gvr := schema.GroupVersionResource{
		Group:    "wgpolicyk8s.io",
		Version:  "v1alpha2",
		Resource: "policyreports",
	}

	list, err := ctx.DynamicClient.Resource(gvr).Namespace(ctx.Namespace).List(ctx.Context, metav1.ListOptions{
		LabelSelector: ctx.LabelSelector,
	})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, nil // Policy engine (Kyverno/Gatekeeper) not installed or using different GVR
		}
		return nil, err
	}

	var results []Result
	for _, item := range list.Items {
		results = append(results, p.parsePolicyReport(item)...)
	}

	// Also check ClusterPolicyReports
	cgvr := schema.GroupVersionResource{
		Group:    "wgpolicyk8s.io",
		Version:  "v1alpha2",
		Resource: "clusterpolicyreports",
	}
	clist, err := ctx.DynamicClient.Resource(cgvr).List(ctx.Context, metav1.ListOptions{
		LabelSelector: ctx.LabelSelector,
	})
	if err == nil {
		for _, item := range clist.Items {
			results = append(results, p.parsePolicyReport(item)...)
		}
	}

	return results, nil
}

func (p *PolicyAnalyzer) parsePolicyReport(item unstructured.Unstructured) []Result {
	var results []Result
	resultsList, ok, _ := unstructured.NestedSlice(item.Object, "results")
	if !ok {
		return nil
	}

	for _, res := range resultsList {
		r, ok := res.(map[string]interface{})
		if !ok {
			continue
		}

		resultStatus, _, _ := unstructured.NestedString(r, "result")
		if resultStatus == "fail" || resultStatus == "warn" {
			policy, _, _ := unstructured.NestedString(r, "policy")
			message, _, _ := unstructured.NestedString(r, "message")
			category, _, _ := unstructured.NestedString(r, "category")
			
			// Extract affected resource
			var targetName, targetKind string
			if resources, ok := r["resources"].([]interface{}); ok && len(resources) > 0 {
				if firstRes, ok := resources[0].(map[string]interface{}); ok {
					targetName, _ = firstRes["name"].(string)
					targetKind, _ = firstRes["kind"].(string)
				}
			}

			results = append(results, Result{
				Kind:          item.GetKind(),
				Name:          item.GetName(),
				Namespace:     item.GetNamespace(),
				Symptom:       fmt.Sprintf("Policy violation: %s", policy),
				Category:      CategoryConfig,
				LikelyCause:   fmt.Sprintf("A security or compliance policy (%s) has flagged this resource: %s", policy, message),
				Confidence:    90,
				PatchStrategy: PatchNone,
				Evidence:      []string{fmt.Sprintf("Policy: %s, Category: %s, Message: %s", policy, category, message)},
				Related:       []string{fmt.Sprintf("%s/%s", targetKind, targetName)},
			})
		}
	}
	return results
}
