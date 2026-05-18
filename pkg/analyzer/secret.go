package analyzer

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type SecretAnalyzer struct{}

func (s *SecretAnalyzer) Analyze(ctx Context) ([]Result, error) {
	secrets, err := ctx.Client.CoreV1().Secrets(ctx.Namespace).List(ctx.Context, metav1.ListOptions{
		LabelSelector: ctx.LabelSelector,
	})
	if err != nil {
		return nil, err
	}

	pods, err := ctx.Client.CoreV1().Pods(ctx.Namespace).List(ctx.Context, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	usedSecrets := make(map[string]bool)
	for _, pod := range pods.Items {
		for _, vol := range pod.Spec.Volumes {
			if vol.Secret != nil {
				usedSecrets[vol.Secret.SecretName] = true
			}
		}
		for _, container := range pod.Spec.Containers {
			for _, envFrom := range container.EnvFrom {
				if envFrom.SecretRef != nil {
					usedSecrets[envFrom.SecretRef.Name] = true
				}
			}
			for _, env := range container.Env {
				if env.ValueFrom != nil && env.ValueFrom.SecretKeyRef != nil {
					usedSecrets[env.ValueFrom.SecretKeyRef.Name] = true
				}
			}
		}
		for _, ips := range pod.Spec.ImagePullSecrets {
			usedSecrets[ips.Name] = true
		}
	}

	var results []Result
	for _, secret := range secrets.Items {
		if secret.Type == "kubernetes.io/service-account-token" {
			continue // Skip SA tokens
		}
		if !usedSecrets[secret.Name] {
			results = append(results, Result{
				Kind:          "Secret",
				Name:          secret.Name,
				Namespace:     secret.Namespace,
				Symptom:       "Unused Secret detected",
				Category:      CategoryConfig,
				LikelyCause:   "The Secret is not referenced by any Pods or ImagePullSecrets in the current namespace.",
				Confidence:    60,
				PatchStrategy: PatchNone,
				Evidence:      []string{fmt.Sprintf("Secret %s is not used by any pods.", secret.Name)},
			})
		}
	}

	return results, nil
}
