# Fixora

> AI-Powered Kubernetes Forensic Detective & Remediation Engine.

Fixora is an "Omni-Aware" K8s & App Diagnostics platform that analyzes critical cluster failures in real-time and generates automated, self-healing Pull Requests. It goes beyond isolated resource errors, acting as an enterprise-grade reliability platform that emphasizes zero-trust security and undeniable mathematical evidence.

## 📖 Documentation

The full documentation is available at [https://baka126.github.io/Fixora](https://baka126.github.io/Fixora) (or in the `docs/` folder).

### Quick Links
* [**Overview**](docs/README.md)
* [**Deployment & Setup**](docs/deployment.md)
* [**Architecture**](docs/README.md#features)

## Features

- **Omni-Aware Diagnostics**: Handles any Kubernetes and application failure scenario, including scheduling/capacity issues (`Pending`, `NodeNotReady`), configuration errors (`ImagePullBackOff`), and application-layer spikes (5xx, latency).
- **Real-time Event Streaming**: Actively watches the K8s Event stream to build real-time dependency graphs and contextualize failures.
- **Evidence Chain**: `[Metric Proof] + [Dependency Graph Context] + [Historical Pattern] = [Root Cause] & [Confidence Score]`.
- **AI Confidence Scoring**: Intelligent auto-downgrade logic. If AI confidence is below 85%, Fixora automatically switches from Auto-PR to Dry-Run mode to ensure safety.
- **Pre-Flight Validation**: Proposed fixes are validated in a sandbox using `helm template` or `kubectl diff` before being presented or applied.
- **FinOps & Cost of Downtime (CoD)**: Beyond resource cost changes, Fixora calculates the dollar impact of downtime (CoD) to align engineering fixes with business value.
- **Next-Gen ChatOps**: Interactive triage trees in Slack and Google Chat with dynamic buttons for `[Show Stack Trace]`, `[View FinOps Impact]`, and `[Execute Fix]`.
- **Zero-Trust Security**: No inbound production access required. Minimal privilege API queries for Prometheus, K8s Metrics, and Events.
- **Stateful Predictive Analysis**: Dedicated PostgreSQL database persists incident history, resolutions, and dependency graphs to power predictive AI models.
- **GitOps & Multi-Tenant VCS**: Automated PR/MR generation via GitHub/GitLab with support for namespace-specific secrets and ArgoCD discovery.

## Getting Started

To deploy Fixora in your cluster, follow the [**Deployment Guide**](docs/deployment.md).

```bash
# Quick view of requirements
cat docs/deployment.md | grep "System Prerequisites" -A 10
```

## License

See [LICENSE](LICENSE) (if available) or contact the maintainers.
