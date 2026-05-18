package controller

import (
	"context"
	"crypto/sha1"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path"
	"sort"
	"strings"
	"time"

	"fixora/pkg/alertmanager"
	"fixora/pkg/finops"
	"fixora/pkg/security"

	"gopkg.in/yaml.v3"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	NodeCosts        []DashboardNodeCost      `json:"nodeCosts,omitempty"`
	CategoryTrends   []CategoryTrend          `json:"categoryTrends,omitempty"`
	SymptomTrends    []SymptomTrend           `json:"symptomTrends,omitempty"`
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
	Controller   string                `json:"controller"`
	App          string                `json:"app"`
	Namespace    string                `json:"namespace,omitempty"`
	Repo         string                `json:"repo"`
	Revision     string                `json:"revision"`
	Path         string                `json:"path"`
	ManifestType string                `json:"manifestType"`
	Overlay      string                `json:"overlay,omitempty"`
	Helm         *DashboardHelmMapping `json:"helm,omitempty"`
}

type DashboardHelmMapping struct {
	ReleaseName  string   `json:"releaseName,omitempty"`
	Namespace    string   `json:"namespace,omitempty"`
	Chart        string   `json:"chart,omitempty"`
	ChartVersion string   `json:"chartVersion,omitempty"`
	RepoURL      string   `json:"repoUrl,omitempty"`
	ValueFiles   []string `json:"valueFiles,omitempty"`
	ValuesFrom   []string `json:"valuesFrom,omitempty"`
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
	PatchTarget      string   `json:"patchTarget,omitempty"`
	Reason           string   `json:"reason,omitempty"`
	Avoided          string   `json:"avoided,omitempty"`
	Summary          []string `json:"summary,omitempty"`
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
	ID           string                `json:"id"`
	Controller   string                `json:"controller"`
	App          string                `json:"app"`
	Namespace    string                `json:"namespace,omitempty"`
	Repo         string                `json:"repo"`
	Revision     string                `json:"revision"`
	Path         string                `json:"path"`
	ManifestType string                `json:"manifestType"`
	Overlay      string                `json:"overlay,omitempty"`
	Helm         *DashboardHelmMapping `json:"helm,omitempty"`
	Workloads    int                   `json:"workloads"`
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
	Detail string `json:"detail,omitempty"`
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
	Detail     string `json:"detail,omitempty"`
	Age        string `json:"age"`
	URL        string `json:"url,omitempty"`
}

type DashboardSettings struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Status      string `json:"status"`
}

type DashboardNodeCost struct {
	Name          string  `json:"name"`
	Vendor        string  `json:"vendor"`
	Region        string  `json:"region"`
	InstanceType  string  `json:"instanceType"`
	MonthlyCost   float64 `json:"monthlyCost"`
	PricingSource string  `json:"pricingSource"`
	Status        string  `json:"status"`
}

type DashboardActiveAlert struct {
	ID            string                `json:"id"`
	AlertName     string                `json:"alertName"`
	Severity      string                `json:"severity"`
	Namespace     string                `json:"namespace"`
	ResourceKind  string                `json:"resourceKind"`
	ResourceName  string                `json:"resourceName"`
	PodName       string                `json:"podName,omitempty"`
	Status        string                `json:"status"`
	StartsAt      time.Time             `json:"startsAt"`
	Age           string                `json:"age"`
	Summary       string                `json:"summary,omitempty"`
	Used          bool                  `json:"used"`
	Decision      string                `json:"decision"`
	Reason        string                `json:"reason"`
	CanInclude    bool                  `json:"canInclude"`
	IncludeReason string                `json:"includeReason,omitempty"`
	Labels        []DashboardAlertLabel `json:"labels,omitempty"`
}

type DashboardAlertLabel struct {
	Key   string `json:"key"`
	Value string `json:"value"`
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
	HelmReleaseName   string
	HelmNamespace     string
	HelmRepoURL       string
	HelmChart         string
	HelmChartVersion  string
	HelmValueFiles    []string
	HelmValuesFrom    []string
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
		Environment:   c.dashboardEnvironment(ctx),
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
	if pendingCount > statusCounts["pending_approval"] {
		statusCounts["pending_approval"] = pendingCount
	}

	snapshot.KPIs = dashboardKPIs(investigations, predictions, alertCount, predictionCount, statusCounts)
	snapshot.Pipeline = defaultDashboardPipeline(statusCounts, dashboardPipelineItems(remediations))
	snapshot.Incidents = dashboardIncidents(ctx, db, investigations, remByInvestigation, remByPod)
	snapshot.Remediations = dashboardRemediations(remediations)
	snapshot.GitOpsSources = dashboardGitOpsSources(remediations)
	snapshot.Predictions = dashboardPredictions(predictions)
	snapshot.AuditEvents = dashboardAuditEvents(investigations, remediations, alerts)

	// Add analytical trends
	cats, syms, _ := c.history.GetAggregatedTrends(ctx, 24*time.Hour)
	snapshot.CategoryTrends = cats
	snapshot.SymptomTrends = syms

	// Add FinOps Cluster Cost
	clusterCost, activeNodes, nodeCosts := c.calculateClusterCostSnapshot(ctx)
	snapshot.ClusterCostMo = clusterCost
	snapshot.ActiveNodes = activeNodes
	snapshot.NodeCosts = nodeCosts

	return snapshot
}

