package controller

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path"
	"sort"
	"strings"
	"time"

	"fixora/pkg/security"
)

type DashboardSnapshot struct {
	Environment      string                   `json:"environment"`
	TimeRange        string                   `json:"timeRange"`
	GeneratedAt      time.Time                `json:"generatedAt"`
	Metadata         DashboardMetadata        `json:"metadata"`
	Policy           DashboardPolicy          `json:"policy"`
	Integrations     []DashboardIntegration   `json:"integrations"`
	Availability     []DashboardAvailability  `json:"availability"`
	KPIs             []DashboardKPI           `json:"kpis"`
	Incidents        []DashboardIncident      `json:"incidents"`
	Remediations     []DashboardRemediation   `json:"remediations"`
	GitOpsSources    []DashboardGitOpsSource  `json:"gitopsSources"`
	Predictions      []DashboardPrediction    `json:"predictions"`
	AuditEvents      []DashboardAuditEvent    `json:"auditEvents"`
	Pipeline         []DashboardPipelineStage `json:"pipeline"`
	SettingsSections []DashboardSettings      `json:"settingsSections"`
	ClusterCostMo    float64                  `json:"clusterCostMo"`
	ActiveNodes      int                      `json:"activeNodes"`
}

type DashboardMetadata struct {
	Version       string `json:"version"`
	BuildDate     string `json:"buildDate"`
	SystemHealth  string `json:"systemHealth"`
	Notifications int    `json:"notifications"`
}

type DashboardPolicy struct {
	Mode        string `json:"mode"`
	Description string `json:"description"`
}

type DashboardIntegration struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}

type DashboardAvailability struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type DashboardKPI struct {
	Label  string `json:"label"`
	Value  string `json:"value"`
	Detail string `json:"detail"`
	Icon   string `json:"icon"`
	Tone   string `json:"tone"`
	Delta  string `json:"delta,omitempty"`
	Trend  []int  `json:"trend,omitempty"`
}

type DashboardIncident struct {
	ID         string                    `json:"id"`
	Workload   DashboardWorkload         `json:"workload"`
	Status     string                    `json:"status"`
	Cause      string                    `json:"cause"`
	Confidence int                       `json:"confidence"`
	Source     string                    `json:"source"`
	Age        string                    `json:"age"`
	Severity   string                    `json:"severity"`
	Priority   string                    `json:"priority"`
	Risk       string                    `json:"risk"`
	Evidence   []DashboardEvidence       `json:"evidence,omitempty"`
	GitOps     *DashboardGitOpsMapping   `json:"gitops,omitempty"`
	PR         *DashboardRecommendedPR   `json:"pr,omitempty"`
	Guardrails []DashboardGuardrail      `json:"guardrails,omitempty"`
	Graph      []DashboardDependencyNode `json:"graph,omitempty"`
	Edges      [][2]string               `json:"edges,omitempty"`
}

type DashboardWorkload struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	PodName   string `json:"podName,omitempty"`
}

type DashboardEvidence struct {
	Icon  string `json:"icon"`
	Label string `json:"label"`
	Value string `json:"value"`
	Count int    `json:"count,omitempty"`
	Link  string `json:"link,omitempty"`
}

type DashboardGitOpsMapping struct {
	Controller   string `json:"controller"`
	App          string `json:"app"`
	Namespace    string `json:"namespace,omitempty"`
	Repo         string `json:"repo"`
	Revision     string `json:"revision"`
	Path         string `json:"path"`
	ManifestType string `json:"manifestType"`
	Overlay      string `json:"overlay,omitempty"`
}

type DashboardRecommendedPR struct {
	Branch           string   `json:"branch"`
	Title            string   `json:"title"`
	Files            []string `json:"files"`
	FileCount        int      `json:"fileCount"`
	Strategy         string   `json:"strategy"`
	Status           string   `json:"status"`
	Risk             string   `json:"risk"`
	ApproverRequired bool     `json:"approverRequired"`
	URL              string   `json:"url,omitempty"`
}

type DashboardRemediation struct {
	ID            int64                   `json:"id"`
	Status        string                  `json:"status"`
	Title         string                  `json:"title"`
	Repository    string                  `json:"repository"`
	BaseBranch    string                  `json:"baseBranch"`
	HeadBranch    string                  `json:"headBranch"`
	PRURL         string                  `json:"prUrl,omitempty"`
	Files         []string                `json:"files"`
	Strategy      string                  `json:"strategy"`
	FailureReason string                  `json:"failureReason,omitempty"`
	Age           string                  `json:"age"`
	Workload      DashboardWorkload       `json:"workload"`
	GitOps        *DashboardGitOpsMapping `json:"gitops,omitempty"`
}

type DashboardGitOpsSource struct {
	ID           string `json:"id"`
	Controller   string `json:"controller"`
	App          string `json:"app"`
	Namespace    string `json:"namespace,omitempty"`
	Repo         string `json:"repo"`
	Revision     string `json:"revision"`
	Path         string `json:"path"`
	ManifestType string `json:"manifestType"`
	Overlay      string `json:"overlay,omitempty"`
	Workloads    int    `json:"workloads"`
}

type DashboardPrediction struct {
	ID               string  `json:"id"`
	Namespace        string  `json:"namespace"`
	PodName          string  `json:"podName"`
	LastAlertAge     string  `json:"lastAlertAge"`
	LastGrowthRate   float64 `json:"lastGrowthRate"`
	Risk             string  `json:"risk"`
	PreventionCostMo float64 `json:"preventionCostMo"`
	DowntimeRiskHr   float64 `json:"downtimeRiskHr"`
}

