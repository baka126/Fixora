package ai

import (
	"context"
	"encoding/json"
	"fmt"

	anthropic "github.com/liushuangls/go-anthropic/v2"
)

type AnthropicProvider struct {
	client *anthropic.Client
	model  anthropic.Model
}

func NewAnthropicProvider(apiKey, modelName string) (*AnthropicProvider, error) {
	client := anthropic.NewClient(apiKey)
	if modelName == "" {
		modelName = string(anthropic.ModelClaude3Dot5Sonnet20240620)
	}
	return &AnthropicProvider{
		client: client,
		model:  anthropic.Model(modelName),
	}, nil
}

func (a *AnthropicProvider) AnalyzeLog(ctx context.Context, logs string) (string, error) {
	resp, err := a.client.CreateMessages(ctx, anthropic.MessagesRequest{
		Model: a.model,
		Messages: []anthropic.Message{
			{
				Role: anthropic.RoleUser,
				Content: []anthropic.MessageContent{
					{
						Type: anthropic.MessagesContentTypeText,
						Text: StringPtr(fmt.Sprintf(PromptAnalyzeLog, logs)),
					},
				},
			},
		},
		MaxTokens: 1024,
	})

	if err != nil {
		return "", err
	}

	if len(resp.Content) == 0 {
		return "No content in response", nil
	}

	return *resp.Content[0].Text, nil
}

func (a *AnthropicProvider) AnalyzeEvents(ctx context.Context, events string) (string, error) {
	resp, err := a.client.CreateMessages(ctx, anthropic.MessagesRequest{
		Model: a.model,
		Messages: []anthropic.Message{
			{
				Role: anthropic.RoleUser,
				Content: []anthropic.MessageContent{
					{
						Type: anthropic.MessagesContentTypeText,
						Text: StringPtr(fmt.Sprintf(PromptAnalyzeEvents, events)),
					},
				},
			},
		},
		MaxTokens: 1024,
	})

	if err != nil {
		return "", err
	}

	if len(resp.Content) == 0 {
		return "No content in response", nil
	}

	return *resp.Content[0].Text, nil
}

func (a *AnthropicProvider) AnalyzeRootCause(ctx context.Context, evidence string) (string, error) {
	resp, err := a.client.CreateMessages(ctx, anthropic.MessagesRequest{
		Model: a.model,
		Messages: []anthropic.Message{
			{
				Role: anthropic.RoleUser,
				Content: []anthropic.MessageContent{
					{
						Type: anthropic.MessagesContentTypeText,
						Text: StringPtr(fmt.Sprintf(PromptAnalyzeRootCause, evidence)),
					},
				},
			},
		},
		MaxTokens: 1024,
	})

	if err != nil {
		return "", err
	}

	if len(resp.Content) == 0 {
		return "No content in response", nil
	}

	return *resp.Content[0].Text, nil
}

func (a *AnthropicProvider) PerformForensics(ctx context.Context, forensicCtx ForensicContext) (AIResponse, error) {
	resp, err := a.client.CreateMessages(ctx, anthropic.MessagesRequest{
		Model: a.model,
		Messages: []anthropic.Message{
			{
				Role: anthropic.RoleUser,
				Content: []anthropic.MessageContent{
					{
						Type: anthropic.MessagesContentTypeText,
						Text: StringPtr(fmt.Sprintf(PromptForensics,
							forensicCtx.Namespace, forensicCtx.PodName, forensicCtx.Reason,
							forensicCtx.Metrics, forensicCtx.Events, forensicCtx.Logs, forensicCtx.History)),
					},
				},
			},
		},
		MaxTokens: 1024,
	})

	if err != nil {
		return AIResponse{}, err
	}

	if len(resp.Content) == 0 {
		return AIResponse{Analysis: "No content in response", Confidence: 0}, nil
	}

	raw := *resp.Content[0].Text
	var aiResp AIResponse
	if err := json.Unmarshal([]byte(raw), &aiResp); err != nil {
		return AIResponse{Analysis: raw, Confidence: 50}, nil
	}

	return aiResp, nil
}

func (a *AnthropicProvider) PerformPredictiveForensics(ctx context.Context, namespace, podName, history, metrics string) (AIResponse, error) {
	resp, err := a.client.CreateMessages(ctx, anthropic.MessagesRequest{
		Model: a.model,
		Messages: []anthropic.Message{
			{
				Role: anthropic.RoleUser,
				Content: []anthropic.MessageContent{
					{
						Type: anthropic.MessagesContentTypeText,
						Text: StringPtr(fmt.Sprintf(PromptPredictiveForensics, namespace, podName, history, metrics)),
					},
				},
			},
		},
		MaxTokens: 1024,
	})

	if err != nil {
		return AIResponse{}, err
	}

	if len(resp.Content) == 0 {
		return AIResponse{Analysis: "No predictive analysis generated", Confidence: 0}, nil
	}

	raw := *resp.Content[0].Text
	var aiResp AIResponse
	if err := json.Unmarshal([]byte(raw), &aiResp); err != nil {
		return AIResponse{Analysis: raw, Confidence: 50}, nil
	}

	return aiResp, nil
}

func (a *AnthropicProvider) GeneratePatch(ctx context.Context, currentContent []byte, evidence string) (AIResponse, error) {
	resp, err := a.client.CreateMessages(ctx, anthropic.MessagesRequest{
		Model: a.model,
		Messages: []anthropic.Message{
			{
				Role: anthropic.RoleUser,
				Content: []anthropic.MessageContent{
					{
						Type: anthropic.MessagesContentTypeText,
						Text: StringPtr(fmt.Sprintf(PromptGeneratePatch, string(currentContent), evidence)),
					},
				},
			},
		},
		MaxTokens: 4096,
	})

	if err != nil {
		return AIResponse{}, err
	}

	if len(resp.Content) == 0 {
		return AIResponse{}, fmt.Errorf("no patch generated")
	}

	raw := *resp.Content[0].Text
	var aiResp AIResponse
	if err := json.Unmarshal([]byte(raw), &aiResp); err != nil {
		return AIResponse{Patch: string(CleanPatch(raw)), Confidence: 50}, nil
	}

	aiResp.Patch = string(CleanPatch(aiResp.Patch))
	return aiResp, nil
}

func StringPtr(s string) *string {
	return &s
}
