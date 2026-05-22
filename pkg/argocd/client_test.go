package argocd

import (
	"strings"
	"testing"
)

func TestExtractMatchIncludesApplicationStatus(t *testing.T) {
	client := New(nil, "argocd", "", "")
	app := map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":      "checkout-api",
			"namespace": "argocd",
		},
		"spec": map[string]interface{}{
			"destination": map[string]interface{}{"namespace": "checkout"},
			"source": map[string]interface{}{
				"repoURL":        "https://github.com/acme/platform.git",
				"path":           "overlays/prod/us-east-1/checkout",
				"targetRevision": "main",
			},
		},
		"status": map[string]interface{}{
			"health": map[string]interface{}{"status": "Degraded"},
			"sync": map[string]interface{}{
				"status":   "OutOfSync",
				"revision": "abc123",
			},
			"operationState": map[string]interface{}{"phase": "Running"},
			"resources": []interface{}{
				map[string]interface{}{"kind": "Deployment", "name": "api", "namespace": "checkout"},
				map[string]interface{}{"kind": "Service", "name": "api", "namespace": "checkout"},
			},
		},
	}

	info := client.extractMatch(app, "checkout", "api", "Deployment")
	if info == nil {
		t.Fatal("expected ArgoCD application match")
	}
	if info.Name != "checkout-api" || info.HealthStatus != "Degraded" || info.SyncStatus != "OutOfSync" || info.Revision != "abc123" || info.ResourceCount != 2 {
		t.Fatalf("unexpected app info: %#v", info)
	}
	summary := info.Summary()
	for _, want := range []string{"app=argocd/checkout-api", "health=Degraded", "sync=OutOfSync", "liveRevision=abc123", "resources=2"} {
		if !strings.Contains(summary, want) {
			t.Fatalf("summary %q missing %q", summary, want)
		}
	}
}
