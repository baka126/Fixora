package controller

import (
	"context"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/fake"
)

func TestCorrelatePodResourcesIncludesOwnersServiceAndConfig(t *testing.T) {
	replicas := int32(2)
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "api-abc",
			Namespace: "payments",
			Labels:    map[string]string{"app": "api"},
			OwnerReferences: []metav1.OwnerReference{{
				Kind: "ReplicaSet",
				Name: "api-rs",
			}},
		},
		Spec: v1.PodSpec{
			NodeName: "node-a",
			Containers: []v1.Container{{
				Name: "api",
				Env: []v1.EnvVar{{
					Name: "DATABASE_URL",
					ValueFrom: &v1.EnvVarSource{SecretKeyRef: &v1.SecretKeySelector{
						LocalObjectReference: v1.LocalObjectReference{Name: "api-secret"},
						Key:                  "database-url",
					}},
				}},
				Ports: []v1.ContainerPort{{ContainerPort: 8080}},
			}},
			Volumes: []v1.Volume{{
				Name:         "config",
				VolumeSource: v1.VolumeSource{ConfigMap: &v1.ConfigMapVolumeSource{LocalObjectReference: v1.LocalObjectReference{Name: "api-config"}}},
			}},
		},
	}
	client := fake.NewSimpleClientset(
		pod,
		&appsv1.ReplicaSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "api-rs",
				Namespace: "payments",
				OwnerReferences: []metav1.OwnerReference{{
					Kind: "Deployment",
					Name: "api",
				}},
			},
		},
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "payments"},
			Spec:       appsv1.DeploymentSpec{Replicas: &replicas},
			Status:     appsv1.DeploymentStatus{UpdatedReplicas: 1, AvailableReplicas: 1, UnavailableReplicas: 1},
		},
		&v1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "payments"},
			Spec: v1.ServiceSpec{
				Selector: map[string]string{"app": "api"},
				Ports:    []v1.ServicePort{{Port: 80, TargetPort: intstr.FromInt(8080)}},
			},
		},
		&v1.Endpoints{
			ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "payments"},
			Subsets: []v1.EndpointSubset{{
				Addresses: []v1.EndpointAddress{{TargetRef: &v1.ObjectReference{Kind: "Pod", Name: "other-pod"}}},
			}},
		},
		&networkingv1.Ingress{
			ObjectMeta: metav1.ObjectMeta{Name: "api-ing", Namespace: "payments"},
			Spec: networkingv1.IngressSpec{
				Rules: []networkingv1.IngressRule{{
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{{
								Backend: networkingv1.IngressBackend{
									Service: &networkingv1.IngressServiceBackend{Name: "api"},
								},
							}},
						},
					},
				}},
			},
		},
		&v1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "api-secret", Namespace: "payments"},
			Data:       map[string][]byte{},
		},
		&v1.Node{
			ObjectMeta: metav1.ObjectMeta{Name: "node-a"},
			Status: v1.NodeStatus{Conditions: []v1.NodeCondition{{
				Type:   v1.NodeMemoryPressure,
				Status: v1.ConditionTrue,
				Reason: "KubeletHasInsufficientMemory",
			}}},
		},
		&networkingv1.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{Name: "api-net", Namespace: "payments"},
			Spec: networkingv1.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{MatchLabels: map[string]string{"app": "api"}},
			},
		},
	)
	ctrl := &Controller{clientset: client}

	corr := ctrl.correlatePodResources(context.Background(), pod)
	got := corr.Summary()
	for _, want := range []string{
		"Owner chain: Pod/api-abc -> ReplicaSet/api-rs -> Deployment/api",
		"Deployment api rollout: desired=2 updated=1 available=1 unavailable=1",
		"Service api selects this pod but endpoints do not include it",
		"Ingress api-ing routes traffic to Service api",
		"Secret api-secret is missing referenced key database-url",
		"ConfigMap api-config is referenced but missing",
		"Node node-a has MemoryPressure",
		"NetworkPolicies selecting this pod: api-net",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected summary to contain %q, got:\n%s", want, got)
		}
	}
	top := corr.TopCorrelations(3)
	if len(top) == 0 {
		t.Fatal("expected scored correlations")
	}
	if top[0].Ref != "Deployment/api" {
		t.Fatalf("top correlation = %#v, want Deployment/api", top[0])
	}
	if !strings.Contains(got, "Top correlated resources:") {
		t.Fatalf("expected scored correlation summary, got:\n%s", got)
	}
}

