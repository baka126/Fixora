package controller

import (
	"context"
	"strings"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/fake"
)

func TestDependencyEnvSuggestionFindsDatabaseServiceAndSecret(t *testing.T) {
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "api-1", Namespace: "checkout"},
		Spec: v1.PodSpec{Containers: []v1.Container{{
			Name:  "api",
			Image: "example/api:v1",
			Env: []v1.EnvVar{{
				Name:  "DB_HOST",
				Value: "localhost",
			}},
		}}},
	}
	ctrl := &Controller{clientset: fake.NewSimpleClientset(
		pod,
		&v1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "postgres",
				Namespace: "checkout",
				Labels:    map[string]string{"app.kubernetes.io/name": "postgresql"},
			},
			Spec: v1.ServiceSpec{
				Ports: []v1.ServicePort{{Name: "postgres", Port: 5432, TargetPort: intstr.FromInt(5432)}},
			},
		},
		&v1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "db-creds", Namespace: "checkout"},
			Data: map[string][]byte{
				"username": []byte("user"),
				"password": []byte("secret"),
			},
		},
	)}

	suggestion, ok := ctrl.dependencyEnvSuggestion(context.Background(), pod, Diagnosis{
		Symptom:     "CrashLoopBackOff",
		LikelyCause: "application failed to start",
	}, CollectedEvidence{
		Logs: "ACCESS_DENIED connecting to database postgres",
	})
	if !ok {
		t.Fatal("expected dependency env suggestion")
	}
	if suggestion.ServiceName != "postgres" || suggestion.ServicePort != 5432 {
		t.Fatalf("service = %s:%d, want postgres:5432", suggestion.ServiceName, suggestion.ServicePort)
	}
	if suggestion.SecretName != "db-creds" {
		t.Fatalf("secret = %q, want db-creds", suggestion.SecretName)
	}
	if got := strings.Join(suggestion.MissingEnv, ","); got != "POSTGRES_HOST,POSTGRES_PORT" {
		t.Fatalf("missing env = %s, want POSTGRES_HOST,POSTGRES_PORT", got)
	}

	diagnosis := Diagnosis{}
	var corr ResourceCorrelation
	applyDependencyEnvSuggestion(&diagnosis, &corr, suggestion)
	if diagnosis.Category != CategoryConfig || diagnosis.PatchStrategy != PatchEnvOrVolumeRef {
		t.Fatalf("diagnosis = %s/%s, want config/%s", diagnosis.Category, diagnosis.PatchStrategy, PatchEnvOrVolumeRef)
	}
	summary := corr.Summary()
	if !strings.Contains(summary, "Dependency env candidate: Service postgres exposes port 5432") {
		t.Fatalf("correlation summary missing dependency candidate:\n%s", summary)
	}
}

func TestDependencyEnvSuggestionSkipsWhenServiceEnvAlreadyMatches(t *testing.T) {
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "api-1", Namespace: "checkout"},
		Spec: v1.PodSpec{Containers: []v1.Container{{
			Name: "api",
			Env: []v1.EnvVar{
				{Name: "POSTGRES_HOST", Value: "postgres.checkout.svc.cluster.local"},
				{Name: "POSTGRES_PORT", Value: "5432"},
			},
		}}},
	}
	ctrl := &Controller{clientset: fake.NewSimpleClientset(
		pod,
		&v1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "postgres", Namespace: "checkout"},
			Spec: v1.ServiceSpec{
				Ports: []v1.ServicePort{{Name: "postgres", Port: 5432}},
			},
		},
	)}

	_, ok := ctrl.dependencyEnvSuggestion(context.Background(), pod, Diagnosis{}, CollectedEvidence{
		Logs: "database connection refused postgres",
	})
	if ok {
		t.Fatal("did not expect suggestion when DB_HOST and DB_PORT already point at the service")
	}
}
