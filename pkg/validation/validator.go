package validation

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
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

type SemanticTarget struct {
	Kind          string
	Name          string
	Namespace     string
	ContainerName string
	PatchStrategy string
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
	return renderSandboxFiles(source, files, changedSandboxPaths(changes), opts)
}

func ValidateSemanticRender(source gitops.WorkloadSource, sourceFiles map[string][]byte, changes []vcs.FileChange, target SemanticTarget, opts SandboxOptions) ValidationResult {
	if !opts.Enabled {
		return ValidationResult{Valid: true, Skipped: true, Output: "semantic render validation disabled"}
	}
	if strings.TrimSpace(target.PatchStrategy) == "" || target.PatchStrategy == "none" {
		return ValidationResult{Valid: true, Skipped: true, Output: "semantic render validation skipped: no patch strategy"}
	}
	if strings.TrimSpace(target.Kind) == "" || strings.TrimSpace(target.Name) == "" {
		return ValidationResult{Valid: false, Output: "semantic render validation failed: missing target workload identity", Error: errors.New("missing semantic target identity")}
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 15 * time.Second
	}

	before, after, result := semanticRenderDocuments(source, sourceFiles, changes, opts)
	if !result.Valid || result.Skipped {
		return result
	}
	changed, detail, err := semanticTargetChanged(before, after, target)
	if err != nil {
		return ValidationResult{Valid: false, Output: "semantic render validation failed: " + err.Error(), Error: err}
	}
	if !changed {
		err := fmt.Errorf("rendered %s/%s did not change expected %s fields", target.Kind, target.Name, target.PatchStrategy)
		if detail != "" {
			err = fmt.Errorf("%w: %s", err, detail)
		}
		return ValidationResult{Valid: false, Output: err.Error(), Error: err}
	}
	return ValidationResult{Valid: true, Output: "semantic render validation passed: " + detail}
}

func renderSandboxFiles(source gitops.WorkloadSource, files map[string][]byte, changed map[string]bool, opts SandboxOptions) ValidationResult {
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
		return renderHelm(sandbox.Dir, paths, source, changed, opts)
	default:
		return ValidationResult{Valid: true, Skipped: true, Output: "render validation not required for raw manifests"}
	}
}

