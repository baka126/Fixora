package controller

import "testing"

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

func guardrailStatus(guardrails []DashboardGuardrail, label string) string {
	for _, guardrail := range guardrails {
		if guardrail.Label == label {
			return guardrail.Status
		}
	}
	return ""
}
