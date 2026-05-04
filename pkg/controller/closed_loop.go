package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"fixora/pkg/gitops"
	"fixora/pkg/vcs"
	v1 "k8s.io/api/core/v1"
)

type RemediationStatus string

const (
	RemediationGenerated        RemediationStatus = "generated"
	RemediationPendingApproval  RemediationStatus = "pending_approval"
	RemediationPROpened         RemediationStatus = "pr_opened"
	RemediationPRFailed         RemediationStatus = "pr_failed"
	RemediationObserving        RemediationStatus = "observing"
	RemediationSucceeded        RemediationStatus = "succeeded"
	RemediationProductionFailed RemediationStatus = "production_failed"
	RemediationRevertOpened     RemediationStatus = "revert_opened"
	RemediationRevertFailed     RemediationStatus = "revert_failed"
	RemediationReverted         RemediationStatus = "reverted"
)

type RemediationRecord struct {
	ID                int64
	InvestigationID   int64
	Namespace         string
	PodName           string
	DiagnosisCategory string
	PatchStrategy     string
	Status            RemediationStatus
	VCSType           string
	Options           vcs.PullRequestOptions
	Source            gitops.WorkloadSource
	PRURL             string
	FailureReason     string
	UpdatedAt         time.Time
	ChangedFiles      []remediationChangedFile
	RevertPRURL       string
	RevertHeadBranch  string
	WorkloadKind      string
	WorkloadName      string
	WorkloadSelector  string
}

type remediationChangedFile struct {
	FilePath        string `json:"file_path"`
	PreviousContent []byte `json:"previous_content,omitempty"`
	HasPrevious     bool   `json:"has_previous"`
	Create          bool   `json:"create"`
}

func (h *historyCache) SaveRemediation(ctx context.Context, rec RemediationRecord) int64 {
	if h == nil || h.db == nil {
		return 0
	}

	if rec.Status == "" {
		rec.Status = RemediationGenerated
	}
	now := time.Now()
	changedFilesJSON, err := json.Marshal(remediationChangedFiles(rec.Options.Files))
	if err != nil {
		slog.Error("Failed to marshal remediation changed files", "error", err)
		changedFilesJSON = []byte("[]")
	}

	query := `
		INSERT INTO remediation_outcomes (
			investigation_id, namespace, pod_name, diagnosis_category, patch_strategy,
			status, vcs_type, repo_owner, repo_name, base_branch, head_branch,
			pr_title, gitops_controller, gitops_app, gitops_namespace, gitops_repo_url, gitops_revision,
			gitops_path, manifest_type, overlay_role, environment, region,
			changed_files, failure_reason, workload_kind, workload_name, workload_selector,
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9, $10, $11,
			$12, $13, $14, $15, $16,
			$17, $18, $19, $20, $21,
			$22, $23, $24, $25, $26,
			$27, $28, $29
		)
		RETURNING id
	`
	var id int64
	err = h.db.QueryRowContext(ctx, query,
		nullableInt64(rec.InvestigationID), rec.Namespace, rec.PodName, rec.DiagnosisCategory, rec.PatchStrategy,
		string(rec.Status), rec.VCSType, rec.Options.RepoOwner, rec.Options.RepoName, rec.Options.Base, rec.Options.Head,
		rec.Options.Title, string(rec.Source.Controller), rec.Source.AppName, rec.Source.AppNamespace, rec.Source.RepoURL, rec.Source.TargetRevision,
		rec.Source.Path, string(rec.Source.ManifestType), string(rec.Source.OverlayRole), rec.Source.Environment, rec.Source.Region,
		changedFilesJSON, rec.FailureReason, rec.WorkloadKind, rec.WorkloadName, rec.WorkloadSelector,
		now, now,
	).Scan(&id)
	if err != nil {
		slog.Error("Failed to save remediation outcome", "error", err)
		return 0
	}
	return id
}

