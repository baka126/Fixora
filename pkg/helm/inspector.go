package helm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const (
	DefaultTimeout  = 10 * time.Second
	DefaultMaxBytes = 64 * 1024
)

type RuntimeInspector struct {
	CommandPath string
	Timeout     time.Duration
	MaxBytes    int
}

type ReleaseInspection struct {
	ReleaseName     string
	Namespace       string
	Status          string
	Chart           string
	AppVersion      string
	Revision        int
	ValuesPreview   string
	ManifestPreview string
}

func NewRuntimeInspector() RuntimeInspector {
	return RuntimeInspector{
		CommandPath: "helm",
		Timeout:     DefaultTimeout,
		MaxBytes:    DefaultMaxBytes,
	}
}

func (i RuntimeInspector) InspectRelease(ctx context.Context, releaseName, namespace string) (ReleaseInspection, error) {
	releaseName = strings.TrimSpace(releaseName)
	namespace = strings.TrimSpace(namespace)
	if releaseName == "" {
		return ReleaseInspection{}, errors.New("helm release name is required")
	}
	if namespace == "" {
		namespace = "default"
	}
	out := ReleaseInspection{ReleaseName: releaseName, Namespace: namespace}
	status, err := i.status(ctx, releaseName, namespace)
	if err != nil {
		return out, err
	}
	out.Status = status.Info.Status
	out.Chart = status.Chart.Metadata.Name
	if status.Chart.Metadata.Version != "" {
		if out.Chart != "" {
			out.Chart += "-"
		}
		out.Chart += status.Chart.Metadata.Version
	}
	out.AppVersion = status.Chart.Metadata.AppVersion
	out.Revision = status.Version
	out.ValuesPreview = i.preview(ctx, "get", "values", releaseName, "-n", namespace, "-o", "yaml")
	out.ManifestPreview = i.preview(ctx, "get", "manifest", releaseName, "-n", namespace)
	return out, nil
}

func (i RuntimeInspector) Available(ctx context.Context) bool {
	_, err := i.run(ctx, "version", "--short")
	return err == nil
}

func (i RuntimeInspector) status(ctx context.Context, releaseName, namespace string) (helmStatus, error) {
	raw, err := i.run(ctx, "status", releaseName, "-n", namespace, "-o", "json")
	if err != nil {
		return helmStatus{}, err
	}
	var out helmStatus
	if err := json.Unmarshal(raw, &out); err != nil {
		return helmStatus{}, fmt.Errorf("parse helm status: %w", err)
	}
	return out, nil
}

func (i RuntimeInspector) preview(ctx context.Context, args ...string) string {
	raw, err := i.run(ctx, args...)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(raw))
}

func (i RuntimeInspector) run(ctx context.Context, args ...string) ([]byte, error) {
	timeout := i.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	maxBytes := i.MaxBytes
	if maxBytes <= 0 {
		maxBytes = DefaultMaxBytes
	}
	commandPath := strings.TrimSpace(i.CommandPath)
	if commandPath == "" {
		commandPath = "helm"
	}
	if _, err := exec.LookPath(commandPath); err != nil {
		return nil, err
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(runCtx, commandPath, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	stdout.Grow(minInt(maxBytes, 4096))
	cmd.Stdout = &limitedWriter{buf: &stdout, remaining: maxBytes}
	cmd.Stderr = &limitedWriter{buf: &stderr, remaining: minInt(maxBytes, 4096)}
	if err := cmd.Run(); err != nil {
		if runCtx.Err() != nil {
			return nil, runCtx.Err()
		}
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return nil, fmt.Errorf("helm %s failed: %w: %s", strings.Join(args, " "), err, msg)
		}
		return nil, fmt.Errorf("helm %s failed: %w", strings.Join(args, " "), err)
	}
	return stdout.Bytes(), nil
}

type helmStatus struct {
	Name    string `json:"name"`
	Version int    `json:"version"`
	Info    struct {
		Status string `json:"status"`
	} `json:"info"`
	Chart struct {
		Metadata struct {
			Name       string `json:"name"`
			Version    string `json:"version"`
			AppVersion string `json:"appVersion"`
		} `json:"metadata"`
	} `json:"chart"`
}

type limitedWriter struct {
	buf       *bytes.Buffer
	remaining int
}

func (w *limitedWriter) Write(p []byte) (int, error) {
	if w.remaining <= 0 {
		return len(p), nil
	}
	if len(p) > w.remaining {
		w.buf.Write(p[:w.remaining])
		w.remaining = 0
		return len(p), nil
	}
	w.buf.Write(p)
	w.remaining -= len(p)
	return len(p), nil
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
