package argocd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

var (
	appGVR = schema.GroupVersionResource{
		Group:    "argoproj.io",
		Version:  "v1alpha1",
		Resource: "applications",
	}
)

type AppInfo struct {
	RepoURL        string
	Path           string
	TargetRevision string
	Name           string
	Namespace      string
	HealthStatus   string
	SyncStatus     string
	Revision       string
	OperationPhase string
	ResourceCount  int
}

type Client struct {
	dynamicClient dynamic.Interface
	namespace     string
	apiURL        string
	apiToken      string
	httpClient    *http.Client
}

func New(dynamicClient dynamic.Interface, namespace, apiURL, apiToken string) *Client {
	return &Client{
		dynamicClient: dynamicClient,
		namespace:     namespace,
		apiURL:        apiURL,
		apiToken:      apiToken,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *Client) GetAppForResource(ctx context.Context, namespace, name, kind string) (*AppInfo, error) {
	// 1. Try CRD first (if dynamicClient is provided)
	if c.dynamicClient != nil {
		info, err := c.getAppViaCRD(ctx, namespace, name, kind)
		if err == nil {
			return info, nil
		}
	}

	// 2. Fallback to API if configured
	if c.apiURL != "" && c.apiToken != "" {
		return c.getAppViaAPI(ctx, namespace, name, kind)
	}

	return nil, fmt.Errorf("no application found and direct API not configured")
}

func (c *Client) getAppViaCRD(ctx context.Context, namespace, name, kind string) (*AppInfo, error) {
	apps, err := c.dynamicClient.Resource(appGVR).Namespace(c.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, app := range apps.Items {
		if info := c.extractMatch(app.Object, namespace, name, kind); info != nil {
			return info, nil
		}
	}
	return nil, fmt.Errorf("not found in CRDs")
}

func (c *Client) getAppViaAPI(ctx context.Context, namespace, name, kind string) (*AppInfo, error) {
	url := fmt.Sprintf("%s/api/v1/applications", strings.TrimSuffix(c.apiURL, "/"))
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("argocd api returned status: %d", resp.StatusCode)
	}

	var result struct {
		Items []map[string]interface{} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	for _, app := range result.Items {
		if info := c.extractMatch(app, namespace, name, kind); info != nil {
			return info, nil
		}
	}

	return nil, fmt.Errorf("no argocd application found managing %s/%s via API", namespace, name)
}

func (c *Client) extractMatch(app map[string]interface{}, namespace, name, kind string) *AppInfo {
	appName, appNamespace := metadataNameNamespace(app)
	spec, ok := app["spec"].(map[string]interface{})
	if !ok {
		return nil
	}

	// Basic check: destination namespace
	destination, ok := spec["destination"].(map[string]interface{})
	if ok && destination["namespace"] != namespace {
		return nil
	}

	status, ok := app["status"].(map[string]interface{})
	if !ok {
		return nil
	}

	resources, ok := status["resources"].([]interface{})
	if !ok {
		return nil
	}

	for _, res := range resources {
		r, ok := res.(map[string]interface{})
		if !ok {
			continue
		}

		if r["kind"] == kind && r["name"] == name && r["namespace"] == namespace {
			source, ok := spec["source"].(map[string]interface{})
			if !ok {
				return nil
			}

			repoURL, _ := source["repoURL"].(string)
			path, _ := source["path"].(string)
			targetRevision, _ := source["targetRevision"].(string)
			healthStatus, syncStatus, revision, operationPhase := argoStatusSummary(status)
			return &AppInfo{
				RepoURL:        repoURL,
				Path:           path,
				TargetRevision: targetRevision,
				Name:           appName,
				Namespace:      appNamespace,
				HealthStatus:   healthStatus,
				SyncStatus:     syncStatus,
				Revision:       revision,
				OperationPhase: operationPhase,
				ResourceCount:  len(resources),
			}
		}
	}

	return nil
}

func (a AppInfo) Summary() string {
	var parts []string
	if a.Name != "" {
		if a.Namespace != "" {
			parts = append(parts, fmt.Sprintf("app=%s/%s", a.Namespace, a.Name))
		} else {
			parts = append(parts, "app="+a.Name)
		}
	}
	if a.RepoURL != "" {
		parts = append(parts, "repo="+a.RepoURL)
	}
	if a.Path != "" {
		parts = append(parts, "path="+a.Path)
	}
	if a.TargetRevision != "" {
		parts = append(parts, "targetRevision="+a.TargetRevision)
	}
	if a.Revision != "" {
		parts = append(parts, "liveRevision="+a.Revision)
	}
	if a.SyncStatus != "" {
		parts = append(parts, "sync="+a.SyncStatus)
	}
	if a.HealthStatus != "" {
		parts = append(parts, "health="+a.HealthStatus)
	}
	if a.OperationPhase != "" {
		parts = append(parts, "operation="+a.OperationPhase)
	}
	if a.ResourceCount > 0 {
		parts = append(parts, fmt.Sprintf("resources=%d", a.ResourceCount))
	}
	return strings.Join(parts, ", ")
}

func metadataNameNamespace(obj map[string]interface{}) (string, string) {
	metadata, ok := obj["metadata"].(map[string]interface{})
	if !ok {
		return "", ""
	}
	name, _ := metadata["name"].(string)
	namespace, _ := metadata["namespace"].(string)
	return name, namespace
}

func argoStatusSummary(status map[string]interface{}) (healthStatus, syncStatus, revision, operationPhase string) {
	if health, ok := status["health"].(map[string]interface{}); ok {
		healthStatus, _ = health["status"].(string)
	}
	if sync, ok := status["sync"].(map[string]interface{}); ok {
		syncStatus, _ = sync["status"].(string)
		revision, _ = sync["revision"].(string)
	}
	if operation, ok := status["operationState"].(map[string]interface{}); ok {
		operationPhase, _ = operation["phase"].(string)
	}
	return healthStatus, syncStatus, revision, operationPhase
}
