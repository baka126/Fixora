package controller

import (
	"context"
	"strings"
	"testing"

	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/fake"
)

func TestProbeCorrectionSuggestionUsesListenerHintsAndIngressImpact(t *testing.T) {
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "api-1",
			Namespace: "checkout",
			Labels:    map[string]string{"app": "api"},
		},
		Spec: v1.PodSpec{Containers: []v1.Container{{
			Name: "api",
			ReadinessProbe: &v1.Probe{ProbeHandler: v1.ProbeHandler{HTTPGet: &v1.HTTPGetAction{
				Path: "/healthz",
				Port: intstr.FromInt(8080),
			}}},
		}}},
	}
	ctrl := &Controller{clientset: fake.NewSimpleClientset(
		pod,
		&v1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "checkout"},
			Spec: v1.ServiceSpec{
				Selector: map[string]string{"app": "api"},
				Ports:    []v1.ServicePort{{Name: "http", Port: 80, TargetPort: intstr.FromInt(9090)}},
			},
		},
		&v1.Endpoints{ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "checkout"}},
		&networkingv1.Ingress{
			ObjectMeta: metav1.ObjectMeta{Name: "api-ing", Namespace: "checkout"},
			Spec: networkingv1.IngressSpec{Rules: []networkingv1.IngressRule{{
				IngressRuleValue: networkingv1.IngressRuleValue{HTTP: &networkingv1.HTTPIngressRuleValue{
					Paths: []networkingv1.HTTPIngressPath{{
						Backend: networkingv1.IngressBackend{Service: &networkingv1.IngressServiceBackend{Name: "api"}},
					}},
				}},
			}}},
		},
	)}

	suggestion, ok := ctrl.probeCorrectionSuggestion(context.Background(), pod, Diagnosis{
		Symptom:       "Health probe is failing",
		PatchStrategy: PatchProbe,
	}, CollectedEvidence{
		EventTimeline: "Readiness probe failed: HTTP probe failed with statuscode: 404",
		Logs:          "HTTP server listening on port 9090; registered health endpoint /readyz",
	})
	if !ok {
		t.Fatal("expected probe correction suggestion")
	}
	if suggestion.Path != "/readyz" || suggestion.Port != "9090" {
		t.Fatalf("suggestion path/port = %s/%s, want /readyz/9090", suggestion.Path, suggestion.Port)
	}
	if got := strings.Join(suggestion.ImpactedRoutes, ","); !strings.Contains(got, "Service/api") || !strings.Contains(got, "Ingress/api-ing") {
		t.Fatalf("impact = %s, want Service/api and Ingress/api-ing", got)
	}

	diagnosis := Diagnosis{PatchStrategy: PatchNone, Confidence: 20}
	var corr ResourceCorrelation
	applyProbeCorrectionSuggestion(&diagnosis, &corr, suggestion)
	if diagnosis.PatchStrategy != PatchProbe || diagnosis.Confidence < 86 {
		t.Fatalf("diagnosis strategy/confidence = %s/%d, want probe >=86", diagnosis.PatchStrategy, diagnosis.Confidence)
	}
	if !strings.Contains(corr.Summary(), "Probe correction candidate") {
		t.Fatalf("expected probe correction correlation, got:\n%s", corr.Summary())
	}
}

func TestProbeCorrectionSuggestionSkipsWhenProbeAlreadyMatches(t *testing.T) {
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "api-1", Namespace: "checkout", Labels: map[string]string{"app": "api"}},
		Spec: v1.PodSpec{Containers: []v1.Container{{
			Name: "api",
			ReadinessProbe: &v1.Probe{ProbeHandler: v1.ProbeHandler{HTTPGet: &v1.HTTPGetAction{
				Path: "/readyz",
				Port: intstr.FromInt(9090),
			}}},
		}}},
	}
	ctrl := &Controller{clientset: fake.NewSimpleClientset(
		pod,
		&v1.Service{ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "checkout"}, Spec: v1.ServiceSpec{Selector: map[string]string{"app": "api"}}},
		&v1.Endpoints{ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "checkout"}},
	)}

	_, ok := ctrl.probeCorrectionSuggestion(context.Background(), pod, Diagnosis{PatchStrategy: PatchProbe}, CollectedEvidence{
		EventTimeline: "Readiness probe failed",
		Logs:          "HTTP server listening on port 9090; registered health endpoint /readyz",
	})
	if ok {
		t.Fatal("did not expect suggestion when probe already matches listener hints")
	}
}
