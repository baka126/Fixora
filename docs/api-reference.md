# API Reference

Fixora provides a REST API for its dashboard and integrations. Most endpoints require Bearer authentication.

## Authentication

Fixora uses JWT-based authentication.

### Login
`POST /api/v1/auth/login`

**Request Body:**
```json
{
  "username": "admin",
  "password": "your-password"
}
```

**Response:**
```json
{
  "token": "eyJhbG...",
  "user": {
    "id": 1,
    "username": "admin",
    "role": "admin",
    "created_at": "2023-10-27T10:00:00Z"
  }
}
```

---

## Alerts & Diagnostics

### Get Active Alerts
`GET /api/v1/alerts/active`
Requires: `operator` or `admin` role.

Fetches firing alerts from Alertmanager that Fixora has identified but not yet investigated (if configured to wait).

### Include Alert for Investigation
`POST /api/v1/alerts/active/{alert_id}/include`
Requires: `operator` or `admin` role.

Manually triggers an investigation for a specific Alertmanager alert.

---

## Dashboard & Audit

### Dashboard Snapshot
`GET /api/v1/dashboard`
Requires: `viewer` role.

Returns a summary of current system health, active investigations, and recent remediations.

### Investigation Details
`GET /api/v1/audit/investigations/{id}`
Requires: `viewer` role.

Returns the full Evidence Chain and log history for a specific investigation.

### Remediation History
`GET /api/v1/remediations/`
Requires: `viewer` role.

Returns a list of all GitOps remediation attempts and their status.

---

## Webhooks (Internal & External)

### Alertmanager Webhook
`POST /webhook/alertmanager`
Authentication: Shared Secret (X-Fixora-Token or Basic Auth)

Receives alert payloads from Prometheus Alertmanager.

### Slack Interactions
`POST /slack/interactions`
Authentication: Slack Signature Verification

Handles button clicks and modal submissions from Slack.

### Google Chat Interactions
`POST /googlechat/interactions`
Authentication: Shared Secret

Handles interactions from Google Chat cards and buttons.

---

## System

### Health Check
`GET /health`
Returns `OK`.

### Prometheus Metrics
`GET /metrics`
Exposes internal system metrics in Prometheus format.
