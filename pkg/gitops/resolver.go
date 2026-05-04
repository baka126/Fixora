package gitops

import (
	"context"
	"fmt"
	"path"
	"sort"
	"strings"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

type ControllerType string

const (
	ControllerAnnotation ControllerType = "annotation"
	ControllerArgoCD     ControllerType = "argocd"
	ControllerFlux       ControllerType = "flux"
)

type ManifestType string

const (
	ManifestUnknown         ManifestType = "unknown"
	ManifestRaw             ManifestType = "raw"
	ManifestHelm            ManifestType = "helm"
	ManifestKustomize       ManifestType = "kustomize"
	ManifestFluxHelmRelease ManifestType = "flux-helmrelease"
)

type OverlayRole string

const (
	OverlayUnknown OverlayRole = "unknown"
	OverlayBase    OverlayRole = "base"
	OverlayEnv     OverlayRole = "overlay"
)

type WorkloadSource struct {
	Controller     ControllerType
	AppName        string
	AppNamespace   string
	RepoURL        string
	TargetRevision string
	Path           string
	ManifestType   ManifestType
	OverlayRole    OverlayRole
	Environment    string
	Region         string
	Reason         string
}

func (s WorkloadSource) Summary() string {
	parts := []string{
		fmt.Sprintf("controller=%s", s.Controller),
		fmt.Sprintf("repo=%s", s.RepoURL),
		fmt.Sprintf("revision=%s", firstNonEmpty(s.TargetRevision, "main")),
		fmt.Sprintf("path=%s", s.Path),
		fmt.Sprintf("manifestType=%s", s.ManifestType),
		fmt.Sprintf("overlayRole=%s", s.OverlayRole),
	}
	if s.Environment != "" {
		parts = append(parts, "environment="+s.Environment)
	}
	if s.Region != "" {
		parts = append(parts, "region="+s.Region)
	}
	if s.AppName != "" {
		parts = append(parts, "app="+s.AppName)
	}
	if s.AppNamespace != "" {
		parts = append(parts, "appNamespace="+s.AppNamespace)
	}
	if s.Reason != "" {
		parts = append(parts, "reason="+s.Reason)
	}
	return strings.Join(parts, ", ")
}

type Resolver struct {
	clientset        kubernetes.Interface
	dynamicClient    dynamic.Interface
	argoCDNamespace  string
	argoCDEnabled    bool
	annotationPrefix string
}

type ResolverConfig struct {
	ArgoCDEnabled   bool
	ArgoCDNamespace string
}

func NewResolver(clientset kubernetes.Interface, dynamicClient dynamic.Interface, cfg ResolverConfig) *Resolver {
	return &Resolver{
		clientset:        clientset,
		dynamicClient:    dynamicClient,
		argoCDEnabled:    cfg.ArgoCDEnabled,
		argoCDNamespace:  firstNonEmpty(cfg.ArgoCDNamespace, "argocd"),
		annotationPrefix: "fixora.io",
	}
}

func (r *Resolver) ResolvePod(ctx context.Context, pod *v1.Pod) ([]WorkloadSource, error) {
	var sources []WorkloadSource

	if r.dynamicClient != nil {
		if r.argoCDEnabled {
			sources = append(sources, r.resolveArgoCD(ctx, pod)...)
		}
		sources = append(sources, r.resolveFlux(ctx, pod)...)
	}

	if len(sources) == 0 {
		if src, ok := r.resolveAnnotations(pod); ok {
			sources = append(sources, src)
		}
	}

	sources = dedupeSources(sources)
	sort.Slice(sources, func(i, j int) bool {
		return sourceRank(sources[i]) < sourceRank(sources[j])
	})
	return sources, nil
}

func (r *Resolver) resolveArgoCD(ctx context.Context, pod *v1.Pod) []WorkloadSource {
	apps, err := r.dynamicClient.Resource(schema.GroupVersionResource{
		Group:    "argoproj.io",
		Version:  "v1alpha1",
		Resource: "applications",
	}).Namespace(r.argoCDNamespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil
	}

	var sources []WorkloadSource
	workloads := workloadRefsForPod(ctx, r.clientset, pod)
	for _, app := range apps.Items {
		if !argoAppManagesWorkload(app.Object, workloads) {
			continue
		}
		for _, source := range argoSources(app.Object) {
			source.Controller = ControllerArgoCD
			source.AppName = app.GetName()
			source.AppNamespace = app.GetNamespace()
			source.ManifestType = inferManifestType(source.ManifestType, source.Path, source.RepoURL)
			enrichOverlay(&source)
			source.Reason = "matched ArgoCD application status.resources"
			sources = append(sources, source)
		}
	}
	return sources
}

func (r *Resolver) resolveFlux(ctx context.Context, pod *v1.Pod) []WorkloadSource {
	var sources []WorkloadSource
	sources = append(sources, r.resolveFluxKustomizations(ctx, pod)...)
	sources = append(sources, r.resolveFluxHelmReleases(ctx, pod)...)
	return sources
}

func (r *Resolver) resolveFluxKustomizations(ctx context.Context, pod *v1.Pod) []WorkloadSource {
	var sources []WorkloadSource
	for _, gvr := range []schema.GroupVersionResource{
		{Group: "kustomize.toolkit.fluxcd.io", Version: "v1", Resource: "kustomizations"},
		{Group: "kustomize.toolkit.fluxcd.io", Version: "v1beta2", Resource: "kustomizations"},
	} {
		items, err := r.dynamicClient.Resource(gvr).Namespace(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
		if err != nil {
			continue
		}
		for _, item := range items.Items {
			if !fluxInventoryContains(item.Object, pod.Namespace, pod.Name, "Pod") && !fluxInventoryContainsAny(item.Object, workloadRefsForPod(ctx, r.clientset, pod)) {
				continue
			}
			src := r.sourceFromFluxKustomization(ctx, item)
			if src.RepoURL == "" {
				continue
			}
			src.Controller = ControllerFlux
			src.AppName = item.GetName()
			src.AppNamespace = item.GetNamespace()
			src.ManifestType = ManifestKustomize
			enrichOverlay(&src)
			src.Reason = "matched Flux Kustomization inventory"
			sources = append(sources, src)
		}
		break
	}
	return sources
}

func (r *Resolver) resolveFluxHelmReleases(ctx context.Context, pod *v1.Pod) []WorkloadSource {
	releaseName := firstNonEmpty(
		pod.Labels["helm.toolkit.fluxcd.io/name"],
		pod.Labels["app.kubernetes.io/instance"],
	)
	if releaseName == "" {
		return nil
	}

	var sources []WorkloadSource
	for _, gvr := range []schema.GroupVersionResource{
		{Group: "helm.toolkit.fluxcd.io", Version: "v2", Resource: "helmreleases"},
		{Group: "helm.toolkit.fluxcd.io", Version: "v2beta2", Resource: "helmreleases"},
	} {
		items, err := r.dynamicClient.Resource(gvr).Namespace(pod.Namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			continue
		}
		for _, item := range items.Items {
			name := item.GetName()
			specReleaseName, _, _ := unstructured.NestedString(item.Object, "spec", "releaseName")
			if releaseName != name && releaseName != specReleaseName {
				continue
			}
			src := r.sourceFromFluxHelmRelease(ctx, item)
			src.Controller = ControllerFlux
			src.AppName = item.GetName()
			src.AppNamespace = item.GetNamespace()
			src.ManifestType = ManifestFluxHelmRelease
			enrichOverlay(&src)
			src.Reason = "matched Flux HelmRelease labels"
			sources = append(sources, src)
		}
		break
	}
	return sources
}

func (r *Resolver) resolveAnnotations(pod *v1.Pod) (WorkloadSource, bool) {
	repoURL := pod.Annotations[r.annotationPrefix+"/repo-url"]
	filePath := pod.Annotations[r.annotationPrefix+"/repo-path"]
	if repoURL == "" || filePath == "" {
		return WorkloadSource{}, false
	}
	cleanPath := path.Clean(filePath)
	if ext := path.Ext(cleanPath); ext == ".yaml" || ext == ".yml" {
		cleanPath = path.Dir(cleanPath)
	}
	source := WorkloadSource{
		Controller:     ControllerAnnotation,
		RepoURL:        repoURL,
		Path:           cleanPath,
		TargetRevision: pod.Annotations[r.annotationPrefix+"/target-revision"],
		ManifestType:   inferManifestType(ManifestUnknown, cleanPath, repoURL),
		Reason:         "matched Fixora pod annotations",
	}
	enrichOverlay(&source)
	return source, true
}

func (r *Resolver) sourceFromFluxKustomization(ctx context.Context, obj unstructured.Unstructured) WorkloadSource {
	sourceRef, _, _ := unstructured.NestedStringMap(obj.Object, "spec", "sourceRef")
	sourceNamespace := firstNonEmpty(sourceRef["namespace"], obj.GetNamespace())
	repoURL, revision := r.resolveFluxSourceRef(ctx, sourceNamespace, sourceRef["kind"], sourceRef["name"])
	pathValue, _, _ := unstructured.NestedString(obj.Object, "spec", "path")
	return WorkloadSource{
		RepoURL:        repoURL,
		TargetRevision: revision,
		Path:           strings.TrimPrefix(pathValue, "./"),
	}
}

func (r *Resolver) sourceFromFluxHelmRelease(ctx context.Context, obj unstructured.Unstructured) WorkloadSource {
	sourceRef, _, _ := unstructured.NestedStringMap(obj.Object, "spec", "chart", "spec", "sourceRef")
	sourceNamespace := firstNonEmpty(sourceRef["namespace"], obj.GetNamespace())
	repoURL, revision := r.resolveFluxSourceRef(ctx, sourceNamespace, sourceRef["kind"], sourceRef["name"])
	chartPath, _, _ := unstructured.NestedString(obj.Object, "spec", "chart", "spec", "chart")
	return WorkloadSource{
		RepoURL:        repoURL,
		TargetRevision: revision,
		Path:           strings.TrimPrefix(chartPath, "./"),
	}
}

func (r *Resolver) resolveFluxSourceRef(ctx context.Context, namespace, kind, name string) (string, string) {
	if name == "" {
		return "", ""
	}
	switch kind {
	case "", "GitRepository":
		return r.resolveFluxGitRepository(ctx, namespace, name)
	case "OCIRepository":
		return r.resolveFluxSource(ctx, namespace, name, schema.GroupVersionResource{Group: "source.toolkit.fluxcd.io", Version: "v1", Resource: "ocirepositories"})
	case "HelmRepository":
		return r.resolveFluxSource(ctx, namespace, name, schema.GroupVersionResource{Group: "source.toolkit.fluxcd.io", Version: "v1", Resource: "helmrepositories"})
	default:
		return "", ""
	}
}

func (r *Resolver) resolveFluxGitRepository(ctx context.Context, namespace, name string) (string, string) {
	for _, version := range []string{"v1", "v1beta2"} {
		repoURL, revision := r.resolveFluxSource(ctx, namespace, name, schema.GroupVersionResource{Group: "source.toolkit.fluxcd.io", Version: version, Resource: "gitrepositories"})
		if repoURL != "" {
			return repoURL, revision
		}
	}
	return "", ""
}

func (r *Resolver) resolveFluxSource(ctx context.Context, namespace, name string, gvr schema.GroupVersionResource) (string, string) {
	obj, err := r.dynamicClient.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", ""
	}
	url, _, _ := unstructured.NestedString(obj.Object, "spec", "url")
	branch, _, _ := unstructured.NestedString(obj.Object, "spec", "ref", "branch")
	tag, _, _ := unstructured.NestedString(obj.Object, "spec", "ref", "tag")
	commit, _, _ := unstructured.NestedString(obj.Object, "spec", "ref", "commit")
	return url, firstNonEmpty(branch, tag, commit)
}

func workloadRefsForPod(ctx context.Context, clientset kubernetes.Interface, pod *v1.Pod) []workloadRef {
	refs := []workloadRef{{Namespace: pod.Namespace, Name: pod.Name, Kind: "Pod"}}
	for _, owner := range pod.OwnerReferences {
		refs = append(refs, workloadRef{Namespace: pod.Namespace, Name: owner.Name, Kind: owner.Kind})
		if owner.Kind == "ReplicaSet" && clientset != nil {
			rs, err := clientset.AppsV1().ReplicaSets(pod.Namespace).Get(ctx, owner.Name, metav1.GetOptions{})
			if err == nil {
				for _, rsOwner := range rs.OwnerReferences {
					refs = append(refs, workloadRef{Namespace: pod.Namespace, Name: rsOwner.Name, Kind: rsOwner.Kind})
				}
			}
		}
	}
	return refs
}

type workloadRef struct {
	Namespace string
	Name      string
	Kind      string
}

func argoAppManagesWorkload(app map[string]interface{}, refs []workloadRef) bool {
	resources, ok, _ := unstructured.NestedSlice(app, "status", "resources")
	if !ok {
		return false
	}
	for _, item := range resources {
		res, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		kind, _ := res["kind"].(string)
		name, _ := res["name"].(string)
		namespace, _ := res["namespace"].(string)
		for _, ref := range refs {
			if ref.Kind == kind && ref.Name == name && (namespace == "" || ref.Namespace == namespace) {
				return true
			}
		}
	}
	return false
}

func argoSources(app map[string]interface{}) []WorkloadSource {
	var sources []WorkloadSource
	if source, ok, _ := unstructured.NestedMap(app, "spec", "source"); ok {
		if src := sourceFromArgoMap(source); src.RepoURL != "" {
			sources = append(sources, src)
		}
	}
	if rawSources, ok, _ := unstructured.NestedSlice(app, "spec", "sources"); ok {
		for _, raw := range rawSources {
			source, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			if src := sourceFromArgoMap(source); src.RepoURL != "" {
				sources = append(sources, src)
			}
		}
	}
	return sources
}

func sourceFromArgoMap(source map[string]interface{}) WorkloadSource {
	repoURL, _ := source["repoURL"].(string)
	pathValue, _ := source["path"].(string)
	targetRevision, _ := source["targetRevision"].(string)
	chart, _ := source["chart"].(string)
	manifestType := ManifestUnknown
	if chart != "" {
		manifestType = ManifestHelm
		pathValue = chart
	}
	if _, ok := source["helm"]; ok {
		manifestType = ManifestHelm
	}
	if _, ok := source["kustomize"]; ok {
		manifestType = ManifestKustomize
	}
	return WorkloadSource{
		RepoURL:        repoURL,
		Path:           strings.TrimPrefix(pathValue, "./"),
		TargetRevision: targetRevision,
		ManifestType:   manifestType,
	}
}

func fluxInventoryContainsAny(obj map[string]interface{}, refs []workloadRef) bool {
	for _, ref := range refs {
		if fluxInventoryContains(obj, ref.Namespace, ref.Name, ref.Kind) {
			return true
		}
	}
	return false
}

func fluxInventoryContains(obj map[string]interface{}, namespace, name, kind string) bool {
	entries, ok, _ := unstructured.NestedSlice(obj, "status", "inventory", "entries")
	if !ok {
		return false
	}
	for _, item := range entries {
		entry, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		id, _ := entry["id"].(string)
		id = strings.ToLower(id)
		if strings.Contains(id, strings.ToLower(namespace)) && strings.Contains(id, strings.ToLower(name)) && strings.Contains(id, strings.ToLower(kind)) {
			return true
		}
	}
	return false
}

func inferManifestType(current ManifestType, pathValue, repoURL string) ManifestType {
	if current != ManifestUnknown && current != "" {
		return current
	}
	lowerPath := strings.ToLower(pathValue)
	lowerRepo := strings.ToLower(repoURL)
	switch {
	case strings.Contains(lowerPath, "kustomization") || strings.Contains(lowerPath, "overlays/") || strings.Contains(lowerPath, "/base"):
		return ManifestKustomize
	case strings.Contains(lowerPath, "chart") || strings.Contains(lowerPath, "helm") || strings.Contains(lowerRepo, "chart"):
		return ManifestHelm
	default:
		return ManifestRaw
	}
}

func DetectManifestTypeFromFiles(current ManifestType, sourcePath string, files map[string][]byte) ManifestType {
	if current == ManifestFluxHelmRelease {
		return current
	}
	for filePath := range files {
		name := strings.ToLower(path.Base(filePath))
		switch name {
		case "kustomization.yaml", "kustomization.yml":
			return ManifestKustomize
		case "chart.yaml":
			return ManifestHelm
		}
	}
	return inferManifestType(current, sourcePath, "")
}

func enrichOverlay(source *WorkloadSource) {
	segments := strings.Split(strings.Trim(source.Path, "/"), "/")
	source.OverlayRole = OverlayUnknown
	for i, segment := range segments {
		switch strings.ToLower(segment) {
		case "base", "bases":
			source.OverlayRole = OverlayBase
		case "overlay", "overlays":
			source.OverlayRole = OverlayEnv
			if i+1 < len(segments) {
				source.Environment = segments[i+1]
			}
			if i+2 < len(segments) {
				source.Region = segments[i+2]
			}
		}
	}
}

func dedupeSources(sources []WorkloadSource) []WorkloadSource {
	seen := map[string]bool{}
	var deduped []WorkloadSource
	for _, source := range sources {
		key := strings.Join([]string{string(source.Controller), source.RepoURL, source.TargetRevision, source.Path, string(source.ManifestType)}, "|")
		if seen[key] {
			continue
		}
		seen[key] = true
		deduped = append(deduped, source)
	}
	return deduped
}

func sourceRank(source WorkloadSource) int {
	score := 100
	switch source.Controller {
	case ControllerArgoCD, ControllerFlux:
		score -= 50
	case ControllerAnnotation:
		score -= 10
	}
	if source.OverlayRole == OverlayEnv {
		score -= 20
	}
	if source.ManifestType != ManifestUnknown {
		score -= 5
	}
	return score
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
