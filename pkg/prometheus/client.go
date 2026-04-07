package prometheus

import (
	"context"
	"fmt"
	"time"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

type Client struct {
	api v1.API
}

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

func (c *Client) GetPodMemoryUsage(namespace, podName string, duration time.Duration) (float64, error) {
	query := fmt.Sprintf(`sum(container_memory_working_set_bytes{namespace="%s", pod="%s", container!=""})`, namespace, podName)
	result, _, err := c.api.Query(context.TODO(), query, time.Now())
	if err != nil {
		return 0, err
	}

	vector, ok := result.(model.Vector)
	if !ok || len(vector) == 0 {
		return 0, fmt.Errorf("no memory usage data found for pod %s/%s", namespace, podName)
	}

	return float64(vector[0].Value), nil
}

func (c *Client) GetPodMemoryLimit(namespace, podName string) (float64, error) {
	query := fmt.Sprintf(`sum(kube_pod_container_resource_limits{namespace="%s", pod="%s", resource="memory"})`, namespace, podName)
	result, _, err := c.api.Query(context.TODO(), query, time.Now())
	if err != nil {
		return 0, err
	}

	vector, ok := result.(model.Vector)
	if !ok || len(vector) == 0 {
		return 0, fmt.Errorf("no memory limit data found for pod %s/%s", namespace, podName)
	}

	return float64(vector[0].Value), nil
}

func (c *Client) GetHistoricalMemoryUsage(namespace, podName string, duration time.Duration) (model.Matrix, error) {
	query := fmt.Sprintf(`sum(container_memory_working_set_bytes{namespace="%s", pod="%s", container!=""})`, namespace, podName)
	r := v1.Range{
		Start: time.Now().Add(-duration),
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
