package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type WorkloadRolloutSnapshot struct {
	CapturedAt time.Time `json:"capturedAt"`
	Cluster    string    `json:"cluster"`
	Namespace  string    `json:"namespace"`
	Kind       string    `json:"kind"`
	Name       string    `json:"name"`
	Desired    int32     `json:"desired"`
	Ready      int32     `json:"ready"`
	Available  int32     `json:"available"`
	Updated    int32     `json:"updated"`
	Pods       int       `json:"pods"`
	Services   int       `json:"services"`
	Ingresses  int       `json:"ingresses"`
	Nodes      int       `json:"nodes"`
	Status     string    `json:"status"`
}

func (c *Controller) captureWorkloadSnapshot(ctx context.Context, namespace, kind, name string) (WorkloadRolloutSnapshot, bool) {
	world := c.BuildWorldSnapshot(ctx)
	workload := world.Workloads[worldID(world.Cluster, namespace, kind, name)]
	if workload == nil {
		return WorkloadRolloutSnapshot{}, false
	}
	return workloadRolloutSnapshot(world, workload), true
}

func workloadRolloutSnapshot(world *WorldSnapshot, workload *WorldWorkload) WorkloadRolloutSnapshot {
	return WorkloadRolloutSnapshot{
		CapturedAt: world.GeneratedAt,
		Cluster:    world.Cluster,
		Namespace:  workload.Namespace,
		Kind:       workload.Kind,
		Name:       workload.Name,
		Desired:    workload.Desired,
		Ready:      workload.Ready,
		Available:  workload.Available,
		Updated:    workload.Updated,
		Pods:       len(workload.Pods),
		Services:   len(workload.Services),
		Ingresses:  len(workload.Ingresses),
		Nodes:      len(workload.NodeNames),
		Status:     workload.Status,
	}
}

func workloadSnapshotRegressionReason(before, after WorkloadRolloutSnapshot) string {
	if before.Kind == "" || after.Kind == "" {
		return ""
	}
	if before.Namespace != after.Namespace || before.Kind != after.Kind || before.Name != after.Name {
		return ""
	}
	if before.Ready > 0 && after.Ready == 0 {
		return fmt.Sprintf("%s %s ready replicas dropped from %d to 0 after remediation", after.Kind, after.Name, before.Ready)
	}
	if before.Available > 0 && after.Available == 0 {
		return fmt.Sprintf("%s %s available replicas dropped from %d to 0 after remediation", after.Kind, after.Name, before.Available)
	}
	if before.Desired > 0 && after.Desired > before.Desired && after.Ready < before.Ready {
		return fmt.Sprintf("%s %s desired replicas increased from %d to %d while ready replicas dropped from %d to %d", after.Kind, after.Name, before.Desired, after.Desired, before.Ready, after.Ready)
	}
	if after.Pods == 0 && before.Pods > 0 {
		return fmt.Sprintf("%s %s lost all pods after remediation", after.Kind, after.Name)
	}
	return ""
}

func encodeWorkloadSnapshot(snapshot WorkloadRolloutSnapshot) []byte {
	data, _ := json.Marshal(snapshot)
	return data
}

func decodeWorkloadSnapshot(data []byte) (WorkloadRolloutSnapshot, error) {
	var snapshot WorkloadRolloutSnapshot
	err := json.Unmarshal(data, &snapshot)
	return snapshot, err
}
