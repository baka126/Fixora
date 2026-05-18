package controller

import (
	"context"
	"strings"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestIncidentAnalyzerOrchestrationFiltersUnrelatedFindings(t *testing.T) {
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "api-1",
			Namespace: "default",
			Labels:    map[string]string{"app": "api"},
		},
		Spec:   v1.PodSpec{Containers: []v1.Container{{Name: "api", Image: "example/api:1"}}},
		Status: v1.PodStatus{Phase: v1.PodRunning},
	}
	apiSvc := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "default"},
		Spec:       v1.ServiceSpec{Selector: map[string]string{"app": "api"}},
	}
	otherSvc := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "unrelated", Namespace: "default"},
		Spec:       v1.ServiceSpec{Selector: map[string]string{"app": "other"}},
	}
	apiEndpoints := &v1.Endpoints{ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "default"}}
	otherEndpoints := &v1.Endpoints{ObjectMeta: metav1.ObjectMeta{Name: "unrelated", Namespace: "default"}}
	ctrl := &Controller{clientset: fake.NewSimpleClientset(pod, apiSvc, otherSvc, apiEndpoints, otherEndpoints)}

	corr := ctrl.correlatePodResources(context.Background(), pod)
	primary := Diagnosis{Symptom: "manual endpoint check", Category: CategoryUnknown, Confidence: 10, PatchStrategy: PatchNone}
	got := ctrl.runIncidentAnalyzers(context.Background(), pod, "endpoint mismatch", primary, corr)

	if len(got.Findings) != 1 {
		t.Fatalf("expected one related finding, got %d: %+v", len(got.Findings), got.Findings)
	}
	if got.Findings[0].Kind != "Service" || got.Findings[0].Name != "api" {
		t.Fatalf("expected Service/api finding, got %+v", got.Findings[0])
	}
	if strings.Contains(got.Summary(), "unrelated") {
		t.Fatalf("expected unrelated service to be filtered, got summary:\n%s", got.Summary())
	}
}

func TestMergeAnalyzerFindingsPromotesHighConfidenceFinding(t *testing.T) {
	primary := Diagnosis{Symptom: "manual", Category: CategoryUnknown, Confidence: 10, PatchStrategy: PatchNone}
	merged := mergeAnalyzerFindings(primary, []Diagnosis{{
		Kind:          "Service",
		Name:          "api",
		Symptom:       "Service has no traffic targets",
		Category:      CategoryNetwork,
		LikelyCause:   "No pods are currently matching the service selector.",
		Confidence:    90,
		PatchStrategy: PatchServiceSelector,
	}})

	if merged.Category != CategoryNetwork || merged.PatchStrategy != PatchServiceSelector {
		t.Fatalf("expected network service-selector diagnosis, got %+v", merged)
	}
}
