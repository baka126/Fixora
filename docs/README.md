# Fixora

> AI-Powered K8s Forensic Detective

Fixora is a Read-Only Slackbot and diagnostic platform tailored for DevOps and platform teams. It focuses exclusively on diagnosing and safely remediating `OOMKilled` (Exit 137) and `CrashLoopBackOff` events in Kubernetes.

## 🚀 Quick Links

* [**Deployment Guide**](deployment.md) - Get Fixora running in your cluster.
* [**Overview**](/) - Learn about the core architecture and features.

## Features

- **Zero-Trust Security**: No inbound access required. Requests are cryptographically verified.
- **Evidence Chain**: Metric Proof + Cluster Context + Historical Pattern = Root Cause.
- **AI-Powered Forensics**: Multi-LLM support (Gemini, OpenAI, Anthropic) with custom model selection.
- **Stateful Predictive Analysis**: Persists incident history in Kubernetes CRDs to identify recurring patterns.
- **Operating Modes**: Choose between `auto-fix`, `click-to-fix` (manual approval), and `dry-run`.
- **Multi-Platform Notifications**: Supports both **Slack** and **Google Workspace (Chat)**.
- **GitOps Ready**: Automated PR/MR generation via GitHub/GitLab.
- **ArgoCD Integrated**: Automatic repository discovery.

