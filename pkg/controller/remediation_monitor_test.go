package controller

import (
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestFluxObjectReadyReportsFailure(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]interface{}{
		"status": map[string]interface{}{
			"conditions": []interface{}{
				map[string]interface{}{
					"type":    "Ready",
					"status":  "False",
					"reason":  "BuildFailed",
					"message": "kustomize build failed",
				},
			},
		},
	}}

	ready, failure := fluxObjectReady(obj, "payments")
	if !ready || !strings.Contains(failure, "BuildFailed") {
		t.Fatalf("expected ready-for-observation failure, got ready=%v failure=%q", ready, failure)
	}
}

func TestFluxObjectReadyWaitsWhenReadyUnknown(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]interface{}{
		"status": map[string]interface{}{
			"conditions": []interface{}{
				map[string]interface{}{"type": "Ready", "status": "Unknown"},
			},
		},
	}}

	ready, failure := fluxObjectReady(obj, "payments")
	if ready || failure != "" {
		t.Fatalf("expected monitor to keep waiting, got ready=%v failure=%q", ready, failure)
	}
}
