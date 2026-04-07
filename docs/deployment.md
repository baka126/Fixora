# Deploying Fixora: AI-Powered K8s Forensic Detective

Fixora is a diagnostic platform that listens for Alertmanager webhooks, analyzes Kubernetes failures (OOMKilled, CrashLoopBackOff) using AI, and generates automated remediation Pull Requests.

---

## 1. Prerequisites

Before deploying, ensure you have the following ready:

### A. Slack Integration
1.  Create a Slack App at [api.slack.com](https://api.slack.com/apps).
2.  Add the `chat:write` scope.
3.  Install the app to your workspace and get the **Bot User OAuth Token** (`xoxb-...`).
4.  Invite the bot to your diagnostic channel (e.g., `#ops-diagnostics`).

### B. AI Provider API Key
Fixora supports:
*   **Google Gemini** (Default): Get a key at [aistudio.google.com](https://aistudio.google.com/).
*   **OpenAI**: Get a key at [platform.openai.com](https://platform.openai.com/).
*   **Anthropic**: Get a key at [console.anthropic.com](https://console.anthropic.com/).

### C. GitOps Tokens (Optional)
To enable automated PR generation:
*   **GitHub:** Personal Access Token with `repo` scope.
*   **GitLab:** Personal Access Token with `api` scope.

---

## 2. Configuration

Fixora is configured via environment variables or Helm values.

### Key Configuration Parameters

| Value | Environment Variable | Description |
| :--- | :--- | :--- |
| `slack.token` | `SLACK_TOKEN` | Slack Bot User OAuth Token. |
| `slack.channel` | `SLACK_CHANNEL` | Slack channel ID or name (e.g., `#alerts`). |
| `ai.apiKey` | `AI_API_KEY` | API Key for your chosen AI provider. |
| `ai.provider` | `AI_PROVIDER` | `gemini`, `openai`, or `anthropic`. |
| `webhook.token` | `WEBHOOK_TOKEN` | (Optional) Bearer token for Alertmanager auth. |
| `features.argocd.enabled` | `ARGOCD_ENABLED` | Set `true` to auto-discover repos via ArgoCD. |

---

## 3. Installation via Helm

The recommended way to deploy Fixora is using the provided Helm chart.

```bash
# 1. Clone the repository
git clone https://github.com/your-org/fixora.git
cd fixora

# 2. Update values.yaml or create a custom-values.yaml
# Ensure you fill in slack.token, slack.channel, and ai.apiKey

# 3. Install the chart
helm install fixora ./charts/fixora -n fixora --create-namespace
```

---

## 4. Connecting Alertmanager

Fixora acts as an Alertmanager **receiver**. You must configure your Alertmanager to send firing alerts to Fixora's `/alerts` endpoint.

Update your `alertmanager.yaml` or `AlertmanagerConfig` CRD:

```yaml
receivers:
- name: 'fixora'
  webhook_configs:
  - url: 'http://fixora.fixora.svc.cluster.local/alerts'
    http_config:
      bearer_token: 'your-webhook-token-here' # Match your WEBHOOK_TOKEN config

route:
  group_by: ['alertname', 'namespace', 'pod']
  routes:
  - matchers:
    - alertname =~ "KubePodCrashLooping|KubeMemoryOvercommit|PodOOMKilled"
    receiver: 'fixora'
    continue: true
```

---

## 5. Enabling GitOps Remediation

Fixora can discover which Git repository manages a Pod in two ways:

### Method A: ArgoCD (Automatic)
Ensure `features.argocd.enabled: true` is set in your values. Fixora will automatically query ArgoCD to find the source repo and path for any crashing pod.

### Method B: Annotations (Manual)
Add these annotations to your Deployment or Pod template:

```yaml
annotations:
  fixora.io/repo-url: "https://github.com/my-org/my-app"
  fixora.io/repo-path: "deploy/values.yaml"
  fixora.io/vcs-type: "github" # or gitlab
```

---

## 6. Verification

Once deployed, you can verify Fixora is running by checking the logs:

```bash
kubectl logs -l app.kubernetes.io/name=fixora -n fixora
```

You should see:
`[INFO] Server listening on port 8080`
`[INFO] Informer cache synced. Ready to diagnose.`

When an alert fires, Fixora will post a Slack message with the **Evidence Chain** and a link to the remediation PR.
