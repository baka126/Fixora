package ai

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sashabaranov/go-openai"
)

type OpenAIProvider struct {
	client *openai.Client
	model  string
}

func NewOpenAIProvider(apiKey, modelName, baseURL string) (*OpenAIProvider, error) {
	var client *openai.Client
	if baseURL != "" {
		config := openai.DefaultConfig(apiKey)
		config.BaseURL = baseURL
		client = openai.NewClientWithConfig(config)
	} else {
		client = openai.NewClient(apiKey)
	}

	if modelName == "" {
		modelName = openai.GPT4oMini
	}
	return &OpenAIProvider{
		client: client,
		model:  modelName,
	}, nil
}

func (o *OpenAIProvider) AnalyzeLog(ctx context.Context, logs string) (string, error) {
	resp, err := o.client.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model: o.model,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleUser,
					Content: fmt.Sprintf(PromptAnalyzeLog, logs),
				},
			},
		},
	)

	if err != nil {
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no analysis generated")
	}

	return resp.Choices[0].Message.Content, nil
}

func (o *OpenAIProvider) AnalyzeEvents(ctx context.Context, events string) (string, error) {
	resp, err := o.client.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model: o.model,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleUser,
					Content: fmt.Sprintf(PromptAnalyzeEvents, events),
				},
			},
		},
	)

	if err != nil {
		return "", err
	}

	return resp.Choices[0].Message.Content, nil
}

func (o *OpenAIProvider) AnalyzeRootCause(ctx context.Context, evidence string) (string, error) {
	resp, err := o.client.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model: o.model,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleUser,
					Content: fmt.Sprintf(PromptAnalyzeRootCause, evidence),
				},
			},
		},
	)

	if err != nil {
		return "", err
	}

	return resp.Choices[0].Message.Content, nil
}

func (o *OpenAIProvider) PerformForensics(ctx context.Context, forensicCtx ForensicContext) (AIResponse, error) {
	prompt := GetForensicPrompt(forensicCtx.PromptType, forensicCtx)

	resp, err := o.client.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model: o.model,
			ResponseFormat: &openai.ChatCompletionResponseFormat{
				Type: openai.ChatCompletionResponseFormatTypeJSONObject,
			},
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleUser,
					Content: prompt,
				},
			},
		},
	)

	if err != nil {
		return AIResponse{}, err
	}
	if len(resp.Choices) == 0 {
		return AIResponse{Analysis: "No analysis generated", Confidence: 0, RawPrompt: prompt}, nil
	}

	raw := resp.Choices[0].Message.Content
	var aiResp AIResponse
	aiResp.RawPrompt = prompt
	if err := json.Unmarshal([]byte(raw), &aiResp); err != nil {
		aiResp.Analysis = raw
		aiResp.Confidence = 50
		return aiResp, nil
	}

	return aiResp, nil
}

func (o *OpenAIProvider) PerformPredictiveForensics(ctx context.Context, namespace, podName, history, metrics string) (AIResponse, error) {
	prompt := fmt.Sprintf(PromptPredictiveForensics, namespace, podName, history, metrics)
	resp, err := o.client.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model: o.model,
			ResponseFormat: &openai.ChatCompletionResponseFormat{
				Type: openai.ChatCompletionResponseFormatTypeJSONObject,
			},
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleUser,
					Content: prompt,
				},
			},
		},
	)

	if err != nil {
		return AIResponse{}, err
	}
	if len(resp.Choices) == 0 {
		return AIResponse{Analysis: "No predictive analysis generated", Confidence: 0, RawPrompt: prompt}, nil
	}

	raw := resp.Choices[0].Message.Content
	var aiResp AIResponse
	aiResp.RawPrompt = prompt
	if err := json.Unmarshal([]byte(raw), &aiResp); err != nil {
		aiResp.Analysis = raw
		aiResp.Confidence = 50
		return aiResp, nil
	}

	return aiResp, nil
}

func (o *OpenAIProvider) GeneratePatch(ctx context.Context, currentContent string, evidence string) (AIResponse, error) {
	prompt := fmt.Sprintf(PromptGeneratePatch, currentContent, evidence)
	resp, err := o.client.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model: o.model,
			ResponseFormat: &openai.ChatCompletionResponseFormat{
				Type: openai.ChatCompletionResponseFormatTypeJSONObject,
			},
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleUser,
					Content: prompt,
				},
			},
		},
	)

	if err != nil {
		return AIResponse{}, err
	}
	if len(resp.Choices) == 0 {
		return AIResponse{RawPrompt: prompt}, fmt.Errorf("no patch generated")
	}

	raw := resp.Choices[0].Message.Content
	var aiResp AIResponse
	aiResp.RawPrompt = prompt
	if err := json.Unmarshal([]byte(raw), &aiResp); err != nil {
		aiResp.Patch = string(CleanPatch(raw))
		aiResp.Confidence = 50
		return aiResp, nil
	}

	for i, p := range aiResp.Patches {
		aiResp.Patches[i].Content = string(CleanPatch(p.Content))
	}
	if aiResp.Patch != "" {
		aiResp.Patch = string(CleanPatch(aiResp.Patch))
	}
	return aiResp, nil
}
