package validation

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type ValidationResult struct {
	Valid   bool
	Output  string
	Error   error
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
	fullPath := filepath.Join(s.Dir, path)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return err
	}
	return os.WriteFile(fullPath, content, 0644)
}
