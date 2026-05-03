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
	PromptGeneratePatch       = "You are a Kubernetes GitOps expert. Generate the complete new file content for the necessary resources. No markdown outside of the JSON string values.\n\n[CURRENT CONTEXT]\n%s\n\n[EVIDENCE]\n%s\n\nYou MUST respond with a JSON object containing: 'patches' (a JSON array of objects, where each object has 'repo_owner', 'repo_name', 'file_path', and 'content' fields with the full file content) and 'confidence' (a percentage from 0 to 100 representing your certainty that these patches are correct and safe)."
	PromptPredictiveForensics = "You are a Kubernetes predictive AI. Analyze the historical OOM incidents and current memory trajectory for pod %s/%s.\n\n[HISTORY]\n%s\n\n[CURRENT METRICS]\n%s\n\nProvide a 2-sentence early warning predicting if an OOM is imminent and suggesting immediate action to prevent downtime.\n\nYou MUST respond with a JSON object containing: 'analysis' (the prediction) and 'confidence' (a percentage from 0 to 100 representing your certainty)."
)

type ForensicContext struct {
	Namespace string
	PodName   string
	Reason    string
	Logs      string
	Events    string
	Metrics   string
	History   string
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
}

type Provider interface {
	AnalyzeLog(ctx context.Context, logs string) (string, error)
	AnalyzeEvents(ctx context.Context, events string) (string, error)
	AnalyzeRootCause(ctx context.Context, evidence string) (string, error)
	PerformForensics(ctx context.Context, forensicCtx ForensicContext) (AIResponse, error)
	PerformPredictiveForensics(ctx context.Context, namespace, podName, history, metrics string) (AIResponse, error)
	GeneratePatch(ctx context.Context, currentContent string, evidence string) (AIResponse, error)
}

func NewProvider(providerName, apiKey, modelName string) (Provider, error) {
	switch providerName {
	case "gemini":
		return NewGeminiProvider(apiKey, modelName)
	case "openai":
		return NewOpenAIProvider(apiKey, modelName)
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
