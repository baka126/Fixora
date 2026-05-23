package controller

import (
	"context"
	"fixora/pkg/config"
	"fixora/pkg/metrics"
	"testing"
	"time"

	"github.com/prometheus/common/model"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

type mockMetricsProvider struct {
	usage      float64
	limit      float64
	request    float64
	cpuLimit   float64
	cpuRequest float64
	historical model.Matrix
}

func (m *mockMetricsProvider) GetPodUsage(ns, pod string) (float64, error) {
	return m.usage, nil
}
func (m *mockMetricsProvider) GetPodLimits(ns, pod string) (float64, float64, error) {
	return m.request, m.limit, nil
}
func (m *mockMetricsProvider) GetPodCPULimits(ns, pod string) (float64, float64, error) {
	return m.cpuRequest, m.cpuLimit, nil
}
func (m *mockMetricsProvider) GetHistory(ns, pod string, d time.Duration) (model.Matrix, error) {
	return m.historical, nil
}
func (m *mockMetricsProvider) GetPodMemoryRSS(ns, pod string) (float64, error) {
	return m.usage * 0.9, nil
}
func (m *mockMetricsProvider) GetPodMemoryCache(ns, pod string) (float64, error) {
	return m.usage * 0.1, nil
}
func (m *mockMetricsProvider) GetHTTPRequestsPerSecond(ns, pod string) (float64, error) {
	return 10.0, nil
}
func (m *mockMetricsProvider) GetP99Latency(ns, pod string) (float64, error) {
	return 0.1, nil
}
func (m *mockMetricsProvider) GetHTTPErrorRate(ns, pod string) (float64, error) {
	return 0.0, nil
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
		history:    newHistoryCache(cfg),
	}

	// 4. Run Scan
	t.Log("Running scanForLeaks with synthetic growth trend...")
	ctrl.scanForLeaks()
}

func TestScanForRightSizingRecordsOpportunity(t *testing.T) {
	ns := "default"
	podName := "oversized-pod"
	clientset := fake.NewSimpleClientset(
		&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}},
		&v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      podName,
				Namespace: ns,
			},
			Status: v1.PodStatus{Phase: v1.PodRunning},
		},
	)

	values := make([]model.SamplePair, 12)
	startTime := time.Now().Add(-24 * time.Hour)
	for i := range values {
		values[i] = model.SamplePair{
			Timestamp: model.TimeFromUnix(startTime.Add(time.Duration(i*2) * time.Hour).Unix()),
			Value:     model.SampleValue(100 * 1024 * 1024),
		}
	}

	cfg := &config.Config{
		RightSizingEnabled:       true,
		RightSizingLookback:      24 * time.Hour,
		RightSizingPeakThreshold: 0.35,
		RightSizingSafetyFactor:  1.5,
		RightSizingMinSavings:    0.01,
		Mode:                     config.DryRun,
		PrivacySendGitToAI:       false,
	}
	history := newHistoryCache(cfg)
	ctrl := &Controller{
		clientset: clientset,
		promClient: &mockMetricsProvider{
			request:    4 * 1024 * 1024 * 1024,
			limit:      4 * 1024 * 1024 * 1024,
			cpuRequest: 1,
			historical: model.Matrix{&model.SampleStream{Values: values}},
		},
		config:  cfg,
		history: history,
	}

	ctrl.scanForRightSizing()

	got, ok := history.Get(context.Background(), ns, podName)
	if !ok || len(got.Incidents) == 0 {
		t.Fatalf("expected right-sizing incident to be recorded, got ok=%v history=%#v", ok, got)
	}
	if got.Incidents[0].Reason != "RightSizingOpportunity" {
		t.Fatalf("reason = %q, want RightSizingOpportunity", got.Incidents[0].Reason)
	}
}

func TestScanForRightSizingDeduplicatesByOwnerWorkload(t *testing.T) {
	ns := "default"
	replicas := int32(2)
	clientset := fake.NewSimpleClientset(
		&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}},
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: ns},
			Spec: appsv1.DeploymentSpec{
				Replicas: &replicas,
				Selector: &metav1.LabelSelector{MatchLabels: map[string]string{
					"app": "api",
				}},
			},
		},
		&appsv1.ReplicaSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "api-abc123",
				Namespace: ns,
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
					Name:       "api",
				}},
			},
		},
		&v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "api-abc123-one",
				Namespace: ns,
				Labels: map[string]string{
					"app":               "api",
					"pod-template-hash": "abc123",
				},
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion: "apps/v1",
					Kind:       "ReplicaSet",
					Name:       "api-abc123",
				}},
			},
			Status: v1.PodStatus{Phase: v1.PodRunning},
		},
		&v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "api-abc123-two",
				Namespace: ns,
				Labels: map[string]string{
					"app":               "api",
					"pod-template-hash": "abc123",
				},
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion: "apps/v1",
					Kind:       "ReplicaSet",
					Name:       "api-abc123",
				}},
			},
			Status: v1.PodStatus{Phase: v1.PodRunning},
		},
	)

	values := make([]model.SamplePair, 12)
	startTime := time.Now().Add(-24 * time.Hour)
	for i := range values {
		values[i] = model.SamplePair{
			Timestamp: model.TimeFromUnix(startTime.Add(time.Duration(i*2) * time.Hour).Unix()),
			Value:     model.SampleValue(100 * 1024 * 1024),
		}
	}

	cfg := &config.Config{
		RightSizingEnabled:       true,
		RightSizingLookback:      24 * time.Hour,
		RightSizingPeakThreshold: 0.35,
		RightSizingSafetyFactor:  1.5,
		RightSizingMinSavings:    0.01,
		Mode:                     config.DryRun,
		PrivacySendGitToAI:       false,
	}
	history := newHistoryCache(cfg)
	ctrl := &Controller{
		clientset: clientset,
		promClient: &mockMetricsProvider{
			request:    4 * 1024 * 1024 * 1024,
			limit:      4 * 1024 * 1024 * 1024,
			cpuRequest: 1,
			historical: model.Matrix{&model.SampleStream{Values: values}},
		},
		config:  cfg,
		history: history,
	}

	ctrl.scanForRightSizing()

	got, ok := history.Get(context.Background(), ns, "api")
	if !ok {
		t.Fatalf("expected right-sizing incident to be recorded against owner workload")
	}
	if len(got.Incidents) != 1 {
		t.Fatalf("incidents = %d, want 1 owner-level opportunity", len(got.Incidents))
	}
	if got.Incidents[0].Reason != "RightSizingOpportunity" {
		t.Fatalf("reason = %q, want RightSizingOpportunity", got.Incidents[0].Reason)
	}
	if _, ok := history.Get(context.Background(), ns, "api-abc123-two"); ok {
		t.Fatalf("second replica should not create a pod-level right-sizing incident")
	}
}
