package gitops

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
)

func TestResolveArgoCDSourceForDeploymentOwner(t *testing.T) {
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "api-7d9f-abc",
			Namespace: "prod",
			OwnerReferences: []metav1.OwnerReference{{
				Kind: "ReplicaSet",
				Name: "api-7d9f",
			}},
		},
	}
	rs := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "api-7d9f",
			Namespace: "prod",
			OwnerReferences: []metav1.OwnerReference{{
				Kind: "Deployment",
				Name: "api",
			}},
		},
	}
	app := app("payments", "https://github.com/acme/fleet.git", "apps/api/overlays/prod/us-east-1", "main", map[string]interface{}{
		"kind":      "Deployment",
		"name":      "api",
		"namespace": "prod",
	})
	resolver := NewResolver(
		fake.NewSimpleClientset(pod, rs),
		dynamicClient(app),
		ResolverConfig{ArgoCDEnabled: true, ArgoCDNamespace: "argocd"},
	)

	got, err := resolver.ResolvePod(context.Background(), pod)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 source, got %d", len(got))
	}
	if got[0].Controller != ControllerArgoCD || got[0].ManifestType != ManifestKustomize {
		t.Fatalf("unexpected source: %+v", got[0])
	}
	if got[0].OverlayRole != OverlayEnv || got[0].Environment != "prod" || got[0].Region != "us-east-1" {
		t.Fatalf("expected prod/us-east-1 overlay metadata, got %+v", got[0])
	}
}

func TestResolveAnnotationFallback(t *testing.T) {
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "api",
			Namespace: "default",
			Annotations: map[string]string{
				"fixora.io/repo-url":        "https://github.com/acme/app.git",
				"fixora.io/repo-path":       "deploy/prod/deployment.yaml",
				"fixora.io/target-revision": "release",
			},
		},
	}
	resolver := NewResolver(fake.NewSimpleClientset(pod), nil, ResolverConfig{})

	got, err := resolver.ResolvePod(context.Background(), pod)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 source, got %d", len(got))
	}
	if got[0].Controller != ControllerAnnotation || got[0].Path != "deploy/prod" || got[0].TargetRevision != "release" {
		t.Fatalf("unexpected annotation source: %+v", got[0])
	}
}

func TestResolveAnnotationFallbackFromOwnerDeployment(t *testing.T) {
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "api-7d9f9d7f6f-x2abc",
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{{
				Kind: "ReplicaSet",
				Name: "api-7d9f9d7f6f",
			}},
		},
	}
	rs := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "api-7d9f9d7f6f",
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{{
				Kind: "Deployment",
				Name: "api",
			}},
		},
	}
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "api",
			Namespace: "default",
			Annotations: map[string]string{
				"fixora.io/repo-url":        "https://github.com/acme/app.git",
				"fixora.io/repo-path":       "deploy/prod/deployment.yaml",
				"fixora.io/target-revision": "release",
			},
		},
	}
	resolver := NewResolver(fake.NewSimpleClientset(pod, rs, deployment), nil, ResolverConfig{})

	got, err := resolver.ResolvePod(context.Background(), pod)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 owner annotation source, got %d", len(got))
	}
	if got[0].Controller != ControllerAnnotation || got[0].Path != "deploy/prod" || got[0].TargetRevision != "release" {
		t.Fatalf("unexpected owner annotation source: %+v", got[0])
	}
	if got[0].Reason != "matched Fixora Deployment annotations" {
		t.Fatalf("unexpected source reason %q", got[0].Reason)
	}
}

func TestResolveArgoCDHelmSourceMetadata(t *testing.T) {
	pod := &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "nginx-abc", Namespace: "prod", Labels: map[string]string{"app.kubernetes.io/instance": "edge"}}}
	app := appWithSource("edge", map[string]interface{}{
		"repoURL":        "https://charts.bitnami.com/bitnami",
		"chart":          "nginx",
		"targetRevision": "15.4.2",
		"helm": map[string]interface{}{
			"releaseName": "edge",
			"valueFiles":  []interface{}{"values-prod.yaml", "values-us-east-1.yaml"},
			"parameters": []interface{}{
				map[string]interface{}{"name": "image.tag", "value": "1.25.3"},
			},
			"values": "resources:\n  limits:\n    memory: 256Mi\n",
		},
	}, map[string]interface{}{
		"kind":      "Pod",
		"name":      "nginx-abc",
		"namespace": "prod",
	})
	resolver := NewResolver(fake.NewSimpleClientset(pod), dynamicClient(app), ResolverConfig{ArgoCDEnabled: true, ArgoCDNamespace: "argocd"})

	got, err := resolver.ResolvePod(context.Background(), pod)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 source, got %d", len(got))
	}
	src := got[0]
	if src.ManifestType != ManifestHelm || src.Helm.Chart != "nginx" || src.Helm.ChartVersion != "15.4.2" {
		t.Fatalf("expected Helm chart metadata, got %+v", src)
	}
	if src.Helm.ReleaseName != "edge" || len(src.Helm.ValueFiles) != 2 || src.Helm.Parameters["image.tag"] != "1.25.3" || !src.Helm.HasInlineValues {
		t.Fatalf("expected Helm values metadata, got %+v", src.Helm)
	}
}

