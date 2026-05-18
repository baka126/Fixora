package analyzer

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type WebhookAnalyzer struct{}

func (w *WebhookAnalyzer) Analyze(ctx Context) ([]Result, error) {
	var results []Result

	// Validating Webhooks
	vwList, err := ctx.Client.AdmissionregistrationV1().ValidatingWebhookConfigurations().List(ctx.Context, metav1.ListOptions{
		LabelSelector: ctx.LabelSelector,
	})
	if err == nil {
		for _, vw := range vwList.Items {
			var failures []string
			var related []string
			for _, wh := range vw.Webhooks {
				if wh.ClientConfig.Service != nil {
					svc := wh.ClientConfig.Service
					_, err := ctx.Client.CoreV1().Services(svc.Namespace).Get(ctx.Context, svc.Name, metav1.GetOptions{})
					if err != nil {
						failures = append(failures, fmt.Sprintf("Webhook %s references non-existent service %s/%s", wh.Name, svc.Namespace, svc.Name))
					} else {
						related = append(related, fmt.Sprintf("Service/%s/%s", svc.Namespace, svc.Name))
					}
				}
			}
			if len(failures) > 0 {
				results = append(results, Result{
					Kind:          "ValidatingWebhookConfiguration",
					Name:          vw.Name,
					Namespace:     "", // Cluster-scoped
					Symptom:       "Webhook configuration error",
					Category:      CategoryConfig,
					LikelyCause:   "The admission webhook references a service that does not exist, which may cause API requests to fail or hang.",
					Confidence:    90,
					PatchStrategy: PatchNone,
					Evidence:      failures,
					Related:       related,
				})
			}
		}
	}

	// Mutating Webhooks
	mwList, err := ctx.Client.AdmissionregistrationV1().MutatingWebhookConfigurations().List(ctx.Context, metav1.ListOptions{
		LabelSelector: ctx.LabelSelector,
	})
	if err == nil {
		for _, mw := range mwList.Items {
			var failures []string
			var related []string
			for _, wh := range mw.Webhooks {
				if wh.ClientConfig.Service != nil {
					svc := wh.ClientConfig.Service
					_, err := ctx.Client.CoreV1().Services(svc.Namespace).Get(ctx.Context, svc.Name, metav1.GetOptions{})
					if err != nil {
						failures = append(failures, fmt.Sprintf("Webhook %s references non-existent service %s/%s", wh.Name, svc.Namespace, svc.Name))
					} else {
						related = append(related, fmt.Sprintf("Service/%s/%s", svc.Namespace, svc.Name))
					}
				}
			}
			if len(failures) > 0 {
				results = append(results, Result{
					Kind:          "MutatingWebhookConfiguration",
					Name:          mw.Name,
					Namespace:     "", // Cluster-scoped
					Symptom:       "Webhook configuration error",
					Category:      CategoryConfig,
					LikelyCause:   "The mutation webhook references a service that does not exist, which may cause resource creation/updates to fail.",
					Confidence:    90,
					PatchStrategy: PatchNone,
					Evidence:      failures,
					Related:       related,
				})
			}
		}
	}

	return results, nil
}
