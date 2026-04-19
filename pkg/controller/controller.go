package controller

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"

	"fixora/pkg/ai"
	"fixora/pkg/alertmanager"
	"fixora/pkg/argocd"
	"fixora/pkg/config"
	"fixora/pkg/finops"
	"fixora/pkg/metrics"
	"fixora/pkg/notifications"
	"fixora/pkg/prometheus"
	"fixora/pkg/vcs"
	giturls "github.com/chainguard-dev/git-urls"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	metricsclientset "k8s.io/metrics/pkg/client/clientset/versioned"
)

// podWorkItem represents a unit of diagnostic work for a specific pod.
type podWorkItem struct {
	namespace string
	name      string
	reason    string
}

// Controller is the core engine of Fixora. It watches for pod failures,
// gathers forensic evidence from metrics/logs, and triggers AI analysis.

type PendingFix struct {
	Options      vcs.PullRequestOptions
	VCSType      string
	PodNamespace string
	PodName      string
}

type Controller struct {
	clientset    kubernetes.Interface
	factory      informers.SharedInformerFactory
	config       *config.Config
	promClient   metrics.MetricsProvider
	amClient     *alertmanager.Client
	argoClient   *argocd.Client
	aiProvider   ai.Provider
	ghProvider   *vcs.GitHubProvider
	glProvider   *vcs.GitLabProvider
	queue        workqueue.RateLimitingInterface
	history      *historyCache
	pendingFixes map[string]PendingFix
	pendingMu    sync.Mutex
}

// NewController initializes a new diagnostic controller with all required clients.
func NewController(clientset kubernetes.Interface, dynamicClient dynamic.Interface, metricsClient metricsclientset.Interface, cfg *config.Config) *Controller {
	factory := informers.NewSharedInformerFactory(clientset, time.Second*30)

	var primary metrics.MetricsProvider
	if cfg.PrometheusURL != "" {
		var err error
		primary, err = prometheus.New(cfg.PrometheusURL)
		if err != nil {
			slog.Error("Failed to create Prometheus client", "error", err)
		}
	}

	var secondary metrics.MetricsProvider
	if metricsClient != nil {
		secondary = metrics.NewK8sMetricsProvider(clientset, metricsClient)
	}

	var promClient metrics.MetricsProvider
	if primary != nil && secondary != nil {
		promClient = metrics.NewFallbackProvider(primary, secondary)
	} else if primary != nil {
		promClient = primary
	} else if secondary != nil {
		promClient = secondary
	}

	var amClient *alertmanager.Client
	if cfg.AlertmanagerURL != "" {
		amClient = alertmanager.New(cfg.AlertmanagerURL)
	}

	var argoClient *argocd.Client
	if cfg.ArgoCDEnabled {
		argoClient = argocd.New(dynamicClient, cfg.ArgoCDNamespace, cfg.ArgoCDURL, cfg.ArgoCDToken)
	}

	var aiProvider ai.Provider
	if cfg.AIProvider != "" && cfg.AIAPIKey != "" {
		aiProvider, _ = ai.NewProvider(cfg.AIProvider, cfg.AIAPIKey, cfg.AIModel)
	}

	var ghProvider *vcs.GitHubProvider
	if cfg.GitHubToken != "" {
		ghProvider = vcs.NewGitHubProvider(cfg.GitHubToken)
	}

	var glProvider *vcs.GitLabProvider
	if cfg.GitLabToken != "" {
		glProvider, _ = vcs.NewGitLabProvider(cfg.GitLabToken, cfg.GitLabBaseURL)
	}

	return &Controller{
		clientset:    clientset,
		factory:      factory,
		config:       cfg,
		promClient:   promClient,
		amClient:     amClient,
		argoClient:   argoClient,
		aiProvider:   aiProvider,
		ghProvider:   ghProvider,
		glProvider:   glProvider,
		queue:        workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "fixora"),
		history:      newHistoryCache(cfg),
		pendingFixes: make(map[string]PendingFix),
	}
}

