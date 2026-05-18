package analyzer

import (
	"sync"
)

func GetAnalyzerMap() map[string]IAnalyzer {
	return map[string]IAnalyzer{
		"pod":           &PodAnalyzer{},
		"ingress":       &IngressAnalyzer{},
		"hpa":           &HpaAnalyzer{},
		"service":       &ServiceAnalyzer{},
		"storage":       &StorageAnalyzer{},
		"node":          &NodeAnalyzer{},
		"cronjob":       &CronJobAnalyzer{},
		"configmap":     &ConfigMapAnalyzer{},
		"networkpolicy": &NetworkPolicyAnalyzer{},
		"deployment":    &DeploymentAnalyzer{},
		"statefulset":   &StatefulSetAnalyzer{},
		"pdb":           &PDBAnalyzer{},
		"webhook":       &WebhookAnalyzer{},
		"daemonset":     &DaemonSetAnalyzer{},
		"job":           &JobAnalyzer{},
		"replicaset":    &ReplicaSetAnalyzer{},
		"gateway":       &GatewayAnalyzer{},
		"gatewayclass":  &GatewayClassAnalyzer{},
		"httproute":     &HTTPRouteAnalyzer{},
		"subscription":  &SubscriptionAnalyzer{},
		"csv":           &CSVAnalyzer{},
		"installplan":   &InstallPlanAnalyzer{},
		"catalogsource": &CatalogSourceAnalyzer{},
		"security":      &SecurityAnalyzer{},
		"secret":        &SecretAnalyzer{},
		"policy":        &PolicyAnalyzer{},
	}
}

func RunAllAnalyzers(ctx Context) ([]Result, error) {
	var allResults []Result
	var mutex sync.Mutex
	var wg sync.WaitGroup

	// Set a reasonable concurrency limit (e.g. 10) to avoid overwhelming the K8s API
	semaphore := make(chan struct{}, 10)

	for _, ana := range GetAnalyzerMap() {
		wg.Add(1)
		semaphore <- struct{}{}

		go func(a IAnalyzer) {
			defer wg.Done()
			defer func() { <-semaphore }()

			results, err := a.Analyze(ctx)
			if err != nil {
				// Log error but continue with other analyzers to ensure Omni-Awareness
				return
			}

			mutex.Lock()
			allResults = append(allResults, results...)
			mutex.Unlock()
		}(ana)
	}

	wg.Wait()
	return allResults, nil
}
