package analyzer

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// GetRootOwner recursively follows OwnerReferences to find the top-level controller (e.g. Deployment, StatefulSet).
func GetRootOwner(ctx context.Context, client kubernetes.Interface, namespace string, meta metav1.ObjectMeta) (string, string) {
	if len(meta.OwnerReferences) == 0 {
		return "", ""
	}

	owner := meta.OwnerReferences[0]

	// Stop recursion at known top-level controllers or if we can't find the parent
	switch owner.Kind {
	case "ReplicaSet":
		rs, err := client.AppsV1().ReplicaSets(namespace).Get(ctx, owner.Name, metav1.GetOptions{})
		if err == nil {
			parentKind, parentName := GetRootOwner(ctx, client, namespace, rs.ObjectMeta)
			if parentKind != "" {
				return parentKind, parentName
			}
			return "ReplicaSet", rs.Name
		}
	case "Deployment":
		return "Deployment", owner.Name
	case "StatefulSet":
		return "StatefulSet", owner.Name
	case "DaemonSet":
		return "DaemonSet", owner.Name
	case "Job":
		job, err := client.BatchV1().Jobs(namespace).Get(ctx, owner.Name, metav1.GetOptions{})
		if err == nil {
			parentKind, parentName := GetRootOwner(ctx, client, namespace, job.ObjectMeta)
			if parentKind != "" {
				return parentKind, parentName
			}
			return "Job", job.Name
		}
	case "CronJob":
		return "CronJob", owner.Name
	}

	return owner.Kind, owner.Name
}

func hasAcceptedCondition(conditions []interface{}) bool {
	for _, raw := range conditions {
		condition, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		condType, _ := condition["type"].(string)
		status, _ := condition["status"].(string)
		if condType == "Accepted" && status == "True" {
			return true
		}
	}
	return false
}