func TestWorkloadIdentityForPodUsesDeploymentSelector(t *testing.T) {
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "api-abc",
			Namespace: "payments",
			Labels:    map[string]string{"app": "api", "pod-template-hash": "abc"},
			OwnerReferences: []metav1.OwnerReference{{
				Kind: "ReplicaSet",
				Name: "api-rs",
			}},
		},
	}
	client := fake.NewSimpleClientset(
		pod,
		&appsv1.ReplicaSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "api-rs",
				Namespace: "payments",
				OwnerReferences: []metav1.OwnerReference{{
					Kind: "Deployment",
					Name: "api",
				}},
			},
		},
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "payments"},
			Spec: appsv1.DeploymentSpec{
				Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "api"}},
			},
		},
	)
	ctrl := &Controller{clientset: client}

	got := ctrl.workloadIdentityForPod(context.Background(), pod)
	if got.Kind != "Deployment" || got.Name != "api" || got.Selector != "app=api" {
		t.Fatalf("unexpected workload identity: %#v", got)
	}
}

func TestWorkloadIdentityInfersDeploymentWhenReplicaSetReadRaces(t *testing.T) {
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "checkout-api-7f9c9d6b6f-k2p4x",
			Namespace: "payments",
			Labels:    map[string]string{"app": "checkout-api", "pod-template-hash": "7f9c9d6b6f"},
			OwnerReferences: []metav1.OwnerReference{{
				Kind: "ReplicaSet",
				Name: "checkout-api-7f9c9d6b6f",
			}},
		},
	}
	ctrl := &Controller{clientset: fake.NewSimpleClientset(pod)}

	got := ctrl.workloadIdentityForPod(context.Background(), pod)
	if got.Kind != "Deployment" || got.Name != "checkout-api" {
		t.Fatalf("expected inferred Deployment/checkout-api, got %#v", got)
	}
}

func TestWorkloadIdentityKeepsDirectReplicaSetWithoutDeploymentHash(t *testing.T) {
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "direct-rs-pod",
			Namespace: "payments",
			Labels:    map[string]string{"app": "direct-rs"},
			OwnerReferences: []metav1.OwnerReference{{
				Kind: "ReplicaSet",
				Name: "direct-rs",
			}},
		},
	}
	ctrl := &Controller{clientset: fake.NewSimpleClientset(pod)}

	got := ctrl.workloadIdentityForPod(context.Background(), pod)
	if got.Kind != "ReplicaSet" || got.Name != "direct-rs" {
		t.Fatalf("expected direct ReplicaSet identity, got %#v", got)
	}
}

func TestWorkloadRegressionReasonUsesStoredSelectorForReplacementPods(t *testing.T) {
	replacement := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "api-new",
			Namespace: "payments",
			Labels:    map[string]string{"app": "api"},
		},
		Status: v1.PodStatus{
			Phase: v1.PodRunning,
			ContainerStatuses: []v1.ContainerStatus{{
				Name: "api",
				State: v1.ContainerState{Waiting: &v1.ContainerStateWaiting{
					Reason: "CrashLoopBackOff",
				}},
			}},
		},
	}
	ctrl := &Controller{clientset: fake.NewSimpleClientset(replacement)}

	got := ctrl.workloadRegressionReason(context.Background(), RemediationRecord{
		Namespace:        "payments",
		PodName:          "api-old",
		WorkloadKind:     "Deployment",
		WorkloadName:     "api",
		WorkloadSelector: "app=api",
	})
	if !strings.Contains(got, "CrashLoopBackOff") {
		t.Fatalf("expected replacement pod crash to be detected, got %q", got)
	}
}
