package ai

import (
	"context"
	"fmt"

	anthropic "github.com/liushuangls/go-anthropic/v2"
)

type AnthropicProvider struct {
	client *anthropic.Client
}

func NewAnthropicProvider(apiKey string) (*AnthropicProvider, error) {
	client := anthropic.NewClient(apiKey)
	return &AnthropicProvider{
		client: client,
	}, nil
}

func (a *AnthropicProvider) AnalyzeLog(ctx context.Context, logs string) (string, error) {
	resp, err := a.client.CreateMessages(ctx, anthropic.MessagesRequest{
		Model: anthropic.ModelClaude3Dot5Sonnet20240620,
		Messages: []anthropic.Message{
			{
				Role: anthropic.RoleUser,
				Content: []anthropic.MessageContent{
					{
						Type: anthropic.MessagesContentTypeText,
						Text: StringPtr(fmt.Sprintf("You are a Kubernetes forensic expert. Provide a strict 2-sentence TL;DR in plain English of the log failure. No jargon, no extra text.\n\nLogs:\n%s", logs)),
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
		Model: anthropic.ModelClaude3Dot5Sonnet20240620,
		Messages: []anthropic.Message{
			{
				Role: anthropic.RoleUser,
				Content: []anthropic.MessageContent{
					{
						Type: anthropic.MessagesContentTypeText,
						Text: StringPtr(fmt.Sprintf("You are a Kubernetes forensic expert. Provide a strict 2-sentence TL;DR in plain English of the pod events. No jargon, no extra text.\n\nEvents:\n%s", events)),
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
		Model: anthropic.ModelClaude3Dot5Sonnet20240620,
		Messages: []anthropic.Message{
			{
				Role: anthropic.RoleUser,
				Content: []anthropic.MessageContent{
					{
						Type: anthropic.MessagesContentTypeText,
						Text: StringPtr(fmt.Sprintf("Based on the following evidence chain, determine the root cause and suggest a fix:\n\n%s", evidence)),
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

func (a *AnthropicProvider) PerformForensics(ctx context.Context, forensicCtx ForensicContext) (string, error) {
	resp, err := a.client.CreateMessages(ctx, anthropic.MessagesRequest{
		Model: anthropic.ModelClaude3Dot5Sonnet20240620,
		Messages: []anthropic.Message{
			{
				Role: anthropic.RoleUser,
				Content: []anthropic.MessageContent{
					{
						Type: anthropic.MessagesContentTypeText,
						Text: StringPtr(fmt.Sprintf(`You are a Kubernetes forensic expert. Analyze failure for pod %s/%s. Reason: %s
Metrics: %s
Events: %s
Logs: %s

Provide a clear, 3-sentence summary: 1. Root Cause, 2. Proof, 3. Recommended fix.`, 
							forensicCtx.Namespace, forensicCtx.PodName, forensicCtx.Reason, 
							forensicCtx.Metrics, forensicCtx.Events, forensicCtx.Logs)),
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

func (a *AnthropicProvider) GeneratePatch(ctx context.Context, currentContent []byte, evidence string) ([]byte, error) {
	resp, err := a.client.CreateMessages(ctx, anthropic.MessagesRequest{
		Model: anthropic.ModelClaude3Dot5Sonnet20240620,
		Messages: []anthropic.Message{
			{
				Role: anthropic.RoleUser,
				Content: []anthropic.MessageContent{
					{
						Type: anthropic.MessagesContentTypeText,
						Text: StringPtr(fmt.Sprintf(`You are a Kubernetes GitOps expert. Generate ONLY the complete new file content. No markdown.

[CURRENT CONTENT]
%s

[EVIDENCE]
%s`, string(currentContent), evidence)),
					},
				},
			},
		},
		MaxTokens: 4096,
	})

	if err != nil {
		return nil, err
	}

	if len(resp.Content) == 0 {
		return nil, fmt.Errorf("no patch generated")
	}

	return []byte(*resp.Content[0].Text), nil
}

func StringPtr(s string) *string {
	return &s
}