// Run starts the controller workers and informers.
func (c *Controller) Run(stopCh <-chan struct{}) {
	defer c.queue.ShutDown()

	podInformer := c.factory.Core().V1().Pods().Informer()
	if !c.config.AlertmanagerEnabled {
		podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
			UpdateFunc: func(oldObj, newObj interface{}) {
				c.enqueuePod(newObj)
			},
		})
	}

	c.factory.Start(stopCh)
	if !cache.WaitForCacheSync(stopCh, podInformer.HasSynced) {
		return
	}

	// Start diagnostic workers
	go wait.Until(c.runWorker, time.Second, stopCh)

	// Start predictive leak scanner if enabled
	if c.config.PredictiveEnabled {
		go wait.Until(c.scanForLeaks, c.config.PredictiveScanInterval, stopCh)
	}

	<-stopCh
}

// scanForLeaks periodically checks all running pods for consistent memory growth patterns.
// It actively analyzes incident history alongside Prometheus metrics to predict time-to-OOM.
func (c *Controller) scanForLeaks() {
	if c.promClient == nil {
		return
	}

	slog.Info("Scanning for memory leak trajectories")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pods, err := c.clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		slog.Error("Failed to list pods for leak scan", "error", err)
		return
	}

	for _, pod := range pods.Items {
		if pod.Status.Phase != v1.PodRunning {
			continue
		}

		matrix, err := c.promClient.GetHistory(pod.Namespace, pod.Name, 1*time.Hour)
		if err != nil || len(matrix) == 0 || len(matrix[0].Values) < c.config.PredictiveMinDataPoints {
			continue
		}

		values := matrix[0].Values
		first := float64(values[0].Value)
		last := float64(values[len(values)-1].Value)

		growthRate := (last - first) / first

		// Flag pods with growth exceeding threshold
		if first > 0 && growthRate > c.config.PredictiveGrowthThreshold {
			// Cooldown logic: don't re-alert for 4 hours unless growth rate increases significantly (>50% increase)
			state, exists := c.history.GetPredictionState(ctx, pod.Namespace, pod.Name)
			if exists {
				cooldown := 4 * time.Hour
				timeSinceLast := time.Since(state.LastAlertTime)
				growthIncrease := (growthRate - state.LastGrowthRate) / state.LastGrowthRate

				if timeSinceLast < cooldown && growthIncrease < 0.50 {
					slog.Debug("Suppressing duplicate leak alert (cooldown/insignificant growth)", "pod", pod.Name, "time_since", timeSinceLast, "growth_increase", growthIncrease)
					continue
				}
			}

			var metricProof strings.Builder
			metricProof.WriteString(fmt.Sprintf("Memory Growth: %.1f%% in the last hour.\n", growthRate*100))
			metricProof.WriteString(fmt.Sprintf("Current Usage: %.2f MiB.\n", last/1024/1024))

			// Fetch more granular metrics if supported
			rss, cache := c.getGranularMetrics(pod.Namespace, pod.Name)
			if rss > 0 || cache > 0 {
				metricProof.WriteString(fmt.Sprintf("RSS: %.2f MiB, Cache: %.2f MiB.\n", rss/1024/1024, cache/1024/1024))
			}

			// Predict time-to-OOM based on limits and requests
			request, limit, err := c.promClient.GetPodLimits(pod.Namespace, pod.Name)

			var hoursToOOM float64
			if err == nil && limit > 0 {
				metricProof.WriteString(fmt.Sprintf("Memory Limit: %.2f MiB.\n", limit/1024/1024))
				if last < limit {
					bytesPerHour := last - first
					if bytesPerHour > 0 {
						hoursToOOM = (limit - last) / bytesPerHour
						metricProof.WriteString(fmt.Sprintf("Estimated Time-to-OOM: %.1f hours.\n", hoursToOOM))
					}
				} else {
					metricProof.WriteString("Warning: Pod is currently EXCEEDING its memory limit.\n")
				}
			}

			if err == nil && request > 0 {
				metricProof.WriteString(fmt.Sprintf("Memory Request: %.2f MiB.\n", request/1024/1024))
				if last > request {
					metricProof.WriteString(fmt.Sprintf("Warning: Pod is exceeding its Request by %.2f MiB (Risk of eviction if node is under pressure).\n", (last-request)/1024/1024))
				}
			}

			// Analyze incident history
			historySummary := "This is the first time we've analyzed this pod."
			var historyStr string
			if hist, ok := c.history.Get(ctx, pod.Namespace, pod.Name); ok {
				oomCount := 0
				var sb strings.Builder
				for _, inc := range hist.Incidents {
					if inc.Reason == "OOMKilled" || inc.Reason == "CrashLoopBackOff" {
						oomCount++
					}
					sb.WriteString(fmt.Sprintf("- [%s] Reason: %s, RootCause: %s\n", inc.Timestamp.Format(time.RFC3339), inc.Reason, inc.RootCause))
				}
				historyStr = sb.String()
				if oomCount > 0 {
					historySummary = fmt.Sprintf("Recurring Issue: This pod has historically crashed %d times (OOMKilled/CrashLoopBackOff). High risk of recurrence.", oomCount)
				} else {
					historySummary = fmt.Sprintf("History Analysis: The pod has %d prior incidents.", len(hist.Incidents))
				}
			}

			clusterCtx := fmt.Sprintf("Namespace: %s, Pod: %s, Status: Predictive Warning (Potential OOM Trajectory)", pod.Namespace, pod.Name)

			evidence := notifications.EvidenceChain{
				MetricProof:         metricProof.String(),
				ClusterContext:      clusterCtx,
				HistoricalPattern:   historySummary,
				PredictiveWarning:   true,
				EstimatedHoursToOOM: hoursToOOM,
			}

			// Gathers Kubernetes events
			events, err := c.getPodEvents(ctx, &pod)
			if err == nil {
				evidence.EventTimeline = events
			}

			// Execute Multi-Modal AI Forensics
			if c.aiProvider != nil {
				rootCause, err := c.aiProvider.PerformPredictiveForensics(ctx, pod.Namespace, pod.Name, historyStr, evidence.MetricProof)
				if err != nil {
					slog.Error("AI Predictive Forensics failed", "pod", pod.Name, "error", err)
					evidence.RootCause = "Predictive analysis failed: " + err.Error()
				} else {
					evidence.RootCause = rootCause
					// Save the prediction to history
					c.history.Update(ctx, pod.Namespace, pod.Name, "LeakPrediction", rootCause)
				}
			} else {
				evidence.RootCause = "AI Provider not configured"
			}

			// FinOps Impact estimation for leaks
			cpuReq, _, _ := c.promClient.GetPodCPULimits(pod.Namespace, pod.Name)
			memReq, _, _ := c.promClient.GetPodLimits(pod.Namespace, pod.Name)
			currentCost := finops.CalculateMonthlyCost(cpuReq, memReq, finops.AWSDefaultProfile)
			// Assume AI fix will increase memory by 20% or 256MiB for leak prevention if OOM is imminent
			newCost := finops.CalculateMonthlyCost(cpuReq, memReq+256*1024*1024, finops.AWSDefaultProfile)
			evidence.FinOpsImpact = fmt.Sprintf("%s AWS compute cost vs. preventing a $5,000 outage", finops.FormatImpact(currentCost, newCost, "$"))

			slog.Warn("Potential memory leak detected", "namespace", pod.Namespace, "pod", pod.Name, "increase_pct", growthRate*100)
			notifications.SendEvidenceChain(c.config, evidence)

			// Update prediction state to handle cooldowns
			c.history.UpdatePredictionState(ctx, pod.Namespace, pod.Name, time.Now(), growthRate)

			// Attempt automated remediation for predicted leaks
			c.handleRemediation(ctx, &pod, evidence)
		}
	}
}

