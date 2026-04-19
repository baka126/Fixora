package ai

import (
	"context"
	"strings"
)

const (
	PromptAnalyzeLog          = "You are a Kubernetes forensic expert. Provide a strict 2-sentence TL;DR in plain English of the log failure. No jargon, no extra text.\n\nLogs:\n%s"
	PromptAnalyzeEvents       = "You are a Kubernetes forensic expert. Provide a strict 2-sentence TL;DR in plain English of the pod events. No jargon, no extra text.\n\nEvents:\n%s"
	PromptAnalyzeRootCause    = "Based on the following evidence chain, determine the root cause and suggest a fix:\n\n%s"
	PromptForensics           = "You are a Kubernetes forensic expert. Analyze failure for pod %s/%s. Reason: %s\nMetrics: %s\nEvents: %s\nLogs: %s\nPast History:\n%s\n\nProvide a clear, 3-sentence summary: 1. Root Cause, 2. Proof, 3. Recommended fix. Use Past History to run predictive analysis, give future prediction, and offer a long-term solution if this is a recurring issue."
	PromptGeneratePatch       = "You are a Kubernetes GitOps expert. Generate ONLY the complete new file content. No markdown.\n\n[CURRENT CONTENT]\n%s\n\n[EVIDENCE]\n%s"
	PromptPredictiveForensics = "You are a Kubernetes predictive AI. Analyze the historical OOM incidents and current memory trajectory for pod %s/%s.\n\n[HISTORY]\n%s\n\n[CURRENT METRICS]\n%s\n\nProvide a 2-sentence early warning predicting if an OOM is imminent and suggesting immediate action to prevent downtime."
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

type Provider interface {
	AnalyzeLog(ctx context.Context, logs string) (string, error)
	AnalyzeEvents(ctx context.Context, events string) (string, error)
	AnalyzeRootCause(ctx context.Context, evidence string) (string, error)
	PerformForensics(ctx context.Context, forensicCtx ForensicContext) (string, error)
	PerformPredictiveForensics(ctx context.Context, namespace, podName, history, metrics string) (string, error)
	GeneratePatch(ctx context.Context, currentContent []byte, evidence string) ([]byte, error)
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
		return nil, nil
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
