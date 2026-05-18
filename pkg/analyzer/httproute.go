package analyzer

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type HTTPRouteAnalyzer struct{}

func (h *HTTPRouteAnalyzer) Analyze(ctx Context) ([]Result, error) {
	if ctx.DynamicClient == nil {
		return nil, nil
	}

	gvr := schema.GroupVersionResource{
		Group:    "gateway.networking.k8s.io",
		Version:  "v1",
		Resource: "httproutes",
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
		var failures []string
		var related []string

		// Check ParentRefs (Gateways)
		if parents, ok, _ := unstructured.NestedSlice(item.Object, "spec", "parentRefs"); ok {
			for _, rawParent := range parents {
				parent, ok := rawParent.(map[string]interface{})
				if !ok {
					continue
				}
				pName, _ := parent["name"].(string)
				if pName == "" {
					continue
				}
				pNS := item.GetNamespace()
				if ns, ok := parent["namespace"].(string); ok && ns != "" {
					pNS = ns
				}

				gtwGVR := schema.GroupVersionResource{
					Group:    "gateway.networking.k8s.io",
					Version:  "v1",
					Resource: "gateways",
				}
				_, err := ctx.DynamicClient.Resource(gtwGVR).Namespace(pNS).Get(ctx.Context, pName, metav1.GetOptions{})
				if err != nil {
					failures = append(failures, fmt.Sprintf("HTTPRoute references non-existent Gateway: %s/%s", pNS, pName))
				}
			}
		}

		// Check backendRefs for Kubernetes Service targets.
		if rules, ok, _ := unstructured.NestedSlice(item.Object, "spec", "rules"); ok {
			for _, rawRule := range rules {
				rule, ok := rawRule.(map[string]interface{})
				if !ok {
					continue
				}
				backendRefs, ok := rule["backendRefs"].([]interface{})
				if !ok {
					continue
				}
				for _, rawBackend := range backendRefs {
					backend, ok := rawBackend.(map[string]interface{})
					if !ok {
						continue
					}
					kind, _ := backend["kind"].(string)
					if kind != "" && kind != "Service" {
						continue
					}
					name, _ := backend["name"].(string)
					if name == "" {
						continue
					}
					namespace := item.GetNamespace()
					if ns, ok := backend["namespace"].(string); ok && ns != "" {
						namespace = ns
					}
					if _, err := ctx.Client.CoreV1().Services(namespace).Get(ctx.Context, name, metav1.GetOptions{}); errors.IsNotFound(err) {
						failures = append(failures, fmt.Sprintf("HTTPRoute references non-existent backend Service: %s/%s", namespace, name))
					} else if err == nil {
						related = append(related, fmt.Sprintf("Service/%s/%s", namespace, name))
					}
				}
			}
		}

		if len(failures) > 0 {
			results = append(results, Result{
				Kind:          "HTTPRoute",
				Name:          item.GetName(),
				Namespace:     item.GetNamespace(),
				Symptom:       "HTTPRoute routing error",
				Category:      CategoryNetwork,
				LikelyCause:   "The HTTPRoute refers to missing Gateways or backends.",
				Confidence:    85,
				PatchStrategy: PatchNone,
				Evidence:      failures,
				Related:       related,
			})
		}
	}

	return results, nil
}