func TestResolveFluxHelmReleaseMetadata(t *testing.T) {
	pod := &v1.Pod{ObjectMeta: metav1.ObjectMeta{
		Name:      "api-abc",
		Namespace: "prod",
		Labels:    map[string]string{"helm.toolkit.fluxcd.io/name": "api"},
	}}
	hr := fluxHelmRelease("api", "prod", map[string]interface{}{
		"releaseName":     "api-prod",
		"targetNamespace": "prod",
		"chart": map[string]interface{}{"spec": map[string]interface{}{
			"chart":   "podinfo",
			"version": "6.6.2",
			"sourceRef": map[string]interface{}{
				"kind":      "HelmRepository",
				"name":      "podinfo",
				"namespace": "flux-system",
			},
		}},
		"valuesFrom": []interface{}{
			map[string]interface{}{"kind": "ConfigMap", "name": "api-values", "valuesKey": "values.yaml"},
		},
		"values": map[string]interface{}{"replicaCount": int64(2)},
	})
	repo := fluxHelmRepository("podinfo", "flux-system", "https://stefanprodan.github.io/podinfo")
	resolver := NewResolver(fake.NewSimpleClientset(pod), dynamicClient(hr, repo), ResolverConfig{})

	got, err := resolver.ResolvePod(context.Background(), pod)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 source, got %d", len(got))
	}
	src := got[0]
	if src.ManifestType != ManifestFluxHelmRelease || src.Helm.Chart != "podinfo" || src.Helm.ChartVersion != "6.6.2" {
		t.Fatalf("expected Flux HelmRelease chart metadata, got %+v", src)
	}
	if src.Helm.ReleaseName != "api-prod" || src.Helm.SourceRefKind != "HelmRepository" || len(src.Helm.ValuesFrom) != 1 || !src.Helm.HasInlineValues {
		t.Fatalf("expected Flux Helm values metadata, got %+v", src.Helm)
	}
}

func TestResolveFluxHelmReleasePrefersFleetRepoSource(t *testing.T) {
	pod := &v1.Pod{ObjectMeta: metav1.ObjectMeta{
		Name:      "api-abc",
		Namespace: "prod",
		Labels:    map[string]string{"helm.toolkit.fluxcd.io/name": "api"},
	}}
	hr := fluxHelmRelease("api", "prod", map[string]interface{}{
		"chart": map[string]interface{}{"spec": map[string]interface{}{
			"chart":   "podinfo",
			"version": "6.6.2",
			"sourceRef": map[string]interface{}{
				"kind":      "HelmRepository",
				"name":      "podinfo",
				"namespace": "flux-system",
			},
		}},
	})
	chartRepo := fluxHelmRepository("podinfo", "flux-system", "https://stefanprodan.github.io/podinfo")
	fleetRepo := fluxGitRepository("fleet", "flux-system", "https://github.com/acme/fleet.git", "main")
	kustomization := fluxKustomization("apps-prod", "flux-system", "fleet", "./clusters/prod/us-east-1", []interface{}{
		map[string]interface{}{"id": "prod_prod_api_helm.toolkit.fluxcd.io_HelmRelease"},
	})
	resolver := NewResolver(fake.NewSimpleClientset(pod), dynamicClient(hr, chartRepo, fleetRepo, kustomization), ResolverConfig{})

	got, err := resolver.ResolvePod(context.Background(), pod)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 source, got %d: %+v", len(got), got)
	}
	src := got[0]
	if src.RepoURL != "https://github.com/acme/fleet.git" || src.Path != "clusters/prod/us-east-1" {
		t.Fatalf("expected fleet Git source for PR target, got %+v", src)
	}
	if src.ManifestType != ManifestFluxHelmRelease || src.Helm.RepoURL != "https://stefanprodan.github.io/podinfo" || src.Helm.Chart != "podinfo" {
		t.Fatalf("expected Helm chart metadata preserved on fleet source, got %+v", src)
	}
}

