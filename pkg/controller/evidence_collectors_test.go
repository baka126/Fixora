package controller

import (
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestExtractStackTrace(t *testing.T) {
	logs := "starting\npanic: crash\nstack trace:\nline 1\nline 2\n"
	got := extractStackTrace(logs)
	if got == "" || got == logs {
		t.Fatalf("unexpected stack trace extraction: %q", got)
	}
}

func TestHelmReleaseForPodPrefersAnnotations(t *testing.T) {
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "workloads",
			Annotations: map[string]string{
				"meta.helm.sh/release-name":      "checkout",
				"meta.helm.sh/release-namespace": "prod",
			},
			Labels: map[string]string{
				"app.kubernetes.io/instance": "fallback",
			},
		},
	}

	name, namespace := helmReleaseForPod(pod)
	if name != "checkout" || namespace != "prod" {
		t.Fatalf("release = %s/%s, want prod/checkout", namespace, name)
	}
}

func TestHelmReleaseForPodFallsBackToInstanceLabel(t *testing.T) {
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Labels: map[string]string{
				"app.kubernetes.io/instance": "redis",
			},
		},
	}

	name, namespace := helmReleaseForPod(pod)
	if name != "redis" || namespace != "default" {
		t.Fatalf("release = %s/%s, want default/redis", namespace, name)
	}
}
