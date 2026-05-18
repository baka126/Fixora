package analyzer

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type IngressAnalyzer struct{}

func (i *IngressAnalyzer) Analyze(ctx Context) ([]Result, error) {
	list, err := ctx.Client.NetworkingV1().Ingresses(ctx.Namespace).List(ctx.Context, metav1.ListOptions{
		LabelSelector: ctx.LabelSelector,
	})
	if err != nil {
		return nil, err
	}

	var results []Result
	for _, ing := range list.Items {
		var failures []string
		var related []string

		// Check for Ingress Class
		if ing.Spec.IngressClassName == nil {
			if _, ok := ing.Annotations["kubernetes.io/ingress.class"]; !ok {
				failures = append(failures, "Ingress does not specify an IngressClass.")
			}
		}

		// Check Backends
		if ing.Spec.DefaultBackend != nil {
			if ing.Spec.DefaultBackend.Service != nil {
				svcName := ing.Spec.DefaultBackend.Service.Name
				if !i.serviceExists(ctx, ing.Namespace, svcName) {
					failures = append(failures, fmt.Sprintf("Default backend service %s does not exist.", svcName))
				} else {
					related = append(related, "Service/"+svcName)
				}
			}
		}

		for _, rule := range ing.Spec.Rules {
			if rule.HTTP == nil {
				continue
			}
			for _, path := range rule.HTTP.Paths {
				if path.Backend.Service != nil {
					svcName := path.Backend.Service.Name
					if !i.serviceExists(ctx, ing.Namespace, svcName) {
						failures = append(failures, fmt.Sprintf("Service %s referenced in rules does not exist.", svcName))
					} else {
						related = append(related, "Service/"+svcName)
					}
				}
			}
		}

		if len(failures) > 0 {
			results = append(results, Result{
				Kind:          "Ingress",
				Name:          ing.Name,
				Namespace:     ing.Namespace,
				Symptom:       "Ingress routing issues detected",
				Category:      CategoryNetwork,
				LikelyCause:   "The Ingress configuration refers to missing services or lacks an IngressClass.",
				Confidence:    85,
				PatchStrategy: PatchServiceSelector, // Or PatchConfig
				Evidence:      failures,
				Related:       related,
			})
		}
	}

	return results, nil
}

func (i *IngressAnalyzer) serviceExists(ctx Context, namespace, name string) bool {
	_, err := ctx.Client.CoreV1().Services(namespace).Get(ctx.Context, name, metav1.GetOptions{})
	return err == nil
}