type DashboardAuditEvent struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Status    string `json:"status"`
	Subject   string `json:"subject"`
	Detail    string `json:"detail"`
	Timestamp string `json:"timestamp"`
}

type DashboardGuardrail struct {
	Label  string `json:"label"`
	Status string `json:"status"`
}

type DashboardDependencyNode struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Detail string `json:"detail"`
	X      int    `json:"x"`
	Y      int    `json:"y"`
	Kind   string `json:"kind"`
}

type DashboardPipelineStage struct {
	ID    string                  `json:"id"`
	Label string                  `json:"label"`
	Count string                  `json:"count"`
	State string                  `json:"state"`
	Note  string                  `json:"note"`
	Icon  string                  `json:"icon"`
	Items []DashboardPipelineItem `json:"items,omitempty"`
}

type DashboardPipelineItem struct {
	ID         string `json:"id"`
	Label      string `json:"label"`
	Repository string `json:"repository,omitempty"`
	Age        string `json:"age"`
	URL        string `json:"url,omitempty"`
}

type DashboardSettings struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Status      string `json:"status"`
}

type dashboardInvestigationRow struct {
	ID                int64
	Namespace         string
	PodName           string
	Timestamp         time.Time
	Reason            string
	MetricProof       string
	ClusterContext    string
	HistoricalPattern string
	EventTimeline     string
	StackTrace        string
	RootCause         string
	AIConfidence      int
	FinOpsImpact      string
	FinOpsDetails     string
}

type dashboardRemediationRow struct {
	ID                int64
	InvestigationID   int64
	Namespace         string
	PodName           string
	DiagnosisCategory string
	PatchStrategy     string
	Status            string
	RepoOwner         string
	RepoName          string
	BaseBranch        string
	HeadBranch        string
	PRURL             string
	PRTitle           string
	GitOpsController  string
	GitOpsApp         string
	GitOpsNamespace   string
	GitOpsRepoURL     string
	GitOpsRevision    string
	GitOpsPath        string
	ManifestType      string
	OverlayRole       string
	Environment       string
	Region            string
	ChangedFiles      []string
	FailureReason     string
	WorkloadKind      string
	WorkloadName      string
	WorkloadSelector  string
	UpdatedAt         time.Time
}

type dashboardFileChange struct {
	FilePath string `json:"file_path"`
}

type dashboardPredictionRow struct {
	Namespace        string
	PodName          string
	LastAlertTime    time.Time
	LastGrowthRate   float64
	PreventionCostMo float64
	DowntimeRiskHr   float64
}

type dashboardAlertRow struct {
	ID         int64
	Namespace  string
	PodName    string
	AlertName  string
	Status     string
	Source     string
	ReceivedAt time.Time
}

func (c *Controller) GetInvestigation(ctx context.Context, id int64) (map[string]interface{}, error) {
	if c.history == nil || c.history.db == nil {
		return nil, fmt.Errorf("database not configured")
	}

	query := `
		SELECT 
			namespace, pod_name, timestamp, reason, COALESCE(metric_proof, ''), 
			COALESCE(cluster_context, ''), COALESCE(historical_pattern, ''), 
			COALESCE(event_timeline, ''), COALESCE(stack_trace, ''), 
			COALESCE(root_cause, ''), ai_confidence, 
			COALESCE(finops_impact, ''), COALESCE(finops_details, ''),
			COALESCE(ai_prompt, ''), COALESCE(ai_response, '')
		FROM investigations WHERE id = $1
	`
	var inv map[string]interface{} = make(map[string]interface{})
	var ns, pod, reason, proof, context, pattern, timeline, trace, rootCause, impact, details, prompt, response string
	var timestamp time.Time
	var confidence int

	err := c.history.db.QueryRowContext(ctx, query, id).Scan(
		&ns, &pod, &timestamp, &reason, &proof, &context, &pattern, &timeline, &trace, &rootCause, &confidence, &impact, &details, &prompt, &response,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("investigation not found")
	}
	if err != nil {
		return nil, err
	}

	inv["id"] = id
	inv["namespace"] = ns
	inv["podName"] = pod
	inv["timestamp"] = timestamp
	inv["reason"] = reason
	inv["metricProof"] = proof
	inv["clusterContext"] = context
	inv["historicalPattern"] = pattern
	inv["eventTimeline"] = timeline
	inv["stackTrace"] = trace
	inv["rootCause"] = rootCause
	inv["aiConfidence"] = confidence
	inv["finopsImpact"] = impact
	inv["finopsDetails"] = details
	inv["aiPrompt"] = security.ScrubPII(prompt)
	inv["aiResponse"] = security.ScrubPII(response)

	return inv, nil
}

