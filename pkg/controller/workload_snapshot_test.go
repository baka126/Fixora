package controller

import "testing"

func TestWorkloadSnapshotRegressionReasonDetectsReplicaDrop(t *testing.T) {
	before := WorkloadRolloutSnapshot{Namespace: "default", Kind: "Deployment", Name: "api", Ready: 3, Available: 3, Pods: 3}
	after := WorkloadRolloutSnapshot{Namespace: "default", Kind: "Deployment", Name: "api", Ready: 0, Available: 0, Pods: 1}

	if got := workloadSnapshotRegressionReason(before, after); got == "" {
		t.Fatalf("expected regression reason")
	}
}

func TestWorkloadSnapshotRegressionIgnoresDifferentWorkload(t *testing.T) {
	before := WorkloadRolloutSnapshot{Namespace: "default", Kind: "Deployment", Name: "api", Ready: 3}
	after := WorkloadRolloutSnapshot{Namespace: "default", Kind: "Deployment", Name: "worker", Ready: 0}

	if got := workloadSnapshotRegressionReason(before, after); got != "" {
		t.Fatalf("regression reason = %q, want empty", got)
	}
}
