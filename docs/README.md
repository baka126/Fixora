# Fixora

> AI-Powered K8s Forensic Detective

Fixora is a Read-Only Slackbot and diagnostic platform tailored for DevOps and platform teams. It focuses exclusively on diagnosing and safely remediating `OOMKilled` (Exit 137) and `CrashLoopBackOff` events in Kubernetes.

## 🚀 Quick Links

* [**Deployment Guide**](deployment.md) - Get Fixora running in your cluster.
* [**Overview**](/) - Learn about the core architecture and features.

## Features

- **Zero-Trust Security**: No inbound access required. Requests are cryptographically verified.
- **Security & Privacy**: Built-in **PII Scrubbing** automatically removes emails, IPs, and tokens from logs before AI analysis.
- **Evidence Chain**: Metric Proof + Cluster Context + Historical Pattern = Root Cause.
- **FinOps Methodology**: Real-time **cost-impact analysis** for every fix, showing the dollar impact of resource changes.
- **AI-Powered Forensics**: Multi-LLM support (Gemini, OpenAI, Anthropic) with custom model selection.
- **Stateful Predictive Analysis**: Persists incident history in a dedicated database to identify recurring patterns and predict time-to-OOM.
- **Operating Modes**: Choose between `auto-fix`, `click-to-fix`, and `dry-run`.
- **Multi-Platform Notifications**: Native support for **Slack** and **Google Workspace (Chat)**.
- **GitOps Ready**: Automated PR/MR generation via GitHub/GitLab with **Multi-Tenant VCS** support.
- **ArgoCD Integrated**: Automatic repository discovery.

