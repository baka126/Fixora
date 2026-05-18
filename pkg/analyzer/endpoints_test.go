package analyzer

import (
	"context"
	"testing"

	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestEndpointSliceAnalyzerFlagsNoReadyEndpoints(t *testing.T) {
	notReady := false
	client := fake.NewSimpleClientset(&discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "api-abc",
			Namespace: "default",
			Labels:    map[string]string{discoveryv1.LabelServiceName: "api"},
		},
		Endpoints: []discoveryv1.Endpoint{{Conditions: discoveryv1.EndpointConditions{Ready: &notReady}}},
	})
	results, err := (&EndpointSliceAnalyzer{}).Analyze(Context{Client: client, Context: context.Background(), Namespace: "default"})
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if len(results) != 1 || results[0].Name != "api" {
		t.Fatalf("results = %#v, want service api finding", results)
	}
}
