# Deploying Fixora

Fixora is an enterprise-grade diagnostic platform that acts as an intelligent webhook receiver for Prometheus Alertmanager. It intercepts critical Kubernetes cluster alerts (e.g., `OOMKilled`, `CrashLoopBackOff`), executes context-aware AI analysis against cluster states, and generates automated Pull Requests for immediate remediation.

---

## 1. System Prerequisites

Ensure your infrastructure meets the following requirements before initiating the deployment:

### A. Communication & Alerting
* **Slack Workspace (Optional):** Create a Slack App via [api.slack.com](https://api.slack.com/apps).
    * Requires `chat:write` OAuth scope.
    * Capture the **Bot User OAuth Token** (`xoxb-...`) and the **Signing Secret** for request verification.
* **Google Workspace (Optional):** Enable a **Webhook** in a Google Chat space.
    * Capture the **Webhook URL**.
* **Prometheus Stack:** A functional `kube-prometheus-stack` or standalone Alertmanager instance routing to your cluster.

### B. LLM Provider Credentials
Fixora requires an active API key from a supported LLM provider:
* **Google Gemini** (Default): Provision at [Google AI Studio](https://aistudio.google.com/).
* **OpenAI**: Provision at [OpenAI Platform](https://platform.openai.com/).
* **Anthropic**: Provision at [Anthropic Console](https://console.anthropic.com/).

### C. Version Control Authentication (Optional)
To enable automated GitOps remediation (Pull Request generation), provide a Personal Access Token (PAT) with appropriate scopes:
* **GitHub:** `repo` scope.
* **GitLab:** `api` scope.

---

## 2. Cluster RBAC Requirements

Fixora requires specific Role-Based Access Control (RBAC) permissions to inspect failing pods, fetch logs, and query replica sets. 

The Helm chart provisions a `ServiceAccount`, `ClusterRole`, and `ClusterRoleBinding` by default. If deploying manually, ensure Fixora has `get`, `list`, and `watch` permissions on the following resources:
* `pods`, `pods/log`
* `deployments`, `statefulsets`, `daemonsets`, `replicasets`
* `events`

---

## 3. Configuration Parameters

Fixora utilizes a hierarchical configuration model via `values.yaml` or injected environment variables.

| Helm Value | Environment Variable | Type | Description |
| :--- | :--- | :--- | :--- |
| `slack.token` | `SLACK_TOKEN` | `string` | Slack Bot User OAuth Token (`xoxb-`). |
| `slack.signingSecret` | `SLACK_SIGNING_SECRET` | `secret` | Slack App Signing Secret for request verification. |
| `slack.channel` | `SLACK_CHANNEL` | `string` | Target Slack channel ID or name (e.g., `#ops-diagnostics`). |
| `googleChat.webhookUrl` | `GOOGLE_CHAT_WEBHOOK_URL` | `string` | Google Chat incoming webhook URL. |
| `mode` | `FIXORA_MODE` | `string` | Operating mode: `auto-fix`, `click-to-fix`, or `dry-run`. |
| `ai.apiKey` | `AI_API_KEY` | `secret` | API Key for your designated LLM provider. |
| `ai.provider` | `AI_PROVIDER` | `string` | Selected engine: `gemini`, `openai`, or `anthropic`. |
| `ai.model` | `AI_MODEL` | `string` | (Optional) Specific model version (e.g., `gpt-4o-mini`). |
| `webhook.token` | `WEBHOOK_TOKEN` | `secret` | (Optional) Bearer token for securing the Alertmanager endpoint. |
| `alertmanager.enabled` | `ALERTMANAGER_ENABLED` | `boolean` | (Optional) Toggles whether to listen for Alertmanager webhooks or watch pods directly. |
| `features.argocd.enabled` | `ARGOCD_ENABLED` | `boolean` | Toggles automatic repository discovery via ArgoCD API. |
| `features.database.host`| `DB_HOST` | `string` | Postgres Database Host for persisting incident history. |

---

## 4. Installation via Helm

Deploying via the official Helm chart is the recommended standard for production environments.

```bash
# 1. Clone the repository
git clone [https://github.com/baka126/fixora.git](https://github.com/baka126/fixora.git)
cd fixora

# 2. Create a custom configuration file
cat <<EOF > fixora-values.yaml
slack:
  token: "xoxb-your-token"
  signingSecret: "your-signing-secret"
  channel: "#ops-diagnostics"
googleChat:
  webhookUrl: "https://chat.googleapis.com/v1/spaces/..."
ai:
  provider: "gemini"
  apiKey: "your-ai-api-key"
mode: "click-to-fix"
features:
  argocd:
    enabled: true
  history:
    crdEnabled: true
EOF

# 3. Deploy the chart
helm upgrade --install fixora ./charts/fixora \
  --namespace fixora \
  --create-namespace \
  -f fixora-values.yaml

```
## 4. Connecting Alertmanager

Fixora acts as an Alertmanager **receiver**. You must configure your Alertmanager to send firing alerts to Fixora's `/alerts` endpoint.

Update your `alertmanager.yaml` or `AlertmanagerConfig` CRD:

```yaml
receivers:
  - name: 'fixora-analyzer'
    webhook_configs:
      - url: '[http://fixora.fixora.svc.cluster.local:8080/alerts](http://fixora.fixora.svc.cluster.local:8080/alerts)'
        send_resolved: false
        http_config:
          bearer_token: 'your-webhook-token-here' # Must match WEBHOOK_TOKEN

route:
  group_by: ['alertname', 'namespace', 'pod']
  group_wait: 30s
  group_interval: 5m
  routes:
    - matchers:
        - alertname =~ "KubePodCrashLooping|KubeMemoryOvercommit|PodOOMKilled"
      receiver: 'fixora-analyzer'
      continue: true # Allows other receivers (like PagerDuty) to still trigger
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
metadata:
  annotations:
    fixora.io/repo-url: "[https://github.com/my-org/core-services](https://github.com/my-org/core-services)"
    fixora.io/repo-path: "helm/values/production.yaml"
    fixora.io/vcs-type: "github"
```

---

## 6. Verification

Once deployed, you can verify Fixora is running by checking the logs:

```bash
kubectl logs -l app.kubernetes.io/name=fixora -n fixora
```

You should see:
`{"level":"info","msg":"Initializing Fixora AI Engine","provider":"gemini"}`
`{"level":"info","msg":"Informer cache synced successfully"}`
`{"level":"info","msg":"Webhook listener active","port":8080}`

When an alert fires, Fixora will post a message to **Slack** and/or **Google Chat** with the **Evidence Chain**. 

Depending on your `mode`:
- **`auto-fix`**: Fixora will automatically create a remediation Pull Request.
- **`click-to-fix`**: Fixora will provide an "Approve" button in the chat to trigger PR creation.
- **`dry-run`**: Fixora will only report the findings without taking action.