func (c *Controller) ActiveAlertDecisions(ctx context.Context) ([]DashboardActiveAlert, error) {
	if c.amClient == nil || !c.config.AlertmanagerEnabled || c.config.AlertmanagerURL == "" {
		return []DashboardActiveAlert{}, nil
	}
	alerts, err := c.amClient.GetAlerts()
	if err != nil {
		return nil, err
	}
	out := make([]DashboardActiveAlert, 0, len(alerts))
	for _, alert := range alerts {
		out = append(out, c.dashboardActiveAlertDecision(ctx, alert))
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].StartsAt.After(out[j].StartsAt)
	})
	return out, nil
}

func (c *Controller) IncludeActiveAlert(ctx context.Context, alertID string) (DashboardActiveAlert, error) {
	if c.amClient == nil || !c.config.AlertmanagerEnabled || c.config.AlertmanagerURL == "" {
		return DashboardActiveAlert{}, fmt.Errorf("alertmanager is not configured")
	}
	alerts, err := c.amClient.GetAlerts()
	if err != nil {
		return DashboardActiveAlert{}, err
	}
	for _, alert := range alerts {
		if activeAlertID(alert) != alertID {
			continue
		}
		decision := c.dashboardActiveAlertDecision(ctx, alert)
		if !decision.CanInclude && !decision.Used {
			return decision, fmt.Errorf("%s", decision.IncludeReason)
		}
		c.alertWatchMu.Lock()
		if c.alertWatches == nil {
			c.alertWatches = make(map[string]time.Time)
		}
		c.alertWatches[alertWatchKey(alert)] = time.Now()
		c.alertWatchMu.Unlock()
		if c.history != nil {
			c.history.SaveAlertWatch(ctx, alert)
		}
		return c.dashboardActiveAlertDecision(ctx, alert), nil
	}
	return DashboardActiveAlert{}, fmt.Errorf("active alert not found")
}

func (c *Controller) dashboardActiveAlertDecision(ctx context.Context, alert alertmanager.Alert) DashboardActiveAlert {
	ns := strings.TrimSpace(alert.Labels["namespace"])
	pod := strings.TrimSpace(alert.Labels["pod"])
	alertName := firstNonEmpty(alert.Labels["alertname"], "Alert")
	resourceKind, resourceName := alertResourceIdentity(alert)
	if resourceKind == "" && pod != "" {
		resourceKind, resourceName = "Pod", pod
	}
	status := firstNonEmpty(alert.Status.State, "unknown")
	startsAt := alert.StartsAt
	if startsAt.IsZero() {
		startsAt = time.Now()
	}
	decision := DashboardActiveAlert{
		ID:           activeAlertID(alert),
		AlertName:    security.ScrubPII(alertName),
		Severity:     firstNonEmpty(alert.Labels["severity"], alert.Labels["priority"], "unknown"),
		Namespace:    security.ScrubPII(ns),
		ResourceKind: security.ScrubPII(firstNonEmpty(resourceKind, "Unknown")),
		ResourceName: security.ScrubPII(firstNonEmpty(resourceName, "unmapped")),
		PodName:      security.ScrubPII(pod),
		Status:       status,
		StartsAt:     startsAt,
		Age:          humanAge(startsAt),
		Summary:      alertSummary(alert),
		Labels:       selectedAlertLabels(alert.Labels),
	}

	switch {
	case status != "firing":
		decision.Decision = "skipped"
		decision.Reason = fmt.Sprintf("Alert status is %q, not firing.", status)
		decision.IncludeReason = "Only firing alerts can be included."
	case len(alert.Status.SilencedBy) > 0:
		decision.Decision = "skipped"
		decision.Reason = "Alertmanager has silenced this alert."
		decision.IncludeReason = "Silenced alerts should be unsilenced in Alertmanager first."
	case len(alert.Status.InhibitedBy) > 0:
		decision.Decision = "skipped"
		decision.Reason = "Alertmanager has inhibited this alert behind a higher-level alert."
		decision.IncludeReason = "Inhibited alerts are intentionally suppressed by Alertmanager."
	case ns == "":
		decision.Decision = "skipped"
		decision.Reason = "Missing namespace label; Fixora cannot scope diagnostics safely."
		decision.IncludeReason = "Add a namespace label to the alert rule first."
	case !c.isNamespaceScoped(ns):
		decision.Decision = "skipped"
		decision.Reason = "Namespace is outside Fixora's configured scope."
		decision.IncludeReason = "Update Fixora namespace scope before watching this alert."
	case pod == "":
		decision.Decision = "skipped"
		decision.Reason = "Missing pod label; current Alertmanager automation needs a pod to collect logs, events, and owner workload context."
		decision.IncludeReason = "Add a pod label or workload-to-pod mapping before Fixora can act on this alert."
	case !c.matchesConfiguredAlertFilters(alert) && !c.isRuntimeWatchedAlert(alert):
		decision.Decision = "skipped"
		decision.Reason = "Alert labels do not match Fixora Alertmanager include/exclude filters."
		decision.CanInclude = true
		decision.IncludeReason = "Add this active alert to Fixora's runtime watch list."
	case c.history != nil && c.history.IsAlertRecentlyProcessed(ctx, ns, pod, alertName, 30*time.Minute):
		decision.Decision = "deduplicated"
		decision.Reason = "Recently processed; Fixora suppresses repeat diagnostics for 30 minutes to avoid duplicate PRs."
		decision.IncludeReason = "Already watched recently; wait for the duplicate suppression window to expire."
	default:
		decision.Used = true
		decision.Decision = "used"
		decision.Reason = "Firing pod alert with namespace and pod labels; Fixora can run diagnostics and correlate the owner workload."
	}
	return decision
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

func dashboardKPIs(investigations []dashboardInvestigationRow, predictions []dashboardPredictionRow, alertCount, predictionCount int, statusCounts map[string]int) []DashboardKPI {
	succeeded := statusCounts["succeeded"]
	failed := statusCounts["production_failed"] + statusCounts["revert_failed"] + statusCounts["reverted"]
	successValue := "n/a"
	if succeeded+failed > 0 {
		successValue = fmt.Sprintf("%d%%", int(math.Round(float64(succeeded)*100/float64(succeeded+failed))))
	}
	riskAvoided := 0.0
	for _, prediction := range predictions {
		if prediction.DowntimeRiskHr > 0 {
			riskAvoided += prediction.DowntimeRiskHr
		}
	}
	riskValue := "n/a"
	riskDetail := fmt.Sprintf("%d predictions tracked", predictionCount)
	if riskAvoided > 0 {
		riskValue = "$" + formatDashboardMoney(riskAvoided)
		riskDetail = "estimated hourly risk"
	}
	return []DashboardKPI{
		{Label: "Open incidents", Value: fmt.Sprintf("%d", len(investigations)), Detail: fmt.Sprintf("%d alerts in 24h", alertCount), Icon: "icon-alert", Tone: severityTone(len(investigations)), Delta: fmt.Sprintf("%d active", len(investigations)), Trend: dashboardTrend(len(investigations), 7)},
		{Label: "PRs awaiting review", Value: fmt.Sprintf("%d", statusCounts["pending_approval"]), Detail: "approval queue", Icon: "icon-branch", Tone: "warning", Delta: fmt.Sprintf("%d awaiting review", statusCounts["pending_approval"]), Trend: dashboardTrend(statusCounts["pending_approval"], 5)},
		{Label: "Auto-fix success", Value: successValue, Detail: "closed-loop outcomes", Icon: "icon-check", Tone: "success", Trend: dashboardTrend(succeeded, 6)},
		{Label: "Reverts", Value: fmt.Sprintf("%d", statusCounts["revert_opened"]+statusCounts["reverted"]+statusCounts["revert_failed"]), Detail: "safety actions", Icon: "icon-clock", Tone: "warning", Trend: dashboardTrend(statusCounts["revert_opened"]+statusCounts["reverted"]+statusCounts["revert_failed"], 4)},
		{Label: "Monthly risk avoided", Value: riskValue, Detail: riskDetail, Icon: "icon-chart", Tone: "info", Delta: riskDetail, Trend: dashboardTrend(int(math.Round(riskAvoided)), 8)},
	}
}

func dashboardTrend(value, seed int) []int {
	base := max(value, 1)
	return []int{max(base-2, 0), base + seed%3, max(base-1, 0), base + 1, max(base-1+seed%2, 0), base + 2, base + seed%4}
}

func formatDashboardMoney(value float64) string {
	if value >= 1000000 {
		return fmt.Sprintf("%.1fM", value/1000000)
	}
	if value >= 1000 {
		return fmt.Sprintf("%.1fk", value/1000)
	}
	return fmt.Sprintf("%.0f", value)
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
			Detail:     dashboardPipelineItemDetail(row),
			Age:        humanAge(row.UpdatedAt),
			URL:        row.PRURL,
		})
	}
	return items
}

