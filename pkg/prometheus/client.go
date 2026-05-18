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

// GetPodCPULimits returns the CPU requests and limits for a pod.
func (c *Client) GetPodCPULimits(namespace, pod string) (float64, float64, error) {
	reqQuery := fmt.Sprintf(`sum(kube_pod_container_resource_requests{namespace="%s", pod="%s", resource="cpu"})`, namespace, pod)
	limitQuery := fmt.Sprintf(`sum(kube_pod_container_resource_limits{namespace="%s", pod="%s", resource="cpu"})`, namespace, pod)

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

// GetHTTPErrorRate calculates the 5xx error rate over the last 5 minutes for a given pod or service
// using common ingress metrics (e.g., nginx ingress controller).
func (c *Client) GetHTTPErrorRate(namespace, pod string) (float64, error) {
	// A generic query checking 5xx responses grouped by namespace/pod.
	// Adjust based on the actual ingress/service metrics being emitted (e.g., nginx_ingress_controller_requests)
	query := fmt.Sprintf(`
		sum(rate(http_requests_total{namespace="%s", pod=~".*%s.*", status=~"5.."}[5m]))
		/
		sum(rate(http_requests_total{namespace="%s", pod=~".*%s.*"}[5m]))
	`, namespace, pod, namespace, pod)

	result, _, err := c.api.Query(context.TODO(), query, time.Now())
	if err != nil {
		return 0, err
	}

	vector, ok := result.(model.Vector)
	if !ok || len(vector) == 0 {
		return 0, fmt.Errorf("no http error rate data found for pod %s/%s", namespace, pod)
	}

	return float64(vector[0].Value), nil
}

// GetP99Latency returns the 99th percentile HTTP request latency over the last 5 minutes.
func (c *Client) GetP99Latency(namespace, pod string) (float64, error) {
	query := fmt.Sprintf(`
		histogram_quantile(0.99, sum(rate(http_request_duration_seconds_bucket{namespace="%s", pod=~".*%s.*"}[5m])) by (le))
	`, namespace, pod)

	result, _, err := c.api.Query(context.TODO(), query, time.Now())
	if err != nil {
		return 0, err
	}

	vector, ok := result.(model.Vector)
	if !ok || len(vector) == 0 {
		return 0, fmt.Errorf("no p99 latency data found for pod %s/%s", namespace, pod)
	}

	return float64(vector[0].Value), nil
}

// GetHTTPRequestsPerSecond returns the total requests per second over the last 5 minutes.
func (c *Client) GetHTTPRequestsPerSecond(namespace, pod string) (float64, error) {
	query := fmt.Sprintf(`sum(rate(http_requests_total{namespace="%s", pod=~".*%s.*"}[5m]))`, namespace, pod)

	result, _, err := c.api.Query(context.TODO(), query, time.Now())
	if err != nil {
		return 0, err
	}

	vector, ok := result.(model.Vector)
	if !ok || len(vector) == 0 {
		return 0, fmt.Errorf("no RPS data found for pod %s/%s", namespace, pod)
	}

	return float64(vector[0].Value), nil
}

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

// Ensure Client implements metrics.BulkMetricsProvider
var _ metrics.BulkMetricsProvider = (*Client)(nil)

// GetHighErrorRatePods finds all pods exceeding the error rate threshold in a single query.
func (c *Client) GetHighErrorRatePods(threshold float64) ([]metrics.PodMetricResult, error) {
	query := fmt.Sprintf(`
		(
			sum by (namespace, pod) (rate(http_requests_total{status=~"5.."}[5m]))
			/
			sum by (namespace, pod) (rate(http_requests_total[5m]))
		) > %f
	`, threshold)

	result, _, err := c.api.Query(context.TODO(), query, time.Now())
	if err != nil {
		return nil, err
	}

	vector, ok := result.(model.Vector)
	if !ok {
		return nil, fmt.Errorf("unexpected result type: %T", result)
	}

	var results []metrics.PodMetricResult
	for _, sample := range vector {
		results = append(results, metrics.PodMetricResult{
			Namespace: string(sample.Metric["namespace"]),
			PodName:   string(sample.Metric["pod"]),
			Value:     float64(sample.Value),
		})
	}
	return results, nil
}

// GetHighLatencyPods finds all pods exceeding the latency threshold in a single query.
func (c *Client) GetHighLatencyPods(threshold float64) ([]metrics.PodMetricResult, error) {
	query := fmt.Sprintf(`
		histogram_quantile(0.99, sum by (le, namespace, pod) (rate(http_request_duration_seconds_bucket[5m]))) > %f
	`, threshold)

	result, _, err := c.api.Query(context.TODO(), query, time.Now())
	if err != nil {
		return nil, err
	}

	vector, ok := result.(model.Vector)
	if !ok {
		return nil, fmt.Errorf("unexpected result type: %T", result)
	}

	var results []metrics.PodMetricResult
	for _, sample := range vector {
		results = append(results, metrics.PodMetricResult{
			Namespace: string(sample.Metric["namespace"]),
			PodName:   string(sample.Metric["pod"]),
			Value:     float64(sample.Value),
		})
	}
	return results, nil
}

// GetHighSLOBurnRatePods detects pods consuming error budget quickly across
// both a short and long window. It uses generic http_requests_total metrics.
func (c *Client) GetHighSLOBurnRatePods(objective float64, shortWindow, longWindow time.Duration, threshold float64) ([]metrics.SLOBurnRateResult, error) {
	errorBudget := 1 - objective
	if errorBudget <= 0 || errorBudget >= 1 {
		errorBudget = 0.01
	}
	if shortWindow <= 0 {
		shortWindow = 5 * time.Minute
	}
	if longWindow <= 0 {
		longWindow = time.Hour
	}
	if threshold <= 0 {
		threshold = 14.4
	}

	shortRates, err := c.queryHTTPErrorRatiosByPod(shortWindow)
	if err != nil {
		return nil, err
	}
	longRates, err := c.queryHTTPErrorRatiosByPod(longWindow)
	if err != nil {
		return nil, err
	}

	var out []metrics.SLOBurnRateResult
	longThreshold := threshold / 2
	for key, shortRatio := range shortRates {
		longRatio := longRates[key]
		shortBurn := shortRatio / errorBudget
		longBurn := longRatio / errorBudget
		if shortBurn < threshold || longBurn < longThreshold {
			continue
		}
		ns, pod := splitMetricKey(key)
		out = append(out, metrics.SLOBurnRateResult{
			Namespace:     ns,
			PodName:       pod,
			ShortBurnRate: shortBurn,
			LongBurnRate:  longBurn,
			ErrorBudget:   errorBudget,
		})
	}
	return out, nil
}

func (c *Client) queryHTTPErrorRatiosByPod(window time.Duration) (map[string]float64, error) {
	query := fmt.Sprintf(`
		sum by (namespace, pod) (rate(http_requests_total{status=~"5.."}[%s]))
		/
		sum by (namespace, pod) (rate(http_requests_total[%s]))
	`, promDuration(window), promDuration(window))
	result, _, err := c.api.Query(context.TODO(), query, time.Now())
	if err != nil {
		return nil, err
	}
	vector, ok := result.(model.Vector)
	if !ok {
		return nil, fmt.Errorf("unexpected result type: %T", result)
	}
	out := make(map[string]float64, len(vector))
	for _, sample := range vector {
		ns := string(sample.Metric["namespace"])
		pod := string(sample.Metric["pod"])
		if ns == "" || pod == "" {
			continue
		}
		out[metricKey(ns, pod)] = float64(sample.Value)
	}
	return out, nil
}

// GetTrafficEdges returns service-mesh workload traffic edges when common
// Istio request metrics are available. Missing metrics simply produce no edges.
func (c *Client) GetTrafficEdges(window time.Duration, minRPS float64) ([]metrics.TrafficEdge, error) {
	if window <= 0 {
		window = 5 * time.Minute
	}
	query := fmt.Sprintf(`
		sum by (source_workload_namespace, source_workload, destination_workload_namespace, destination_workload) (
			rate(istio_requests_total{source_workload!="",destination_workload!=""}[%s])
		) > %f
	`, promDuration(window), minRPS)
	result, _, err := c.api.Query(context.TODO(), query, time.Now())
	if err != nil {
		return nil, err
	}
	vector, ok := result.(model.Vector)
	if !ok {
		return nil, fmt.Errorf("unexpected result type: %T", result)
	}
	out := make([]metrics.TrafficEdge, 0, len(vector))
	for _, sample := range vector {
		edge := metrics.TrafficEdge{
			SourceNamespace:      string(sample.Metric["source_workload_namespace"]),
			SourceWorkload:       string(sample.Metric["source_workload"]),
			DestinationNamespace: string(sample.Metric["destination_workload_namespace"]),
			DestinationWorkload:  string(sample.Metric["destination_workload"]),
			RequestsPerSecond:    float64(sample.Value),
		}
		if edge.SourceWorkload == "" || edge.DestinationWorkload == "" {
			continue
		}
		out = append(out, edge)
	}
	return out, nil
}

func promDuration(d time.Duration) string {
	if d <= 0 {
		return "5m"
	}
	if d%time.Hour == 0 {
		return fmt.Sprintf("%dh", int(d/time.Hour))
	}
	if d%time.Minute == 0 {
		return fmt.Sprintf("%dm", int(d/time.Minute))
	}
	if d%time.Second == 0 {
		return fmt.Sprintf("%ds", int(d/time.Second))
	}
	return fmt.Sprintf("%ds", int(d.Seconds()))
}

func metricKey(namespace, pod string) string {
	return namespace + "\x00" + pod
}

func splitMetricKey(key string) (string, string) {
	for i := range key {
		if key[i] == 0 {
			return key[:i], key[i+1:]
		}
	}
	return "", key
}
