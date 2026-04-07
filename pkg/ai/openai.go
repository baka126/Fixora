package ai

import (
	"context"
	"fmt"

	"github.com/sashabaranov/go-openai"
)

type OpenAIProvider struct {
	client *openai.Client
	model  string
}

func NewOpenAIProvider(apiKey string) (*OpenAIProvider, error) {
	client := openai.NewClient(apiKey)
	return &OpenAIProvider{
		client: client,
		model:  openai.GPT4oMini,
	}, nil
}

func (o *OpenAIProvider) AnalyzeLog(ctx context.Context, logs string) (string, error) {
	resp, err := o.client.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model: o.model,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleSystem,
					Content: "You are a Kubernetes forensic expert. Provide a strict 2-sentence TL;DR in plain English of the log failure. No jargon, no extra text.",
				},
				{
					Role:    openai.ChatMessageRoleUser,
					Content: fmt.Sprintf("Analyze these logs:\n\n%s", logs),
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
					Role:    openai.ChatMessageRoleSystem,
					Content: "You are a Kubernetes forensic expert. Provide a strict 2-sentence TL;DR in plain English of the pod events. No jargon, no extra text.",
				},
				{
					Role:    openai.ChatMessageRoleUser,
					Content: fmt.Sprintf("Analyze these events:\n\n%s", events),
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
					Content: fmt.Sprintf("Based on the following evidence chain, determine the root cause and suggest a fix:\n\n%s", evidence),
				},
			},
		},
	)

	if err != nil {
		return "", err
	}

	return resp.Choices[0].Message.Content, nil
}

func (o *OpenAIProvider) PerformForensics(ctx context.Context, forensicCtx ForensicContext) (string, error) {
	resp, err := o.client.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model: o.model,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleSystem,
					Content: "You are a Kubernetes forensic expert. Analyze the relationship between metrics, events, and logs to find the root cause.",
				},
				{
					Role: openai.ChatMessageRoleUser,
					Content: fmt.Sprintf(`Analyze failure for pod %s/%s. Reason: %s
Metrics: %s
Events: %s
Logs: %s

Provide a clear, 3-sentence summary: 1. Root Cause, 2. Proof, 3. Recommended fix.`, 
						forensicCtx.Namespace, forensicCtx.PodName, forensicCtx.Reason, 
						forensicCtx.Metrics, forensicCtx.Events, forensicCtx.Logs),
				},
			},
		},
	)

	if err != nil {
		return "", err
	}

	return resp.Choices[0].Message.Content, nil
}

func (o *OpenAIProvider) GeneratePatch(ctx context.Context, currentContent []byte, evidence string) ([]byte, error) {
	resp, err := o.client.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model: o.model,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleSystem,
					Content: "You are a Kubernetes GitOps expert. Generate ONLY the complete new file content. No markdown.",
				},
				{
					Role: openai.ChatMessageRoleUser,
					Content: fmt.Sprintf(`Given this current file and the evidence, generate a fixed version:

[CURRENT CONTENT]
%s

[EVIDENCE]
%s`, string(currentContent), evidence),
				},
			},
		},
	)

	if err != nil {
		return nil, err
	}

	return []byte(resp.Choices[0].Message.Content), nil
}
