package controller

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestBuildWorldSnapshotMapsControllerServiceIngressAndNode(t *testing.T) {
	replicas := int32(2)
	ctrl := &Controller{
		clientset: fake.NewSimpleClientset(
			&v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node-a",
					Labels: map[string]string{
						"node.kubernetes.io/instance-type": "m6i.large",
						"topology.kubernetes.io/region":    "us-west-2",
						"topology.kubernetes.io/zone":      "us-west-2a",
						"alpha.eksctl.io/cluster-name":     "prod-us-west-2",
					},
				},
				Spec: v1.NodeSpec{ProviderID: "aws:///us-west-2a/i-123"},
				Status: v1.NodeStatus{Conditions: []v1.NodeCondition{{
					Type:   v1.NodeReady,
					Status: v1.ConditionTrue,
				}}},
			},
			&appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "checkout"},
				Spec: appsv1.DeploymentSpec{
					Replicas: &replicas,
					Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "api"}},
				},
				Status: appsv1.DeploymentStatus{ReadyReplicas: 1, AvailableReplicas: 1, UpdatedReplicas: 1},
			},
			&appsv1.ReplicaSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "api-6f7d",
					Namespace: "checkout",
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
						Name:       "api",
					}},
				},
				Spec: appsv1.ReplicaSetSpec{Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "api"}}},
			},
			&v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "api-6f7d-x9q2p",
					Namespace: "checkout",
					Labels:    map[string]string{"app": "api", "pod-template-hash": "6f7d"},
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion: "apps/v1",
						Kind:       "ReplicaSet",
						Name:       "api-6f7d",
					}},
				},
				Spec: v1.PodSpec{
					NodeName:   "node-a",
					Containers: []v1.Container{{Name: "api", Image: "example/api:v1"}},
				},
				Status: v1.PodStatus{
					Phase: v1.PodRunning,
					PodIP: "10.42.0.10",
					Conditions: []v1.PodCondition{{
						Type:   v1.PodReady,
						Status: v1.ConditionTrue,
					}},
				},
			},
			&v1.Service{
				ObjectMeta: metav1.ObjectMeta{Name: "api-svc", Namespace: "checkout"},
				Spec: v1.ServiceSpec{
					Selector: map[string]string{"app": "api"},
					Type:     v1.ServiceTypeClusterIP,
					Ports:    []v1.ServicePort{{Name: "http", Port: 80, Protocol: v1.ProtocolTCP}},
				},
			},
			&networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{Name: "api-ing", Namespace: "checkout"},
				Spec: networkingv1.IngressSpec{Rules: []networkingv1.IngressRule{{
					Host: "api.example.test",
					IngressRuleValue: networkingv1.IngressRuleValue{HTTP: &networkingv1.HTTPIngressRuleValue{
						Paths: []networkingv1.HTTPIngressPath{{
							Path: "/",
							Backend: networkingv1.IngressBackend{Service: &networkingv1.IngressServiceBackend{
								Name: "api-svc",
								Port: networkingv1.ServiceBackendPort{Number: 80},
							}},
						}},
					}},
				}}},
			},
		),
	}

	world, _ := ctrl.BuildWorldSnapshot(context.Background())
	workloadID := worldID("prod-us-west-2", "checkout", "Deployment", "api")
	workload := world.Workloads[workloadID]
	if workload == nil {
		t.Fatalf("expected Deployment/api workload in world snapshot")
	}
	if workload.Ready != 1 || workload.Desired != 2 {
		t.Fatalf("workload status = desired %d ready %d, want 2/1", workload.Desired, workload.Ready)
	}
	if len(workload.Pods) != 1 || len(workload.Services) != 1 || len(workload.Ingresses) != 1 {
		t.Fatalf("workload related resources = pods %v services %v ingresses %v", workload.Pods, workload.Services, workload.Ingresses)
	}
	node := world.Nodes[worldNodeID("node-a")]
	if node == nil || node.Region != "us-west-2" || node.InstanceType != "m6i.large" || !node.Ready {
		t.Fatalf("node metadata = %#v, want ready us-west-2 m6i.large", node)
	}
	if len(world.Edges) == 0 {
		t.Fatalf("expected world edges")
	}
}

