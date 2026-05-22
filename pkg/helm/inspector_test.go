package helm

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestRuntimeInspectorInspectRelease(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test helper shell script is unix-specific")
	}
	dir := t.TempDir()
	helmPath := filepath.Join(dir, "helm")
	script := `#!/bin/sh
case "$1 $2" in
  "status checkout")
    printf '{"name":"checkout","version":7,"info":{"status":"deployed"},"chart":{"metadata":{"name":"api","version":"1.2.3","appVersion":"4.5.6"}}}'
    ;;
  "get values")
    printf 'image:\n  tag: 4.5.6\n'
    ;;
  "get manifest")
    printf 'kind: Deployment\nmetadata:\n  name: checkout\n'
    ;;
  "version --short")
    printf 'v3.15.0\n'
    ;;
  *)
    echo "unexpected $*" >&2
    exit 1
    ;;
esac
`
	if err := os.WriteFile(helmPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	inspector := RuntimeInspector{CommandPath: helmPath, Timeout: 5 * time.Second, MaxBytes: 1024}
	got, err := inspector.InspectRelease(context.Background(), "checkout", "prod")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "deployed" || got.Chart != "api-1.2.3" || got.AppVersion != "4.5.6" || got.Revision != 7 {
		t.Fatalf("unexpected release inspection: %#v", got)
	}
	if !strings.Contains(got.ValuesPreview, "tag: 4.5.6") || !strings.Contains(got.ManifestPreview, "kind: Deployment") {
		t.Fatalf("missing previews: %#v", got)
	}
}

func TestRuntimeInspectorRequiresReleaseName(t *testing.T) {
	_, err := NewRuntimeInspector().InspectRelease(context.Background(), "", "default")
	if err == nil {
		t.Fatal("expected missing release name error")
	}
}