func TestDetectManifestTypeFromFiles(t *testing.T) {
	tests := []struct {
		name  string
		files map[string][]byte
		want  ManifestType
	}{
		{name: "kustomize", files: map[string][]byte{"apps/api/overlays/prod/kustomization.yaml": nil}, want: ManifestKustomize},
		{name: "helm", files: map[string][]byte{"charts/api/Chart.yaml": nil}, want: ManifestHelm},
		{name: "raw", files: map[string][]byte{"deploy/api.yaml": nil}, want: ManifestRaw},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DetectManifestTypeFromFiles(ManifestUnknown, "", tt.files); got != tt.want {
				t.Fatalf("got %s, want %s", got, tt.want)
			}
		})
	}
}

func dynamicClient(objects ...runtime.Object) *dynamicfake.FakeDynamicClient {
	listKinds := map[schema.GroupVersionResource]string{
		{Group: "argoproj.io", Version: "v1alpha1", Resource: "applications"}:                  "ApplicationList",
		{Group: "kustomize.toolkit.fluxcd.io", Version: "v1", Resource: "kustomizations"}:      "KustomizationList",
		{Group: "kustomize.toolkit.fluxcd.io", Version: "v1beta2", Resource: "kustomizations"}: "KustomizationList",
		{Group: "helm.toolkit.fluxcd.io", Version: "v2", Resource: "helmreleases"}:             "HelmReleaseList",
		{Group: "helm.toolkit.fluxcd.io", Version: "v2beta2", Resource: "helmreleases"}:        "HelmReleaseList",
		{Group: "source.toolkit.fluxcd.io", Version: "v1", Resource: "gitrepositories"}:        "GitRepositoryList",
		{Group: "source.toolkit.fluxcd.io", Version: "v1beta2", Resource: "gitrepositories"}:   "GitRepositoryList",
		{Group: "source.toolkit.fluxcd.io", Version: "v1", Resource: "ocirepositories"}:        "OCIRepositoryList",
		{Group: "source.toolkit.fluxcd.io", Version: "v1", Resource: "helmrepositories"}:       "HelmRepositoryList",
	}
	return dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), listKinds, objects...)
}

func app(name, repoURL, path, revision string, resource map[string]interface{}) *unstructured.Unstructured {
	return appWithSource(name, map[string]interface{}{
		"repoURL":        repoURL,
		"path":           path,
		"targetRevision": revision,
	}, resource)
}

func appWithSource(name string, source map[string]interface{}, resource map[string]interface{}) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "argoproj.io/v1alpha1",
		"kind":       "Application",
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": "argocd",
		},
		"spec": map[string]interface{}{
			"source": source,
			"destination": map[string]interface{}{
				"namespace": "prod",
			},
		},
		"status": map[string]interface{}{
			"resources": []interface{}{resource},
		},
	}}
}

func fluxHelmRelease(name, namespace string, spec map[string]interface{}) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "helm.toolkit.fluxcd.io/v2",
		"kind":       "HelmRelease",
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": namespace,
		},
		"spec": spec,
	}}
}

func fluxHelmRepository(name, namespace, url string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "source.toolkit.fluxcd.io/v1",
		"kind":       "HelmRepository",
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": namespace,
		},
		"spec": map[string]interface{}{
			"url": url,
		},
	}}
}

func fluxGitRepository(name, namespace, url, branch string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "source.toolkit.fluxcd.io/v1",
		"kind":       "GitRepository",
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": namespace,
		},
		"spec": map[string]interface{}{
			"url": url,
			"ref": map[string]interface{}{
				"branch": branch,
			},
		},
	}}
}

func fluxKustomization(name, namespace, sourceName, path string, inventory []interface{}) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "kustomize.toolkit.fluxcd.io/v1",
		"kind":       "Kustomization",
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": namespace,
		},
		"spec": map[string]interface{}{
			"path": path,
			"sourceRef": map[string]interface{}{
				"kind": "GitRepository",
				"name": sourceName,
			},
		},
		"status": map[string]interface{}{
			"inventory": map[string]interface{}{
				"entries": inventory,
			},
		},
	}}
}
