package controller

import (
	"context"
	"strings"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestClassifyPodIssueImagePull(t *testing.T) {
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "api-1", Namespace: "default"},
		Spec: v1.PodSpec{Containers: []v1.Container{
			{Name: "api", Image: "registry.example.com/api:missing"},
		}},
		Status: v1.PodStatus{
			Phase: v1.PodPending,
			ContainerStatuses: []v1.ContainerStatus{{
				Name: "api",
				State: v1.ContainerState{Waiting: &v1.ContainerStateWaiting{
					Reason:  "ImagePullBackOff",
					Message: "Back-off pulling image",
				}},
			}},
		},
	}
	event := podEvent(pod, "Failed", `Failed to pull image "registry.example.com/api:missing": manifest unknown`)
	ctrl := &Controller{clientset: fake.NewSimpleClientset(pod, event)}

	got := ctrl.classifyPodIssue(context.Background(), pod, "ImagePullBackOff")

	if got.Category != CategoryRollout || got.PatchStrategy != PatchImage {
		t.Fatalf("got category=%s strategy=%s, want rollout/%s", got.Category, got.PatchStrategy, PatchImage)
	}
	if got.Confidence < 80 {
		t.Fatalf("confidence too low: %d", got.Confidence)
	}
}

func TestClassifyPodIssueMissingSecret(t *testing.T) {
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "worker-1", Namespace: "default"},
		Spec: v1.PodSpec{Containers: []v1.Container{{
			Name: "worker",
			EnvFrom: []v1.EnvFromSource{{
				SecretRef: &v1.SecretEnvSource{LocalObjectReference: v1.LocalObjectReference{Name: "worker-config"}},
			}},
		}}},
		Status: v1.PodStatus{
			Phase: v1.PodPending,
			ContainerStatuses: []v1.ContainerStatus{{
				Name: "worker",
				State: v1.ContainerState{Waiting: &v1.ContainerStateWaiting{
					Reason:  "CreateContainerConfigError",
					Message: `secret "worker-config" not found`,
				}},
			}},
		},
	}
	event := podEvent(pod, "Failed", `Error: secret "worker-config" not found`)
	ctrl := &Controller{clientset: fake.NewSimpleClientset(pod, event)}

	got := ctrl.classifyPodIssue(context.Background(), pod, "CreateContainerConfigError")

	if got.Category != CategoryConfig || got.PatchStrategy != PatchEnvOrVolumeRef {
		t.Fatalf("got category=%s strategy=%s, want config/%s", got.Category, got.PatchStrategy, PatchEnvOrVolumeRef)
	}
	if !strings.Contains(strings.Join(got.Related, ","), "Secret/worker-config") {
		t.Fatalf("expected related secret, got %v", got.Related)
	}
}

func TestClassifyPodIssueScheduling(t *testing.T) {
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "job-1", Namespace: "default"},
		Spec: v1.PodSpec{
			NodeSelector: map[string]string{"workload": "gpu"},
			Containers:   []v1.Container{{Name: "job", Image: "example/job:1"}},
		},
		Status: v1.PodStatus{
			Phase: v1.PodPending,
			Conditions: []v1.PodCondition{{
				Type:   v1.PodScheduled,
				Status: v1.ConditionFalse,
				Reason: "Unschedulable",
			}},
		},
	}
	event := podEvent(pod, "FailedScheduling", "0/3 nodes are available: 3 node(s) did not match Pod's node affinity/selector.")
	ctrl := &Controller{clientset: fake.NewSimpleClientset(pod, event)}

	got := ctrl.classifyPodIssue(context.Background(), pod, "Pending (Unschedulable)")

	if got.Category != CategoryScheduling || got.PatchStrategy != PatchSchedulingPolicy {
		t.Fatalf("got category=%s strategy=%s, want scheduling/%s", got.Category, got.PatchStrategy, PatchSchedulingPolicy)
	}
	if !strings.Contains(strings.Join(got.Evidence, "\n"), "Node selector") {
		t.Fatalf("expected node selector evidence, got %v", got.Evidence)
	}
}

func TestClassifyPodIssueServiceEndpointMismatch(t *testing.T) {
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "api-1",
			Namespace: "default",
			Labels:    map[string]string{"app": "api"},
		},
		Spec:   v1.PodSpec{Containers: []v1.Container{{Name: "api", Image: "example/api:1"}}},
		Status: v1.PodStatus{Phase: v1.PodRunning},
	}
	svc := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "default"},
		Spec:       v1.ServiceSpec{Selector: map[string]string{"app": "api"}},
	}
	endpoints := &v1.Endpoints{ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "default"}}
	ctrl := &Controller{clientset: fake.NewSimpleClientset(pod, svc, endpoints)}

	got := ctrl.classifyPodIssue(context.Background(), pod, "manual")

	if got.Category != CategoryNetwork || got.PatchStrategy != PatchServiceSelector {
		t.Fatalf("got category=%s strategy=%s, want network/%s", got.Category, got.PatchStrategy, PatchServiceSelector)
	}
}

func podEvent(pod *v1.Pod, reason, message string) *v1.Event {
	return &v1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pod.Name + "." + reason,
			Namespace: pod.Namespace,
		},
		InvolvedObject: v1.ObjectReference{
			Kind:      "Pod",
			Namespace: pod.Namespace,
			Name:      pod.Name,
		},
		Reason:  reason,
		Message: message,
	}
}
