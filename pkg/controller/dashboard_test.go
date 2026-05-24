package controller

import (
	"context"
	"strings"
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

func TestDashboardGuardrailsExplainDryRunRemediation(t *testing.T) {
	guardrails := dashboardGuardrails(dashboardRemediationRow{
		ID:     42,
		Status: "dry_run",
	})

	var detail string
	for _, guardrail := range guardrails {
		if guardrail.Label == "Remediation state" {
			detail = guardrail.Detail
			break
		}
	}
	if !strings.Contains(detail, "Dry-run mode") {
		t.Fatalf("Remediation state detail = %q, want dry-run explanation", detail)
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

func TestDashboardIncidentsSkipHealthyCurrentWorkloadsAndDedupe(t *testing.T) {
	world := &WorldSnapshot{
		Cluster: "prod",
		Workloads: map[string]*WorldWorkload{
			worldID("prod", "checkout", "Deployment", "api"): {
				Cluster:   "prod",
				Namespace: "checkout",
				Kind:      "Deployment",
				Name:      "api",
				Desired:   1,
				Ready:     1,
			},
			worldID("prod", "checkout", "Deployment", "worker"): {
				Cluster:   "prod",
				Namespace: "checkout",
				Kind:      "Deployment",
				Name:      "worker",
				Desired:   1,
				Ready:     0,
			},
		},
		Pods: map[string]*WorldPod{
			worldID("prod", "checkout", "Pod", "api-abc"): {
				Cluster:    "prod",
				Namespace:  "checkout",
				Name:       "api-abc",
				Phase:      "Running",
				Ready:      true,
				WorkloadID: worldID("prod", "checkout", "Deployment", "api"),
			},
			worldID("prod", "checkout", "Pod", "worker-abc"): {
				Cluster:    "prod",
				Namespace:  "checkout",
				Name:       "worker-abc",
				Phase:      "Pending",
				Reason:     "ImagePullBackOff",
				Ready:      false,
				WorkloadID: worldID("prod", "checkout", "Deployment", "worker"),
			},
			worldID("prod", "checkout", "Pod", "worker-def"): {
				Cluster:    "prod",
				Namespace:  "checkout",
				Name:       "worker-def",
				Phase:      "Pending",
				Reason:     "ImagePullBackOff",
				Ready:      false,
				WorkloadID: worldID("prod", "checkout", "Deployment", "worker"),
			},
		},
	}
	world.Workloads[worldID("prod", "checkout", "Deployment", "api")].Pods = []string{worldID("prod", "checkout", "Pod", "api-abc")}
	world.Workloads[worldID("prod", "checkout", "Deployment", "worker")].Pods = []string{
		worldID("prod", "checkout", "Pod", "worker-abc"),
		worldID("prod", "checkout", "Pod", "worker-def"),
	}

	got := dashboardIncidents(context.Background(), nil, world, []dashboardInvestigationRow{
		{
			ID:             3,
			Namespace:      "checkout",
			PodName:        "api-abc",
			Timestamp:      time.Now(),
			Reason:         "CrashLoopBackOff",
			ClusterContext: "Workload Kind: Deployment, Workload Name: api, Reason: CrashLoopBackOff",
		},
		{
			ID:             2,
			Namespace:      "checkout",
			PodName:        "worker-abc",
			Timestamp:      time.Now(),
			Reason:         "ImagePullBackOff",
			ClusterContext: "Workload Kind: Deployment, Workload Name: worker, Reason: ImagePullBackOff",
		},
		{
			ID:             1,
			Namespace:      "checkout",
			PodName:        "worker-def",
			Timestamp:      time.Now().Add(-time.Minute),
			Reason:         "ImagePullBackOff",
			ClusterContext: "Workload Kind: Deployment, Workload Name: worker, Reason: ImagePullBackOff",
		},
	}, nil, nil, DashboardPolicy{Mode: "Auto-fix"}, 0.99, 14.4)

	if len(got) != 1 {
		t.Fatalf("incident count = %d, want only one active deduped incident", len(got))
	}
	if got[0].Workload.Name != "worker" || got[0].Workload.PodName != "worker-abc" {
		t.Fatalf("incident = %#v, want latest worker incident", got[0].Workload)
	}
}

func TestDashboardWorkloadViewsExposeIncidentRCAAndPolicy(t *testing.T) {
	incidents := []DashboardIncident{{
		ID:          "investigation-7",
		Workload:    DashboardWorkload{Kind: "Deployment", Name: "api", Namespace: "checkout", PodName: "api-123"},
		Severity:    "critical",
		Status:      "CrashLoopBackOff",
		Evidence:    []DashboardEvidence{{Label: "Logs / stack trace", Value: "exec format error"}},
		LogPatterns: []DashboardLogPattern{{Label: "Log pattern", Pattern: "exec format error", Source: "logs", Severity: "critical", Count: 1}},
		RCA:         &DashboardRCA{Summary: "Architecture mismatch", Confidence: 91, Signal: "Logs", Risk: "Low risk"},
		PolicyState: &DashboardWorkloadPolicy{Mode: "Click-to-fix", ApprovalRequired: true, AvailabilitySLO: 0.99},
	}}
	remediations := []DashboardRemediation{{
		ID:       42,
		Status:   "pr_opened",
		Workload: DashboardWorkload{Kind: "Deployment", Name: "api", Namespace: "checkout", PodName: "api-123"},
		GitOps:   &DashboardGitOpsMapping{Controller: "ArgoCD", App: "checkout-api", Repo: "github.com/acme/platform", Path: "overlays/prod"},
	}}

	got := dashboardWorkloadViews(nil, incidents, remediations, nil, nil, nil, DashboardPolicy{Mode: "Click-to-fix"}, 0.99, 14.4)
	if len(got) != 1 {
		t.Fatalf("workload view count = %d, want 1", len(got))
	}
	if got[0].LatestIncidentID != "investigation-7" || got[0].ActiveRemediationID != 42 {
		t.Fatalf("workload ids = incident %q remediation %d, want investigation-7/42", got[0].LatestIncidentID, got[0].ActiveRemediationID)
	}
	if got[0].RCA == nil || got[0].RCA.Summary != "Architecture mismatch" {
		t.Fatalf("RCA = %#v, want Architecture mismatch", got[0].RCA)
	}
	if got[0].PolicyState == nil || got[0].PolicyState.AvailabilitySLO != 0.99 {
		t.Fatalf("policy = %#v, want availability SLO", got[0].PolicyState)
	}
	if got[0].GitOps == nil || got[0].GitOps.App != "checkout-api" {
		t.Fatalf("gitops = %#v, want checkout-api", got[0].GitOps)
	}
}

func TestDashboardWorkloadViewsCollapseHelmChildren(t *testing.T) {
	workloadID := worldID("prod", "checkout", "Deployment", "api")
	world := &WorldSnapshot{
		Cluster: "prod",
		Workloads: map[string]*WorldWorkload{
			workloadID: {
				ID:        workloadID,
				Cluster:   "prod",
				Namespace: "checkout",
				Kind:      "Deployment",
				Name:      "api",
				Status:    "desired=2 ready=1 available=1 updated=2",
				Desired:   2,
				Ready:     1,
				Labels: map[string]string{
					"app.kubernetes.io/managed-by": "Helm",
					"app.kubernetes.io/instance":   "checkout-api",
					"helm.sh/chart":                "api-1.2.3",
				},
				Annotations: map[string]string{
					"meta.helm.sh/release-name":      "checkout-api",
					"meta.helm.sh/release-namespace": "checkout",
				},
			},
		},
	}

	got := dashboardWorkloadViews(world, nil, nil, nil, nil, nil, DashboardPolicy{Mode: "Dry-run"}, 0.99, 14.4)
	if len(got) != 1 {
		t.Fatalf("workload view count = %d, want one HelmRelease row: %#v", len(got), got)
	}
	if got[0].Workload.Kind != "HelmRelease" || got[0].Workload.Name != "checkout-api" {
		t.Fatalf("workload = %#v, want HelmRelease/checkout-api", got[0].Workload)
	}
	if got[0].Helm == nil || got[0].Helm.Chart != "api" || got[0].Helm.ChartVersion != "1.2.3" {
		t.Fatalf("helm = %#v, want api@1.2.3", got[0].Helm)
	}
	if len(got[0].Children) != 1 || got[0].Children[0].Kind != "Deployment" || got[0].Children[0].Name != "api" {
		t.Fatalf("children = %#v, want Deployment/api", got[0].Children)
	}
	if got[0].Desired != 2 || got[0].Ready != 1 || got[0].Health != "degraded" {
		t.Fatalf("rollup = desired %d ready %d health %q, want 2/1/degraded", got[0].Desired, got[0].Ready, got[0].Health)
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

	total, active, rows := ctrl.calculateClusterCostSnapshot(context.Background(), nil)
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
