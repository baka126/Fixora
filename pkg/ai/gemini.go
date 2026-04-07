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

func NewGeminiProvider(apiKey string) (*GeminiProvider, error) {
	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, err
	}

	model := client.GenerativeModel("gemini-1.5-flash")
	return &GeminiProvider{
		client: client,
		model:  model,
	}, nil
}

func (g *GeminiProvider) AnalyzeLog(ctx context.Context, logs string) (string, error) {
	prompt := fmt.Sprintf(`You are a Kubernetes forensic expert. Analyze the following 50 lines of pod logs and provide a strict 2-sentence TL;DR in plain English explaining the actual failure reason. 
Do not include technical jargon unless necessary, and do not output anything other than the 2-sentence summary.

Logs:
%s`, logs)
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
	prompt := fmt.Sprintf(`You are a Kubernetes forensic expert. Analyze the following pod events and summarize the key issues in 2 sentences. 
Do not include technical jargon unless necessary, and do not output anything other than the 2-sentence summary.

Events:
%s`, events)
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
	prompt := fmt.Sprintf(`Based on the following evidence chain, determine the root cause and suggest a fix:

%s`, evidence)
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
	prompt := fmt.Sprintf(`You are a Kubernetes forensic expert. Analyze the following failure for pod %s/%s.
Reason: %s

[METRICS]
%s

[EVENTS]
%s

[LOGS]
%s

Analyze the relationship between these three data sources. 
Provide a clear, 3-sentence summary:
1. What happened (Root Cause).
2. The undeniable proof (from logs/metrics).
3. The recommended fix (GitOps change).`, 
		forensicCtx.Namespace, forensicCtx.PodName, forensicCtx.Reason, 
		forensicCtx.Metrics, forensicCtx.Events, forensicCtx.Logs)

	resp, err := g.model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return "", err
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return "No analysis generated", nil
	}

	return fmt.Sprintf("%v", resp.Candidates[0].Content.Parts[0]), nil
}

func (g *GeminiProvider) GeneratePatch(ctx context.Context, currentContent []byte, evidence string) ([]byte, error) {
	prompt := fmt.Sprintf(`You are a Kubernetes GitOps expert. Given the current file content and the forensic evidence of a failure, generate a patched version of the file that fixes the issue (e.g., increases resource limits).

[CURRENT CONTENT]
%s

[FORENSIC EVIDENCE]
%s

Output ONLY the complete new file content. Do not include markdown formatting like backticks or language labels.`, 
		string(currentContent), evidence)

	resp, err := g.model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return nil, err
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("no patch generated")
	}

	return []byte(fmt.Sprintf("%v", resp.Candidates[0].Content.Parts[0])), nil
}
