package controller

import (
	"fixora/pkg/config"
	"fixora/pkg/metrics"
	"testing"
	"time"

	"github.com/prometheus/common/model"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

type mockMetricsProvider struct {
	usage      float64
	limit      float64
	request    float64
	historical model.Matrix
}

func (m *mockMetricsProvider) GetPodUsage(ns, pod string) (float64, error) {
	return m.usage, nil
}
func (m *mockMetricsProvider) GetPodLimits(ns, pod string) (float64, float64, error) {
	return m.request, m.limit, nil
}
func (m *mockMetricsProvider) GetHistory(ns, pod string, d time.Duration) (model.Matrix, error) {
	return m.historical, nil
}

// Ensure mockMetricsProvider implements metrics.MetricsProvider
var _ metrics.MetricsProvider = (*mockMetricsProvider)(nil)

func TestScanForLeaksSimulator(t *testing.T) {
	ns := "default"
	podName := "leaky-pod"

	// 1. Setup Fake K8s
	clientset := fake.NewSimpleClientset(&v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: ns,
		},
		Status: v1.PodStatus{
			Phase: v1.PodRunning,
		},
	})

	// 2. Setup Mock Metrics Provider with a growth trend
	// Start at 100MiB, grow to 150MiB over 10 points (50% growth)
	values := make([]model.SamplePair, 10)
	startTime := time.Now().Add(-1 * time.Hour)
	for i := 0; i < 10; i++ {
		values[i] = model.SamplePair{
			Timestamp: model.TimeFromUnix(startTime.Add(time.Duration(i*5) * time.Minute).Unix()),
			Value:     model.SampleValue(100*1024*1024 + float64(i*5*1024*1024)),
		}
	}

	mockMetrics := &mockMetricsProvider{
		usage: 150 * 1024 * 1024,
		limit: 200 * 1024 * 1024,
		historical: model.Matrix{
			&model.SampleStream{
				Values: values,
			},
		},
	}

	// 3. Setup Controller
	cfg := &config.Config{
		PredictiveEnabled:         true,
		PredictiveGrowthThreshold: 0.20,
		PredictiveMinDataPoints:   10,
	}

	ctrl := &Controller{
		clientset:  clientset,
		promClient: mockMetrics,
		config:     cfg,
		history:    &historyCache{}, // Nil DB is fine
	}

	// 4. Run Scan
	t.Log("Running scanForLeaks with synthetic growth trend...")
	ctrl.scanForLeaks()
}