// enqueuePod filters pod updates and adds problematic pods to the work queue.
func (c *Controller) enqueuePod(obj interface{}) {
	pod := obj.(*v1.Pod)
	for _, containerStatus := range pod.Status.ContainerStatuses {
		reason := ""
		if containerStatus.State.Waiting != nil && containerStatus.State.Waiting.Reason == "CrashLoopBackOff" {
			reason = "CrashLoopBackOff"
		} else if containerStatus.State.Terminated != nil && containerStatus.State.Terminated.Reason == "OOMKilled" {
			reason = "OOMKilled"
		}

		if reason != "" {
			c.queue.Add(podWorkItem{
				namespace: pod.Namespace,
				name:      pod.Name,
				reason:    reason,
			})
		}
	}
}

func (c *Controller) runWorker() {
	for c.processNextItem() {
	}
}

func (c *Controller) processNextItem() bool {
	item, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(item)

	work := item.(podWorkItem)
	err := c.processDiagnostic(work)
	if err != nil {
		slog.Error("Diagnostic failed", "pod", work.name, "error", err)
		c.queue.AddRateLimited(item)
	} else {
		c.queue.Forget(item)
	}

	return true
}

func (c *Controller) processDiagnostic(work podWorkItem) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pod, err := c.clientset.CoreV1().Pods(work.namespace).Get(ctx, work.name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	c.diagnosePod(ctx, pod, work.reason)
	return nil
}

