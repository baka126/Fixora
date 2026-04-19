package config

import (
	"os"
	"strconv"
	"strings"
	"time"
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
	AlertmanagerEnabled  bool
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

	// Database Config
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string

	// Feature Toggles & Predictive Tuning
	PredictiveEnabled         bool
	PredictiveGrowthThreshold float64
	PredictiveScanInterval    time.Duration
	PredictiveMinDataPoints   int

	InfracostAPIKey string
	TrustedVCSDomains []string
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
		AlertmanagerEnabled:  getEnvBool("ALERTMANAGER_ENABLED", true),
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

		DBHost:     os.Getenv("DB_HOST"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBUser:     os.Getenv("DB_USER"),
		DBPassword: os.Getenv("DB_PASSWORD"),
		DBName:     getEnv("DB_NAME", "fixora"),

		PredictiveEnabled:         getEnvBool("PREDICTIVE_ENABLED", true),
		PredictiveGrowthThreshold: getEnvFloat("PREDICTIVE_GROWTH_THRESHOLD", 0.20),
		PredictiveScanInterval:    getEnvDuration("PREDICTIVE_SCAN_INTERVAL", 5*time.Minute),
		PredictiveMinDataPoints:   getEnvInt("PREDICTIVE_MIN_DATA_POINTS", 10),

		InfracostAPIKey: os.Getenv("INFRACOST_API_KEY"),
		TrustedVCSDomains: getEnvSlice("TRUSTED_VCS_DOMAINS", []string{"github.com", "gitlab.com"}),
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

func getEnvFloat(key string, fallback float64) float64 {
	if value, ok := os.LookupEnv(key); ok {
		f, err := strconv.ParseFloat(value, 64)
		if err == nil {
			return f
		}
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if value, ok := os.LookupEnv(key); ok {
		i, err := strconv.Atoi(value)
		if err == nil {
			return i
		}
	}
	return fallback
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	if value, ok := os.LookupEnv(key); ok {
		d, err := time.ParseDuration(value)
		if err == nil {
			return d
		}
	}
	return fallback
}

func getEnvSlice(key string, fallback []string) []string {
	if value, ok := os.LookupEnv(key); ok {
		return strings.Split(value, ",")
	}
	return fallback
}