func (c *Controller) saveRemediationPlan(ctx context.Context, pod *v1.Pod, diagnosis Diagnosis, investigationID int64, vcsType string, plan remediationPROption, status RemediationStatus, failureReason string) int64 {
	if c == nil || c.history == nil {
		return 0
	}
	identity := c.workloadIdentityForPod(ctx, pod)
	return c.history.SaveRemediation(ctx, RemediationRecord{
		InvestigationID:   investigationID,
		Namespace:         pod.Namespace,
		PodName:           pod.Name,
		DiagnosisCategory: string(diagnosis.Category),
		PatchStrategy:     string(diagnosis.PatchStrategy),
		Status:            status,
		VCSType:           vcsType,
		Options:           plan.Options,
		Source:            plan.Source,
		FailureReason:     failureReason,
		WorkloadKind:      identity.Kind,
		WorkloadName:      identity.Name,
		WorkloadSelector:  identity.Selector,
	})
}

func (c *Controller) markRemediationStatus(ctx context.Context, id int64, status RemediationStatus, prURL, failureReason string) {
	if c == nil || c.history == nil {
		return
	}
	c.history.MarkRemediationStatus(ctx, id, status, prURL, failureReason)
}

func (h *historyCache) MarkRemediationStatus(ctx context.Context, id int64, status RemediationStatus, prURL, failureReason string) {
	if h == nil || h.db == nil || id <= 0 {
		return
	}
	query := `
		UPDATE remediation_outcomes
		SET status = $1,
		    pr_url = COALESCE(NULLIF($2, ''), pr_url),
		    failure_reason = COALESCE(NULLIF($3, ''), failure_reason),
		    updated_at = $4
		WHERE id = $5
	`
	if _, err := h.db.ExecContext(ctx, query, string(status), prURL, failureReason, time.Now(), id); err != nil {
		slog.Error("Failed to update remediation outcome", "id", id, "status", status, "error", err)
	}
}

func (h *historyCache) RemediationFeedback(ctx context.Context, namespace, podName string, diagnosis Diagnosis) string {
	if h == nil || h.db == nil {
		return ""
	}
	query := `
		SELECT patch_strategy, status, repo_owner, repo_name, head_branch,
		       COALESCE(pr_url, ''), COALESCE(failure_reason, ''), COALESCE(changed_files::text, '[]'), updated_at
		FROM remediation_outcomes
		WHERE (
			(namespace = $1 AND pod_name = $2)
			OR (diagnosis_category = $3 AND patch_strategy = $4)
		)
		AND status IN ('pr_failed', 'production_failed', 'revert_opened', 'revert_failed', 'reverted')
		ORDER BY updated_at DESC
		LIMIT 5
	`
	rows, err := h.db.QueryContext(ctx, query, namespace, podName, string(diagnosis.Category), string(diagnosis.PatchStrategy))
	if err != nil {
		slog.Error("Failed to query remediation feedback", "error", err)
		return ""
	}
	defer rows.Close()

	var feedback []remediationFeedbackRow
	for rows.Next() {
		var row remediationFeedbackRow
		if err := rows.Scan(&row.PatchStrategy, &row.Status, &row.RepoOwner, &row.RepoName, &row.HeadBranch, &row.PRURL, &row.FailureReason, &row.ChangedFilesJSON, &row.UpdatedAt); err != nil {
			slog.Error("Failed to scan remediation feedback", "error", err)
			continue
		}
		feedback = append(feedback, row)
	}
	return formatRemediationFeedback(feedback)
}