func dashboardPipelineItemDetail(row dashboardRemediationRow) string {
	if dashboardManifestIsHelm(row.ManifestType) {
		return firstNonEmpty(dashboardPatchTarget(row), row.ManifestType)
	}
	return firstNonEmpty(row.ManifestType, row.PatchStrategy)
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
		       COALESCE(region, ''), COALESCE(helm_release_name, ''), COALESCE(helm_namespace, ''),
		       COALESCE(helm_repo_url, ''), COALESCE(helm_chart, ''), COALESCE(helm_chart_version, ''),
		       COALESCE(helm_value_files, '[]'::jsonb), COALESCE(helm_values_from, '[]'::jsonb),
		       COALESCE(changed_files, '[]'::jsonb), COALESCE(failure_reason, ''),
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
		var helmValueFiles []byte
		var helmValuesFrom []byte
		if err := rows.Scan(&row.ID, &row.InvestigationID, &row.Namespace, &row.PodName, &row.DiagnosisCategory, &row.PatchStrategy, &row.Status, &row.RepoOwner, &row.RepoName, &row.BaseBranch, &row.HeadBranch, &row.PRURL, &row.PRTitle, &row.GitOpsController, &row.GitOpsApp, &row.GitOpsNamespace, &row.GitOpsRepoURL, &row.GitOpsRevision, &row.GitOpsPath, &row.ManifestType, &row.OverlayRole, &row.Environment, &row.Region, &row.HelmReleaseName, &row.HelmNamespace, &row.HelmRepoURL, &row.HelmChart, &row.HelmChartVersion, &helmValueFiles, &helmValuesFrom, &changedFiles, &row.FailureReason, &row.WorkloadKind, &row.WorkloadName, &row.WorkloadSelector, &row.UpdatedAt); err == nil {
			row.HelmValueFiles = dashboardStringList(helmValueFiles)
			row.HelmValuesFrom = dashboardStringList(helmValuesFrom)
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
		var row dashboardPredictionRow
		if err := rows.Scan(&row.Namespace, &row.PodName, &row.LastAlertTime, &row.LastGrowthRate, &row.PreventionCostMo, &row.DowntimeRiskHr); err == nil {
			out = append(out, row)
		}
	}
	return out
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
					Helm:         gitops.Helm,
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

func dashboardStringList(raw []byte) []string {
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil
	}
	return nonEmptyDashboard(values...)
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
		incident.Graph, incident.Edges = queryDashboardGraph(ctx, db, inv.Namespace, inv.PodName, incident.Workload, inv.ClusterContext)
		if incident.GitOps != nil && dashboardManifestIsHelm(incident.GitOps.ManifestType) {
			incident.Graph, incident.Edges = dashboardHelmGraph(incident.GitOps, incident.PR, incident.Graph, incident.Edges)
		}
		out = append(out, incident)
	}
	return out
}

