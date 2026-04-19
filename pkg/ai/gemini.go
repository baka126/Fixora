package ai

import (
	"context"
	"fmt"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

type GeminiProvider struct {
	client *genai.Client
	model  *genai.GenerativeModel
}

func NewGeminiProvider(apiKey, modelName string) (*GeminiProvider, error) {
	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, err
	}

	if modelName == "" {
		modelName = "gemini-1.5-flash"
	}
	model := client.GenerativeModel(modelName)
	return &GeminiProvider{
		client: client,
		model:  model,
	}, nil
}

func (g *GeminiProvider) AnalyzeLog(ctx context.Context, logs string) (string, error) {
	prompt := fmt.Sprintf(PromptAnalyzeLog, logs)
	resp, err := g.model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return "", err
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return "No analysis generated", nil
	}

	return fmt.Sprintf("%v", resp.Candidates[0].Content.Parts[0]), nil
}

func (g *GeminiProvider) AnalyzeEvents(ctx context.Context, events string) (string, error) {
	prompt := fmt.Sprintf(PromptAnalyzeEvents, events)
	resp, err := g.model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return "", err
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return "No analysis generated", nil
	}

	return fmt.Sprintf("%v", resp.Candidates[0].Content.Parts[0]), nil
}

func (g *GeminiProvider) AnalyzeRootCause(ctx context.Context, evidence string) (string, error) {
	prompt := fmt.Sprintf(PromptAnalyzeRootCause, evidence)
	resp, err := g.model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return "", err
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return "No root cause analysis generated", nil
	}

	return fmt.Sprintf("%v", resp.Candidates[0].Content.Parts[0]), nil
}

func (g *GeminiProvider) PerformForensics(ctx context.Context, forensicCtx ForensicContext) (string, error) {
	prompt := fmt.Sprintf(PromptForensics,
		forensicCtx.Namespace, forensicCtx.PodName, forensicCtx.Reason,
		forensicCtx.Metrics, forensicCtx.Events, forensicCtx.Logs, forensicCtx.History)

	resp, err := g.model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return "", err
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return "No analysis generated", nil
	}

	return fmt.Sprintf("%v", resp.Candidates[0].Content.Parts[0]), nil
}

func (g *GeminiProvider) PerformPredictiveForensics(ctx context.Context, namespace, podName, history, metrics string) (string, error) {
	prompt := fmt.Sprintf(PromptPredictiveForensics, namespace, podName, history, metrics)
	resp, err := g.model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return "", err
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return "No predictive analysis generated", nil
	}

	return fmt.Sprintf("%v", resp.Candidates[0].Content.Parts[0]), nil
}

func (g *GeminiProvider) GeneratePatch(ctx context.Context, currentContent []byte, evidence string) ([]byte, error) {
	prompt := fmt.Sprintf(PromptGeneratePatch,
		string(currentContent), evidence)

	resp, err := g.model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return nil, err
	}

	rawResponse := fmt.Sprintf("%v", resp.Candidates[0].Content.Parts[0])
	return CleanPatch(rawResponse), nil
}