func semanticRenderDocuments(source gitops.WorkloadSource, sourceFiles map[string][]byte, changes []vcs.FileChange, opts SandboxOptions) ([]*yaml.Node, []*yaml.Node, ValidationResult) {
	switch source.ManifestType {
	case gitops.ManifestKustomize, gitops.ManifestHelm, gitops.ManifestFluxHelmRelease:
		before := renderSandboxFiles(source, sourceFiles, nil, opts)
		if !before.Valid {
			return nil, nil, ValidationResult{Valid: false, Output: "semantic render validation failed before patch: " + validationOutput(before), Error: before.Error}
		}
		if before.Skipped {
			return nil, nil, ValidationResult{Valid: true, Skipped: true, Output: "semantic render validation skipped: " + before.Output}
		}
		after := renderSandboxFiles(source, mergeSandboxFiles(sourceFiles, changes), changedSandboxPaths(changes), opts)
		if !after.Valid {
			return nil, nil, ValidationResult{Valid: false, Output: "semantic render validation failed after patch: " + validationOutput(after), Error: after.Error}
		}
		if after.Skipped {
			return nil, nil, ValidationResult{Valid: true, Skipped: true, Output: "semantic render validation skipped: " + after.Output}
		}
		beforeDocs, err := parseYAMLDocuments([]byte(before.Output))
		if err != nil {
			return nil, nil, ValidationResult{Valid: false, Output: "semantic render validation failed: cannot parse original render: " + err.Error(), Error: err}
		}
		afterDocs, err := parseYAMLDocuments([]byte(after.Output))
		if err != nil {
			return nil, nil, ValidationResult{Valid: false, Output: "semantic render validation failed: cannot parse patched render: " + err.Error(), Error: err}
		}
		return beforeDocs, afterDocs, ValidationResult{Valid: true}
	default:
		beforeDocs, err := parseYAMLDocuments(joinYAMLFiles(sourceFiles))
		if err != nil {
			return nil, nil, ValidationResult{Valid: false, Output: "semantic render validation failed: cannot parse original manifests: " + err.Error(), Error: err}
		}
		afterDocs, err := parseYAMLDocuments(joinYAMLFiles(mergeSandboxFiles(sourceFiles, changes)))
		if err != nil {
			return nil, nil, ValidationResult{Valid: false, Output: "semantic render validation failed: cannot parse patched manifests: " + err.Error(), Error: err}
		}
		return beforeDocs, afterDocs, ValidationResult{Valid: true}
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

func validationOutput(result ValidationResult) string {
	if strings.TrimSpace(result.Output) != "" {
		return strings.TrimSpace(result.Output)
	}
	if result.Error != nil {
		return result.Error.Error()
	}
	return "unknown validation error"
}

func joinYAMLFiles(files map[string][]byte) []byte {
	paths := make([]string, 0, len(files))
	for path := range files {
		if isYAMLPath(path) {
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)
	var out bytes.Buffer
	for _, path := range paths {
		out.WriteString("\n---\n")
		out.Write(files[path])
		if len(files[path]) > 0 && files[path][len(files[path])-1] != '\n' {
			out.WriteByte('\n')
		}
	}
	return out.Bytes()
}

func isYAMLPath(path string) bool {
	lower := strings.ToLower(filepath.Base(path))
	return strings.HasSuffix(lower, ".yaml") || strings.HasSuffix(lower, ".yml")
}

func parseYAMLDocuments(content []byte) ([]*yaml.Node, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(content))
	var docs []*yaml.Node
	for {
		var doc yaml.Node
		err := decoder.Decode(&doc)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		if doc.Kind == 0 || len(doc.Content) == 0 {
			continue
		}
		docs = append(docs, &doc)
	}
	return docs, nil
}

func semanticTargetChanged(beforeDocs, afterDocs []*yaml.Node, target SemanticTarget) (bool, string, error) {
	strategy := strings.TrimSpace(target.PatchStrategy)
	if strategy == "service-selector" {
		if changed, detail := anyKindFieldsChanged(beforeDocs, afterDocs, "Service", []string{"spec.selector"}, "service selector"); changed {
			return true, detail, nil
		}
	}
	if strategy == "pvc-or-volume" {
		if changed, detail := anyKindFieldsChanged(beforeDocs, afterDocs, "PersistentVolumeClaim", []string{"spec"}, "persistent volume claim"); changed {
			return true, detail, nil
		}
	}

	before := findManifestByIdentity(beforeDocs, target.Kind, target.Name, target.Namespace)
	if before == nil {
		return false, "", fmt.Errorf("target %s/%s was not present before patch", target.Kind, target.Name)
	}
	after := findManifestByIdentity(afterDocs, target.Kind, target.Name, target.Namespace)
	if after == nil {
		return false, "", fmt.Errorf("target %s/%s was not present after patch", target.Kind, target.Name)
	}

	switch strategy {
	case "image":
		return containerFieldsChanged(before, after, target.ContainerName, []string{"image", "imagePullPolicy"}, "container image")
	case "resources":
		return containerFieldsChanged(before, after, target.ContainerName, []string{"resources"}, "container resources")
	case "env-or-volume-ref":
		changed, detail := containerFieldsChangedNoError(before, after, target.ContainerName, []string{"env", "envFrom", "volumeMounts", "command", "args"}, "container env/volume references")
		if changed {
			return true, detail, nil
		}
		return podSpecFieldsChanged(before, after, []string{"volumes"}, "pod volumes")
	case "security-context":
		changed, detail := containerFieldsChangedNoError(before, after, target.ContainerName, []string{"securityContext", "volumeMounts"}, "container security hardening")
		if changed {
			return true, detail, nil
		}
		return podSpecFieldsChanged(before, after, []string{"securityContext", "volumes"}, "pod security hardening")
	case "probe":
		return containerFieldsChanged(before, after, target.ContainerName, []string{"livenessProbe", "readinessProbe", "startupProbe"}, "container probes")
	case "scheduling-policy":
		return podSpecFieldsChanged(before, after, []string{"nodeSelector", "affinity", "tolerations", "topologySpreadConstraints", "schedulerName", "runtimeClassName", "priorityClassName"}, "pod scheduling policy")
	case "pvc-or-volume":
		changed, detail := podSpecFieldsChangedNoError(before, after, []string{"volumes"}, "pod volumes")
		if changed {
			return true, detail, nil
		}
		return manifestFieldsChanged(before, after, []string{"spec.volumeClaimTemplates", "spec.storageClassName", "spec.resources"}, "PVC or volume fields")
	case "service-selector":
		changed, detail := manifestFieldsChangedNoError(before, after, []string{"spec.selector", "spec.template.metadata.labels", "metadata.labels"}, "service selector or workload labels")
		if changed {
			return true, detail, nil
		}
		return false, "service selector strategy did not alter selector or labels on target manifest", nil
	default:
		return manifestChanged(before, after), "target manifest changed", nil
	}
}

func anyKindFieldsChanged(beforeDocs, afterDocs []*yaml.Node, kind string, fields []string, label string) (bool, string) {
	beforeByName := map[string]*yaml.Node{}
	for _, doc := range beforeDocs {
		mapping := documentMapping(doc)
		if mapping == nil || scalarValue(mapping, "kind") != kind {
			continue
		}
		meta := mappingValue(mapping, "metadata")
		name := scalarValue(meta, "name")
		namespace := scalarValue(meta, "namespace")
		if name != "" {
			beforeByName[namespace+"/"+name] = mapping
		}
	}
	for _, doc := range afterDocs {
		after := documentMapping(doc)
		if after == nil || scalarValue(after, "kind") != kind {
			continue
		}
		meta := mappingValue(after, "metadata")
		key := scalarValue(meta, "namespace") + "/" + scalarValue(meta, "name")
		before := beforeByName[key]
		if before == nil {
			continue
		}
		if changed, detail := manifestFieldsChangedNoError(before, after, fields, label); changed {
			return true, detail
		}
	}
	return false, label + " unchanged"
}

func findManifestByIdentity(docs []*yaml.Node, kind, name, namespace string) *yaml.Node {
	kind = strings.TrimSpace(kind)
	name = strings.TrimSpace(name)
	namespace = strings.TrimSpace(namespace)
	for _, doc := range docs {
		mapping := documentMapping(doc)
		if mapping == nil {
			continue
		}
		if scalarValue(mapping, "kind") != kind {
			continue
		}
		meta := mappingValue(mapping, "metadata")
		if meta == nil || scalarValue(meta, "name") != name {
			continue
		}
		docNamespace := scalarValue(meta, "namespace")
		if namespace != "" && docNamespace != "" && docNamespace != namespace {
			continue
		}
		return mapping
	}
	return nil
}

func containerFieldsChanged(before, after *yaml.Node, containerName string, fields []string, label string) (bool, string, error) {
	changed, detail := containerFieldsChangedNoError(before, after, containerName, fields, label)
	if changed {
		return true, detail, nil
	}
	return false, label + " unchanged", nil
}

func containerFieldsChangedNoError(before, after *yaml.Node, containerName string, fields []string, label string) (bool, string) {
	beforeContainer := workloadContainerMapping(before, scalarValue(before, "kind"), containerName)
	afterContainer := workloadContainerMapping(after, scalarValue(after, "kind"), containerName)
	if beforeContainer == nil || afterContainer == nil {
		return false, fmt.Sprintf("%s unchanged: target container %q not found", label, containerName)
	}
	for _, field := range fields {
		if !yamlNodesEqual(mappingValue(beforeContainer, field), mappingValue(afterContainer, field)) {
			return true, fmt.Sprintf("%s changed field %s on container %s", label, field, firstNonEmpty(containerName, scalarValue(afterContainer, "name")))
		}
	}
	return false, label + " unchanged"
}

func podSpecFieldsChanged(before, after *yaml.Node, fields []string, label string) (bool, string, error) {
	changed, detail := podSpecFieldsChangedNoError(before, after, fields, label)
	if changed {
		return true, detail, nil
	}
	return false, detail, nil
}

func podSpecFieldsChangedNoError(before, after *yaml.Node, fields []string, label string) (bool, string) {
	beforeSpec := workloadPodSpec(before, scalarValue(before, "kind"))
	afterSpec := workloadPodSpec(after, scalarValue(after, "kind"))
	if beforeSpec == nil || afterSpec == nil {
		return false, label + " unchanged: pod spec not found"
	}
	for _, field := range fields {
		if !yamlNodesEqual(mappingValue(beforeSpec, field), mappingValue(afterSpec, field)) {
			return true, fmt.Sprintf("%s changed field %s", label, field)
		}
	}
	return false, label + " unchanged"
}

func manifestFieldsChanged(before, after *yaml.Node, fields []string, label string) (bool, string, error) {
	changed, detail := manifestFieldsChangedNoError(before, after, fields, label)
	if changed {
		return true, detail, nil
	}
	return false, detail, nil
}

func manifestFieldsChangedNoError(before, after *yaml.Node, fields []string, label string) (bool, string) {
	for _, field := range fields {
		if !yamlNodesEqual(nodeAtPath(before, field), nodeAtPath(after, field)) {
			return true, fmt.Sprintf("%s changed field %s", label, field)
		}
	}
	return false, label + " unchanged"
}

func manifestChanged(before, after *yaml.Node) bool {
	return !yamlNodesEqual(before, after)
}

func workloadContainerMapping(doc *yaml.Node, kind, containerName string) *yaml.Node {
	for _, containers := range workloadContainerNodes(doc, kind) {
		if containers == nil || containers.Kind != yaml.SequenceNode {
			continue
		}
		for _, container := range containers.Content {
			if container.Kind != yaml.MappingNode {
				continue
			}
			if strings.TrimSpace(containerName) == "" || scalarValue(container, "name") == containerName {
				return container
			}
		}
	}
	return nil
}

func workloadContainerNodes(doc *yaml.Node, kind string) []*yaml.Node {
	spec := workloadPodSpec(doc, kind)
	if spec == nil {
		return nil
	}
	return []*yaml.Node{
		mappingValue(spec, "initContainers"),
		mappingValue(spec, "containers"),
	}
}

func workloadPodSpec(doc *yaml.Node, kind string) *yaml.Node {
	spec := mappingValue(doc, "spec")
	if spec == nil {
		return nil
	}
	switch kind {
	case "Pod":
		return spec
	case "Deployment", "StatefulSet", "DaemonSet", "ReplicaSet", "Job":
		return nodeAtPath(spec, "template.spec")
	case "CronJob":
		return nodeAtPath(spec, "jobTemplate.spec.template.spec")
	default:
		return nodeAtPath(spec, "template.spec")
	}
}

func documentMapping(root *yaml.Node) *yaml.Node {
	if root == nil {
		return nil
	}
	if root.Kind == yaml.DocumentNode && len(root.Content) > 0 && root.Content[0].Kind == yaml.MappingNode {
		return root.Content[0]
	}
	if root.Kind == yaml.MappingNode {
		return root
	}
	return nil
}

func nodeAtPath(node *yaml.Node, dottedPath string) *yaml.Node {
	current := node
	for _, part := range strings.Split(dottedPath, ".") {
		if strings.TrimSpace(part) == "" {
			return nil
		}
		current = mappingValue(current, part)
		if current == nil {
			return nil
		}
	}
	return current
}

func mappingValue(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}

func scalarValue(node *yaml.Node, key string) string {
	value := mappingValue(node, key)
	if value == nil || value.Kind != yaml.ScalarNode {
		return ""
	}
	return value.Value
}

func yamlNodesEqual(a, b *yaml.Node) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	left, leftErr := yaml.Marshal(a)
	right, rightErr := yaml.Marshal(b)
	if leftErr != nil || rightErr != nil {
		return fmt.Sprintf("%#v", a) == fmt.Sprintf("%#v", b)
	}
	return bytes.Equal(left, right)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
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

func renderHelm(root string, paths []string, source gitops.WorkloadSource, changed map[string]bool, opts SandboxOptions) ValidationResult {
	chart := firstMatchingBase(paths, "Chart.yaml")
	if chart == "" {
		return renderSkippedOrRequired(opts, "helm render skipped: no Chart.yaml found")
	}
	tool, ok := lookPath("helm")
	if !ok {
		return renderSkippedOrRequired(opts, "helm render skipped: helm is not available")
	}
	args := buildHelmTemplateArgs(root, chart, paths, source, changed)
	return runRenderCommand(opts.Timeout, tool, args...)
}

func buildHelmTemplateArgs(root, chart string, paths []string, source gitops.WorkloadSource, changed map[string]bool) []string {
	chartDir := filepath.Join(root, filepath.Dir(chart))
	args := []string{"template"}
	if source.Helm.ReleaseName != "" {
		args = append(args, source.Helm.ReleaseName)
	}
	args = append(args, chartDir)
	if source.Helm.Namespace != "" {
		args = append(args, "--namespace", source.Helm.Namespace)
	}
	for _, values := range helmValueFileOrder(chart, paths, source, changed) {
		args = append(args, "-f", filepath.Join(root, values))
	}
	for key, value := range source.Helm.Parameters {
		if strings.TrimSpace(key) == "" {
			continue
		}
		args = append(args, "--set", key+"="+value)
	}
	for key, value := range source.Helm.FileParameters {
		if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
			continue
		}
		filePath, ok := helmExistingPath(chart, paths, value)
		if !ok {
			continue
		}
		args = append(args, "--set-file", key+"="+filepath.Join(root, filePath))
	}
	return args
}

func helmValueFileOrder(chart string, paths []string, source gitops.WorkloadSource, changed map[string]bool) []string {
	pathSet := map[string]bool{}
	for _, item := range paths {
		pathSet[filepath.ToSlash(filepath.Clean(item))] = true
	}

	var out []string
	add := func(item string) {
		item = filepath.ToSlash(filepath.Clean(strings.TrimSpace(item)))
		if item == "." || item == "" || filepath.IsAbs(item) || strings.HasPrefix(item, "../") {
			return
		}
		if pathSet[item] && !containsString(out, item) {
			out = append(out, item)
		}
	}

	chartDir := filepath.ToSlash(filepath.Dir(chart))
	for _, item := range source.Helm.ValueFiles {
		if item == "" || strings.HasPrefix(strings.TrimSpace(item), "$") {
			continue
		}
		add(item)
		add(filepath.ToSlash(filepath.Join(chartDir, item)))
	}
	if len(out) > 0 {
		return out
	}

	for _, item := range []string{
		filepath.ToSlash(filepath.Join(chartDir, "values.yaml")),
		filepath.ToSlash(filepath.Join(chartDir, "values.yml")),
	} {
		add(item)
	}
	for item := range changed {
		if isValuesYAML(item) {
			add(item)
		}
	}
	return out
}

func helmExistingPath(chart string, paths []string, value string) (string, bool) {
	pathSet := map[string]bool{}
	for _, item := range paths {
		pathSet[filepath.ToSlash(filepath.Clean(item))] = true
	}
	cleanValue := filepath.ToSlash(filepath.Clean(strings.TrimSpace(value)))
	if cleanValue == "." || cleanValue == "" || filepath.IsAbs(cleanValue) || strings.HasPrefix(cleanValue, "../") {
		return "", false
	}
	candidates := []string{cleanValue}
	if chart != "" {
		candidates = append(candidates, filepath.ToSlash(filepath.Join(filepath.Dir(chart), cleanValue)))
	}
	for _, candidate := range candidates {
		if pathSet[candidate] {
			return candidate, true
		}
	}
	return "", false
}

func changedSandboxPaths(changes []vcs.FileChange) map[string]bool {
	out := map[string]bool{}
	for _, change := range changes {
		out[filepath.ToSlash(filepath.Clean(change.FilePath))] = true
	}
	return out
}

func isValuesYAML(item string) bool {
	base := strings.ToLower(filepath.Base(item))
	return (strings.HasSuffix(base, ".yaml") || strings.HasSuffix(base, ".yml")) &&
		(strings.HasPrefix(base, "values") || strings.Contains(base, "values"))
}

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
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
