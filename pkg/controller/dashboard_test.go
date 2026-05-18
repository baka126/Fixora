package controller

import (
	"context"
	"testing"
	"time"

	"fixora/pkg/alertmanager"
	"fixora/pkg/config"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestDashboardGuardrailsShowRenderPassedForRecordedPendingApproval(t *testing.T) {
	guardrails := dashboardGuardrails(dashboardRemediationRow{
		ID:     42,
		Status: "pending_approval",
	})

	got := guardrailStatus(guardrails, "Render validation")
	if got != "passed" {
		t.Fatalf("Render validation status = %q, want passed", got)
	}

	got = guardrailStatus(guardrails, "Remediation state")
	if got != "pending" {
		t.Fatalf("Remediation state status = %q, want pending", got)
	}

	got = guardrailStatus(guardrails, "Semantic target check")
	if got != "passed" {
		t.Fatalf("Semantic target check status = %q, want passed", got)
	}
}

func TestDashboardGuardrailsSurfaceValidationFailure(t *testing.T) {
	guardrails := dashboardGuardrails(dashboardRemediationRow{
		ID:            42,
		Status:        "pr_failed",
		FailureReason: "render sandbox validation failed: helm template error",
	})

	got := guardrailStatus(guardrails, "Render validation")
	if got != "failed" {
		t.Fatalf("Render validation status = %q, want failed", got)
	}
}

func TestDashboardGuardrailsSurfaceSemanticValidationFailure(t *testing.T) {
	guardrails := dashboardGuardrails(dashboardRemediationRow{
		ID:            42,
		Status:        "pr_failed",
		FailureReason: "semantic render validation failed: rendered Deployment/api did not change expected image fields",
	})

	got := guardrailStatus(guardrails, "Semantic target check")
	if got != "failed" {
		t.Fatalf("Semantic target check status = %q, want failed", got)
	}
}

func TestDashboardGuardrailsSurfacePolicyFailure(t *testing.T) {
	guardrails := dashboardGuardrails(dashboardRemediationRow{
		ID:            42,
		Status:        "pr_failed",
		FailureReason: "policy guardrail rejected remediation patch: touches RBAC manifests",
	})

	got := guardrailStatus(guardrails, "Privileged paths blocked")
	if got != "failed" {
		t.Fatalf("Privileged paths blocked status = %q, want failed", got)
	}
}

func TestDashboardWorkloadPrefersOwnerFromContext(t *testing.T) {
	got := dashboardWorkload(dashboardInvestigationRow{
		PodName: "api-abc",
		ClusterContext: `Related Resources:
- Service/api
- Deployment/api`,
	}, dashboardRemediationRow{})

	if got.Kind != "Deployment" || got.Name != "api" || got.PodName != "api-abc" {
		t.Fatalf("dashboardWorkload() = %#v, want Deployment/api with pod api-abc", got)
	}
}

func TestFallbackDashboardGraphUsesRelatedResources(t *testing.T) {
	nodes, edges := fallbackDashboardGraph(DashboardWorkload{Kind: "Deployment", Name: "api"}, `Related Resources:
- ConfigMap/api-env
- Secret/api-creds
- Service/api-svc
- Ingress/api-ing`)

	if len(nodes) != 5 {
		t.Fatalf("node count = %d, want 5: %#v", len(nodes), nodes)
	}
	if len(edges) != 4 {
		t.Fatalf("edge count = %d, want 4: %#v", len(edges), edges)
	}
	if edges[3][0] == "workload" {
		t.Fatalf("expected ingress edge to hang from service, got %#v", edges[3])
	}
}

func TestDashboardActiveAlertDecisionUsesPodAlert(t *testing.T) {
	cfg := &config.Config{AlertmanagerEnabled: true}
	ctrl := &Controller{config: cfg, history: newHistoryCache(cfg)}
	alert := alertmanager.Alert{
		Labels: map[string]string{
			"alertname": "KubePodCrashLooping",
			"namespace": "payments",
			"pod":       "api-123",
			"severity":  "critical",
		},
		StartsAt: time.Now().Add(-5 * time.Minute),
	}
	alert.Status.State = "firing"

	got := ctrl.dashboardActiveAlertDecision(context.Background(), alert)
	if !got.Used || got.Decision != "used" {
		t.Fatalf("decision = used:%v %q, want used", got.Used, got.Decision)
	}
	if got.ResourceKind != "Pod" || got.ResourceName != "api-123" {
		t.Fatalf("resource = %s/%s, want Pod/api-123", got.ResourceKind, got.ResourceName)
	}
}

func TestDashboardActiveAlertDecisionExplainsMissingPod(t *testing.T) {
	cfg := &config.Config{AlertmanagerEnabled: true}
	ctrl := &Controller{config: cfg, history: newHistoryCache(cfg)}
	alert := alertmanager.Alert{
		Labels: map[string]string{
			"alertname":  "KubeDeploymentReplicasMismatch",
			"namespace":  "payments",
			"deployment": "api",
			"severity":   "warning",
		},
		StartsAt: time.Now().Add(-5 * time.Minute),
	}
	alert.Status.State = "firing"

	got := ctrl.dashboardActiveAlertDecision(context.Background(), alert)
	if got.Used || got.Decision != "skipped" {
		t.Fatalf("decision = used:%v %q, want skipped", got.Used, got.Decision)
	}
	if got.ResourceKind != "Deployment" || got.ResourceName != "api" {
		t.Fatalf("resource = %s/%s, want Deployment/api", got.ResourceKind, got.ResourceName)
	}
	if got.Reason == "" {
		t.Fatalf("expected skipped reason")
	}
}

func TestRuntimeWatchedAlertBypassesConfiguredIncludeFilter(t *testing.T) {
	cfg := &config.Config{
		AlertmanagerEnabled:       true,
		AlertmanagerIncludeLabels: map[string]string{"fixora": "true"},
	}
	alert := alertmanager.Alert{
		Labels: map[string]string{
			"alertname": "KubePodCrashLooping",
			"namespace": "payments",
			"pod":       "api-123",
		},
	}
	ctrl := &Controller{config: cfg, history: newHistoryCache(cfg), alertWatches: map[string]time.Time{alertWatchKey(alert): time.Now()}}

	if !ctrl.matchesAlertFilters(alert) {
		t.Fatalf("runtime watched alert should bypass configured include filters")
	}
}

func TestDashboardEnvironmentUsesNodeClusterLabels(t *testing.T) {
	ctrl := &Controller{
		clientset: fake.NewSimpleClientset(&v1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "node-a",
				Labels: map[string]string{
					"alpha.eksctl.io/cluster-name": "prod-us-west-2",
				},
			},
		}),
	}

	got := ctrl.dashboardEnvironment(context.Background())
	if got != "prod-us-west-2" {
		t.Fatalf("dashboardEnvironment() = %q, want prod-us-west-2", got)
	}
}