// DiagnosePodByName allows manual or Alertmanager-driven diagnostic triggers.
func (c *Controller) DiagnosePodByName(namespace, podName, reason string) {
	c.queue.Add(podWorkItem{
		namespace: namespace,
		name:      podName,
		reason:    reason,
	})
}

// diagnosePod performs the full forensic evidence gathering and AI correlation.
func (c *Controller) diagnosePod(ctx context.Context, pod *v1.Pod, reason string) {
	slog.Info("Diagnosing Pod", "namespace", pod.Namespace, "name", pod.Name, "reason", reason)

	var historyStr string
	historySummary := "This is the first time we've diagnosed this pod."
	if prev, exists := c.history.Get(ctx, pod.Namespace, pod.Name); exists {
		historySummary = fmt.Sprintf("Recurring Issue: This pod has crashed %d times previously.", len(prev.Incidents))
		var sb strings.Builder
		for _, inc := range prev.Incidents {
			sb.WriteString(fmt.Sprintf("- [%s] Reason: %s, RootCause: %s\n", inc.Timestamp.Format(time.RFC3339), inc.Reason, inc.RootCause))
			if inc.AppliedFix != "" {
				sb.WriteString(fmt.Sprintf("  Applied Fix:\n%s\n", inc.AppliedFix))
			}
		}
		historyStr = sb.String()
	}

	evidence := notifications.EvidenceChain{
		ClusterContext:    fmt.Sprintf("Namespace: %s, Pod: %s, Reason: %s", pod.Namespace, pod.Name, reason),
		HistoricalPattern: historySummary,
	}

	// Gathers related alerts from Alertmanager API
	if c.amClient != nil {
		alerts, err := c.amClient.GetAlertsForPod(pod.Namespace, pod.Name)
		if err == nil && len(alerts) > 0 {
			var alertDetails []string
			for _, a := range alerts {
				alertDetails = append(alertDetails, fmt.Sprintf("%s (%s)", a.Labels["alertname"], a.Status.State))
			}
			evidence.ClusterContext += fmt.Sprintf("\nActive Alerts: %s", strings.Join(alertDetails, ", "))
		}
	}

	// Gathers memory metrics for proof
	if c.promClient != nil {
		usage, _ := c.promClient.GetPodUsage(pod.Namespace, pod.Name)
		request, limit, _ := c.promClient.GetPodLimits(pod.Namespace, pod.Name)

		rss, cache := c.getGranularMetrics(pod.Namespace, pod.Name)

		metricSource := "Prometheus"
		_, err := c.promClient.GetHistory(pod.Namespace, pod.Name, time.Hour)
		if err != nil {
			metricSource = "K8s API (Historical trend unavailable)"
		}

		evidence.MetricProof = fmt.Sprintf("Metric Source: %s\nMemory Usage: %.2f MiB (RSS: %.2f, Cache: %.2f)\nLimit: %.2f MiB, Request: %.2f MiB",
			metricSource, usage/1024/1024, rss/1024/1024, cache/1024/1024, limit/1024/1024, request/1024/1024)
	}

	// Gathers Kubernetes events
	events, err := c.getPodEvents(ctx, pod)
	if err == nil {
		evidence.EventTimeline = events
	}

	// Execute Multi-Modal AI Forensics
	var rootCause string
	if c.aiProvider != nil {
		logs, err := c.getPodLogs(ctx, pod)
		if err != nil {
			slog.Warn("Error fetching logs", "pod", pod.Name, "error", err)
		}

		forensicCtx := ai.ForensicContext{
			Namespace: pod.Namespace,
			PodName:   pod.Name,
			Reason:    reason,
			Logs:      logs,
			Events:    events,
			Metrics:   evidence.MetricProof,
			History:   historyStr,
		}

		rootCause, err = c.aiProvider.PerformForensics(ctx, forensicCtx)
		if err != nil {
			slog.Error("AI Forensics failed", "pod", pod.Name, "error", err)
			evidence.RootCause = "Forensic analysis failed: " + err.Error()
		} else {
			evidence.RootCause = rootCause
			c.history.Update(ctx, pod.Namespace, pod.Name, reason, rootCause)
		}
	} else {
		evidence.RootCause = "AI Provider not configured"
	}

	// FinOps Impact estimation for OOMKilled/CrashLoopBackOff
	if c.promClient != nil {
		cpuReq, _, _ := c.promClient.GetPodCPULimits(pod.Namespace, pod.Name)
		memReq, _, _ := c.promClient.GetPodLimits(pod.Namespace, pod.Name)
		currentCost := finops.CalculateMonthlyCost(cpuReq, memReq, finops.AWSDefaultProfile)
		// Assume AI fix will increase memory by 256MiB
		newCost := finops.CalculateMonthlyCost(cpuReq, memReq+256*1024*1024, finops.AWSDefaultProfile)
		evidence.FinOpsImpact = fmt.Sprintf("%s AWS compute cost vs. preventing a $5,000 outage", finops.FormatImpact(currentCost, newCost, "$"))
	} else {
		evidence.FinOpsImpact = "+$2.10/mo AWS compute cost vs. preventing a $5,000 outage"
	}

	// Sends the report to Slack
	notifications.SendEvidenceChain(c.config, evidence)

	// Attempts automated remediation
	c.handleRemediation(ctx, pod, evidence)
}

