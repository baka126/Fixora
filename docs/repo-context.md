# Fixora Repo Context

Last updated: 2026-05-03

## Purpose

Fixora is an AI-assisted Kubernetes diagnostics and GitOps remediation service. It watches cluster failures, gathers evidence from Kubernetes, Prometheus, Alertmanager, logs, history, and optional ArgoCD repository metadata, then reports findings and can generate remediation pull requests.

## Current Architecture

- `cmd/fixora/main.go`: process startup, Kubernetes client setup, signal handling, server/controller lifecycle.
- `pkg/config`: environment-driven configuration and operating mode policy.
- `pkg/server`: HTTP endpoints for health checks, Alertmanager webhooks, Slack interactions, and Google Chat interactions.
- `pkg/controller`: core diagnostic loop, pod watching, scanners, evidence generation, remediation orchestration, pending approval handling, and issue classifiers.
- `pkg/ai`: provider abstraction and prompt logic for OpenAI, Anthropic, and Gemini.
- `pkg/vcs`: GitHub/GitLab PR creation and repository file fetching.
- `pkg/validation`: YAML/Kubernetes manifest validation before PR creation.
- `pkg/notifications`: Slack and Google Chat reporting.
- `pkg/metrics` and `pkg/prometheus`: metrics provider abstraction and Prometheus queries.
- `pkg/events`: dependency graph/event streaming into Postgres.
- `pkg/finops`: cost and risk impact estimation.
- `charts/fixora`: Helm chart for Kubernetes deployment.

## Safety Posture Already Improved

- Default operating mode is now `dry-run`, not `auto-fix`.
- Inbound HTTP handlers require explicit methods, bounded request bodies, and authentication.
- Slack interactions verify Slack request signatures.
- Alertmanager and Google Chat inbound calls require `WEBHOOK_TOKEN` or basic auth.
- Server uses its own mux, timeouts, signal-aware shutdown, and graceful controller stop.
- AI-generated patches are constrained to trusted discovered repositories and allowlisted YAML files that Fixora fetched.
- Namespace-specific VCS tokens are not persisted for pending approvals; approval time re-reads namespace/global provider configuration.
- Helm defaults run the app as non-root with dropped capabilities, read-only root filesystem, writable `/tmp`, generated embedded DB password, and deployment mutation RBAC disabled by default.

## Issue Finding Work Completed

Structured issue classification was added in `pkg/controller/diagnosis.go` and integrated into `diagnosePod`.

Classifiers now produce:

- `Symptom`
- `Category`
- `LikelyCause`
- `Confidence`
- `PatchStrategy`
- `Evidence`
- `Related` resources

Current categories:

- `scheduling-capacity`
- `runtime`
- `configuration`
- `rollout`
- `network`
- `storage`
- `unknown`

Current patch strategies:

- `resources`
- `image`
- `env-or-volume-ref`
- `scheduling-policy`
- `probe`
- `service-selector`
- `pvc-or-volume`
- `none`

Focused tests live in `pkg/controller/diagnosis_test.go`.

## PR Workflow Work Completed

Step 1, smaller targeted PR generation, is implemented in `pkg/controller/pr_workflow.go`.

Current behavior:

- PRs are built from the structured diagnosis patch strategy instead of generic "multi-resource" metadata.
- Branch names include the patch strategy, pod name, target file scope, and timestamp.
- Titles and commit messages are strategy-specific, for example resource, image, config reference, scheduling, probe, service routing, or volume fixes.
- One repo/file change group becomes one PR option, so unrelated manifest edits are not bundled into a broad multi-resource PR.
- Click-to-fix mode now creates a pending approval for each targeted PR plan instead of only saving the first plan.
- Auto-fix rate limiting is evaluated per targeted PR.
- Dry-run notifications report how many targeted PR plans were generated.

Focused tests live in `pkg/controller/pr_workflow_test.go`.

## GitOps Source Resolution Work Completed

The first implementation pass for deep GitOps integration is in place.

New package:

- `pkg/gitops`: deterministic workload-to-source resolution.

Current behavior:

- Resolves ArgoCD `Application` sources by matching the failing pod owner chain against `status.resources`.
- Resolves Flux `Kustomization` sources by matching Flux inventory entries to the pod or owner workload.
- Resolves Flux `HelmRelease` sources by matching common Flux/Helm labels on pods.
- Falls back to `fixora.io/repo-url`, `fixora.io/repo-path`, and `fixora.io/target-revision` annotations only when controller resolution finds nothing.
- Extracts repo URL, revision, path, controller type, app name, app namespace, manifest type, overlay role, environment, and region.
- Detects manifest type as raw manifests, Helm, Kustomize, or Flux HelmRelease using controller metadata and source-path/filesystem hints.
- Identifies fleet/overlay paths such as `overlays/prod/us-east-1` and prefers overlay metadata over base-level guessing.

Remediation integration:

