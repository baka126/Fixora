package ai

import "fmt"

const (
	PromptTypeDefault    = "default"
	PromptTypeScheduling = "scheduling"
	PromptTypeNetwork    = "network"
	PromptTypeStorage    = "storage"
	PromptTypeConfig     = "config"
	PromptTypeRuntime    = "runtime"
)

var promptTemplates = map[string]string{
	PromptTypeDefault: `You are a Kubernetes forensic expert. Analyze failure for pod %s/%s.
Reason: %s
Metrics: %s
Events: %s
Logs: %s
Past History:
%s

Provide a clear, 3-sentence summary: 1. Root Cause, 2. Proof, 3. Recommended fix.
If Metrics state that historical trends are unavailable, focus your analysis on Logs and Events.
Use Past History to run predictive analysis, give future prediction, and offer a long-term solution if this is a recurring issue.

You MUST respond with a JSON object containing: 'analysis' (the summary) and 'confidence' (a percentage from 0 to 100 representing your certainty).`,

	PromptTypeScheduling: `You are a Kubernetes Scheduler expert. A pod is stuck in Pending state or failing to schedule.
Pod: %s/%s
Reason: %s
Cluster Metrics/Capacity: %s
Relevant Events: %s
Past History: %s

Analyze why the scheduler cannot place this pod. Focus on Taints, Tolerations, Node Selectors, Affinity rules, or Resource Quotas.
Provide a clear, 3-sentence summary: 1. Bottleneck, 2. Constraint found, 3. Recommended fix (e.g. adding a node, updating tolerations).

You MUST respond with a JSON object containing: 'analysis' (the summary) and 'confidence' (a percentage from 0 to 100 representing your certainty).`,

	PromptTypeNetwork: `You are a Kubernetes Networking expert (CNI, Services, Ingress, Gateway API).
Resource: %s/%s
Reason: %s
Network Metrics/Connectivity: %s
Networking Events: %s
Logs (if any): %s

Analyze the connectivity failure. Check Service selectors, Endpoints, NetworkPolicies, Ingress rules, or DNS issues.
Provide a clear, 3-sentence summary: 1. Traffic Blockage point, 2. Misconfiguration found, 3. Recommended fix.

You MUST respond with a JSON object containing: 'analysis' (the summary) and 'confidence' (a percentage from 0 to 100 representing your certainty).`,

	PromptTypeStorage: `You are a Kubernetes Storage expert (CSI, PVC, PV, StorageClass).
Resource: %s/%s
Reason: %s
Storage Events: %s
Logs (if any): %s

Analyze why the volume cannot be bound or mounted. Check StorageClass provisioners, AccessModes, VolumeBindingMode, or Cloud Provider permissions.
Provide a clear, 3-sentence summary: 1. Mounting/Binding failure cause, 2. Missing dependency, 3. Recommended fix.

You MUST respond with a JSON object containing: 'analysis' (the summary) and 'confidence' (a percentage from 0 to 100 representing your certainty).`,

	PromptTypeConfig: `You are a Kubernetes Configuration expert (Secrets, ConfigMaps, RBAC).
Resource: %s/%s
Reason: %s
Events: %s
Logs: %s

Analyze the configuration error. Check if a Secret/ConfigMap is missing, if a key is misnamed, or if there are RBAC permission issues (Permission Denied).
Provide a clear, 3-sentence summary: 1. Configuration gap, 2. Missing resource/key, 3. Recommended fix.

You MUST respond with a JSON object containing: 'analysis' (the summary) and 'confidence' (a percentage from 0 to 100 representing your certainty).`,
}

func GetForensicPrompt(promptType string, ctx ForensicContext) string {
	template, ok := promptTemplates[promptType]
	if !ok {
		template = promptTemplates[PromptTypeDefault]
	}

	switch promptType {
	case PromptTypeScheduling:
		return fmt.Sprintf(template, ctx.Namespace, ctx.PodName, ctx.Reason, ctx.Metrics, ctx.Events, ctx.History)
	case PromptTypeNetwork:
		return fmt.Sprintf(template, ctx.Namespace, ctx.PodName, ctx.Reason, ctx.Metrics, ctx.Events, ctx.Logs)
	case PromptTypeStorage:
		return fmt.Sprintf(template, ctx.Namespace, ctx.PodName, ctx.Reason, ctx.Events, ctx.Logs)
	case PromptTypeConfig:
		return fmt.Sprintf(template, ctx.Namespace, ctx.PodName, ctx.Reason, ctx.Events, ctx.Logs)
	default:
		return fmt.Sprintf(template, ctx.Namespace, ctx.PodName, ctx.Reason, ctx.Metrics, ctx.Events, ctx.Logs, ctx.History)
	}
}