func (h *historyCache) RemediationsForMonitoring(ctx context.Context, limit int) []RemediationRecord {
	if h == nil || h.db == nil {
		return nil
	}
	if limit <= 0 {
		limit = 50
	}

	query := `
		SELECT id, COALESCE(investigation_id, 0), namespace, pod_name,
		       COALESCE(diagnosis_category, ''), COALESCE(patch_strategy, ''),
		       status, COALESCE(vcs_type, ''), repo_owner, repo_name,
		       base_branch, head_branch, COALESCE(pr_url, ''), COALESCE(pr_title, ''),
		       COALESCE(gitops_controller, ''), COALESCE(gitops_app, ''), COALESCE(gitops_namespace, ''),
		       COALESCE(gitops_repo_url, ''), COALESCE(gitops_revision, ''),
		       COALESCE(gitops_path, ''), COALESCE(manifest_type, ''),
		       COALESCE(overlay_role, ''), COALESCE(environment, ''), COALESCE(region, ''),
		       COALESCE(workload_kind, ''), COALESCE(workload_name, ''), COALESCE(workload_selector, ''),
		       updated_at
		FROM remediation_outcomes
		WHERE status IN ('pr_opened', 'observing')
		ORDER BY updated_at ASC
		LIMIT $1
	`
	rows, err := h.db.QueryContext(ctx, query, limit)
	if err != nil {
		slog.Error("Failed to query remediations for monitoring", "error", err)
		return nil
	}
	defer rows.Close()

	var records []RemediationRecord
	for rows.Next() {
		var rec RemediationRecord
		var status string
		err := rows.Scan(
			&rec.ID, &rec.InvestigationID, &rec.Namespace, &rec.PodName,
			&rec.DiagnosisCategory, &rec.PatchStrategy, &status, &rec.VCSType,
			&rec.Options.RepoOwner, &rec.Options.RepoName, &rec.Options.Base, &rec.Options.Head,
			&rec.PRURL, &rec.Options.Title, &rec.Source.Controller, &rec.Source.AppName, &rec.Source.AppNamespace,
			&rec.Source.RepoURL, &rec.Source.TargetRevision, &rec.Source.Path, &rec.Source.ManifestType,
			&rec.Source.OverlayRole, &rec.Source.Environment, &rec.Source.Region,
			&rec.WorkloadKind, &rec.WorkloadName, &rec.WorkloadSelector, &rec.UpdatedAt,
		)
		if err != nil {
			slog.Error("Failed to scan remediation monitor record", "error", err)
			continue
		}
		rec.Status = RemediationStatus(status)
		records = append(records, rec)
	}
	return records
}

func (h *historyCache) RemediationsNeedingRevert(ctx context.Context, limit int) []RemediationRecord {
	if h == nil || h.db == nil {
		return nil
	}
	if limit <= 0 {
		limit = 20
	}

	query := `
		SELECT id, COALESCE(investigation_id, 0), namespace, pod_name,
		       COALESCE(diagnosis_category, ''), COALESCE(patch_strategy, ''),
		       status, COALESCE(vcs_type, ''), repo_owner, repo_name,
		       base_branch, head_branch, COALESCE(pr_url, ''), COALESCE(pr_title, ''),
		       COALESCE(gitops_controller, ''), COALESCE(gitops_app, ''), COALESCE(gitops_namespace, ''),
		       COALESCE(gitops_repo_url, ''), COALESCE(gitops_revision, ''),
		       COALESCE(gitops_path, ''), COALESCE(manifest_type, ''),
		       COALESCE(overlay_role, ''), COALESCE(environment, ''), COALESCE(region, ''),
		       COALESCE(workload_kind, ''), COALESCE(workload_name, ''), COALESCE(workload_selector, ''),
		       COALESCE(changed_files::text, '[]'), COALESCE(failure_reason, ''), updated_at
		FROM remediation_outcomes
		WHERE status = 'production_failed'
		ORDER BY updated_at ASC
		LIMIT $1
	`
	rows, err := h.db.QueryContext(ctx, query, limit)
	if err != nil {
		slog.Error("Failed to query remediations needing revert", "error", err)
		return nil
	}
	defer rows.Close()

	var records []RemediationRecord
	for rows.Next() {
		var rec RemediationRecord
		var status string
		var changedFilesJSON string
		err := rows.Scan(
			&rec.ID, &rec.InvestigationID, &rec.Namespace, &rec.PodName,
			&rec.DiagnosisCategory, &rec.PatchStrategy, &status, &rec.VCSType,
			&rec.Options.RepoOwner, &rec.Options.RepoName, &rec.Options.Base, &rec.Options.Head,
			&rec.PRURL, &rec.Options.Title, &rec.Source.Controller, &rec.Source.AppName, &rec.Source.AppNamespace,
			&rec.Source.RepoURL, &rec.Source.TargetRevision, &rec.Source.Path, &rec.Source.ManifestType,
			&rec.Source.OverlayRole, &rec.Source.Environment, &rec.Source.Region,
			&rec.WorkloadKind, &rec.WorkloadName, &rec.WorkloadSelector,
			&changedFilesJSON, &rec.FailureReason, &rec.UpdatedAt,
		)
		if err != nil {
			slog.Error("Failed to scan remediation revert record", "error", err)
			continue
		}
		rec.Status = RemediationStatus(status)
		rec.ChangedFiles = parseRemediationChangedFiles(changedFilesJSON)
		records = append(records, rec)
	}
	return records
}