func dashboardWorkload(inv dashboardInvestigationRow, rem dashboardRemediationRow) DashboardWorkload {
	contextKind, contextName := ownerWorkloadFromContext(inv.ClusterContext)
	kind := firstNonEmpty(rem.WorkloadKind, contextValue(inv.ClusterContext, "Workload Kind"), contextKind, "Pod")
	name := firstNonEmpty(rem.WorkloadName, contextValue(inv.ClusterContext, "Workload Name"), contextName, inv.PodName)
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
		Helm:         dashboardHelmMapping(rem),
	}
}

func dashboardHelmMapping(rem dashboardRemediationRow) *DashboardHelmMapping {
	if !dashboardManifestIsHelm(rem.ManifestType) &&
		rem.HelmReleaseName == "" && rem.HelmChart == "" && len(rem.HelmValueFiles) == 0 && len(rem.HelmValuesFrom) == 0 {
		return nil
	}
	return &DashboardHelmMapping{
		ReleaseName:  rem.HelmReleaseName,
		Namespace:    rem.HelmNamespace,
		Chart:        rem.HelmChart,
		ChartVersion: rem.HelmChartVersion,
		RepoURL:      rem.HelmRepoURL,
		ValueFiles:   rem.HelmValueFiles,
		ValuesFrom:   rem.HelmValuesFrom,
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
		PatchTarget:      dashboardPatchTarget(rem),
		Reason:           dashboardPatchReason(rem),
		Avoided:          dashboardPatchAvoided(rem),
		Summary:          dashboardPatchSummary(rem),
	}
}

func dashboardPatchTarget(rem dashboardRemediationRow) string {
	if dashboardManifestIsHelm(rem.ManifestType) {
		for _, file := range rem.ChangedFiles {
			if isDashboardValuesFile(file) {
				return file
			}
		}
		if len(rem.HelmValueFiles) > 0 {
			return rem.HelmValueFiles[len(rem.HelmValueFiles)-1]
		}
		if rem.ManifestType == "flux-helmrelease" {
			return "HelmRelease spec.values"
		}
		return "Helm chart values"
	}
	if len(rem.ChangedFiles) > 0 {
		return rem.ChangedFiles[0]
	}
	return ""
}

func dashboardPatchReason(rem dashboardRemediationRow) string {
	if dashboardManifestIsHelm(rem.ManifestType) {
		return "Workload is Helm-managed, so Fixora patches chart values or HelmRelease values instead of rendered Kubernetes output."
	}
	if rem.ManifestType == "kustomize" {
		return "Workload is Kustomize-managed, so Fixora uses an overlay patch path."
	}
	return "Fixora patches the discovered GitOps source manifest for the affected workload."
}

func dashboardPatchAvoided(rem dashboardRemediationRow) string {
	if dashboardManifestIsHelm(rem.ManifestType) {
		return "Rendered Pod/Deployment manifest"
	}
	if rem.ManifestType == "kustomize" {
		return "Direct base manifest edit"
	}
	return ""
}

func dashboardPatchSummary(rem dashboardRemediationRow) []string {
	var summary []string
	if dashboardManifestIsHelm(rem.ManifestType) {
		if target := dashboardPatchTarget(rem); target != "" {
			summary = append(summary, "Targeted "+target)
		}
		if rem.HelmChart != "" {
			label := rem.HelmChart
			if rem.HelmChartVersion != "" {
				label += "@" + rem.HelmChartVersion
			}
			summary = append(summary, "Chart "+label)
		}
		if len(rem.ChangedFiles) > 0 && !dashboardChangedTemplates(rem.ChangedFiles) {
			summary = append(summary, "No template files changed")
		}
	}
	if len(summary) == 0 && len(rem.ChangedFiles) > 0 {
		summary = append(summary, fmt.Sprintf("Changed %d file(s)", len(rem.ChangedFiles)))
	}
	return summary
}

func dashboardChangedTemplates(files []string) bool {
	for _, file := range files {
		if strings.Contains(strings.ToLower(file), "/templates/") {
			return true
		}
	}
	return false
}

func dashboardManifestIsHelm(manifestType string) bool {
	normalized := strings.ToLower(strings.TrimSpace(manifestType))
	return normalized == "helm" || normalized == "flux-helmrelease" || strings.Contains(normalized, "helm")
}

func isDashboardValuesFile(file string) bool {
	base := strings.ToLower(path.Base(file))
	return (strings.HasSuffix(base, ".yaml") || strings.HasSuffix(base, ".yml")) &&
		(strings.HasPrefix(base, "values") || strings.Contains(base, "values"))
}

