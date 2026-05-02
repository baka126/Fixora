# Fixora

> AI-Powered K8s Forensic Detective

Fixora is an "Omni-Aware" diagnostics platform and Slack/Google Chat bot tailored for DevOps and platform teams. It acts as an **Enterprise Reliability Platform**, emphasizing zero-trust security, undeniable mathematical evidence, and safe, isolated remediation.

## 🚀 Quick Links

* [**Deployment Guide**](deployment.md) - Get Fixora running in your cluster.
* [**Architecture & Philosophy**](#core-architecture--philosophy) - Understand the design principles.

## Core Architecture & Philosophy

### Zero-Trust Security
Fixora emphasizes minimal privileges. It securely queries APIs (Prometheus, K8s Metrics, K8s Event stream) without requiring direct, inbound production access.

### The Evidence Chain
AI alone is not trusted. Alerts follow a rigorous visual logic tree:
`[Metric Proof] + [Dependency Graph Context] + [Historical Pattern] = [Root Cause] & [Confidence Score]`

### FinOps & Cost of Downtime (CoD)
Every diagnostic report explicitly states the dollar impact. This includes both the cost change of the proposed fix and the **Cost of Downtime (CoD)** for application errors, aligning engineering fixes with financial practices.

## Features

- **Omni-Aware Diagnostics**: Handles a wide range of failures:
    - **K8s Infrastructure**: Scheduling (`Pending`), Capacity (`NodeNotReady`), Configuration (`ImagePullBackOff`).
    - **Application Layer**: HTTP 5xx spikes, latency degradation, and application panics.
- **Real-time Event Streaming**: Actively watches the K8s Event stream to build real-time dependency graphs.
- **AI Confidence Scoring**: Intelligent auto-downgrade logic. If AI confidence is below 85%, Fixora automatically switches from Auto-PR to Dry-Run mode.
- **Pre-Flight Validation**: Before suggesting a GitOps PR, Fixora runs `helm template` or `kubectl diff` in a sandbox to ensure the fix is valid.
- **Next-Gen ChatOps**: Interactive triage trees in Slack/Google Chat with dynamic buttons (e.g., `[Show Stack Trace]`, `[View FinOps Impact]`, `[Execute Fix]`).
- **Security & Privacy**: Built-in **PII Scrubbing** automatically removes sensitive data from logs before AI analysis.
- **Stateful Predictive Analysis**: Persists incident history in a dedicated PostgreSQL database to power predictive AI models.
- **Multi-Platform Notifications**: Native support for **Slack** and **Google Workspace (Chat)**.
- **GitOps Ready**: Automated PR/MR generation via GitHub/GitLab with **Multi-Tenant VCS** support.

