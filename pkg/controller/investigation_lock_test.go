package controller

import (
	"context"
	"testing"
	"time"

	"fixora/pkg/config"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestDiagnosticLockNameIncludesFailureReason(t *testing.T) {
	pod := &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "oom-test-5"}}
	identity := workloadIdentity{Kind: "Pod", Name: "oom-test-5"}

	archLock := diagnosticLockName(pod, identity, "ContainerCannotRun: exec format error")
	oomLock := diagnosticLockName(pod, identity, "OOMKilled")

	if archLock == oomLock {
		t.Fatalf("expected different lock keys for different failure modes, got %q", archLock)
	}
	if archLock != "pod-oom-test-5/image-architecture" {
		t.Fatalf("unexpected architecture lock key %q", archLock)
	}
	if oomLock != "pod-oom-test-5/oomkilled" {
		t.Fatalf("unexpected oom lock key %q", oomLock)
	}
}

func TestDiagnosticLockNameNormalizesWatcherAndAlertmanagerCrashLoop(t *testing.T) {
	pod := &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "api-abc", Namespace: "default"}}
	identity := workloadIdentity{Kind: "Deployment", Name: "api"}

	watcherLock := diagnosticLockName(pod, identity, "CrashLoopBackOff")
	alertLock := diagnosticLockName(pod, identity, "KubePodCrashLooping")

	if watcherLock != alertLock {
		t.Fatalf("expected watcher and Alertmanager crashloop locks to match, got %q and %q", watcherLock, alertLock)
	}
	if watcherLock != "deployment-api/crashloop" {
		t.Fatalf("unexpected crashloop lock key %q", watcherLock)
	}
}

func TestDiagnosticLockNameUsesDeploymentInsteadOfReplicaSetForOwnedPods(t *testing.T) {
	podA := &v1.Pod{ObjectMeta: metav1.ObjectMeta{
		Name:      "api-7f9c9d6b6f-a",
		Namespace: "default",
		Labels:    map[string]string{"app": "api", "pod-template-hash": "7f9c9d6b6f"},
		OwnerReferences: []metav1.OwnerReference{{
			Kind: "ReplicaSet",
			Name: "api-7f9c9d6b6f",
		}},
	}}
	podB := podA.DeepCopy()
	podB.Name = "api-7f9c9d6b6f-b"
	client := fake.NewSimpleClientset(
		podA,
		podB,
		&appsv1.ReplicaSet{ObjectMeta: metav1.ObjectMeta{
			Name:      "api-7f9c9d6b6f",
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{{
				Kind: "Deployment",
				Name: "api",
			}},
		}},
	)
	ctrl := &Controller{clientset: client}

	identityA := ctrl.workloadIdentityForPod(context.Background(), podA)
	identityB := ctrl.workloadIdentityForPod(context.Background(), podB)
	lockA := diagnosticLockName(podA, identityA, "CrashLoopBackOff")
	lockB := diagnosticLockName(podB, identityB, "KubePodCrashLooping")

	if identityA.Kind != "Deployment" || identityB.Kind != "Deployment" {
		t.Fatalf("expected Deployment identity, got %#v and %#v", identityA, identityB)
	}
	if lockA != lockB {
		t.Fatalf("expected same workload/scenario lock, got %q and %q", lockA, lockB)
	}
}

func TestPodDiagnosticReasonOverridesHighLevelAlertName(t *testing.T) {
	pod := &v1.Pod{
		Status: v1.PodStatus{
			Phase: v1.PodRunning,
			ContainerStatuses: []v1.ContainerStatus{{
				Name: "api",
				State: v1.ContainerState{Waiting: &v1.ContainerStateWaiting{
					Reason: "CrashLoopBackOff",
				}},
			}},
		},
	}

	if got := firstNonEmpty(podDiagnosticReason(pod), "KubeDeploymentReplicasMismatch"); got != "CrashLoopBackOff" {
		t.Fatalf("expected live pod failure reason to win over high-level alert name, got %q", got)
	}
}

func TestInvestigationCooldownDefaultsAndConfig(t *testing.T) {
	if got := (&Controller{}).investigationCooldown(); got != 12*time.Hour {
		t.Fatalf("default investigation cooldown = %s, want 12h", got)
	}

	ctrl := &Controller{config: &config.Config{
		InvestigationCooldown:   4 * time.Hour,
		AlertmanagerDedupWindow: 45 * time.Minute,
	}}
	if got := ctrl.investigationCooldown(); got != 4*time.Hour {
		t.Fatalf("configured investigation cooldown = %s, want 4h", got)
	}
	if got := ctrl.alertmanagerDedupWindow(); got != 45*time.Minute {
		t.Fatalf("configured alertmanager dedup = %s, want 45m", got)
	}

	ctrl.config.AlertmanagerDedupWindow = 0
	if got := ctrl.alertmanagerDedupWindow(); got != 4*time.Hour {
		t.Fatalf("alertmanager dedup fallback = %s, want investigation cooldown", got)
	}
}