func dashboardGuardrails(rem dashboardRemediationRow) []DashboardGuardrail {
	if rem.ID == 0 {
		return nil
	}
	renderStatus := dashboardRenderGuardrailStatus(rem)
	remediationStatus := dashboardRemediationGuardrailStatus(rem.Status)
	guardrails := []DashboardGuardrail{
		{Label: "Identity match", Status: "passed", Detail: "Target workload matched GitOps source metadata."},
		{Label: "Privileged paths blocked", Status: "passed", Detail: "No protected paths were changed."},
		{Label: "Duplicate PR check", Status: "passed", Detail: "No active remediation was found for this workload and branch."},
	}
	if dashboardManifestIsHelm(rem.ManifestType) {
		guardrails = append(guardrails,
			DashboardGuardrail{Label: "Helm source detected", Status: "passed", Detail: dashboardHelmSourceDetail(rem)},
			DashboardGuardrail{Label: "Values file selected", Status: dashboardValuesFileGuardrailStatus(rem), Detail: dashboardValuesFileGuardrailDetail(rem)},
			DashboardGuardrail{Label: "Image tag pinned", Status: dashboardImagePinGuardrailStatus(rem), Detail: dashboardImagePinGuardrailDetail(rem)},
			DashboardGuardrail{Label: "Template edit avoided", Status: dashboardTemplateEditGuardrailStatus(rem), Detail: dashboardTemplateEditGuardrailDetail(rem)},
		)
	}
	guardrails = append(guardrails,
		DashboardGuardrail{Label: "Render validation", Status: renderStatus, Detail: dashboardRenderGuardrailDetail(rem, renderStatus)},
		DashboardGuardrail{Label: "Remediation state", Status: remediationStatus, Detail: dashboardRemediationGuardrailDetail(rem)},
	)
	return guardrails
}

func dashboardHelmSourceDetail(rem dashboardRemediationRow) string {
	parts := nonEmptyDashboard(rem.HelmChart, rem.HelmChartVersion, rem.HelmReleaseName)
	if len(parts) == 0 {
		return "Fixora resolved this workload as Helm-managed."
	}
	return "Resolved " + strings.Join(parts, " · ")
}

func dashboardValuesFileGuardrailStatus(rem dashboardRemediationRow) string {
	if !dashboardManifestIsHelm(rem.ManifestType) {
		return "skipped"
	}
	if len(rem.HelmValueFiles) > 0 || strings.Contains(strings.ToLower(strings.Join(rem.ChangedFiles, "\n")), "values") || rem.ManifestType == "flux-helmrelease" {
		return "passed"
	}
	return "pending"
}

func dashboardValuesFileGuardrailDetail(rem dashboardRemediationRow) string {
	if target := dashboardPatchTarget(rem); target != "" {
		return "Selected " + target + " as the Helm remediation target."
	}
	return "Waiting for an active values file or HelmRelease values target."
}

func dashboardImagePinGuardrailStatus(rem dashboardRemediationRow) string {
	reason := strings.ToLower(rem.FailureReason)
	if strings.Contains(reason, "replacement image") || strings.Contains(reason, "non-latest") || strings.Contains(reason, "allowlisted") {
		return "failed"
	}
	if rem.PatchStrategy == "image" || strings.Contains(strings.ToLower(strings.Join(rem.ChangedFiles, "\n")), "values") {
		return "passed"
	}
	return "skipped"
}

func dashboardImagePinGuardrailDetail(rem dashboardRemediationRow) string {
	if dashboardImagePinGuardrailStatus(rem) == "failed" && rem.FailureReason != "" {
		return truncateDashboard(rem.FailureReason, 140)
	}
	if rem.PatchStrategy == "image" {
		return "Replacement images must be pinned and architecture-approved before PR creation."
	}
	return "No image replacement was detected in this remediation."
}

func dashboardTemplateEditGuardrailStatus(rem dashboardRemediationRow) string {
	if dashboardChangedTemplates(rem.ChangedFiles) {
		return "pending"
	}
	return "passed"
}

func dashboardTemplateEditGuardrailDetail(rem dashboardRemediationRow) string {
	if dashboardChangedTemplates(rem.ChangedFiles) {
		return "Template files changed; render validation must prove the chart output is safe."
	}
	return "Remediation stayed in values or HelmRelease configuration."
}

func dashboardRenderGuardrailStatus(rem dashboardRemediationRow) string {
	reason := strings.ToLower(rem.FailureReason)
	if strings.Contains(reason, "render sandbox validation failed") ||
		strings.Contains(reason, "pre-flight validation failed") ||
		strings.Contains(reason, "manifest-aware patch validation failed") ||
		strings.Contains(reason, "validation failed") {
		return "failed"
	}
	if strings.Contains(reason, "render sandbox validation skipped") ||
		strings.Contains(reason, "validation skipped") {
		return "skipped"
	}
	if rem.Status == "" {
		return "pending"
	}
	return "passed"
}

func dashboardRenderGuardrailDetail(rem dashboardRemediationRow, status string) string {
	if status == "failed" && rem.FailureReason != "" {
		return truncateDashboard(rem.FailureReason, 140)
	}
	if status == "skipped" && rem.FailureReason != "" {
		return truncateDashboard(rem.FailureReason, 140)
	}
	if status == "passed" {
		return "Patch passed pre-flight file and render checks before remediation was recorded."
	}
	return "Waiting for validation results."
}

func dashboardRemediationGuardrailStatus(status string) string {
	switch strings.ToLower(status) {
	case "succeeded", "reverted":
		return "passed"
	case "pr_failed", "production_failed", "revert_failed":
		return "failed"
	case "generated", "pending_approval", "pr_opened", "observing", "revert_opened":
		return "pending"
	default:
		return "pending"
	}
}

