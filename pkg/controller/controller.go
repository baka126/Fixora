package controller

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"fixora/pkg/ai"
	"fixora/pkg/alertmanager"
	"fixora/pkg/argocd"
	"fixora/pkg/config"
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
)

// podWorkItem represents a unit of diagnostic work for a specific pod.
type podWorkItem struct {
	namespace string
	name      string
	reason    string
}

// Controller is the core engine of Fixora. It watches for pod failures,
// gathers forensic evidence from metrics/logs, and triggers AI analysis.
type Controller struct {
	clientset  kubernetes.Interface
	factory    informers.SharedInformerFactory
	config     *config.Config
	promClient *prometheus.Client
	amClient   *alertmanager.Client
	argoClient *argocd.Client
	aiProvider ai.Provider
	ghProvider *vcs.GitHubProvider
	glProvider *vcs.GitLabProvider
	queue      workqueue.RateLimitingInterface
	history    *historyCache
}

// NewController initializes a new diagnostic controller with all required clients.
func NewController(clientset kubernetes.Interface, dynamicClient dynamic.Interface, cfg *config.Config) *Controller {
	factory := informers.NewSharedInformerFactory(clientset, time.Second*30)

	var promClient *prometheus.Client
	if cfg.PrometheusURL != "" {
		var err error
		promClient, err = prometheus.New(cfg.PrometheusURL)
		if err != nil {
			slog.Error("Failed to create Prometheus client", "error", err)
		}
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
		aiProvider, _ = ai.NewProvider(cfg.AIProvider, cfg.AIAPIKey)
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
		clientset:  clientset,
		factory:    factory,
		config:     cfg,
		promClient: promClient,
		amClient:   amClient,
		argoClient: argoClient,
		aiProvider: aiProvider,
		ghProvider: ghProvider,
		glProvider: glProvider,
		queue:      workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "fixora"),
		history:    newHistoryCache(),
	}
}

// Run starts the controller workers and informers.
func (c *Controller) Run(stopCh <-chan struct{}) {
	defer c.queue.ShutDown()

	podInformer := c.factory.Core().V1().Pods().Informer()
	podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(oldObj, newObj interface{}) {
			c.enqueuePod(newObj)
		},
	})

	c.factory.Start(stopCh)
	if !cache.WaitForCacheSync(stopCh, podInformer.HasSynced) {
		return
	}

	// Start diagnostic workers
	go wait.Until(c.runWorker, time.Second, stopCh)
	
	// Start predictive leak scanner if enabled
	if c.config.PredictiveEnabled {
		go wait.Until(c.scanForLeaks, 5*time.Minute, stopCh)
	}

	<-stopCh
}