// DashboardSnapshot returns the live UI model. It intentionally returns empty
// slices and availability messages instead of fabricated sample data.
func (c *Controller) DashboardSnapshot(ctx context.Context) DashboardSnapshot {
	snapshot := DashboardSnapshot{
		Environment:   "cluster",
		TimeRange:     "Last 24h",
		GeneratedAt:   time.Now(),
		Metadata:      dashboardMetadata(),
		Policy:        dashboardPolicy(string(c.config.Mode)),
		Integrations:  c.dashboardIntegrations(),
		Availability:  []DashboardAvailability{},
		KPIs:          []DashboardKPI{},
		Incidents:     []DashboardIncident{},
		Remediations:  []DashboardRemediation{},
		GitOpsSources: []DashboardGitOpsSource{},
		Predictions:   []DashboardPrediction{},
		AuditEvents:   []DashboardAuditEvent{},
		Pipeline:      defaultDashboardPipeline(map[string]int{}, nil),
		SettingsSections: []DashboardSettings{
			{Name: "AI provider", Description: "Root cause and remediation generation", Status: configuredStatus(c.config.AIAPIKey != "" || c.config.AIProvider == "")},
			{Name: "VCS credentials", Description: "GitHub/GitLab PR creation", Status: configuredStatus(c.config.GitHubToken != "" || c.config.GitLabToken != "")},
			{Name: "Slack signing secret", Description: "Interactive approval protection", Status: configuredStatus(c.config.SlackSigningSecret != "")},
			{Name: "Webhook token", Description: "Inbound webhook authentication", Status: configuredStatus(c.config.WebhookToken != "")},
			{Name: "ArgoCD token", Description: "GitOps source resolution", Status: configuredStatus(c.config.ArgoCDToken != "")},
		},
	}

	if c.history == nil || !c.history.HasDB() {
		snapshot.Availability = append(snapshot.Availability, DashboardAvailability{
			Name:    "PostgreSQL",
			Status:  "empty",
			Message: "Database is not configured, so historical investigations and remediation outcomes are not available yet.",
		})
		snapshot.KPIs = emptyDashboardKPIs()
		return snapshot
	}

	db := c.history.DB()
	investigations := queryDashboardInvestigations(ctx, db, 30)
	remediations := queryDashboardRemediations(ctx, db, 80)
	predictions := queryDashboardPredictions(ctx, db, 50)
	alerts := queryDashboardAlerts(ctx, db, 80)
	pendingCount := queryDashboardCount(ctx, db, `SELECT COUNT(*) FROM pending_fixes`)
	alertCount := queryDashboardCount(ctx, db, `SELECT COUNT(*) FROM alerts WHERE received_at >= NOW() - INTERVAL '24 hours'`)
	predictionCount := queryDashboardCount(ctx, db, `SELECT COUNT(*) FROM predictions`)
	dependencyCount := queryDashboardCount(ctx, db, `SELECT COUNT(*) FROM dependency_graph`)

	if len(investigations) == 0 && len(remediations) == 0 && alertCount == 0 {
		snapshot.Availability = append(snapshot.Availability, DashboardAvailability{
			Name:    "Live data",
			Status:  "empty",
			Message: "No investigations, alerts, or remediations have been recorded yet. The dashboard will populate automatically as Fixora observes workloads.",
		})
	}
	if dependencyCount == 0 {
		snapshot.Availability = append(snapshot.Availability, DashboardAvailability{
			Name:    "Dependency graph",
			Status:  "empty",
			Message: "No dependency edges have been recorded yet. They will appear as the Kubernetes watcher observes pods, ConfigMaps, and Secrets.",
		})
	}

	remByInvestigation := make(map[int64]dashboardRemediationRow)
	remByPod := make(map[string]dashboardRemediationRow)
	statusCounts := make(map[string]int)
	for _, rem := range remediations {
		statusCounts[rem.Status]++
		if rem.InvestigationID > 0 {
			remByInvestigation[rem.InvestigationID] = rem
		}
		key := dashboardPodKey(rem.Namespace, rem.PodName)
		if _, ok := remByPod[key]; !ok {
			remByPod[key] = rem
		}
	}
	statusCounts["pending_approval"] += pendingCount

	snapshot.KPIs = dashboardKPIs(investigations, pendingCount, alertCount, predictionCount, statusCounts)
	snapshot.Pipeline = defaultDashboardPipeline(statusCounts, dashboardPipelineItems(remediations))
	snapshot.Incidents = dashboardIncidents(ctx, db, investigations, remByInvestigation, remByPod)
	snapshot.Remediations = dashboardRemediations(remediations)
	snapshot.GitOpsSources = dashboardGitOpsSources(remediations)
	snapshot.Predictions = dashboardPredictions(predictions)
	snapshot.AuditEvents = dashboardAuditEvents(investigations, remediations, alerts)
	
	// Add FinOps Cluster Cost
	clusterCost, activeNodes := c.calculateClusterCostSnapshot(ctx)
	snapshot.ClusterCostMo = clusterCost
	snapshot.ActiveNodes = activeNodes
	
	return snapshot
}

func dashboardMetadata() DashboardMetadata {
	return DashboardMetadata{
		Version:       firstNonEmpty(strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(os.Getenv("FIXORA_VERSION"), "v"), "V")), "0.4.0"),
		BuildDate:     firstNonEmpty(os.Getenv("FIXORA_BUILD_DATE"), time.Now().UTC().Format("2006-01-02")),
		SystemHealth:  "operational",
		Notifications: 0,
	}
}

func dashboardPolicy(mode string) DashboardPolicy {
	switch mode {
	case "auto-fix":
		return DashboardPolicy{Mode: "Auto-fix", Description: "Fixora can open remediation pull requests automatically within policy limits."}
	case "click-to-fix":
		return DashboardPolicy{Mode: "Click-to-fix", Description: "Approval required before opening remediation pull requests."}
	default:
		return DashboardPolicy{Mode: "Dry-run", Description: "Fixora reports diagnostics but does not open remediation pull requests."}
	}
}

