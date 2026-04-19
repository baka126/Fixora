package prometheus

import (
	"context"
	"fmt"
	"time"

	"fixora/pkg/metrics"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

type Client struct {
	api v1.API
}

// New creates a new Prometheus client.
func New(address string) (*Client, error) {
	client, err := api.NewClient(api.Config{
		Address: address,
	})
	if err != nil {
		return nil, err
	}

	return &Client{
		api: v1.NewAPI(client),
	}, nil
}

// GetPodUsage returns the current memory usage (working set bytes) for a pod.
func (c *Client) GetPodUsage(namespace, pod string) (float64, error) {
	query := fmt.Sprintf(`sum(container_memory_working_set_bytes{namespace="%s", pod="%s", container!=""})`, namespace, pod)
	result, _, err := c.api.Query(context.TODO(), query, time.Now())
	if err != nil {
		return 0, err
	}

	vector, ok := result.(model.Vector)
	if !ok || len(vector) == 0 {
		return 0, fmt.Errorf("no memory usage data found for pod %s/%s", namespace, pod)
	}

	return float64(vector[0].Value), nil
}

// GetPodLimits returns the memory requests and limits for a pod.
func (c *Client) GetPodLimits(namespace, pod string) (float64, float64, error) {
	reqQuery := fmt.Sprintf(`sum(kube_pod_container_resource_requests{namespace="%s", pod="%s", resource="memory"})`, namespace, pod)
	limitQuery := fmt.Sprintf(`sum(kube_pod_container_resource_limits{namespace="%s", pod="%s", resource="memory"})`, namespace, pod)

	var request, limit float64

	// Fetch Request
	res, _, err := c.api.Query(context.TODO(), reqQuery, time.Now())
	if err == nil {
		if vector, ok := res.(model.Vector); ok && len(vector) > 0 {
			request = float64(vector[0].Value)
		}
	}

	// Fetch Limit
	res, _, err = c.api.Query(context.TODO(), limitQuery, time.Now())
	if err == nil {
		if vector, ok := res.(model.Vector); ok && len(vector) > 0 {
			limit = float64(vector[0].Value)
		}
	}

	return request, limit, nil
}

// GetHistory returns historical memory usage matrix for a pod.
func (c *Client) GetHistory(namespace, pod string, d time.Duration) (model.Matrix, error) {
	query := fmt.Sprintf(`sum(container_memory_working_set_bytes{namespace="%s", pod="%s", container!=""})`, namespace, pod)
	r := v1.Range{
		Start: time.Now().Add(-d),
		End:   time.Now(),
		Step:  time.Minute * 5,
	}

	result, _, err := c.api.QueryRange(context.TODO(), query, r)
	if err != nil {
		return nil, err
	}

	matrix, ok := result.(model.Matrix)
	if !ok {
		return nil, fmt.Errorf("unexpected result type: %T", result)
	}

	return matrix, nil
}

// Extra methods for granular Prometheus-specific data (not in MetricsProvider but useful)

func (c *Client) GetPodMemoryRSS(namespace, podName string) (float64, error) {
	query := fmt.Sprintf(`sum(container_memory_rss{namespace="%s", pod="%s", container!=""})`, namespace, podName)
	result, _, err := c.api.Query(context.TODO(), query, time.Now())
	if err != nil {
		return 0, err
	}

	vector, ok := result.(model.Vector)
	if !ok || len(vector) == 0 {
		return 0, fmt.Errorf("no memory RSS data found for pod %s/%s", namespace, podName)
	}

	return float64(vector[0].Value), nil
}

func (c *Client) GetPodMemoryCache(namespace, podName string) (float64, error) {
	query := fmt.Sprintf(`sum(container_memory_cache{namespace="%s", pod="%s", container!=""})`, namespace, podName)
	result, _, err := c.api.Query(context.TODO(), query, time.Now())
	if err != nil {
		return 0, err
	}

	vector, ok := result.(model.Vector)
	if !ok || len(vector) == 0 {
		return 0, fmt.Errorf("no memory cache data found for pod %s/%s", namespace, podName)
	}

	return float64(vector[0].Value), nil
}

// Ensure Client implements metrics.MetricsProvider
var _ metrics.MetricsProvider = (*Client)(nil)
