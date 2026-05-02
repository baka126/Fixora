package ai

import (
	"context"
	"encoding/json"
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
	// Force JSON response
	model.ResponseMIMEType = "application/json"

	return &GeminiProvider{
		client: client,
		model:  model,
	}, nil
}

func (g *GeminiProvider) AnalyzeLog(ctx context.Context, logs string) (string, error) {
	// Simple analysis remains plain text for now or we can update it too. 
	// For Step 3, we focus on the main flows.
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

func (g *GeminiProvider) PerformForensics(ctx context.Context, forensicCtx ForensicContext) (AIResponse, error) {
	prompt := fmt.Sprintf(PromptForensics,
		forensicCtx.Namespace, forensicCtx.PodName, forensicCtx.Reason,
		forensicCtx.Metrics, forensicCtx.Events, forensicCtx.Logs, forensicCtx.History)

	resp, err := g.model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return AIResponse{}, err
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return AIResponse{Analysis: "No analysis generated", Confidence: 0}, nil
	}

	raw := fmt.Sprintf("%v", resp.Candidates[0].Content.Parts[0])
	var aiResp AIResponse
	if err := json.Unmarshal([]byte(raw), &aiResp); err != nil {
		return AIResponse{Analysis: raw, Confidence: 50}, nil // Fallback
	}

	return aiResp, nil
}

func (g *GeminiProvider) PerformPredictiveForensics(ctx context.Context, namespace, podName, history, metrics string) (AIResponse, error) {
	prompt := fmt.Sprintf(PromptPredictiveForensics, namespace, podName, history, metrics)
	resp, err := g.model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return AIResponse{}, err
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return AIResponse{Analysis: "No predictive analysis generated", Confidence: 0}, nil
	}

	raw := fmt.Sprintf("%v", resp.Candidates[0].Content.Parts[0])
	var aiResp AIResponse
	if err := json.Unmarshal([]byte(raw), &aiResp); err != nil {
		return AIResponse{Analysis: raw, Confidence: 50}, nil
	}

	return aiResp, nil
}

func (g *GeminiProvider) GeneratePatch(ctx context.Context, currentContent []byte, evidence string) (AIResponse, error) {
	prompt := fmt.Sprintf(PromptGeneratePatch,
		string(currentContent), evidence)

	resp, err := g.model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return AIResponse{}, err
	}

	raw := fmt.Sprintf("%v", resp.Candidates[0].Content.Parts[0])
	var aiResp AIResponse
	if err := json.Unmarshal([]byte(raw), &aiResp); err != nil {
		// If unmarshal fails, we might have raw patch content
		return AIResponse{Patch: string(CleanPatch(raw)), Confidence: 50}, nil
	}

	aiResp.Patch = string(CleanPatch(aiResp.Patch))
	return aiResp, nil
}