func (h *historyCache) MarkRemediationRevertOpened(ctx context.Context, id int64, revertURL, revertHead string) {
	if h == nil || h.db == nil || id <= 0 {
		return
	}
	query := `
		UPDATE remediation_outcomes
		SET status = $1,
		    revert_pr_url = $2,
		    revert_head_branch = $3,
		    updated_at = $4
		WHERE id = $5
	`
	if _, err := h.db.ExecContext(ctx, query, string(RemediationRevertOpened), revertURL, revertHead, time.Now(), id); err != nil {
		slog.Error("Failed to mark remediation revert opened", "id", id, "error", err)
	}
}

type remediationFeedbackRow struct {
	PatchStrategy    string
	Status           string
	RepoOwner        string
	RepoName         string
	HeadBranch       string
	PRURL            string
	FailureReason    string
	ChangedFilesJSON string
	UpdatedAt        time.Time
}

func formatRemediationFeedback(rows []remediationFeedbackRow) string {
	if len(rows) == 0 {
		return ""
	}
	lines := []string{"Previous Fixora remediation attempts failed. Do not repeat the same approach without a clear reason:"}
	for _, row := range rows {
		files := strings.Join(changedFilePathsFromJSON(row.ChangedFilesJSON), ", ")
		if files == "" {
			files = "unknown files"
		}
		reason := firstNonEmpty(row.FailureReason, "no failure reason recorded")
		ref := row.PRURL
		if ref == "" {
			ref = row.HeadBranch
		}
		lines = append(lines, fmt.Sprintf("- %s on %s/%s (%s, files: %s) failed: %s", row.PatchStrategy, row.RepoOwner, row.RepoName, ref, files, reason))
	}
	return strings.Join(lines, "\n")
}

func remediationChangedFiles(files []vcs.FileChange) []remediationChangedFile {
	changed := make([]remediationChangedFile, 0, len(files))
	for _, file := range files {
		changed = append(changed, remediationChangedFile{
			FilePath:        file.FilePath,
			PreviousContent: file.PreviousContent,
			HasPrevious:     !file.Create,
			Create:          file.Create,
		})
	}
	sort.Slice(changed, func(i, j int) bool {
		return changed[i].FilePath < changed[j].FilePath
	})
	return changed
}

func changedFilePathsFromJSON(raw string) []string {
	files := parseRemediationChangedFiles(raw)
	paths := make([]string, 0, len(files))
	for _, file := range files {
		if file.FilePath != "" {
			paths = append(paths, file.FilePath)
		}
	}
	sort.Strings(paths)
	return paths
}

func parseRemediationChangedFiles(raw string) []remediationChangedFile {
	if raw == "" {
		return nil
	}
	var files []remediationChangedFile
	if err := json.Unmarshal([]byte(raw), &files); err != nil {
		return nil
	}
	return files
}

func nullableInt64(v int64) interface{} {
	if v <= 0 {
		return nil
	}
	return v
}
