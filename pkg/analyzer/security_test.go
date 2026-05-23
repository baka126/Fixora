package analyzer

import (
	"context"
	"strings"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestSecurityAnalyzerDetectsWritablePathPermissionDenied(t *testing.T) {
	readOnly := true
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "api-1", Namespace: "default"},
		Spec: v1.PodSpec{Containers: []v1.Container{{
			Name: "api",
			SecurityContext: &v1.SecurityContext{
				ReadOnlyRootFilesystem: &readOnly,
			},
		}}},
	}
	results, err := (&SecurityAnalyzer{}).Analyze(Context{
		Client:        fake.NewSimpleClientset(pod),
		Context:       context.Background(),
		Namespace:     "default",
		LabelSelector: "metadata.name=api-1",
		Logs:          `mkdir /tmp/app-cache: permission denied`,
	})
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("results = %d, want 1", len(results))
	}
	got := results[0]
	if got.PatchStrategy != PatchSecurityContext {
		t.Fatalf("strategy = %s, want %s", got.PatchStrategy, PatchSecurityContext)
	}
	if !strings.Contains(strings.Join(got.Evidence, "\n"), "/tmp") {
		t.Fatalf("expected /tmp evidence, got %v", got.Evidence)
	}
}

func TestSecurityAnalyzerDetectsNonRootRuntimeExpectation(t *testing.T) {
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "api-1", Namespace: "default"},
		Spec:       v1.PodSpec{Containers: []v1.Container{{Name: "api"}}},
	}
	results, err := (&SecurityAnalyzer{}).Analyze(Context{
		Client:        fake.NewSimpleClientset(pod),
		Context:       context.Background(),
		Namespace:     "default",
		LabelSelector: "metadata.name=api-1",
		Logs:          `fatal: refusing to run as root, configure run as non-root`,
	})
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("results = %d, want 1", len(results))
	}
	if results[0].PatchStrategy != PatchSecurityContext {
		t.Fatalf("strategy = %s, want %s", results[0].PatchStrategy, PatchSecurityContext)
	}
}
