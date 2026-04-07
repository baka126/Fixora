# Project Name: Fixora (AI-Powered K8s Forensic Detective)

## 1. Project Overview
Fixora is a Read-Only Slackbot and diagnostic platform tailored for DevOps and platform teams. It focuses exclusively on diagnosing and safely remediating `OOMKilled` (Exit 137) and `CrashLoopBackOff` events in Kubernetes. 

It acts as a "Forensic Detective," emphasizing zero-trust security, undeniable mathematical evidence, and safe, isolated remediation over automated, unverified patching. 

## 2. Core Architecture & Philosophy
* **Zero-Trust Security (Pull-Not-Push):** The bot requires NO inbound access to production. It listens for outbound Alertmanager webhooks and queries the Prometheus/Grafana API securely.
* **The Evidence Chain:** AI alone is not trusted. Slack alerts must follow this visual logic tree: `[Metric Proof] + [Cluster Context] + [Historical Pattern] = [Root Cause]`.
* **Safe Remediation via GitOps:** Fixora does NOT patch production directly. It creates a "Shadow Clone" pod with the proposed fix in a mockup environment (provisioned on our AWS infrastructure using Docker/K8s). It generates a Pull Request/Merge Request for the user to sync via their existing GitOps pipeline.
* **FinOps Hook:** Every diagnostic report explicitly states the dollar impact (e.g., "+$2.10/mo AWS compute cost vs. preventing a $5,000 outage").

## 3. Key Feature Specifications

### 3.1. Read-Only Slackbot & Evidence Engine
* **Trigger:** Ingest JSON payloads from Alertmanager.
* **Action:** Query Prometheus for historical memory/CPU limits and crash logs.
* **Output:** Construct a Slack Block Kit message displaying the "Evidence Chain" without overwhelming the channel.

### 3.2. Multi-Model AI & Predictive Monitoring
* **Provider-Agnostic AI Integration:** The architecture must support multiple AI providers (e.g., Gemini, OpenAI, Anthropic) and allow the user to select their preferred model for log parsing and root-cause analysis based on cost or privacy requirements.
* **Predictive Forecasting:** Implement an AI model that analyzes historical resource utilization metrics from Prometheus. Instead of just reacting to OOMKills, the system flags workloads exhibiting a "leak trajectory" and predicts time-to-OOM.
* **Anomaly Detection:** Use baseline monitoring to identify abnormal pod restart frequencies or network latency spikes *before* they trigger a hard `CrashLoopBackOff`.
* **AI Contextualization:** Use the selected LLM to parse raw `kubectl logs -p` and translate Java/Node/Python stack traces into plain English summaries, appending this to the Evidence Chain.

### 3.3. Interactive Debugging & Shadow Pods
* **Streaming Terminal:** Implement an Xterm.js/WebSocket web interface that allows developers to `kubectl exec` into their isolated Shadow Pod without exposing prod cluster credentials.
* **Ephemeral Environments:** Ensure strictly enforced 15-minute TTLs for all debug sessions using pod-specific RBAC.
* **Side-by-Side Proof:** UI must support Terminal A (Read-Only Prod Logs) alongside Terminal B (Interactive Lab) to visually confirm the fix.
* **Confidence Score:** Calculate and display a parity metric (50%–95%) indicating how closely the Shadow Pod environment mimics production (e.g., accounting for mocked databases or stateful sets).

### 3.4. Version Control Integration
* **GitHub & GitLab Support:** The system must interface seamlessly with both GitHub (via GitHub Apps/Personal Access Tokens) and GitLab (via OAuth/Project Access Tokens) to automatically generate PRs/MRs with the validated configuration fixes.

### 3.5. The Sales "Sandbox"
* **Chaos Event Triggers:** Build a suite of controlled scripts (e.g., a rapid memory allocator, a faulty ConfigMap injector) that users can trigger via Slack in a dedicated test cluster to watch the bot diagnose the issue in real-time.

## 4. Technology Stack Expectations
* **VCS Integration:** GitHub API (PyGithub/Go-GitHub) and GitLab API (python-gitlab/xanzy/go-gitlab).
* **AI / ML:** Multi-LLM routing logic (supporting Google Cloud Vertex AI/Gemini, OpenAI API, Anthropic API).
* **Integration:** Slack API (Bolt).
* **Monitoring/K8s:** Kubernetes API, Prometheus Query Language (PromQL), Grafana dashboards.
* **Frontend/Terminal:** React, Xterm.js, WebSockets.
* **Backend:** Python or Go (preferred for K8s operators/bots).

## 5. Development Instructions for AI CLI
1.  **Iterative Build:** Do not attempt to build the entire system at once. We will vibe code this iteratively.
2.  **Phase 1:** Start by scaffolding the Slackbot webhook listener and the Prometheus query logic. Prove the "Evidence Chain" can be formatted correctly in Slack Block Kit. Build a modular interface for the LLM provider.
3.  **Phase 2:** Implement the automated GitOps PR/MR generator. Build the abstraction layer that handles both GitHub and GitLab API routing.
5.  **Code Quality:** Prioritize clean, modular, and heavily commented code. Assume all Kubernetes manifests and API interactions must adhere to strict least-privilege security standards.