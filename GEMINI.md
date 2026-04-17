# Project Name: Fixora (AI-Powered K8s Forensic Detective)

## 1. Project Overview
Fixora is a diagnostic platform and Slack/Google Chat bot tailored for DevOps and platform teams. It focuses exclusively on diagnosing and safely remediating `OOMKilled` (Exit 137) and `CrashLoopBackOff` events in Kubernetes.

It acts as a "Forensic Detective," emphasizing zero-trust security, undeniable mathematical evidence, and safe, isolated remediation over automated, unverified patching.

## 2. Core Architecture & Philosophy
* **Zero-Trust Security:** The bot emphasizes minimal privileges. It securely queries APIs (Prometheus, K8s Metrics, etc.) without requiring direct, inbound production access where possible.
* **The Evidence Chain:** AI alone is not trusted. Alerts must follow this visual logic tree: `[Metric Proof] + [Cluster Context] + [Historical Pattern] = [Root Cause]`.
* **FinOps Methodology:** Every diagnostic report explicitly states the dollar impact (e.g., "+$2.10/mo AWS compute cost vs. preventing a $5,000 outage") to align engineering fixes with financial practices.
* **Stateful Predictive Analysis (Dedicated Database):** Moving away from CRDs, Fixora persists all incident history, resolutions, and metadata into a dedicated database. This historical context powers the predictive AI model to solve current and future issues.
* **Configuration & Dashboard UI:** A dedicated web UI allows teams to configure application integrations (Slack, VCS, AI providers), monitor application health, and view critical FinOps and predictive metrics.

## 3. Key Feature Specifications

### 3.1. Operational Modes
Fixora supports three distinct working modes:
1. **Alert & Suggest (Dry-Run):** Triggers an alert sent to Slack or Google Workspace containing the Evidence Chain and a suggested fix, without taking any action.
2. **GitOps Remediation (Auto-PR):** Automatically generates and opens a PR/MR to the target GitHub or GitLab repository with the validated configuration fix.
3. **ChatOps Confirmation (Click-to-Fix):** "Auto-fix" mode where Fixora proposes a solution in the chat channel, and the user must manually click to confirm and apply the changes.

### 3.2. Issue Identification (Trigger Mechanisms)
To ensure Fixora works across all K8s platforms, it supports multiple alert identification methods:
* **Primary:** Ingest JSON payloads from Alertmanager via a webhook endpoint.
* **Secondary (Platform Agnostic):** Directly watch the Kubernetes API for Pod events (e.g., `CrashLoopBackOff`, `OOMKilled`) or inspect `kube-state-metrics` to natively identify issues without external alert routers.

### 3.3. Metric Gathering
* **Primary:** Query Prometheus using PromQL for historical memory/CPU limits and usage.
* **Fallback:** If Prometheus is unavailable, query `kube-state-metrics` or directly query the Kubernetes Metrics API (Pod metrics) to gather the required evidence.

### 3.4. Multi-Model AI & Contextualization
* **Provider-Agnostic:** Support multiple AI providers (e.g., Gemini, OpenAI, Anthropic).
* **Predictive Forecasting:** Analyze historical resource utilization metrics from the dedicated database to predict time-to-OOM and flag leak trajectories.
* **Log Translation:** Parse raw `kubectl logs -p` and translate stack traces into plain English summaries for the Evidence Chain.

### 3.5. Version Control & Repo Discovery
* **Automated Discovery:** Dynamically discover the source repository for a failing pod by inspecting ArgoCD Application CRDs or fallback to specific Kubernetes annotations/labels (e.g., `app.kubernetes.io/repository`).
* **Multi-Secret Support:** Support application-specific or namespace-specific VCS secrets (GitHub/GitLab tokens) rather than relying on a single global token, allowing Fixora to operate securely in multi-tenant clusters.

### 3.6. Interactive Debugging & Shadow Pods
* **Streaming Terminal:** Web interface (Xterm.js/WebSockets) for developers to securely `kubectl exec` into an isolated "Shadow Pod" mockup environment to test fixes.
* **Ephemeral Environments:** Strictly enforced 15-minute TTLs with a Confidence Score indicating environmental parity with production.

## 4. Technology Stack Expectations
* **VCS Integration:** GitHub API and GitLab API with multi-secret support.
* **AI / ML:** Multi-LLM routing logic.
* **Integration:** Slack API (Bolt), Google Chat webhooks.
* **Monitoring/K8s:** Kubernetes API, Metrics API, Prometheus, kube-state-metrics.
* **Frontend/UI:** React (for the configuration dashboard and Shadow Pod terminal).
* **Backend:** Go (preferred for K8s operators/bots) with a dedicated Database (e.g., PostgreSQL) for history and state.

## 5. Development Instructions for AI CLI
1.  **Iterative Build:** Do not attempt to build the entire system at once. We will vibe code this iteratively.
2.  **Architecture Alignment:** Update existing logic to reflect the move towards a dedicated database (away from CRDs) and support multiple metric fallbacks.
3.  **Code Quality:** Prioritize clean, modular, and heavily commented Go code. Adhere to strict least-privilege security standards.