func (c *Controller) dashboardIntegrations() []DashboardIntegration {
	return []DashboardIntegration{
		{Name: "PostgreSQL", Status: integrationStatus(c.history != nil && c.history.HasDB())},
		{Name: "Prometheus", Status: integrationStatus(c.config.PrometheusURL != "")},
		{Name: "Alertmanager", Status: integrationStatus(c.config.AlertmanagerEnabled && c.config.AlertmanagerURL != "")},
		{Name: "ArgoCD", Status: integrationStatus(c.config.ArgoCDEnabled), Detail: c.config.ArgoCDNamespace},
		{Name: "Flux", Status: "neutral", Detail: "detected from cluster objects"},
	}
}

func integrationStatus(ok bool) string {
	if ok {
		return "ok"
	}
	return "neutral"
}

func configuredStatus(ok bool) string {
	if ok {
		return "configured"
	}
	return "missing"
}

func emptyDashboardKPIs() []DashboardKPI {
	return []DashboardKPI{
		{Label: "Open incidents", Value: "0", Detail: "waiting for investigations", Icon: "icon-alert", Tone: "info"},
		{Label: "PRs awaiting review", Value: "0", Detail: "none pending", Icon: "icon-branch", Tone: "info"},
		{Label: "Auto-fix success", Value: "n/a", Detail: "no outcomes yet", Icon: "icon-check", Tone: "info"},
		{Label: "Reverts", Value: "0", Detail: "none recorded", Icon: "icon-clock", Tone: "info"},
		{Label: "Monthly risk avoided", Value: "n/a", Detail: "FinOps data unavailable", Icon: "icon-chart", Tone: "info"},
	}
}

func dashboardKPIs(investigations []dashboardInvestigationRow, _ int, alertCount, predictionCount int, statusCounts map[string]int) []DashboardKPI {
	succeeded := statusCounts["succeeded"]
	failed := statusCounts["production_failed"] + statusCounts["revert_failed"] + statusCounts["reverted"]
	successValue := "n/a"
	if succeeded+failed > 0 {
		successValue = fmt.Sprintf("%d%%", int(math.Round(float64(succeeded)*100/float64(succeeded+failed))))
	}
	return []DashboardKPI{
		{Label: "Open incidents", Value: fmt.Sprintf("%d", len(investigations)), Detail: fmt.Sprintf("%d alerts in 24h", alertCount), Icon: "icon-alert", Tone: severityTone(len(investigations)), Delta: fmt.Sprintf("%d active", len(investigations))},
		{Label: "PRs awaiting review", Value: fmt.Sprintf("%d", statusCounts["pending_approval"]), Detail: "approval queue", Icon: "icon-branch", Tone: "info"},
		{Label: "Auto-fix success", Value: successValue, Detail: "closed-loop outcomes", Icon: "icon-check", Tone: "success"},
		{Label: "Reverts", Value: fmt.Sprintf("%d", statusCounts["revert_opened"]+statusCounts["reverted"]+statusCounts["revert_failed"]), Detail: "safety actions", Icon: "icon-clock", Tone: "warning"},
		{Label: "Monthly risk avoided", Value: "n/a", Detail: fmt.Sprintf("%d predictions tracked", predictionCount), Icon: "icon-chart", Tone: "info"},
	}
}

func severityTone(count int) string {
	if count > 0 {
		return "critical"
	}
	return "success"
}

func defaultDashboardPipeline(counts map[string]int, items map[string][]DashboardPipelineItem) []DashboardPipelineStage {
	return []DashboardPipelineStage{
		{ID: "generated", Label: "Generated", Count: fmt.Sprintf("%d", counts["generated"]), State: "pending", Note: "AI patch proposals", Icon: "icon-code", Items: items["generated"]},
		{ID: "approval", Label: "Pending approval", Count: fmt.Sprintf("%d", counts["pending_approval"]), State: "pending", Note: "Slack or UI approval", Icon: "icon-clock", Items: items["pending_approval"]},
		{ID: "opened", Label: "PR opened", Count: fmt.Sprintf("%d", counts["pr_opened"]), State: "pending", Note: "Targeted PRs", Icon: "icon-branch", Items: items["pr_opened"]},
		{ID: "observing", Label: "Observing", Count: fmt.Sprintf("%d", counts["observing"]), State: "pending", Note: "Post-merge monitoring", Icon: "icon-chart", Items: items["observing"]},
		{ID: "succeeded", Label: "Succeeded", Count: fmt.Sprintf("%d", counts["succeeded"]), State: "succeeded", Note: "Validated fixes", Icon: "icon-check", Items: items["succeeded"]},
		{ID: "failed", Label: "Production failed", Count: fmt.Sprintf("%d", counts["production_failed"]), State: "failed", Note: "Regression detected", Icon: "icon-alert", Items: items["production_failed"]},
		{ID: "revert", Label: "Revert opened", Count: fmt.Sprintf("%d", counts["revert_opened"]), State: "failed", Note: "Automated rollback", Icon: "icon-branch", Items: items["revert_opened"]},
	}
}

func dashboardPipelineItems(rows []dashboardRemediationRow) map[string][]DashboardPipelineItem {
	items := map[string][]DashboardPipelineItem{}
	for _, row := range rows {
		if len(items[row.Status]) >= 4 {
			continue
		}
		items[row.Status] = append(items[row.Status], DashboardPipelineItem{
			ID:         fmt.Sprintf("remediation-%d", row.ID),
			Label:      firstNonEmpty(row.HeadBranch, row.PRTitle, row.FailureReason, row.PodName),
			Repository: dashboardRepository(row),
			Age:        humanAge(row.UpdatedAt),
			URL:        row.PRURL,
		})
	}
	return items
}