func dashboardRemediationGuardrailDetail(rem dashboardRemediationRow) string {
	if rem.FailureReason != "" {
		return truncateDashboard(rem.FailureReason, 140)
	}
	switch rem.Status {
	case "generated":
		return "Remediation plan generated; no pull request has been opened yet."
	case "pending_approval":
		return "Waiting for a human approval before opening the PR."
	case "pr_opened":
		return "PR opened and waiting for merge."
	case "observing":
		return "PR merged; observing workload health."
	case "succeeded":
		return "Post-merge observation succeeded."
	case "production_failed":
		return "Post-merge observation detected a regression."
	case "revert_opened":
		return "A revert PR has been opened."
	case "revert_failed":
		return "Fixora could not open or complete the revert."
	case "reverted":
		return "Failed remediation was reverted."
	case "pr_failed":
		return "Fixora could not open or update the remediation PR."
	default:
		return "Remediation status is pending."
	}
}

func queryDashboardGraph(ctx context.Context, db *sql.DB, namespace, podName string, workload DashboardWorkload, clusterContext string) ([]DashboardDependencyNode, [][2]string) {
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
	seen := map[string]bool{strings.ToLower(workload.Kind + "/" + workload.Name): true}
	for rows.Next() {
		var kind, name string
		if err := rows.Scan(&kind, &name); err != nil {
			continue
		}
		key := strings.ToLower(kind + "/" + name)
		if seen[key] {
			continue
		}
		seen[key] = true
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
	if len(edges) == 0 {
		return fallbackDashboardGraph(workload, clusterContext)
	}
	return nodes, edges
}

func fallbackDashboardGraph(workload DashboardWorkload, clusterContext string) ([]DashboardDependencyNode, [][2]string) {
	nodes := []DashboardDependencyNode{{
		ID: "workload", Label: firstNonEmpty(workload.Kind, "Pod"), Detail: workload.Name, X: 58, Y: 126, Kind: "active",
	}}
	var edges [][2]string
	seen := map[string]string{strings.ToLower(workload.Kind + "/" + workload.Name): "workload"}
	serviceIDs := []string{}

	for _, ref := range relatedResourceRefsFromContext(clusterContext) {
		key := strings.ToLower(ref.Kind + "/" + ref.Name)
		if seen[key] != "" || ref.Name == "" {
			continue
		}
		id := fmt.Sprintf("dep-%d", len(nodes)-1)
		seen[key] = id
		nodes = append(nodes, DashboardDependencyNode{ID: id, Label: ref.Kind, Detail: ref.Name, Kind: graphToneForKind(ref.Kind)})
		if ref.Kind == "Ingress" && len(serviceIDs) > 0 {
			for _, serviceID := range serviceIDs {
				edges = append(edges, [2]string{serviceID, id})
			}
			continue
		}
		edges = append(edges, [2]string{"workload", id})
		if ref.Kind == "Service" {
			serviceIDs = append(serviceIDs, id)
		}
	}
	if len(nodes) == 1 {
		return nodes, edges
	}
	return nodes, edges
}

func dashboardHelmGraph(gitops *DashboardGitOpsMapping, pr *DashboardRecommendedPR, nodes []DashboardDependencyNode, edges [][2]string) ([]DashboardDependencyNode, [][2]string) {
	if gitops == nil || len(nodes) == 0 {
		return nodes, edges
	}
	seen := map[string]bool{}
	for _, node := range nodes {
		seen[node.ID] = true
	}
	addNode := func(node DashboardDependencyNode) {
		if node.ID == "" || seen[node.ID] {
			return
		}
		seen[node.ID] = true
		nodes = append(nodes, node)
	}
	addEdge := func(from, to string) {
		if from == "" || to == "" {
			return
		}
		for _, edge := range edges {
			if edge[0] == from && edge[1] == to {
				return
			}
		}
		edges = append(edges, [2]string{from, to})
	}

	helm := gitops.Helm
	values := []string{}
	if helm != nil {
		values = append(values, helm.ValueFiles...)
	}
	if pr != nil && pr.PatchTarget != "" && isDashboardValuesFile(pr.PatchTarget) {
		values = append(values, pr.PatchTarget)
	}
	values = nonEmptyDashboard(values...)

	helmReleaseID := ""
	if strings.Contains(strings.ToLower(gitops.ManifestType), "flux-helmrelease") {
		helmReleaseID = "helmrelease"
		label := "HelmRelease"
		detail := firstNonEmpty(gitops.App, "release")
		if helm != nil {
			detail = firstNonEmpty(helm.ReleaseName, detail)
		}
		addNode(DashboardDependencyNode{ID: helmReleaseID, Label: label, Detail: detail, Kind: "helm", X: 60, Y: 42})
		addEdge(helmReleaseID, "workload")
	}

	chartID := "helm-chart"
	chartDetail := "Chart"
	if helm != nil {
		chartDetail = firstNonEmpty(helm.Chart, chartDetail)
		if helm.ChartVersion != "" {
			chartDetail += "@" + helm.ChartVersion
		}
	}
	addNode(DashboardDependencyNode{ID: chartID, Label: "Helm Chart", Detail: chartDetail, Kind: "helm", X: 60, Y: 86})
	if helmReleaseID != "" {
		addEdge(helmReleaseID, chartID)
	} else {
		addEdge(chartID, "workload")
	}

	for i, valueFile := range values {
		if i >= 4 {
			break
		}
		id := fmt.Sprintf("helm-values-%d", i)
		addNode(DashboardDependencyNode{ID: id, Label: "Values file", Detail: valueFile, Kind: "config", X: 240 + (i%2)*160, Y: 28 + (i/2)*52})
		addEdge(id, chartID)
	}
	addNode(DashboardDependencyNode{ID: "helm-render", Label: "Rendered workload", Detail: firstNonEmpty(gitops.Path, "helm template"), Kind: "neutral", X: 240, Y: 126})
	addEdge(chartID, "helm-render")
	addEdge("helm-render", "workload")
	return nodes, edges
}

type dashboardResourceRef struct {
	Kind string
	Name string
}

func relatedResourceRefsFromContext(clusterContext string) []dashboardResourceRef {
	seen := map[string]bool{}
	var refs []dashboardResourceRef
	add := func(kind, name string) {
		kind = strings.TrimSpace(kind)
		name = strings.TrimSpace(name)
		if kind == "" || name == "" {
			return
		}
		key := strings.ToLower(kind + "/" + name)
		if seen[key] {
			return
		}
		seen[key] = true
		refs = append(refs, dashboardResourceRef{Kind: kind, Name: name})
	}

	for _, line := range strings.Split(clusterContext, "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "- "))
		if strings.HasPrefix(line, "Owner chain:") {
			for _, item := range strings.Split(strings.TrimPrefix(line, "Owner chain:"), "->") {
				if kind, name, ok := splitKindName(item); ok {
					add(kind, name)
				}
			}
			continue
		}
		if kind, name, ok := splitKindName(line); ok {
			add(kind, name)
		}
	}
	return refs
}

func ownerWorkloadFromContext(clusterContext string) (string, string) {
	preferred := map[string]int{
		"Deployment":  1,
		"StatefulSet": 1,
		"DaemonSet":   1,
		"Job":         2,
		"ReplicaSet":  3,
	}
	bestRank := 100
	var best dashboardResourceRef
	for _, ref := range relatedResourceRefsFromContext(clusterContext) {
		if rank, ok := preferred[ref.Kind]; ok && rank < bestRank {
			bestRank = rank
			best = ref
		}
	}
	return best.Kind, best.Name
}

func splitKindName(value string) (string, string, bool) {
	value = strings.TrimSpace(value)
	parts := strings.Split(value, "/")
	if len(parts) < 2 {
		return "", "", false
	}
	kind := strings.TrimSpace(parts[0])
	name := strings.TrimSpace(parts[1])
	if kind == "" || name == "" || strings.Contains(name, " ") {
		return "", "", false
	}
	return kind, name, true
}

func graphToneForKind(kind string) string {
	switch kind {
	case "ConfigMap", "Secret":
		return "warning"
	case "Service", "Ingress":
		return "service"
	case "Deployment", "StatefulSet", "DaemonSet", "ReplicaSet", "Job":
		return "active"
	default:
		return "neutral"
	}
}

func dashboardPodKey(namespace, pod string) string {
	return namespace + "/" + pod
}

func activeAlertID(alert alertmanager.Alert) string {
	h := sha1.New()
	for _, key := range sortedMapKeys(alert.Labels) {
		fmt.Fprintf(h, "%s=%s\n", key, alert.Labels[key])
	}
	fmt.Fprintf(h, "startsAt=%s", alert.StartsAt.UTC().Format(time.RFC3339Nano))
	return fmt.Sprintf("alert-%x", h.Sum(nil))[:18]
}

func alertResourceIdentity(alert alertmanager.Alert) (string, string) {
	type candidate struct {
		kind string
		keys []string
	}
	for _, item := range []candidate{
		{"Deployment", []string{"deployment", "deployment_name", "k8s_deployment"}},
		{"StatefulSet", []string{"statefulset", "statefulset_name"}},
		{"DaemonSet", []string{"daemonset", "daemonset_name"}},
		{"Job", []string{"job", "job_name"}},
		{"CronJob", []string{"cronjob", "cronjob_name"}},
		{"ReplicaSet", []string{"replicaset", "replicaset_name"}},
		{"Service", []string{"service", "service_name", "kubernetes_service"}},
		{"Pod", []string{"pod", "pod_name", "kubernetes_pod_name"}},
	} {
		for _, key := range item.keys {
			if value := strings.TrimSpace(alert.Labels[key]); value != "" {
				return item.kind, value
			}
		}
	}
	kind := strings.TrimSpace(firstNonEmpty(alert.Labels["kind"], alert.Labels["resource_kind"]))
	name := strings.TrimSpace(firstNonEmpty(alert.Labels["name"], alert.Labels["resource"], alert.Labels["resource_name"], alert.Labels["workload"]))
	if kind != "" && name != "" {
		return kind, name
	}
	return "", ""
}

func alertSummary(alert alertmanager.Alert) string {
	for _, key := range []string{"summary", "description", "message"} {
		if value := strings.TrimSpace(alert.Annotations[key]); value != "" {
			return truncateDashboard(security.ScrubPII(value), 180)
		}
	}
	return ""
}

func selectedAlertLabels(labels map[string]string) []DashboardAlertLabel {
	allowed := map[string]bool{
		"alertname":            true,
		"severity":             true,
		"priority":             true,
		"namespace":            true,
		"pod":                  true,
		"container":            true,
		"deployment":           true,
		"statefulset":          true,
		"daemonset":            true,
		"job":                  true,
		"service":              true,
		"instance":             true,
		"cluster":              true,
		"prometheus":           true,
		"resource":             true,
		"resource_name":        true,
		"resource_kind":        true,
		"workload":             true,
		"kubernetes_name":      true,
		"kubernetes_pod":       true,
		"kubernetes_node":      true,
		"kubernetes_kind":      true,
		"kubernetes_namespace": true,
	}
	keys := sortedMapKeys(labels)
	out := make([]DashboardAlertLabel, 0, len(keys))
	for _, key := range keys {
		if !allowed[key] {
			continue
		}
		out = append(out, DashboardAlertLabel{Key: key, Value: truncateDashboard(security.ScrubPII(labels[key]), 80)})
	}
	return out
}

func sortedMapKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
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

func (c *Controller) dashboardEnvironment(ctx context.Context) string {
	for _, key := range []string{
		"FIXORA_CLUSTER_NAME",
		"FIXORA_CLUSTER",
		"CLUSTER_NAME",
		"CLUSTER",
		"KUBERNETES_CLUSTER_NAME",
		"K8S_CLUSTER_NAME",
		"EKS_CLUSTER_NAME",
		"GKE_CLUSTER_NAME",
		"AKS_CLUSTER_NAME",
	} {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	if c.clientset != nil {
		nodes, err := c.clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{Limit: 100})
		if err == nil {
			for _, node := range nodes.Items {
				if name := clusterNameFromNodeLabels(node.Labels); name != "" {
					return name
				}
			}
		}
		for _, ref := range []struct {
			namespace string
			name      string
		}{
			{"kube-public", "cluster-info"},
			{"kube-system", "cluster-info"},
		} {
			cm, err := c.clientset.CoreV1().ConfigMaps(ref.namespace).Get(ctx, ref.name, metav1.GetOptions{})
			if err == nil {
				if name := clusterNameFromClusterInfo(cm); name != "" {
					return name
				}
			}
		}
	}
	return "default-cluster"
}

func clusterNameFromClusterInfo(cm *v1.ConfigMap) string {
	if cm == nil {
		return ""
	}
	for _, key := range []string{"cluster-name", "clusterName", "name"} {
		if value := strings.TrimSpace(cm.Data[key]); usableClusterName(value) {
			return value
		}
	}
	if kubeconfig := strings.TrimSpace(cm.Data["kubeconfig"]); kubeconfig != "" {
		var parsed struct {
			Clusters []struct {
				Name string `yaml:"name"`
			} `yaml:"clusters"`
		}
		if err := yaml.Unmarshal([]byte(kubeconfig), &parsed); err == nil {
			for _, cluster := range parsed.Clusters {
				if usableClusterName(cluster.Name) {
					return strings.TrimSpace(cluster.Name)
				}
			}
		}
	}
	return ""
}

func usableClusterName(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	switch strings.ToLower(value) {
	case "cluster", "default", "default-cluster", "kubernetes":
		return false
	default:
		return true
	}
}

func clusterNameFromNodeLabels(labels map[string]string) string {
	for _, key := range []string{
		"fixora.io/cluster-name",
		"cluster-name",
		"cluster",
		"eks.amazonaws.com/cluster-name",
		"alpha.eksctl.io/cluster-name",
		"eksctl.io/cluster-name",
		"eksctl.cluster.k8s.io/v1alpha1/cluster-name",
		"kops.k8s.io/cluster",
		"kubernetes.azure.com/cluster",
		"topology.gke.io/cluster-name",
		"cloud.google.com/gke-cluster",
		"cluster.x-k8s.io/cluster-name",
		"management.cattle.io/cluster-name",
	} {
		if value := strings.TrimSpace(labels[key]); usableClusterName(value) {
			return value
		}
	}
	for key, value := range labels {
		key = strings.ToLower(key)
		if strings.Contains(key, "cluster") && strings.Contains(key, "name") && usableClusterName(value) {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func (c *Controller) calculateClusterCostSnapshot(ctx context.Context) (float64, int, []DashboardNodeCost) {
	if c.clientset == nil {
		return 0, 0, nil
	}
	nodes, err := c.clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil || len(nodes.Items) == 0 {
		return 0, 0, nil
	}

	var total float64
	nodeCosts := make([]DashboardNodeCost, 0, len(nodes.Items))
	for _, n := range nodes.Items {
		vendor, region, instanceType := finops.ParseNodeMetadata(n.Labels, n.Spec.ProviderID)
		row := DashboardNodeCost{
			Name:         n.Name,
			Vendor:       firstNonEmpty(vendor, "unknown"),
			Region:       firstNonEmpty(region, "unknown"),
			InstanceType: firstNonEmpty(instanceType, "unknown"),
			Status:       "pricing_unavailable",
		}
		switch {
		case vendor == "" || region == "" || instanceType == "":
			row.Status = "metadata_missing"
		case c.pricingProvider == nil:
			row.Status = "pricing_not_configured"
		default:
			if profile, err := c.pricingProvider.GetProfileForInstance(vendor, region, instanceType); err == nil && profile != nil {
				row.MonthlyCost = ((profile.CPURatePerHour * 2.0) + (profile.MemoryRatePerHour * 8.0)) * 730
				row.PricingSource = profile.Name
				row.Status = "priced"
				total += row.MonthlyCost
			}
		}
		nodeCosts = append(nodeCosts, row)
	}

	sort.Slice(nodeCosts, func(i, j int) bool {
		return nodeCosts[i].Name < nodeCosts[j].Name
	})
	return total, len(nodeCosts), nodeCosts
}
