package metrics

import (
	"context"
	"fmt"
	"time"

	"github.com/prometheus/common/model"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	metricsclientset "k8s.io/metrics/pkg/client/clientset/versioned"
)

type K8sMetricsProvider struct {
	clientset       kubernetes.Interface
	metricsClient metricsclientset.Interface
}

// NewK8sMetricsProvider creates a new provider that uses the K8s Metrics API.
func NewK8sMetricsProvider(clientset kubernetes.Interface, metricsClient metricsclientset.Interface) *K8sMetricsProvider {
	return &K8sMetricsProvider{
		clientset:       clientset,
		metricsClient: metricsClient,
	}
}

// GetPodUsage returns the current memory usage (working set) for a pod.
func (p *K8sMetricsProvider) GetPodUsage(ns, podName string) (float64, error) {
	metrics, err := p.metricsClient.MetricsV1beta1().PodMetricses(ns).Get(context.TODO(), podName, metav1.GetOptions{})
	if err != nil {
		return 0, fmt.Errorf("failed to fetch pod metrics: %w", err)
	}

	var totalUsage int64
	for _, container := range metrics.Containers {
		if memory, ok := container.Usage.Memory().AsInt64(); ok {
			totalUsage += memory
		}
	}

	return float64(totalUsage), nil
}

// GetPodLimits calculates the memory requests and limits by reading the Pod Spec.
func (p *K8sMetricsProvider) GetPodLimits(ns, podName string) (float64, float64, error) {
	pod, err := p.clientset.CoreV1().Pods(ns).Get(context.TODO(), podName, metav1.GetOptions{})
	if err != nil {
		return 0, 0, fmt.Errorf("failed to fetch pod spec: %w", err)
	}

	var totalRequests, totalLimits int64
	for _, container := range pod.Spec.Containers {
		if req, ok := container.Resources.Requests.Memory().AsInt64(); ok {
			totalRequests += req
		}
		if lim, ok := container.Resources.Limits.Memory().AsInt64(); ok {
			totalLimits += lim
		}
	}

	return float64(totalRequests), float64(totalLimits), nil
}

// GetHistory is not supported by the K8s Metrics API.
func (p *K8sMetricsProvider) GetHistory(ns, pod string, d time.Duration) (model.Matrix, error) {
	return nil, fmt.Errorf("historical metrics are not supported by the K8s Metrics API (use Prometheus for history)")
}

// Ensure K8sMetricsProvider implements MetricsProvider
var _ MetricsProvider = (*K8sMetricsProvider)(nil)