func queryDashboardInvestigations(ctx context.Context, db *sql.DB, limit int) []dashboardInvestigationRow {
	rows, err := db.QueryContext(ctx, `
		SELECT id, namespace, pod_name, timestamp, reason,
		       COALESCE(metric_proof, ''), COALESCE(cluster_context, ''), COALESCE(historical_pattern, ''),
		       COALESCE(event_timeline, ''), COALESCE(stack_trace, ''), COALESCE(root_cause, ''),
		       COALESCE(ai_confidence, 0), COALESCE(finops_impact, ''), COALESCE(finops_details, '')
		FROM investigations
		ORDER BY timestamp DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var out []dashboardInvestigationRow
	for rows.Next() {
		var row dashboardInvestigationRow
		if err := rows.Scan(&row.ID, &row.Namespace, &row.PodName, &row.Timestamp, &row.Reason, &row.MetricProof, &row.ClusterContext, &row.HistoricalPattern, &row.EventTimeline, &row.StackTrace, &row.RootCause, &row.AIConfidence, &row.FinOpsImpact, &row.FinOpsDetails); err == nil {
			out = append(out, row)
		}
	}
	return out
}

func queryDashboardRemediations(ctx context.Context, db *sql.DB, limit int) []dashboardRemediationRow {
	rows, err := db.QueryContext(ctx, `
		SELECT id, COALESCE(investigation_id, 0), namespace, pod_name,
		       COALESCE(diagnosis_category, ''), COALESCE(patch_strategy, ''), status,
		       repo_owner, repo_name, base_branch, head_branch, COALESCE(pr_url, ''), COALESCE(pr_title, ''),
		       COALESCE(gitops_controller, ''), COALESCE(gitops_app, ''), COALESCE(gitops_namespace, ''),
		       COALESCE(gitops_repo_url, ''), COALESCE(gitops_revision, ''), COALESCE(gitops_path, ''),
		       COALESCE(manifest_type, ''), COALESCE(overlay_role, ''), COALESCE(environment, ''),
		       COALESCE(region, ''), COALESCE(changed_files, '[]'::jsonb), COALESCE(failure_reason, ''),
		       COALESCE(workload_kind, ''), COALESCE(workload_name, ''), COALESCE(workload_selector, ''),
		       updated_at
		FROM remediation_outcomes
		ORDER BY updated_at DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var out []dashboardRemediationRow
	for rows.Next() {
		var row dashboardRemediationRow
		var changedFiles []byte
		if err := rows.Scan(&row.ID, &row.InvestigationID, &row.Namespace, &row.PodName, &row.DiagnosisCategory, &row.PatchStrategy, &row.Status, &row.RepoOwner, &row.RepoName, &row.BaseBranch, &row.HeadBranch, &row.PRURL, &row.PRTitle, &row.GitOpsController, &row.GitOpsApp, &row.GitOpsNamespace, &row.GitOpsRepoURL, &row.GitOpsRevision, &row.GitOpsPath, &row.ManifestType, &row.OverlayRole, &row.Environment, &row.Region, &changedFiles, &row.FailureReason, &row.WorkloadKind, &row.WorkloadName, &row.WorkloadSelector, &row.UpdatedAt); err == nil {
			row.ChangedFiles = dashboardChangedFilePaths(changedFiles)
			out = append(out, row)
		}
	}
	return out
}

func queryDashboardPredictions(ctx context.Context, db *sql.DB, limit int) []dashboardPredictionRow {
	rows, err := db.QueryContext(ctx, `
		SELECT namespace, pod_name, last_alert_time, last_growth_rate, COALESCE(prevention_cost_mo, 0), COALESCE(downtime_risk_hr, 0)
		FROM predictions
		ORDER BY last_alert_time DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var out []dashboardPredictionRow
	for rows.Next() {
}

	for rows.Next() {
}

func queryDashboardAlerts(ctx context.Context, db *sql.DB, limit int) []dashboardAlertRow {
	rows, err := db.QueryContext(ctx, `
		SELECT id, COALESCE(namespace, ''), COALESCE(pod_name, ''), alertname,
		       COALESCE(status, ''), COALESCE(source, ''), received_at
		FROM alerts
		ORDER BY received_at DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var out []dashboardAlertRow
	for rows.Next() {
		var row dashboardAlertRow
		if err := rows.Scan(&row.ID, &row.Namespace, &row.PodName, &row.AlertName, &row.Status, &row.Source, &row.ReceivedAt); err == nil {
			out = append(out, row)
		}
	}
	return out
}

func queryDashboardCount(ctx context.Context, db *sql.DB, query string) int {
	var count int
	if err := db.QueryRowContext(ctx, query).Scan(&count); err != nil {
		return 0
	}
	return count
}

func dashboardRemediations(rows []dashboardRemediationRow) []DashboardRemediation {
	out := make([]DashboardRemediation, 0, len(rows))
	for _, row := range rows {
		out = append(out, DashboardRemediation{
			ID:            row.ID,
			Status:        row.Status,
			Title:         firstNonEmpty(row.PRTitle, row.FailureReason, "Remediation"),
			Repository:    dashboardRepository(row),
			BaseBranch:    row.BaseBranch,
			HeadBranch:    row.HeadBranch,
			PRURL:         row.PRURL,
			Files:         row.ChangedFiles,
			Strategy:      row.PatchStrategy,
			FailureReason: row.FailureReason,
			Age:           humanAge(row.UpdatedAt),
			Workload: DashboardWorkload{
				Kind:      firstNonEmpty(row.WorkloadKind, "Pod"),
				Name:      firstNonEmpty(row.WorkloadName, row.PodName),
				Namespace: row.Namespace,
				PodName:   row.PodName,
			},
			GitOps: dashboardGitOps(row),
		})
	}
	return out
}

func dashboardGitOpsSources(rows []dashboardRemediationRow) []DashboardGitOpsSource {
	type aggregate struct {
		source    DashboardGitOpsSource
		workloads map[string]bool
	}
	aggregates := map[string]*aggregate{}
	for _, row := range rows {
		gitops := dashboardGitOps(row)
		if gitops == nil {
			continue
		}
		key := strings.Join([]string{gitops.Controller, gitops.App, gitops.Repo, gitops.Revision, gitops.Path, gitops.ManifestType, gitops.Overlay}, "\x00")
		if _, ok := aggregates[key]; !ok {
			aggregates[key] = &aggregate{
				source: DashboardGitOpsSource{
					ID:           fmt.Sprintf("gitops-%d", len(aggregates)+1),
					Controller:   gitops.Controller,
					App:          gitops.App,
					Namespace:    gitops.Namespace,
					Repo:         gitops.Repo,
					Revision:     gitops.Revision,
					Path:         gitops.Path,
					ManifestType: gitops.ManifestType,
					Overlay:      gitops.Overlay,
				},
				workloads: map[string]bool{},
			}
		}
		aggregates[key].workloads[dashboardPodKey(row.Namespace, firstNonEmpty(row.WorkloadName, row.PodName))] = true
	}
	out := make([]DashboardGitOpsSource, 0, len(aggregates))
	for _, aggregate := range aggregates {
		aggregate.source.Workloads = len(aggregate.workloads)
		out = append(out, aggregate.source)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Repo+out[i].Path < out[j].Repo+out[j].Path
	})
	return out
}

func dashboardPredictions(rows []dashboardPredictionRow) []DashboardPrediction {
	out := make([]DashboardPrediction, 0, len(rows))
	for i, row := range rows {
		risk := "low"
		if row.LastGrowthRate >= 0.5 {
			risk = "high"
		} else if row.LastGrowthRate >= 0.2 {
			risk = "medium"
		}
		out = append(out, DashboardPrediction{
			ID:               fmt.Sprintf("prediction-%d", i+1),
			Namespace:        row.Namespace,
			PodName:          row.PodName,
			LastAlertAge:     humanAge(row.LastAlertTime),
			LastGrowthRate:   row.LastGrowthRate,
			Risk:             risk,
			PreventionCostMo: row.PreventionCostMo,
			DowntimeRiskHr:   row.DowntimeRiskHr,
		})
	}
	return out
}

func dashboardAuditEvents(investigations []dashboardInvestigationRow, remediations []dashboardRemediationRow, alerts []dashboardAlertRow) []DashboardAuditEvent {
	var events []DashboardAuditEvent
	for _, row := range investigations {
		events = append(events, DashboardAuditEvent{
			ID:        fmt.Sprintf("investigation-%d", row.ID),
			Type:      "Investigation",
			Status:    "completed",
			Subject:   dashboardPodKey(row.Namespace, row.PodName),
			Detail:    firstNonEmpty(shortRootCause(row.RootCause), row.Reason),
			Timestamp: row.Timestamp.Format(time.RFC3339),
		})
	}
	for _, row := range remediations {
		events = append(events, DashboardAuditEvent{
			ID:        fmt.Sprintf("remediation-%d", row.ID),
			Type:      "Remediation",
			Status:    row.Status,
			Subject:   dashboardRepository(row),
			Detail:    firstNonEmpty(row.PRTitle, row.FailureReason, row.HeadBranch),
			Timestamp: row.UpdatedAt.Format(time.RFC3339),
		})
	}
	for _, row := range alerts {
		events = append(events, DashboardAuditEvent{
			ID:        fmt.Sprintf("alert-%d", row.ID),
			Type:      "Alert",
			Status:    firstNonEmpty(row.Status, "received"),
			Subject:   dashboardPodKey(row.Namespace, row.PodName),
			Detail:    firstNonEmpty(row.AlertName, row.Source),
			Timestamp: row.ReceivedAt.Format(time.RFC3339),
		})
	}
	sort.Slice(events, func(i, j int) bool {
		return events[i].Timestamp > events[j].Timestamp
	})
	if len(events) > 80 {
		return events[:80]
	}
	return events
}

func dashboardRepository(row dashboardRemediationRow) string {
	if row.GitOpsRepoURL != "" {
		return row.GitOpsRepoURL
	}
	if row.RepoOwner != "" && row.RepoName != "" {
		return path.Join(row.RepoOwner, row.RepoName)
	}
	return ""
}

func dashboardChangedFilePaths(raw []byte) []string {
	var changes []dashboardFileChange
	if err := json.Unmarshal(raw, &changes); err != nil {
		return nil
	}
	seen := make(map[string]bool)
	var files []string
	for _, change := range changes {
		if change.FilePath == "" || seen[change.FilePath] {
			continue
		}
		seen[change.FilePath] = true
		files = append(files, change.FilePath)
	}
	sort.Strings(files)
	return files
}

func dashboardIncidents(ctx context.Context, db *sql.DB, investigations []dashboardInvestigationRow, remByInvestigation map[int64]dashboardRemediationRow, remByPod map[string]dashboardRemediationRow) []DashboardIncident {
	out := make([]DashboardIncident, 0, len(investigations))
	for _, inv := range investigations {
		rem, ok := remByInvestigation[inv.ID]
		if !ok {
			rem = remByPod[dashboardPodKey(inv.Namespace, inv.PodName)]
		}
		incident := DashboardIncident{
			ID:         fmt.Sprintf("investigation-%d", inv.ID),
			Workload:   dashboardWorkload(inv, rem),
			Status:     firstNonEmpty(contextValue(inv.ClusterContext, "Reason"), inv.Reason),
			Cause:      firstNonEmpty(shortRootCause(inv.RootCause), contextValue(inv.ClusterContext, "Diagnosis"), inv.Reason),
			Confidence: inv.AIConfidence,
			Source:     dashboardIncidentSource(inv),
			Age:        humanAge(inv.Timestamp),
			Severity:   dashboardSeverity(inv),
			Priority:   dashboardPriority(inv),
			Risk:       dashboardRisk(rem),
			Evidence:   dashboardEvidence(inv),
			Guardrails: dashboardGuardrails(rem),
		}
		if gitops := dashboardGitOps(rem); gitops != nil {
			incident.GitOps = gitops
		}
		if pr := dashboardPR(rem); pr != nil {
			incident.PR = pr
		}
		incident.Graph, incident.Edges = queryDashboardGraph(ctx, db, inv.Namespace, inv.PodName, incident.Workload)
		out = append(out, incident)
	}
	return out
}

func dashboardWorkload(inv dashboardInvestigationRow, rem dashboardRemediationRow) DashboardWorkload {
	kind := firstNonEmpty(rem.WorkloadKind, contextValue(inv.ClusterContext, "Workload Kind"), "Pod")
	name := firstNonEmpty(rem.WorkloadName, contextValue(inv.ClusterContext, "Workload Name"), inv.PodName)
	return DashboardWorkload{Kind: kind, Name: name, Namespace: inv.Namespace, PodName: inv.PodName}
}

func dashboardIncidentSource(inv dashboardInvestigationRow) string {
	var sources []string
	if inv.MetricProof != "" {
		sources = append(sources, "Metrics")
	}
	if inv.EventTimeline != "" {
		sources = append(sources, "Events")
	}
	if inv.StackTrace != "" {
		sources = append(sources, "Logs")
	}
	if len(sources) == 0 {
		return "Investigation"
	}
	return strings.Join(sources, " + ")
}

func dashboardSeverity(inv dashboardInvestigationRow) string {
	text := strings.ToLower(inv.Reason + " " + inv.RootCause + " " + inv.ClusterContext)
	if strings.Contains(text, "crash") || strings.Contains(text, "oom") || strings.Contains(text, "failed") || strings.Contains(text, "error") {
		return "critical"
	}
	return "warning"
}

func dashboardPriority(inv dashboardInvestigationRow) string {
	switch dashboardSeverity(inv) {
	case "critical":
		return "P1"
	default:
		return "P2"
	}
}

func dashboardRisk(rem dashboardRemediationRow) string {
	switch rem.Status {
	case "production_failed", "revert_failed":
		return "High risk"
	case "observing", "revert_opened":
		return "Medium risk"
	default:
		return "Low risk"
	}
}

func dashboardEvidence(inv dashboardInvestigationRow) []DashboardEvidence {
	evidence := []DashboardEvidence{}
	if inv.MetricProof != "" {
		evidence = append(evidence, DashboardEvidence{Icon: "icon-chart", Label: "Metric proof", Value: truncateDashboard(inv.MetricProof, 180)})
	}
	if inv.EventTimeline != "" {
		evidence = append(evidence, DashboardEvidence{Icon: "icon-alert", Label: "Cluster events", Value: truncateDashboard(inv.EventTimeline, 180), Count: countNonEmptyLines(inv.EventTimeline)})
	}
	if inv.StackTrace != "" {
		evidence = append(evidence, DashboardEvidence{Icon: "icon-terminal", Label: "Logs / stack trace", Value: truncateDashboard(inv.StackTrace, 180), Link: "View"})
	}
	if inv.HistoricalPattern != "" {
		evidence = append(evidence, DashboardEvidence{Icon: "icon-clock", Label: "History", Value: truncateDashboard(inv.HistoricalPattern, 180), Link: "View"})
	}
	if inv.FinOpsImpact != "" {
		evidence = append(evidence, DashboardEvidence{Icon: "icon-chart", Label: "FinOps", Value: truncateDashboard(inv.FinOpsImpact, 180)})
	}
	return evidence
}

func dashboardGitOps(rem dashboardRemediationRow) *DashboardGitOpsMapping {
	if rem.GitOpsController == "" && rem.GitOpsRepoURL == "" && rem.GitOpsPath == "" && rem.RepoOwner == "" {
		return nil
	}
	repo := rem.GitOpsRepoURL
	if repo == "" && rem.RepoOwner != "" && rem.RepoName != "" {
		repo = path.Join(rem.RepoOwner, rem.RepoName)
	}
	overlay := strings.Trim(strings.Join(nonEmptyDashboard(rem.OverlayRole, rem.Environment, rem.Region), " / "), " /")
	return &DashboardGitOpsMapping{
		Controller:   firstNonEmpty(rem.GitOpsController, "vcs"),
		App:          rem.GitOpsApp,
		Namespace:    rem.GitOpsNamespace,
		Repo:         repo,
		Revision:     firstNonEmpty(rem.GitOpsRevision, rem.BaseBranch),
		Path:         rem.GitOpsPath,
		ManifestType: rem.ManifestType,
		Overlay:      overlay,
	}
}

func dashboardPR(rem dashboardRemediationRow) *DashboardRecommendedPR {
	if rem.HeadBranch == "" && rem.PRTitle == "" && rem.PRURL == "" && len(rem.ChangedFiles) == 0 {
		return nil
	}
	return &DashboardRecommendedPR{
		Branch:           rem.HeadBranch,
		Title:            firstNonEmpty(rem.PRTitle, rem.FailureReason, "Remediation pull request"),
		Files:            rem.ChangedFiles,
		FileCount:        len(rem.ChangedFiles),
		Strategy:         rem.PatchStrategy,
		Status:           rem.Status,
		Risk:             dashboardRisk(rem),
		ApproverRequired: rem.Status == "pending_approval" || rem.Status == "generated",
		URL:              rem.PRURL,
	}
}

func dashboardGuardrails(rem dashboardRemediationRow) []DashboardGuardrail {
	if rem.ID == 0 {
		return nil
	}
	status := "passed"
	if rem.Status == "pr_failed" || rem.Status == "production_failed" || rem.Status == "revert_failed" {
		status = "failed"
	}
	renderStatus := "pending"
	if rem.Status == "pr_opened" || rem.Status == "observing" || rem.Status == "succeeded" {
		renderStatus = "passed"
	}
	return []DashboardGuardrail{
		{Label: "Identity match", Status: "passed"},
		{Label: "Privileged paths blocked", Status: "passed"},
		{Label: "Duplicate PR check", Status: "passed"},
		{Label: "Render validation", Status: renderStatus},
		{Label: "Remediation state", Status: status},
	}
}

func queryDashboardGraph(ctx context.Context, db *sql.DB, namespace, podName string, workload DashboardWorkload) ([]DashboardDependencyNode, [][2]string) {
	rows, err := db.QueryContext(ctx, `
		SELECT source_kind, source_name
		FROM dependency_graph
		WHERE namespace = $1 AND target_kind = 'Pod' AND target_name = $2
		ORDER BY source_kind, source_name
		LIMIT 12
	`, namespace, podName)
	if err != nil {
		return nil, nil
	}
	defer rows.Close()

	nodes := []DashboardDependencyNode{{
		ID: "workload", Label: firstNonEmpty(workload.Kind, "Pod"), Detail: workload.Name, X: 58, Y: 126, Kind: "active",
	}}
	var edges [][2]string
	i := 0
	for rows.Next() {
		var kind, name string
		if err := rows.Scan(&kind, &name); err != nil {
			continue
		}
		id := fmt.Sprintf("dep-%d", i)
		x := 240
		if i%2 == 1 {
			x = 420
		}
		y := 56 + (i%6)*36
		tone := "neutral"
		if kind == "ConfigMap" || kind == "Secret" {
			tone = "warning"
		}
		nodes = append(nodes, DashboardDependencyNode{ID: id, Label: kind, Detail: name, X: x, Y: y, Kind: tone})
		edges = append(edges, [2]string{"workload", id})
		i++
	}
	return nodes, edges
}

func dashboardPodKey(namespace, pod string) string {
	return namespace + "/" + pod
}

func contextValue(context, key string) string {
	prefix := key + ":"
	for _, line := range strings.Split(context, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), prefix) {
			return strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), prefix))
		}
	}
	return ""
}

