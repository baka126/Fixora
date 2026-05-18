package analyzer

import (
	"fmt"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type StorageAnalyzer struct{}

func (s *StorageAnalyzer) Analyze(ctx Context) ([]Result, error) {
	list, err := ctx.Client.CoreV1().PersistentVolumeClaims(ctx.Namespace).List(ctx.Context, metav1.ListOptions{
		LabelSelector: ctx.LabelSelector,
	})
	if err != nil {
		return nil, err
	}

	var results []Result
	for _, pvc := range list.Items {
		var failures []string

		if pvc.Status.Phase == v1.ClaimPending {
			failures = append(failures, fmt.Sprintf("PVC %s is in Pending state.", pvc.Name))

			// Check for storage class existence
			if pvc.Spec.StorageClassName != nil {
				_, err := ctx.Client.StorageV1().StorageClasses().Get(ctx.Context, *pvc.Spec.StorageClassName, metav1.GetOptions{})
				if err != nil {
					failures = append(failures, fmt.Sprintf("StorageClass %s does not exist.", *pvc.Spec.StorageClassName))
				}
			} else {
				failures = append(failures, "No StorageClass specified and no default StorageClass found or configured.")
			}

			// Fetch events for deeper context
			events, _ := ctx.Client.CoreV1().Events(pvc.Namespace).List(ctx.Context, metav1.ListOptions{
				FieldSelector: fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=PersistentVolumeClaim", pvc.Name),
			})
			for _, evt := range events.Items {
				if evt.Type == v1.EventTypeWarning {
					failures = append(failures, fmt.Sprintf("Event %s: %s", evt.Reason, evt.Message))
					break // Just the first warning for brevity
				}
			}
		}

		if len(failures) > 0 {
			results = append(results, Result{
				Kind:          "PersistentVolumeClaim",
				Name:          pvc.Name,
				Namespace:     pvc.Namespace,
				Symptom:       "Storage volume cannot be bound",
				Category:      CategoryStorage,
				LikelyCause:   "The PersistentVolumeClaim is pending because a matching volume cannot be provisioned or the StorageClass is missing.",
				Confidence:    85,
				PatchStrategy: PatchPVC,
				Evidence:      failures,
			})
		}
	}

	return results, nil
}
