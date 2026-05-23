package ai

import (
	"context"
	"fmt"
	"strings"
)

const (
	PromptAnalyzeLog          = "You are a Kubernetes forensic expert. Provide a strict 2-sentence TL;DR in plain English of the log failure. No jargon, no extra text.\n\nLogs:\n%s"
	PromptAnalyzeEvents       = "You are a Kubernetes forensic expert. Provide a strict 2-sentence TL;DR in plain English of the pod events. No jargon, no extra text.\n\nEvents:\n%s"
	PromptAnalyzeRootCause    = "Based on the following evidence chain, determine the root cause and suggest a fix:\n\n%s"
	PromptForensics           = "You are a Kubernetes forensic expert. Analyze failure for pod %s/%s. Reason: %s\nMetrics: %s\nEvents: %s\nLogs: %s\nPast History:\n%s\n\nProvide a clear, 3-sentence summary: 1. Root Cause, 2. Proof, 3. Recommended fix. If Metrics state that historical trends are unavailable, focus your analysis on Logs and Events. Use Past History to run predictive analysis, give future prediction, and offer a long-term solution if this is a recurring issue.\n\nYou MUST respond with a JSON object containing: 'analysis' (the summary) and 'confidence' (a percentage from 0 to 100 representing your certainty)."
	PromptGeneratePatch       = "You are a Kubernetes GitOps expert. Generate the complete new file content for the necessary resources. No markdown outside of the JSON string values. Preserve metadata, annotations, resource names, namespaces, comments, and unrelated field ordering unless the evidence explicitly requires changing them. For image remediation, use only pinned non-latest tags or digests; for CPU architecture or exec format failures, only select an image that is explicitly known or evidenced to support the node architecture, and do not guess arbitrary Docker images. For env/config dependency fixes, only update the target workload's container env/envFrom/volume references; set HOST/PORT variables to discovered Service DNS names and ports, and reference existing Secret/ConfigMap keys with valueFrom without creating, editing, or inlining Secret values. For probe fixes, update only readinessProbe, livenessProbe, or startupProbe on the target workload container; align httpGet path/port to listener evidence, and do not change Service, Ingress, HTTPRoute, labels, or selectors unless routing evidence explicitly requires it. For security hardening fixes, prefer least-privilege changes: add an emptyDir plus volumeMount only for the exact permission-denied writable path, or set runAsNonRoot/runAsUser/runAsGroup only when evidence shows the container expects non-root execution. Never set privileged=true, allowPrivilegeEscalation=true, hostPath, hostPID, hostIPC, hostNetwork, or broad capabilities. For raw Kubernetes Pod image fixes, change only spec.containers[*].image or imagePullPolicy because live Pods reject command, args, resources, restartPolicy, and most other spec updates. For raw Kubernetes manifests, return one valid top-level manifest per file and update the existing fields in place; never nest apiVersion, kind, metadata, or spec under a container, env var, or other child field.\n\n[CURRENT CONTEXT]\n%s\n\n[EVIDENCE]\n%s\n\nYou MUST respond with a JSON object containing: 'patches' (a JSON array of objects, where each object has 'repo_owner', 'repo_name', 'file_path', and 'content' fields with the full file content) and 'confidence' (a percentage from 0 to 100 representing your certainty that these patches are correct and safe)."
	PromptPredictiveForensics = "You are a Kubernetes predictive AI. Analyze the historical OOM incidents and current memory trajectory for pod %s/%s.\n\n[HISTORY]\n%s\n\n[CURRENT METRICS]\n%s\n\nProvide a 2-sentence early warning predicting if an OOM is imminent and suggesting immediate action to prevent downtime.\n\nYou MUST respond with a JSON object containing: 'analysis' (the prediction) and 'confidence' (a percentage from 0 to 100 representing your certainty)."
)