// getGranularMetrics attempts to fetch RSS and Cache metrics from the provider.
func (c *Controller) getGranularMetrics(ns, pod string) (rss, cache float64) {
	type granular interface {
		GetPodMemoryRSS(ns, pod string) (float64, error)
		GetPodMemoryCache(ns, pod string) (float64, error)
	}

	// Direct check
	if g, ok := c.promClient.(granular); ok {
		rss, _ = g.GetPodMemoryRSS(ns, pod)
		cache, _ = g.GetPodMemoryCache(ns, pod)
		return
	}

	// FallbackProvider check
	if fb, ok := c.promClient.(*metrics.FallbackProvider); ok {
		if g, ok := fb.Primary.(granular); ok {
			rss, _ = g.GetPodMemoryRSS(ns, pod)
			cache, _ = g.GetPodMemoryCache(ns, pod)
			if rss > 0 || cache > 0 {
				return
			}
		}
		if g, ok := fb.Secondary.(granular); ok {
			rss, _ = g.GetPodMemoryRSS(ns, pod)
			cache, _ = g.GetPodMemoryCache(ns, pod)
		}
	}
	return
}

// handleRemediation attempts to open a Pull Request with a fix by discovering the pod's source repository.
func (c *Controller) handleRemediation(ctx context.Context, pod *v1.Pod, evidence notifications.EvidenceChain) {
	var repoURL, filePath, vcsType, targetRevision string

	// Attempt discovery via ArgoCD API/CRD
	if c.argoClient != nil {
		for _, owner := range pod.OwnerReferences {
			if owner.Kind == "ReplicaSet" {
				rs, err := c.clientset.AppsV1().ReplicaSets(pod.Namespace).Get(ctx, owner.Name, metav1.GetOptions{})
				if err == nil {
					for _, rsOwner := range rs.OwnerReferences {
						info, err := c.argoClient.GetAppForResource(ctx, pod.Namespace, rsOwner.Name, rsOwner.Kind)
						if err == nil {
							slog.Info("Discovered Git info via ArgoCD", "app", rsOwner.Name, "repo", info.RepoURL)
							repoURL = info.RepoURL
							filePath = info.Path + "/values.yaml"
							targetRevision = info.TargetRevision
							break
						}
					}
				}
			}
		}
	}

	// Fallback to manual annotations
	if repoURL == "" {
		repoURL = pod.Annotations["fixora.io/repo-url"]
		filePath = pod.Annotations["fixora.io/repo-path"]
		vcsType = pod.Annotations["fixora.io/vcs-type"]
	}

	if repoURL == "" || filePath == "" {
		return
	}

	if vcsType == "" {
		vcsType = "github"
	}

	u, err := giturls.Parse(repoURL)
	if err != nil {
		slog.Error("Failed to parse git URL", "url", repoURL, "error", err)
		return
	}

	pathParts := strings.Split(strings.Trim(strings.TrimSuffix(u.Path, ".git"), "/"), "/")
	if len(pathParts) < 2 {
		slog.Warn("Invalid git path", "path", u.Path)
		return
	}
	repoOwner := pathParts[len(pathParts)-2]
	repoName := pathParts[len(pathParts)-1]

	baseBranch := "main"
	if targetRevision != "" && !strings.Contains(targetRevision, "HEAD") {
		baseBranch = targetRevision
	}

	var provider vcs.Provider
	if vcsType == "github" {
		provider = c.ghProvider
	} else if vcsType == "gitlab" {
		provider = c.glProvider
	}

	if provider == nil || c.aiProvider == nil {
		return
	}

	// Check if a PR already exists for this pod to avoid duplicates
	// We check for a branch name that contains the pod name
	// This is a bit loose but works for our naming convention "fixora/patch-%s-%d"
	branchPrefix := fmt.Sprintf("fixora/patch-%s-", pod.Name)
	exists, prURL, err := provider.PullRequestExists(ctx, repoOwner, repoName, branchPrefix)
	if err == nil && exists {
		slog.Info("PR already exists for pod, skipping", "pod", pod.Name, "pr", prURL)
		return
	}

	// Fetch current config content to provide context for the AI patch generator
	currentContent, err := provider.GetFileContent(ctx, repoOwner, repoName, filePath, baseBranch)
	if err != nil {
		slog.Error("Failed to fetch current content", "repo", repoName, "path", filePath, "error", err)
		return
	}

	// Generate the specific patch content using AI
	newContent, err := c.aiProvider.GeneratePatch(ctx, currentContent, evidence.RootCause+"\n"+evidence.MetricProof)
	if err != nil {
		slog.Error("Failed to generate patch", "pod", pod.Name, "error", err)
		return
	}

	c.history.UpdatePatch(ctx, pod.Namespace, pod.Name, string(newContent))

	branchName := fmt.Sprintf("fixora/patch-%s-%d", pod.Name, time.Now().Unix())
	opts := vcs.PullRequestOptions{
		Title:         fmt.Sprintf("Fixora: Automated Fix for %s/%s", pod.Namespace, pod.Name),
		Body:          fmt.Sprintf("### Evidence Chain\n\n* **Root Cause:** %s\n* **Metric Proof:** %s\n\nGenerated by Fixora.", evidence.RootCause, evidence.MetricProof),
		Head:          branchName,
		Base:          baseBranch,
		RepoOwner:     repoOwner,
		RepoName:      repoName,
		FilePath:      filePath,
		NewContent:    newContent,
		CommitMessage: "fix: automated resource adjustment by Fixora",
	}

	// Execute the PR creation
	prURL, err = provider.CreatePullRequest(ctx, opts)
	if err != nil {
		slog.Error("Error creating PR", "pod", pod.Name, "error", err)
	} else if prURL != "" {
		slog.Info("Created PR", "url", prURL)
		notifications.SendNotification(c.config, fmt.Sprintf("🚀 Created remediation PR for %s/%s: %s", pod.Namespace, pod.Name, prURL))
	}
}

