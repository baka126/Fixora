package validation

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"fixora/pkg/gitops"
	"fixora/pkg/vcs"
	"gopkg.in/yaml.v3"
)

type ValidationResult struct {
	Valid   bool
	Output  string
	Error   error
	Skipped bool
}

type SandboxOptions struct {
	Enabled       bool
	RequireRender bool
	Timeout       time.Duration
}

// ValidateYAML checks if the content is a valid YAML structure.
func ValidateYAML(content []byte) ValidationResult {
	var target interface{}
	if err := yaml.Unmarshal(content, &target); err != nil {
		return ValidationResult{Valid: false, Output: err.Error(), Error: err}
	}
	return ValidationResult{Valid: true}
}

// ValidateManifest runs a client-side dry-run apply to check for syntax errors in raw K8s manifests.
func ValidateManifest(content []byte) ValidationResult {
	tmpFile, err := os.CreateTemp("", "fixora-manifest-*.yaml")
	if err != nil {
		return ValidationResult{Valid: false, Error: fmt.Errorf("failed to create temp file: %w", err)}
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write(content); err != nil {
		return ValidationResult{Valid: false, Error: fmt.Errorf("failed to write to temp file: %w", err)}
	}
	tmpFile.Close()

	cmd := exec.Command("kubectl", "apply", "--dry-run=client", "-f", tmpFile.Name())
	out, err := cmd.CombinedOutput()
	if err != nil {
		return ValidationResult{Valid: false, Output: string(out), Error: err}
	}

	return ValidationResult{Valid: true, Output: string(out)}
}

// ValidateHelmValues attempts to run helm template if the full chart is available locally.
// For now, it provides a placeholder that we can expand when local cloning is implemented.
func ValidateHelmValues(chartPath string, valuesPath string) ValidationResult {
	cmd := exec.Command("helm", "template", chartPath, "-f", valuesPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return ValidationResult{Valid: false, Output: string(out), Error: err}
	}
	return ValidationResult{Valid: true, Output: string(out)}
}

// Sandbox represents a temporary directory where validation happens.
type Sandbox struct {
	Dir string
}

func NewSandbox() (*Sandbox, error) {
	dir, err := os.MkdirTemp("", "fixora-sandbox-*")
	if err != nil {
		return nil, err
	}
	return &Sandbox{Dir: dir}, nil
}

func (s *Sandbox) Cleanup() {
	os.RemoveAll(s.Dir)
}

func (s *Sandbox) WriteFile(path string, content []byte) error {
	cleanPath, err := cleanRelativePath(path)
	if err != nil {
		return err
	}
	fullPath := filepath.Join(s.Dir, cleanPath)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return err
	}
	return os.WriteFile(fullPath, content, 0644)
}

func ValidateRenderSandbox(source gitops.WorkloadSource, sourceFiles map[string][]byte, changes []vcs.FileChange, opts SandboxOptions) ValidationResult {
	if !opts.Enabled {
		return ValidationResult{Valid: true, Skipped: true, Output: "render sandbox disabled"}
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 15 * time.Second
	}

	files := mergeSandboxFiles(sourceFiles, changes)
	sandbox, err := NewSandbox()
	if err != nil {
		return ValidationResult{Valid: false, Error: err}
	}
	defer sandbox.Cleanup()

	paths := make([]string, 0, len(files))
	for path, content := range files {
		if err := sandbox.WriteFile(path, content); err != nil {
			return ValidationResult{Valid: false, Error: err}
		}
		paths = append(paths, path)
	}
	sort.Strings(paths)

	switch source.ManifestType {
	case gitops.ManifestKustomize:
		return renderKustomize(sandbox.Dir, paths, opts)
	case gitops.ManifestHelm, gitops.ManifestFluxHelmRelease:
		return renderHelm(sandbox.Dir, paths, opts)
	default:
		return ValidationResult{Valid: true, Skipped: true, Output: "render validation not required for raw manifests"}
	}
}

func mergeSandboxFiles(sourceFiles map[string][]byte, changes []vcs.FileChange) map[string][]byte {
	files := make(map[string][]byte, len(sourceFiles)+len(changes))
	for path, content := range sourceFiles {
		files[path] = append([]byte(nil), content...)
	}
	for _, change := range changes {
		if change.Delete {
			delete(files, change.FilePath)
			continue
		}
		files[change.FilePath] = append([]byte(nil), change.NewContent...)
	}
	return files
}

func renderKustomize(root string, paths []string, opts SandboxOptions) ValidationResult {
	kustomization := firstMatchingBase(paths, "kustomization.yaml", "kustomization.yml")
	if kustomization == "" {
		return renderSkippedOrRequired(opts, "kustomize render skipped: no kustomization.yaml found")
	}
	dir := filepath.Join(root, filepath.Dir(kustomization))
	if tool, ok := lookPath("kustomize"); ok {
		return runRenderCommand(opts.Timeout, tool, "build", dir)
	}
	if tool, ok := lookPath("kubectl"); ok {
		return runRenderCommand(opts.Timeout, tool, "kustomize", dir)
	}
	return renderSkippedOrRequired(opts, "kustomize render skipped: neither kustomize nor kubectl is available")
}

func renderHelm(root string, paths []string, opts SandboxOptions) ValidationResult {
	chart := firstMatchingBase(paths, "Chart.yaml")
	if chart == "" {
		return renderSkippedOrRequired(opts, "helm render skipped: no Chart.yaml found")
	}
	tool, ok := lookPath("helm")
	if !ok {
		return renderSkippedOrRequired(opts, "helm render skipped: helm is not available")
	}
	chartDir := filepath.Join(root, filepath.Dir(chart))
	args := []string{"template", chartDir}
	if values := firstMatchingBase(paths, "values.yaml", "values.yml"); values != "" {
		args = append(args, "-f", filepath.Join(root, values))
	}
	return runRenderCommand(opts.Timeout, tool, args...)
}

func runRenderCommand(timeout time.Duration, tool string, args ...string) ValidationResult {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, tool, args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return ValidationResult{Valid: false, Output: out.String(), Error: fmt.Errorf("render command timed out after %s", timeout)}
	}
	if err != nil {
		return ValidationResult{Valid: false, Output: out.String(), Error: err}
	}
	return ValidationResult{Valid: true, Output: out.String()}
}

func renderSkippedOrRequired(opts SandboxOptions, msg string) ValidationResult {
	if opts.RequireRender {
		return ValidationResult{Valid: false, Output: msg, Error: errors.New(msg)}
	}
	return ValidationResult{Valid: true, Output: msg, Skipped: true}
}

func firstMatchingBase(paths []string, names ...string) string {
	wanted := map[string]bool{}
	for _, name := range names {
		wanted[strings.ToLower(name)] = true
	}
	for _, p := range paths {
		if wanted[strings.ToLower(filepath.Base(p))] {
			return p
		}
	}
	return ""
}

func lookPath(tool string) (string, bool) {
	path, err := exec.LookPath(tool)
	return path, err == nil
}

func cleanRelativePath(path string) (string, error) {
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "." || path == "" {
		return "", fmt.Errorf("empty sandbox path")
	}
	if filepath.IsAbs(path) || strings.HasPrefix(path, ".."+string(filepath.Separator)) || path == ".." {
		return "", fmt.Errorf("unsafe sandbox path: %s", path)
	}
	return path, nil
}