- `handleRemediation` now consumes resolved GitOps sources instead of the older ArgoCD-only dependency map.
- AI context includes GitOps source summaries and manifest-type-specific instructions.
- Kustomize sources are treated specially: raw/base workload files are context, while editable files are `kustomization.yaml`, existing patch files, or an allowed generated patch path under `fixora-patches/`.
- `vcs.FileChange` now has `Create` so GitLab can create generated patch files instead of always issuing update actions.

Focused tests live in `pkg/gitops/resolver_test.go`.

## Closed-Loop Learning Work Completed

The first implementation pass for remediation outcome tracking is in place.

New files:

- `pkg/controller/closed_loop.go`: remediation lifecycle persistence, failed-attempt feedback summaries, and helpers.
- `pkg/controller/remediation_monitor.go`: background PR/MR status polling and post-merge observation.
- `pkg/controller/closed_loop_test.go`: focused tests for changed-file tracking and feedback formatting.

Current behavior:

- Postgres now stores remediation lifecycle rows in `remediation_outcomes`.
- Each generated targeted PR plan is recorded with investigation ID, diagnosis category, patch strategy, VCS type, repo, base/head branch, GitOps source metadata, changed files, status, and failure reason.
- Remediation rows also store workload identity (`workload_kind`, `workload_name`, `workload_selector`) so post-rollout monitoring can evaluate replacement pods, not only the original pod name.
- Click-to-fix pending approvals store `remediation_id`, so approval, expiry, provider failures, and successful PR creation update the same lifecycle row.
- Auto-fix PR creation updates lifecycle rows to `pr_opened` or `pr_failed`.
- Dry-run mode records generated plans without opening PRs.
- Failed prior remediations are fed back into future patch prompts as closed-loop feedback so the AI is warned not to repeat a failed strategy.
- GitHub and GitLab providers can now poll PR/MR state by head branch.
- A background monitor promotes merged remediation PRs into `observing`, waits a short observation delay, checks ArgoCD sync/health or Flux `Ready` conditions when available, and marks `production_failed` if GitOps health degrades, the workload enters obvious bad states, or the workload exceeds the configured HTTP error-rate threshold.
- When a remediation reaches `production_failed`, Fixora now generates a revert PR that only restores files it changed. Existing files are restored from recorded previous content, and generated files are deleted.
- High-risk PR plans are scored before auto-fix. Production/base overlays, image changes, scheduling/dependency changes, CI/RBAC/Secret paths, file creation/deletion, and low confidence can force auto-fix down to click-to-fix approval.

Resource correlation work:

- `pkg/controller/resource_correlation.go` summarizes related Kubernetes resources for a failing pod.
- The evidence chain and AI forensic context now include owner rollout status, Services/Endpoints, PVCs, Secret/ConfigMap references and missing keys, node pressure, and NetworkPolicies selecting the pod.

Manifest-aware PR strategy work:

- Kustomize patch files and `kustomization.yaml` updates are kept together in one PR.
- Kustomize generated patch files are rejected unless `kustomization.yaml` is updated in the same patch set.

Validation and policy work:

- `pkg/validation` now includes a render sandbox that merges fetched source files and AI changes in a temporary workspace.
- The sandbox runs `kustomize build` or `kubectl kustomize` for Kustomize sources when available.
- The sandbox runs `helm template` for Helm sources when `Chart.yaml` is available.
- `VALIDATION_SANDBOX_ENABLED` defaults to true.
- `VALIDATION_REQUIRE_RENDER` defaults to false, so missing render tools skip render validation instead of blocking by default.
- `VALIDATION_TOOL_TIMEOUT` defaults to 15 seconds.
- `POLICY_GUARDRAILS_ENABLED` defaults to true.
- Policy guardrails hard-reject CI workflow changes, RBAC manifests, Secret manifests, and image registries outside `ALLOWED_IMAGE_REGISTRIES` when that allowlist is configured.

Observability work:

- `pkg/telemetry` exposes Prometheus counters for investigations, remediation lifecycle events, validation results, and policy rejections.
- `pkg/server` exposes `/metrics` via `promhttp`.

Not implemented yet:

- External policy engines such as OPA/conftest/Kyverno CLI.
- Render-aware semantic verification that the intended Deployment/container field changed after Helm/Kustomize render.

## Next Target

The next planned improvement is render-aware semantic verification.

Recommended implementation direction:

- After render validation, parse rendered manifests and verify the affected workload/container changed according to the diagnosis strategy.
- Examples: resource changes must alter the target container resources; probe fixes must alter a probe; service selector fixes must alter selectors/labels.
- Feed semantic validation results into PR bodies and remediation outcome rows.

## Validation Commands

Use a writable Go cache in this environment:

```bash
GOCACHE=/private/tmp/fixora-gocache go test ./...
GOCACHE=/private/tmp/fixora-gocache go vet ./...
helm template fixora charts/fixora
```

## Dirty Worktree Notes

At the time this context was created, there were unrelated local changes outside the diagnosis work:

- `.gitignore` modified
- `patch_controller_fixed.py` deleted
- `update_controller.py` deleted

Do not revert unrelated user changes unless explicitly requested.
