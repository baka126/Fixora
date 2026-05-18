package analyzer

import (
	"context"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestPodAnalyzerUsesFieldSelectorForTargetPod(t *testing.T) {
	target := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "api-1", Namespace: "default"},
		Spec:       v1.PodSpec{Containers: []v1.Container{{Name: "api", Image: "registry.example.com/api:missing"}}},
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
	other := &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "worker-1", Namespace: "default"}}
	client := fake.NewSimpleClientset(target, other)

	results, err := (&PodAnalyzer{}).Analyze(Context{
		Client:        client,
		Context:       context.Background(),
		Namespace:     "default",
		LabelSelector: "metadata.name=api-1",
	})
	if err != nil {
		t.Fatalf("analyze failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected one targeted result, got %d", len(results))
	}
	if results[0].Name != "api-1" || results[0].PatchStrategy != PatchImage {
		t.Fatalf("unexpected result: %+v", results[0])
	}
}