type ForensicContext struct {
	Namespace  string
	PodName    string
	Reason     string
	Logs       string
	Events     string
	Metrics    string
	History    string
	PromptType string // Used to select refined instructions
}

type AIPatch struct {
	RepoOwner string `json:"repo_owner"`
	RepoName  string `json:"repo_name"`
	FilePath  string `json:"file_path"`
	Content   string `json:"content"`
}

type AIResponse struct {
	Analysis   string    `json:"analysis"`
	Patch      string    `json:"patch"` // Deprecated: use Patches
	Patches    []AIPatch `json:"patches"`
	Confidence int       `json:"confidence"`
	RawPrompt  string    `json:"-"` // Captured for auditing
}

type Provider interface {
	AnalyzeLog(ctx context.Context, logs string) (string, error)
	AnalyzeEvents(ctx context.Context, events string) (string, error)
	AnalyzeRootCause(ctx context.Context, evidence string) (string, error)
	PerformForensics(ctx context.Context, forensicCtx ForensicContext) (AIResponse, error)
	PerformPredictiveForensics(ctx context.Context, namespace, podName, history, metrics string) (AIResponse, error)
	GeneratePatch(ctx context.Context, currentContent string, evidence string) (AIResponse, error)
}

func NewProvider(providerName, apiKey, modelName string, baseURLs ...string) (Provider, error) {
	baseURL := ""
	if len(baseURLs) > 0 {
		baseURL = baseURLs[0]
	}
	switch providerName {
	case "gemini":
		return NewGeminiProvider(apiKey, modelName)
	case "openai":
		return NewOpenAIProvider(apiKey, modelName, baseURL)
	case "anthropic":
		return NewAnthropicProvider(apiKey, modelName)
	default:
		return nil, fmt.Errorf("unsupported AI provider: %s", providerName)
	}
}

// CleanPatch removes markdown code blocks and other common LLM-injected formatting
// from the generated patch to ensure it is valid YAML/JSON.
func CleanPatch(raw string) []byte {
	clean := strings.TrimSpace(raw)
	// Remove common markdown tags
	prefixes := []string{"```yaml", "```json", "```"}
	for _, p := range prefixes {
		clean = strings.TrimPrefix(clean, p)
	}
	clean = strings.TrimSuffix(clean, "```")
	return []byte(strings.TrimSpace(clean))
}

var (
	interestingLogPatterns = []string{
		"error", "panic", "exception", "failed", "stack trace",
		"fatal", "crit", "warn", "exit", "goroutine",
		"refused", "timeout", "reset", "denied",
	}
)

// SiftLogs intelligently picks the most relevant lines from a large log set.
// It prioritizes lines matching known error patterns and includes a small buffer around them.
func SiftLogs(rawLogs string, maxLines int) string {
	if rawLogs == "" {
		return ""
	}
	lines := strings.Split(rawLogs, "\n")
	if len(lines) <= maxLines {
		return rawLogs
	}

	var results []string
	seen := make(map[int]bool)

	// Phase 1: Pick lines with interesting patterns
	for i, line := range lines {
		lower := strings.ToLower(line)
		isInteresting := false
		for _, pattern := range interestingLogPatterns {
			if strings.Contains(lower, pattern) {
				isInteresting = true
				break
			}
		}

		if isInteresting {
			// Add the interesting line and 1 line of context before/after
			for j := i - 1; j <= i+1; j++ {
				if j >= 0 && j < len(lines) && !seen[j] {
					results = append(results, lines[j])
					seen[j] = true
				}
			}
		}

		if len(results) >= maxLines {
			break
		}
	}

	// Phase 2: If we didn't find enough, take the last few lines
	if len(results) < maxLines/2 {
		start := len(lines) - (maxLines - len(results))
		if start < 0 {
			start = 0
		}
		for i := start; i < len(lines); i++ {
			if !seen[i] {
				results = append(results, lines[i])
				seen[i] = true
			}
		}
	}

	return strings.Join(results, "\n")
}
