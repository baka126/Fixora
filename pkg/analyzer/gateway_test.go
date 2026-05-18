package analyzer

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

func TestGatewayAnalyzerHandlesPartialObjects(t *testing.T) {
	gateway := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "gateway.networking.k8s.io/v1",
		"kind":       "Gateway",
		"metadata": map[string]interface{}{
			"name":      "edge",
			"namespace": "default",
		},
	}}
	gateway.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "gateway.networking.k8s.io",
		Version: "v1",
		Kind:    "Gateway",
	})
	dynamicClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{
		{Group: "gateway.networking.k8s.io", Version: "v1", Resource: "gateways"}: "GatewayList",
	}, gateway)

	results, err := (&GatewayAnalyzer{}).Analyze(Context{
		DynamicClient: dynamicClient,
		Context:       context.Background(),
		Namespace:     "default",
	})
	if err != nil {
		t.Fatalf("analyze failed: %v", err)
	}
	if len(results) > 0 && (results[0].Kind != "Gateway" || results[0].Category != CategoryNetwork) {
		t.Fatalf("unexpected result: %+v", results[0])
	}
}