func TestDashboardEnvironmentUsesClusterInfoConfigMap(t *testing.T) {
	ctrl := &Controller{
		clientset: fake.NewSimpleClientset(&v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: "cluster-info", Namespace: "kube-public"},
			Data: map[string]string{
				"kubeconfig": "apiVersion: v1\nclusters:\n- name: prod-eu-1\n  cluster:\n    server: https://example.invalid\n",
			},
		}),
	}

	got := ctrl.dashboardEnvironment(context.Background())
	if got != "prod-eu-1" {
		t.Fatalf("dashboardEnvironment() = %q, want prod-eu-1", got)
	}
}

func TestCalculateClusterCostSnapshotReturnsNodesWithoutPricingProvider(t *testing.T) {
	ctrl := &Controller{
		clientset: fake.NewSimpleClientset(&v1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "ip-10-0-0-1",
				Labels: map[string]string{
					"node.kubernetes.io/instance-type": "m6i.large",
					"topology.kubernetes.io/region":    "us-west-2",
				},
			},
			Spec: v1.NodeSpec{ProviderID: "aws:///us-west-2a/i-123"},
		}),
	}

	total, active, rows := ctrl.calculateClusterCostSnapshot(context.Background())
	if total != 0 || active != 1 || len(rows) != 1 {
		t.Fatalf("calculateClusterCostSnapshot() = total %.2f active %d rows %d, want 0/1/1", total, active, len(rows))
	}
	if rows[0].Status != "pricing_not_configured" || rows[0].Region != "us-west-2" || rows[0].InstanceType != "m6i.large" {
		t.Fatalf("node row = %#v, want metadata with pricing_not_configured", rows[0])
	}
}

func guardrailStatus(guardrails []DashboardGuardrail, label string) string {
	for _, guardrail := range guardrails {
		if guardrail.Label == label {
			return guardrail.Status
		}
	}
	return ""
}
