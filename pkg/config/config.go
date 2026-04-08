package config

import (
	"os"
	"strconv"
)

type OperatingMode string

const (
	DryRun     OperatingMode = "dry-run"
	ClickToFix OperatingMode = "click-to-fix"
	AutoFix    OperatingMode = "auto-fix"
)

type Config struct {
	SlackToken           string
	SlackSigningSecret   string
	SlackChannel         string
	GoogleChatWebhookURL string
	Mode                 OperatingMode
	PrometheusURL        string
	AlertmanagerURL      string
	AIProvider           string // "gemini", "openai", "anthropic"
	AIModel              string
	AIAPIKey             string
	GitHubToken          string
	GitLabToken          string
	GitLabBaseURL        string
	WebhookToken         string
	WebhookUser          string
	WebhookPassword      string

	// ArgoCD Config
	ArgoCDEnabled   bool
	ArgoCDNamespace string
	ArgoCDURL       string
	ArgoCDToken     string

	// Feature Toggles
	PredictiveEnabled bool
	HistoryCRDEnabled bool
}

func Load() *Config {
	mode := OperatingMode(os.Getenv("FIXORA_MODE"))
	if mode == "" {
		mode = AutoFix
	}

	return &Config{
		SlackToken:           os.Getenv("SLACK_TOKEN"),
		SlackSigningSecret:   os.Getenv("SLACK_SIGNING_SECRET"),
		SlackChannel:         os.Getenv("SLACK_CHANNEL"),
		GoogleChatWebhookURL: os.Getenv("GOOGLE_CHAT_WEBHOOK_URL"),
		Mode:                 mode,
		PrometheusURL:        os.Getenv("PROMETHEUS_URL"),
		AlertmanagerURL:      os.Getenv("ALERTMANAGER_URL"),
		AIProvider:           os.Getenv("AI_PROVIDER"),
		AIModel:              os.Getenv("AI_MODEL"),
		AIAPIKey:             os.Getenv("AI_API_KEY"),
		GitHubToken:          os.Getenv("GITHUB_TOKEN"),
		GitLabToken:          os.Getenv("GITLAB_TOKEN"),
		GitLabBaseURL:        os.Getenv("GITLAB_BASE_URL"),
		WebhookToken:         os.Getenv("WEBHOOK_TOKEN"),
		WebhookUser:          os.Getenv("WEBHOOK_USER"),
		WebhookPassword:      os.Getenv("WEBHOOK_PASSWORD"),

		ArgoCDEnabled:   getEnvBool("ARGOCD_ENABLED", false),
		ArgoCDNamespace: getEnv("ARGOCD_NAMESPACE", "argocd"),
		ArgoCDURL:       getEnv("ARGOCD_URL", ""),
		ArgoCDToken:     os.Getenv("ARGOCD_TOKEN"),

		PredictiveEnabled: getEnvBool("PREDICTIVE_ENABLED", true),
		HistoryCRDEnabled: getEnvBool("HISTORY_CRD_ENABLED", false),
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	if value, ok := os.LookupEnv(key); ok {
		b, err := strconv.ParseBool(value)
		if err == nil {
			return b
		}
	}
	return fallback
}