func (c *Controller) SubmitPendingFix(ctx context.Context, callbackID string) {
	c.pendingMu.Lock()
	fix, ok := c.pendingFixes[callbackID]
	if ok {
		delete(c.pendingFixes, callbackID)
	}
	c.pendingMu.Unlock()

	if !ok {
		slog.Warn("No pending fix found for callback", "id", callbackID)
		notifications.SendNotification(c.config, "⚠️ Could not find a pending fix for this approval. It may have expired or already been processed.")
		return
	}

	var provider vcs.Provider
	if fix.VCSType == "github" {
		provider = c.ghProvider
	} else if fix.VCSType == "gitlab" {
		provider = c.glProvider
	}

	if provider == nil {
		slog.Error("No VCS provider configured to submit pending fix")
		notifications.SendNotification(c.config, "❌ Remediation failed: No VCS provider configured.")
		return
	}

	slog.Info("Executing pending PR creation", "namespace", fix.PodNamespace, "name", fix.PodName)
	prURL, err := provider.CreatePullRequest(ctx, fix.Options)
	if err != nil {
		slog.Error("Error creating pending PR", "pod", fix.PodName, "error", err)
		notifications.SendNotification(c.config, fmt.Sprintf("❌ Remediation PR creation failed for %s/%s: %v", fix.PodNamespace, fix.PodName, err))
	} else if prURL != "" {
		slog.Info("Created PR from pending fix", "url", prURL)
		notifications.SendNotification(c.config, fmt.Sprintf("🚀 Created remediation PR for %s/%s: %s", fix.PodNamespace, fix.PodName, prURL))
	}
}

