package analyzer

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ConfigMapAnalyzer struct{}

func (c *ConfigMapAnalyzer) Analyze(ctx Context) ([]Result, error) {
	cms, err := ctx.Client.CoreV1().ConfigMaps(ctx.Namespace).List(ctx.Context, metav1.ListOptions{
		LabelSelector: ctx.LabelSelector,
	})
	if err != nil {
		return nil, err
	}

	pods, err := ctx.Client.CoreV1().Pods(ctx.Namespace).List(ctx.Context, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	usedCMs := make(map[string]bool)
	for _, pod := range pods.Items {
		for _, vol := range pod.Spec.Volumes {
			if vol.ConfigMap != nil {
				usedCMs[vol.ConfigMap.Name] = true
			}
		}
		for _, container := range pod.Spec.Containers {
			for _, envFrom := range container.EnvFrom {
				if envFrom.ConfigMapRef != nil {
					usedCMs[envFrom.ConfigMapRef.Name] = true
				}
			}
			for _, env := range container.Env {
				if env.ValueFrom != nil && env.ValueFrom.ConfigMapKeyRef != nil {
					usedCMs[env.ValueFrom.ConfigMapKeyRef.Name] = true
				}
			}
		}
	}

	var results []Result
	for _, cm := range cms.Items {
		if !usedCMs[cm.Name] {
			// Orphaned ConfigMap
			results = append(results, Result{
				Kind:          "ConfigMap",
				Name:          cm.Name,
				Namespace:     cm.Namespace,
				Symptom:       "Unused ConfigMap detected",
				Category:      CategoryConfig,
				LikelyCause:   "The ConfigMap is not referenced by any Pods in the current namespace.",
				Confidence:    60,
				PatchStrategy: PatchNone,
				Evidence:      []string{fmt.Sprintf("ConfigMap %s is not used by any pods.", cm.Name)},
			})
		}
	}

	return results, nil
}
