package controller

import (
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