func (c *Controller) getPodLogs(ctx context.Context, pod *v1.Pod) (string, error) {
	podLogOpts := v1.PodLogOptions{TailLines: Int64Ptr(50)}
	req := c.clientset.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &podLogOpts)
	podLogs, err := req.Stream(ctx)
	if err != nil {
		return "", err
	}
	defer podLogs.Close()

	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, podLogs)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

func (c *Controller) getPodEvents(ctx context.Context, pod *v1.Pod) (string, error) {
	events, err := c.clientset.CoreV1().Events(pod.Namespace).List(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("involvedObject.name=%s,involvedObject.namespace=%s", pod.Name, pod.Namespace),
	})
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	limit := 50
	start := 0
	if len(events.Items) > limit {
		start = len(events.Items) - limit
	}
	for _, event := range events.Items[start:] {
		sb.WriteString(fmt.Sprintf("[%s] %s: %s\n", event.LastTimestamp.Format(time.RFC3339), event.Reason, event.Message))
	}
	return sb.String(), nil
}

// PerformRolloutRestart executes a manual rollout restart of a Deployment.
func (c *Controller) PerformRolloutRestart(namespace, deploymentName string) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	deployment, err := c.clientset.AppsV1().Deployments(namespace).Get(ctx, deploymentName, metav1.GetOptions{})
	if err != nil {
		slog.Error("Error getting Deployment", "namespace", namespace, "name", deploymentName, "error", err)
		return
	}

	slog.Info("Triggering rollout restart", "namespace", deployment.Namespace, "name", deployment.Name)
	notifications.SendNotification(c.config, fmt.Sprintf("Triggering rollout restart for Deployment %s/%s", deployment.Namespace, deployment.Name))

	if deployment.Spec.Template.Annotations == nil {
		deployment.Spec.Template.Annotations = make(map[string]string)
	}
	deployment.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = time.Now().Format(time.RFC3339)

	_, err = c.clientset.AppsV1().Deployments(deployment.Namespace).Update(ctx, deployment, metav1.UpdateOptions{})
	if err != nil {
		slog.Error("Error updating Deployment", "namespace", deployment.Namespace, "name", deployment.Name, "error", err)
		return
	}
}

func Int64Ptr(i int64) *int64 { return &i }
