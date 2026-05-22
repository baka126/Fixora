# Troubleshooting Guide

Common issues and solutions when deploying and operating Fixora.

## 1. Database Connection Issues
**Symptom:** Logs show `failed to connect to PostgreSQL` or dashboard is empty.
- **Check:** Ensure the `DB_HOST`, `DB_PORT`, and credentials are correct.
- **Internal DB:** If using `embedded` PostgreSQL, ensure the PV is successfully bound.
- **Connectivity:** Run a `pg_isready` check from within the Fixora pod to the database host.

## 2. AI Provider Errors
**Symptom:** `AI analysis failed: 401 Unauthorized` or `429 Too Many Requests`.
- **API Key:** Verify your `AI_API_KEY` is valid and has sufficient quota.
- **Provider URL:** If using a self-hosted model, ensure `AI_BASE_URL` is accessible from the Fixora pod.
- **Context Length:** Large logs might exceed model context limits. Fixora attempts to compress logs, but extremely large outputs may still fail.

## 3. GitOps PR Creation Fails
**Symptom:** Fixora suggests a fix but no PR is created in GitHub/GitLab.
- **Permissions:** Ensure the VCS token has `repo` and `pull_requests` write access.
- **Discovery:** Verify the pod has the required annotations (`fixora.io/repo-url`) or that ArgoCD is properly integrated.
- **Pre-Flight Validation:** Check the logs for `kubectl diff` or `helm template` errors. The PR will not be opened if validation fails.

## 4. Alertmanager Webhook Not Triggering
**Symptom:** Alerts are firing in Prometheus but Fixora doesn't report them.
- **URL:** Ensure Alertmanager is sending to `http://fixora.namespace.svc:8080/webhook/alertmanager`.
- **Token:** Verify the `WEBHOOK_TOKEN` (or Basic Auth) matches between Alertmanager and Fixora.
- **Labels:** Fixora requires `namespace` and `pod` labels on alerts to identify the target workload.

## 5. Slack/Google Chat Buttons Not Working
**Symptom:** Clicking "Approve" or "View Logs" results in an error.
- **Interactivity URL:** Ensure the Slack/Google Chat app is configured with the correct Request URL (e.g., `https://fixora.example.com/slack/interactions`).
- **Signature/Secret:** Double-check the `SLACK_SIGNING_SECRET` or the shared secret for Google Chat.