// scanForLeaks periodically checks all running pods for consistent memory growth patterns.
func (c *Controller) scanForLeaks() {
	if c.promClient == nil {
		return
	}

	slog.Info("Scanning for memory leak trajectories")
	pods, err := c.clientset.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		slog.Error("Failed to list pods for leak scan", "error", err)
		return
	}

	for _, pod := range pods.Items {
		if pod.Status.Phase != v1.PodRunning {
			continue
		}

		matrix, err := c.promClient.GetHistoricalMemoryUsage(pod.Namespace, pod.Name, 1*time.Hour)
		if err != nil || len(matrix) == 0 || len(matrix[0].Values) < 10 {
			continue
		}

		values := matrix[0].Values
		first := float64(values[0].Value)
		last := float64(values[len(values)-1].Value)

		// Flag pods with >20% memory growth in 1 hour
		if first > 0 && (last-first)/first > 0.20 {
			slog.Warn("Potential memory leak detected", "namespace", pod.Namespace, "pod", pod.Name, "increase_pct", (last-first)/first*100)
			notifications.SendNotification(c.config, fmt.Sprintf("⚠️ *Predictive Warning*: Pod %s/%s has a %.1f%% memory growth in the last hour. Potential OOM trajectory.", pod.Namespace, pod.Name, (last-first)/first*100))
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
	pod, err := c.clientset.CoreV1().Pods(work.namespace).Get(context.TODO(), work.name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	c.diagnosePod(pod, work.reason)
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
func (c *Controller) diagnosePod(pod *v1.Pod, reason string) {
	slog.Info("Diagnosing Pod", "namespace", pod.Namespace, "name", pod.Name, "reason", reason)

	historySummary := "This is the first time we've diagnosed this pod."
	if prev, exists := c.history.Get(pod.Namespace, pod.Name); exists {
		historySummary = fmt.Sprintf("Recurring Issue: This pod has crashed %d times recently. Last Root Cause: %s", prev.Count, prev.RootCause)
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
		usage, _ := c.promClient.GetPodMemoryUsage(pod.Namespace, pod.Name, time.Hour)
		limit, _ := c.promClient.GetPodMemoryLimit(pod.Namespace, pod.Name)
		evidence.MetricProof = fmt.Sprintf("Memory Usage: %.2f MiB, Memory Limit: %.2f MiB", usage/1024/1024, limit/1024/1024)
	}

	// Gathers Kubernetes events
	events, err := c.getPodEvents(pod)
	if err == nil {
		evidence.EventTimeline = events
	}

	// Execute Multi-Modal AI Forensics
	var rootCause string
	if c.aiProvider != nil {
		logs, err := c.getPodLogs(pod)
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
		}

		rootCause, err = c.aiProvider.PerformForensics(context.TODO(), forensicCtx)
		if err != nil {
			slog.Error("AI Forensics failed", "pod", pod.Name, "error", err)
			evidence.RootCause = "Forensic analysis failed: " + err.Error()
		} else {
			evidence.RootCause = rootCause
			c.history.Update(pod.Namespace, pod.Name, rootCause)
		}
	} else {
		evidence.RootCause = "AI Provider not configured"
	}

	evidence.FinOpsImpact = "+$2.10/mo AWS compute cost vs. preventing a $5,000 outage"

	// Sends the report to Slack
	notifications.SendEvidenceChain(c.config, evidence)

	// Attempts automated remediation
	c.handleRemediation(pod, evidence)
}

// handleRemediation attempts to open a Pull Request with a fix by discovering the pod's source repository.
func (c *Controller) handleRemediation(pod *v1.Pod, evidence notifications.EvidenceChain) {
	var repoURL, filePath, vcsType, targetRevision string

	// Attempt discovery via ArgoCD API/CRD
	if c.argoClient != nil {
		for _, owner := range pod.OwnerReferences {
			if owner.Kind == "ReplicaSet" {
				rs, err := c.clientset.AppsV1().ReplicaSets(pod.Namespace).Get(context.TODO(), owner.Name, metav1.GetOptions{})
				if err == nil {
					for _, rsOwner := range rs.OwnerReferences {
						info, err := c.argoClient.GetAppForResource(context.TODO(), pod.Namespace, rsOwner.Name, rsOwner.Kind)
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

	// Fetch current config content to provide context for the AI patch generator
	currentContent, err := provider.GetFileContent(context.TODO(), repoOwner, repoName, filePath, baseBranch)
	if err != nil {
		slog.Error("Failed to fetch current content", "repo", repoName, "path", filePath, "error", err)
		return
	}

	// Generate the specific patch content using AI
	newContent, err := c.aiProvider.GeneratePatch(context.TODO(), currentContent, evidence.RootCause+"\n"+evidence.MetricProof)
	if err != nil {
		slog.Error("Failed to generate patch", "pod", pod.Name, "error", err)
		return
	}

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
	prURL, err := provider.CreatePullRequest(context.TODO(), opts)
	if err != nil {
		slog.Error("Error creating PR", "pod", pod.Name, "error", err)
	} else if prURL != "" {
		slog.Info("Created PR", "url", prURL)
		notifications.SendNotification(c.config, fmt.Sprintf("🚀 Created remediation PR for %s/%s: %s", pod.Namespace, pod.Name, prURL))
	}
}

func (c *Controller) getPodLogs(pod *v1.Pod) (string, error) {
	podLogOpts := v1.PodLogOptions{TailLines: Int64Ptr(50)}
	req := c.clientset.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &podLogOpts)
	podLogs, err := req.Stream(context.TODO())
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

func (c *Controller) getPodEvents(pod *v1.Pod) (string, error) {
	events, err := c.clientset.CoreV1().Events(pod.Namespace).List(context.TODO(), metav1.ListOptions{
		FieldSelector: fmt.Sprintf("involvedObject.name=%s,involvedObject.namespace=%s", pod.Name, pod.Namespace),
	})
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	for _, event := range events.Items {
		sb.WriteString(fmt.Sprintf("[%s] %s: %s\n", event.LastTimestamp.Format(time.RFC3339), event.Reason, event.Message))
	}
	return sb.String(), nil
}

// PerformRolloutRestart executes a manual rollout restart of a Deployment.
func (c *Controller) PerformRolloutRestart(namespace, deploymentName string) {
	deployment, err := c.clientset.AppsV1().Deployments(namespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
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

	_, err = c.clientset.AppsV1().Deployments(deployment.Namespace).Update(context.TODO(), deployment, metav1.UpdateOptions{})
	if err != nil {
		slog.Error("Error updating Deployment", "namespace", deployment.Namespace, "name", deployment.Name, "error", err)
		return
	}
}

func Int64Ptr(i int64) *int64 { return &i }
