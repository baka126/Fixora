# Deploying Fixora

Fixora is an "Omni-Aware" enterprise-grade diagnostic platform that acts as an intelligent forensic detective for your Kubernetes clusters. It monitors real-time event streams and Prometheus alerts to diagnose any failure scenario, providing an Evidence Chain and automated GitOps remediation.

---

## 1. System Prerequisites

Ensure your infrastructure meets the following requirements:

### A. Communication & Alerting
* **Slack Workspace (Optional):** Create a Slack App with `chat:write` and `commands` scopes for interactive triage trees.
* **Google Workspace (Optional):** Enable a Webhook or configure a Chat App for interactive cards.
* **Prometheus Stack:** A functional `kube-prometheus-stack` routing alerts to Fixora.

### B. Persistent Storage (PostgreSQL)
Fixora now uses a dedicated PostgreSQL database to persist incident history, resolutions, and dependency graphs. 
* **Managed Database:** (Recommended) AWS RDS, GCP Cloud SQL, etc.
* **Embedded Database:** The Helm chart can deploy an internal PostgreSQL instance for testing.

### C. Validation Sandbox
To support **Pre-Flight Validation**, the Fixora pod environment must have access to:
* `kubectl`: For running `kubectl diff`.
* `helm`: For running `helm template`.
These are included in the default Fixora image.

### D. LLM Provider Credentials
* **Google Gemini**, **OpenAI**, or **Anthropic** API keys.

---

## 2. Cluster RBAC Requirements

Fixora requires permissions to watch the K8s Event stream and inspect resources. The Helm chart provisions these, but for manual setups ensure `get`, `list`, and `watch` on:
* `pods`, `pods/log`
* `deployments`, `statefulsets`, `daemonsets`, `replicasets`
* `events` (Critical for Omni-Aware streaming)
* `nodes` (For node-failure diagnostics)

---

## 3. Configuration Parameters

| Helm Value | Environment Variable | Type | Description |
| :--- | :--- | :--- | :--- |
| `mode` | `FIXORA_MODE` | `string` | `auto-fix`, `click-to-fix`, or `dry-run`. |
| `ai.confidenceThreshold`| `AI_CONFIDENCE_THRESHOLD` | `float` | Confidence score required for Auto-PR (Default: `0.85`). |
| `features.database.host`| `DB_HOST` | `string` | Postgres Database Host (Mandatory for stateful analysis). |
| `finops.costOfDowntimePerHour`| `FINOPS_COD_PER_HOUR` | `float` | Estimated revenue loss per hour of application downtime. |
| `finops.infracostAPIKey`| `INFRACOST_API_KEY` | `secret` | (Optional) Infracost API key for live cloud pricing. |

---

## 4. Installation via Helm

Deploying via the official Helm chart is the recommended standard for production environments.

```bash
# 1. Clone the repository
git clone https://github.com/baka126/fixora.git
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
  database:
    embedded:
      enabled: true
EOF

# 3. Deploy the chart
helm upgrade --install fixora ./charts/fixora \
  --namespace fixora \
  --create-namespace \
  -f fixora-values.yaml
```

---

## 5. Next-Gen ChatOps Interactivity

Fixora alerts now feature **Interactive Triage Trees**. In Slack or Google Chat, you will see dynamic buttons:

1.  **[Show Stack Trace]**: Deciphers raw logs into a plain-English summary.
2.  **[View FinOps Impact]**: Displays a modal with the CoD (Cost of Downtime) and resource cost changes.
3.  **[Simulate PR]**: Performs a dry-run validation using the sandbox tools.
4.  **[Execute Fix]**: (In `click-to-fix` mode) Triggers the GitOps PR creation.

> **Note:** For Google Chat interactivity, follow the "Google Chat App Interactivity" section below.

---

## 6. Omni-Aware Trigger Mechanisms

Fixora identifies failures across multiple domains:
- **K8s Event Watcher**: Real-time detection of `SchedulingFailed`, `NodeNotReady`, or `ImagePullBackOff`.
- **Alertmanager Webhook**: Ingests JSON payloads for standard Prometheus alerts.
- **Dependency Mapping**: Automatically links failing pods to their parent controllers and source Git repositories.

---

## 7. FinOps & Cost of Downtime (CoD)

Fixora helps align engineering with business value by calculating the **Cost of Downtime**. Configure this by setting `finops.costOfDowntimePerHour` in your `values.yaml`. 

Example: If your app generates $5,000/hour, Fixora will highlight the potential $5,000 loss during an outage, providing clear justification for immediate remediation.

---

## 8. Connecting Alertmanager

Fixora acts as an Alertmanager **receiver**. You must configure your Alertmanager to send firing alerts to Fixora's `/alerts` endpoint.

Update your `alertmanager.yaml` or `AlertmanagerConfig` CRD:

```yaml
receivers:
  - name: 'fixora-analyzer'
    webhook_configs:
      - url: 'http://fixora.fixora.svc.cluster.local:8080/alerts'
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

## 9. Enabling GitOps Remediation

Fixora can discover which Git repository manages a Pod in two ways:

### Method A: ArgoCD (Automatic)
Ensure `features.argocd.enabled: true` is set in your values. Fixora will automatically query ArgoCD to find the source repo and path for any crashing pod.

### Method B: Annotations (Manual)
Add these annotations to your Deployment or Pod template:

```yaml
annotations:
metadata:
  annotations:
    fixora.io/repo-url: "https://github.com/my-org/core-services"
    fixora.io/repo-path: "helm/values/production.yaml"
    fixora.io/vcs-type: "github"
```

---

## 10. Verification

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

---

## 11. Advanced Configuration

### A. Multi-Tenant VCS Support
By default, Fixora uses the global `GITHUB_TOKEN` or `GITLAB_TOKEN`. For multi-tenant clusters, you can provide namespace-specific credentials by creating a Secret named `fixora-vcs` in the target namespace:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: fixora-vcs
  namespace: target-app-namespace
type: Opaque
data:
  github-token: <base64-encoded-token>
  # OR
  gitlab-token: <base64-encoded-token>
```

### B. FinOps & Cost Estimation
Fixora automatically calculates the monthly cost impact of suggested resource changes. 
* **Standard Profiles:** Uses built-in pricing for AWS, Azure, and GCP.
* **Infracost Integration:** For live, accurate pricing based on your specific cloud setup, provide an `INFRACOST_API_KEY`.

### C. Security & PII Scrubbing
Fixora is designed with privacy in mind. Before sending any logs to AI providers or notification channels, it automatically scrubs:
* Email addresses
* IPv4 addresses
* Authentication tokens (Bearer, JWT, etc.)
* Common password/secret patterns

---

## 12. Google Chat App Interactivity (Optional)

To enable interactive features in Google Chat (like the **"Approve"** button or the **"View Logs"** explorer), you must configure Fixora as a **Google Chat App** instead of using a simple incoming webhook.

1.  **Create a Google Cloud Project** and enable the **Google Chat API**.
2.  **Configure the App:**
    *   **App Name:** Fixora
    *   **Interactive features:** Enabled
    *   **Functionality:** Receive 1-to-1 messages, Join spaces.
    *   **Connection settings:** Use **App URL**.
    *   **App URL:** `https://your-fixora-ingress.com/googlechat/interactive`
3.  **Permissions:** The App does not require specific OAuth scopes for basic interactivity, but ensure it is allowed to respond in the spaces it is added to.
4.  **Usage:** In `click-to-fix` mode, Fixora will send cards with buttons. Clicking these buttons will send an event to the App URL, which Fixora will process to execute the remediation.
