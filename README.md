# Fixora

> AI-Powered Kubernetes Forensic Detective & Remediation Engine.

Fixora analyzes critical cluster failures in real-time and generates automated, self-healing Pull Requests. It focuses exclusively on diagnosing and safely remediating `OOMKilled` (Exit 137) and `CrashLoopBackOff` events.

## 📖 Documentation

The full documentation is available at [https://baka126.github.io/Fixora](https://baka126.github.io/Fixora) (or in the `docs/` folder).

### Quick Links
* [**Overview**](docs/README.md)
* [**Deployment & Setup**](docs/deployment.md)
* [**Architecture**](docs/README.md#features)

## Features

- **Zero-Trust Security**: No inbound access required. Requests are cryptographically verified.
- **Evidence Chain**: Metric Proof + Cluster Context + Historical Pattern = Root Cause.
- **AI-Powered Forensics**: Multi-LLM support (Gemini, OpenAI, Anthropic) with custom model selection.
- **Stateful Predictive Analysis**: Persists incident history in Kubernetes CRDs to identify recurring patterns.
- **Operating Modes**: Choose between `auto-fix`, `click-to-fix` (manual approval), and `dry-run`.
- **Multi-Platform Notifications**: Native support for **Slack** and **Google Workspace (Chat)**.
- **GitOps Ready**: Automated PR/MR generation via GitHub/GitLab.
- **ArgoCD Integrated**: Automatic repository discovery.

## Getting Started

To deploy Fixora in your cluster, follow the [**Deployment Guide**](docs/deployment.md).

```bash
# Quick view of requirements
cat docs/deployment.md | grep "System Prerequisites" -A 10
```

## License

See [LICENSE](LICENSE) (if available) or contact the maintainers.