func TestWorldDashboardGraphUsesLiveSnapshot(t *testing.T) {
	world := &WorldSnapshot{
		Cluster:   "prod",
		Workloads: map[string]*WorldWorkload{},
		Pods:      map[string]*WorldPod{},
		Services:  map[string]*WorldService{},
		Ingresses: map[string]*WorldIngress{},
		Nodes:     map[string]*WorldNode{},
	}
	workloadID := worldID("prod", "checkout", "Deployment", "api")
	podID := worldID("prod", "checkout", "Pod", "api-123")
	serviceID := worldID("prod", "checkout", "Service", "api-svc")
	ingressID := worldID("prod", "checkout", "Ingress", "api-ing")
	nodeID := worldNodeID("node-a")
	world.Workloads[workloadID] = &WorldWorkload{
		ID:        workloadID,
		Namespace: "checkout",
		Kind:      "Deployment",
		Name:      "api",
		Pods:      []string{podID},
		Services:  []string{serviceID},
		Ingresses: []string{ingressID},
	}
	world.Pods[podID] = &WorldPod{ID: podID, Namespace: "checkout", Name: "api-123", Phase: "Running", NodeName: "node-a", WorkloadID: workloadID}
	world.Services[serviceID] = &WorldService{ID: serviceID, Namespace: "checkout", Name: "api-svc", Ports: []string{"http:80/TCP"}, WorkloadIDs: []string{workloadID}, IngressIDs: []string{ingressID}}
	world.Ingresses[ingressID] = &WorldIngress{ID: ingressID, Namespace: "checkout", Name: "api-ing", Hosts: []string{"api.example.test"}, ServiceIDs: []string{serviceID}, WorkloadIDs: []string{workloadID}}
	world.Nodes[nodeID] = &WorldNode{ID: nodeID, Name: "node-a", Region: "us-west-2", InstanceType: "m6i.large"}

	nodes, edges, ok := worldDashboardGraph(world, "checkout", "api-123", DashboardWorkload{Kind: "Deployment", Name: "api", Namespace: "checkout"})
	if !ok {
		t.Fatalf("expected world graph")
	}
	if len(nodes) != 5 {
		t.Fatalf("node count = %d, want 5: %#v", len(nodes), nodes)
	}
	if len(edges) != 4 {
		t.Fatalf("edge count = %d, want 4: %#v", len(edges), edges)
	}
}

func TestWorldDashboardGraphPromotesHelmReleaseRoot(t *testing.T) {
	world := &WorldSnapshot{
		Cluster:   "prod",
		Workloads: map[string]*WorldWorkload{},
		Pods:      map[string]*WorldPod{},
		Services:  map[string]*WorldService{},
		Ingresses: map[string]*WorldIngress{},
		Nodes:     map[string]*WorldNode{},
	}
	workloadID := worldID("prod", "checkout", "Deployment", "api")
	podID := worldID("prod", "checkout", "Pod", "api-123")
	world.Workloads[workloadID] = &WorldWorkload{
		ID:        workloadID,
		Namespace: "checkout",
		Kind:      "Deployment",
		Name:      "api",
		Pods:      []string{podID},
		Labels: map[string]string{
			"app.kubernetes.io/managed-by": "Helm",
			"app.kubernetes.io/instance":   "checkout-api",
			"helm.sh/chart":                "api-1.2.3",
		},
		Annotations: map[string]string{
			"meta.helm.sh/release-name":      "checkout-api",
			"meta.helm.sh/release-namespace": "checkout",
		},
	}
	world.Pods[podID] = &WorldPod{ID: podID, Namespace: "checkout", Name: "api-123", Phase: "Running", WorkloadID: workloadID}

	nodes, edges, ok := worldDashboardGraph(world, "checkout", "api-123", DashboardWorkload{Kind: "Deployment", Name: "api", Namespace: "checkout"})
	if !ok {
		t.Fatalf("expected world graph")
	}
	var helmReleaseID, chartID string
	for _, node := range nodes {
		switch node.Label {
		case "HelmRelease":
			helmReleaseID = node.ID
			if node.Detail != "checkout-api" {
				t.Fatalf("HelmRelease detail = %q, want checkout-api", node.Detail)
			}
		case "Helm Chart":
			chartID = node.ID
			if node.Detail != "api-1.2.3" {
				t.Fatalf("Helm Chart detail = %q, want api-1.2.3", node.Detail)
			}
		}
	}
	if helmReleaseID == "" || chartID == "" {
		t.Fatalf("expected HelmRelease and Helm Chart nodes, got %#v", nodes)
	}
	if !dashboardGraphHasEdge(edges, helmReleaseID, chartID) || !dashboardGraphHasEdge(edges, chartID, "workload") {
		t.Fatalf("expected HelmRelease -> Helm Chart -> workload edges, got %#v", edges)
	}
}

func dashboardGraphHasEdge(edges [][2]string, from, to string) bool {
	for _, edge := range edges {
		if edge[0] == from && edge[1] == to {
			return true
		}
	}
	return false
}

func TestIngressServiceNamesIncludesDefaultAndRuleBackends(t *testing.T) {
	ingress := networkingv1.Ingress{Spec: networkingv1.IngressSpec{
		DefaultBackend: &networkingv1.IngressBackend{Service: &networkingv1.IngressServiceBackend{
			Name: "default-svc",
			Port: networkingv1.ServiceBackendPort{Number: 80},
		}},
		Rules: []networkingv1.IngressRule{{
			IngressRuleValue: networkingv1.IngressRuleValue{HTTP: &networkingv1.HTTPIngressRuleValue{
				Paths: []networkingv1.HTTPIngressPath{{
					Backend: networkingv1.IngressBackend{Service: &networkingv1.IngressServiceBackend{
						Name: "api-svc",
						Port: networkingv1.ServiceBackendPort{Number: 80},
					}},
				}},
			}},
		}},
	}}

	got := ingressServiceNames(ingress)
	want := []string{"api-svc", "default-svc"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("ingressServiceNames() = %v, want %v", got, want)
	}
}
