package ai

import "context"

type ForensicContext struct {
	Namespace string
	PodName   string
	Reason    string
	Logs      string
	Events    string
	Metrics   string
}

type Provider interface {
	AnalyzeLog(ctx context.Context, logs string) (string, error)
	AnalyzeEvents(ctx context.Context, events string) (string, error)
	AnalyzeRootCause(ctx context.Context, evidence string) (string, error)
	PerformForensics(ctx context.Context, forensicCtx ForensicContext) (string, error)
	GeneratePatch(ctx context.Context, currentContent []byte, evidence string) ([]byte, error)
}

func NewProvider(providerName, apiKey string) (Provider, error) {
	switch providerName {
	case "gemini":
		return NewGeminiProvider(apiKey)
	case "openai":
		return NewOpenAIProvider(apiKey)
	case "anthropic":
		return NewAnthropicProvider(apiKey)
	default:
		return nil, nil
	}
}
