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
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "argoproj.io/v1alpha1",
		"kind":       "Application",
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": "argocd",
		},
		"spec": map[string]interface{}{
			"source": map[string]interface{}{
				"repoURL":        repoURL,
				"path":           path,
				"targetRevision": revision,
			},
			"destination": map[string]interface{}{
				"namespace": "prod",
			},
		},
		"status": map[string]interface{}{
			"resources": []interface{}{resource},
		},
	}}
}