func nonEmptyDashboard(values ...string) []string {
	var out []string
	for _, value := range values {
		if strings.TrimSpace(value) != "" && strings.TrimSpace(value) != "unknown" {
			out = append(out, strings.TrimSpace(value))
		}
	}
	return out
}

func shortRootCause(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	lines := strings.Split(value, "\n")
	return truncateDashboard(lines[0], 96)
}

func truncateDashboard(value string, limit int) string {
	value = strings.Join(strings.Fields(value), " ")
	if len(value) <= limit {
		return value
	}
	return value[:limit-1] + "..."
}

func countNonEmptyLines(value string) int {
	count := 0
	for _, line := range strings.Split(value, "\n") {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}
	return count
}

func humanAge(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := time.Since(t)
	if d < time.Minute {
		return "now"
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}
	for rows.Next() {
		var row dashboardPredictionRow
		if err := rows.Scan(&row.Namespace, &row.PodName, &row.LastAlertTime, &row.LastGrowthRate, &row.PreventionCostMo, &row.DowntimeRiskHr); err == nil {
			out = append(out, row)
		}
	}
	return out
}

func (c *Controller) calculateClusterCostSnapshot(ctx context.Context) (float64, int) {
	if c.clientset == nil || c.pricingProvider == nil {
		return 0, 0
	}
	nodes, err := c.clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil || len(nodes.Items) == 0 {
		return 0, 0
	}

	var nodeInfos []finops.NodeInfo
	for _, n := range nodes.Items {
		nodeInfos = append(nodeInfos, finops.NodeInfo{
			Name:   n.Name,
			Labels: n.Labels,
		})
	}

	cost, _ := finops.CalculateClusterCost(nodeInfos, c.pricingProvider)
	return cost, len(nodeInfos)
}
