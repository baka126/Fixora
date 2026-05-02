# Project Name: Fixora (AI-Powered K8s Forensic Detective)

## 1. Project Overview
Fixora is a diagnostic platform and Slack/Google Chat bot tailored for DevOps and platform teams. Originally focused on isolated resource errors (`OOMKilled`, `CrashLoopBackOff`), it is expanding to an "Omni-Aware" K8s & App Diagnostics platform capable of handling **any Kubernetes and application failure scenario**.

It acts as an "Enterprise Reliability Platform," emphasizing zero-trust security, undeniable mathematical evidence, and safe, isolated remediation over automated, unverified patching.

## 2. Core Architecture & Philosophy
* **Zero-Trust Security:** The bot emphasizes minimal privileges. It securely queries APIs (Prometheus, K8s Metrics, K8s Event stream, etc.) without requiring direct, inbound production access where possible.
* **The Evidence Chain:** AI alone is not trusted. Alerts must follow this visual logic tree: `[Metric Proof] + [Dependency Graph Context] + [Historical Pattern] = [Root Cause] & [Confidence Score]`.
* **FinOps Methodology:** Every diagnostic report explicitly states the dollar impact (e.g., "+$2.10/mo AWS compute cost vs. preventing a $5,000 outage") to align engineering fixes with financial practices. This includes calculating the **Cost of Downtime (CoD)** for application errors.
* **Stateful Predictive Analysis (Dedicated Database):** Moving away from CRDs, Fixora persists all incident history, resolutions, dependency graphs, and metadata into a dedicated database (PostgreSQL). This historical context powers the predictive AI model.
* **Configuration & Dashboard UI:** A dedicated web UI allows teams to configure application integrations (Slack, VCS, AI providers), monitor application health, and view critical FinOps and predictive metrics.

## 3. Key Feature Specifications

### 3.1. Operational Modes
Fixora supports three distinct working modes:
1. **Alert & Suggest (Dry-Run):** Triggers an alert sent to Slack or Google Workspace containing the Evidence Chain and a suggested fix, without taking any action.
2. **GitOps Remediation (Auto-PR):** Automatically generates and opens a PR/MR to the target GitHub or GitLab repository with the validated configuration fix. *Fallback: If AI confidence is < 85%, this mode automatically downgrades to Dry-Run.*
3. **ChatOps Confirmation (Click-to-Fix):** "Auto-fix" mode where Fixora proposes a solution in the chat channel, and the user must manually click to confirm and apply the changes.

### 3.2. Issue Identification (Trigger Mechanisms)
To handle "any" scenario, Fixora identifies failures across multiple domains:
* **Advanced K8s Infrastructure Failures:** Monitors for scheduling/capacity issues (`Pending`, `NodeNotReady`), configuration errors (`CreateContainerConfigError`, `ImagePullBackOff`), and network/routing misconfigurations.
* **Application-Layer Failures:** Tracks HTTP 5xx spikes and latency degradation (via standard Prometheus/Ingress metrics) and application panics/deadlocks.
* **Primary Source:** Ingest JSON payloads from Alertmanager via a webhook endpoint.
* **Real-time Event Streaming:** Actively watches the K8s Event stream to build real-time dependency graphs.

### 3.3. Metric Gathering
* **Primary:** Query Prometheus using PromQL for historical limits, usage, error rates, and latency.
* **Fallback:** If Prometheus is unavailable, query `kube-state-metrics` or directly query the Kubernetes Metrics API (Pod metrics) to gather the required evidence.

### 3.4. Multi-Model AI & Contextualization
* **Provider-Agnostic:** Support multiple AI providers (e.g., Gemini, OpenAI, Anthropic).
* **Predictive Forecasting:** Analyze historical resource utilization metrics from the dedicated database to predict time-to-OOM and flag leak trajectories.
* **Log Translation & Context:** Parse raw `kubectl logs -p` and translate stack traces into plain English summaries. AI assertions must cite specific sources (e.g., pointing to the exact line in a Secret).
* **Pre-Flight Validation:** Before suggesting a GitOps PR for a configuration change, Fixora runs `helm template` or `kubectl diff` in a sandbox to guarantee the proposed fix compiles successfully.

### 3.5. Version Control & Repo Discovery
* **Automated Discovery:** Dynamically discover the source repository for a failing pod by inspecting ArgoCD Application CRDs or fallback to specific Kubernetes annotations/labels (e.g., `app.kubernetes.io/repository`).
* **Multi-Secret Support:** Support application-specific or namespace-specific VCS secrets (GitHub/GitLab tokens) rather than relying on a single global token.

### 3.6. Next-Gen ChatOps Workflows
* **Interactive Triage Trees:** Alerts feature dynamic buttons in Slack/Google Chat (e.g., `[Show Stack Trace]`, `[View FinOps Impact]`, `[Simulate PR]`, `[Execute Fix]`) to allow deep-dives without channel clutter.
* **Automated Blameless Post-Mortems:** Upon resolution, Fixora automatically compiles the Evidence Chain, timeline, and FinOps impact into a Markdown document and pushes it to documentation platforms.

## 4. Technology Stack Expectations
* **VCS Integration:** GitHub API and GitLab API with multi-secret support.
* **AI / ML:** Multi-LLM routing logic.
* **Integration:** Slack API (Bolt), Google Chat webhooks.
* **Monitoring/K8s:** Kubernetes API, Metrics API, Prometheus, kube-state-metrics. *(Note: 3rd-party dependencies like OpenTelemetry and Log Aggregators are excluded for now.)*
* **Frontend/UI:** React (for the configuration dashboard).
* **Backend:** Go (preferred for K8s operators/bots) with a dedicated Database (e.g., PostgreSQL) for history, state, and dependency graphs.

## 5. Development Instructions for AI CLI
1.  **Iterative Build:** Do not attempt to build the entire system at once. We will vibe code this iteratively.
2.  **Architecture Alignment:** Update existing logic to reflect the move towards a dedicated database (away from CRDs) and support multiple metric fallbacks.
3.  **Code Quality:** Prioritize clean, modular, and heavily commented Go code. Adhere to strict least-privilege security standards.
