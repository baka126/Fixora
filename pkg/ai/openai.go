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

func NewOpenAIProvider(apiKey, modelName string) (*OpenAIProvider, error) {
	client := openai.NewClient(apiKey)
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
	resp, err := o.client.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model: o.model,
			ResponseFormat: &openai.ChatCompletionResponseFormat{
				Type: openai.ChatCompletionResponseFormatTypeJSONObject,
			},
			Messages: []openai.ChatCompletionMessage{
				{
					Role: openai.ChatMessageRoleUser,
					Content: fmt.Sprintf(PromptForensics,
						forensicCtx.Namespace, forensicCtx.PodName, forensicCtx.Reason,
						forensicCtx.Metrics, forensicCtx.Events, forensicCtx.Logs, forensicCtx.History),
				},
			},
		},
	)

	if err != nil {
		return AIResponse{}, err
	}

	raw := resp.Choices[0].Message.Content
	var aiResp AIResponse
	if err := json.Unmarshal([]byte(raw), &aiResp); err != nil {
		return AIResponse{Analysis: raw, Confidence: 50}, nil
	}

	return aiResp, nil
}

func (o *OpenAIProvider) PerformPredictiveForensics(ctx context.Context, namespace, podName, history, metrics string) (AIResponse, error) {
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
					Content: fmt.Sprintf(PromptPredictiveForensics, namespace, podName, history, metrics),
				},
			},
		},
	)

	if err != nil {
		return AIResponse{}, err
	}

	raw := resp.Choices[0].Message.Content
	var aiResp AIResponse
	if err := json.Unmarshal([]byte(raw), &aiResp); err != nil {
		return AIResponse{Analysis: raw, Confidence: 50}, nil
	}

	return aiResp, nil
}

func (o *OpenAIProvider) GeneratePatch(ctx context.Context, currentContent []byte, evidence string) (AIResponse, error) {
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
					Content: fmt.Sprintf(PromptGeneratePatch, string(currentContent), evidence),
				},
			},
		},
	)

	if err != nil {
		return AIResponse{}, err
	}

	raw := resp.Choices[0].Message.Content
	var aiResp AIResponse
	if err := json.Unmarshal([]byte(raw), &aiResp); err != nil {
		return AIResponse{Patch: string(CleanPatch(raw)), Confidence: 50}, nil
	}

	aiResp.Patch = string(CleanPatch(aiResp.Patch))
	return aiResp, nil
}
