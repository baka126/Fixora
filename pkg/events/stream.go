package events

import (
	"context"
	"database/sql"
	"log/slog"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type EventStreamer struct {
	clientset kubernetes.Interface
	db        *sql.DB
}

func NewEventStreamer(clientset kubernetes.Interface, db *sql.DB) *EventStreamer {
	return &EventStreamer{
		clientset: clientset,
		db:        db,
	}
}

func (e *EventStreamer) initDB() {
	if e.db == nil {
		return
	}
	// We build a dependency graph mapping target_kind/target_name to source_kind/source_name
	// E.g. Pod -> depends on -> Secret
	queries := []string{
		`CREATE TABLE IF NOT EXISTS dependency_graph (
			id SERIAL PRIMARY KEY,
			namespace VARCHAR(255) NOT NULL,
			target_kind VARCHAR(100) NOT NULL,
			target_name VARCHAR(255) NOT NULL,
			source_kind VARCHAR(100) NOT NULL,
			source_name VARCHAR(255) NOT NULL,
			UNIQUE(namespace, target_kind, target_name, source_kind, source_name)
		);`,
		`CREATE INDEX IF NOT EXISTS idx_dep_source ON dependency_graph (namespace, source_kind, source_name);`,
		`CREATE INDEX IF NOT EXISTS idx_dep_target ON dependency_graph (namespace, target_kind, target_name);`,
	}
	for _, q := range queries {
		_, err := e.db.Exec(q)
		if err != nil {
			slog.Error("Failed to initialize dependency_graph schema", "query", q, "error", err)
		}
	}
}

func (e *EventStreamer) Start(factory informers.SharedInformerFactory, stopCh <-chan struct{}) {
	e.initDB()

	eventInformer := factory.Core().V1().Events().Informer()
	eventInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			e.processEvent(obj.(*corev1.Event))
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			e.processEvent(newObj.(*corev1.Event))
		},
	})

	podInformer := factory.Core().V1().Pods().Informer()
	podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			e.processPod(obj.(*corev1.Pod))
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			e.processPod(newObj.(*corev1.Pod))
		},
		DeleteFunc: func(obj interface{}) {
			if pod, ok := obj.(*corev1.Pod); ok {
				e.removePod(pod)
			}
		},
	})

	slog.Info("Started EventStreamer for Real-time Dependency Graph mapping")
}

func (e *EventStreamer) processEvent(event *corev1.Event) {
	// Process relevant K8s events and potentially link them in the graph
	// For instance, if a pod is failing to mount a secret
	if event.Reason == "FailedMount" {
		// Log or trigger advanced analysis if needed
		slog.Debug("Detected FailedMount event", "namespace", event.InvolvedObject.Namespace, "pod", event.InvolvedObject.Name, "message", event.Message)
	}
}

func (e *EventStreamer) processPod(pod *corev1.Pod) {
	if e.db == nil {
		return
	}

	ctx := context.Background()
	namespace := pod.Namespace
	targetKind := "Pod"
	targetName := pod.Name

	// Map Secrets from Volumes
	for _, vol := range pod.Spec.Volumes {
		if vol.Secret != nil {
			e.upsertDependency(ctx, namespace, targetKind, targetName, "Secret", vol.Secret.SecretName)
		}
		if vol.ConfigMap != nil {
			e.upsertDependency(ctx, namespace, targetKind, targetName, "ConfigMap", vol.ConfigMap.Name)
		}
	}

	// Map EnvFrom
	for _, container := range pod.Spec.Containers {
		for _, envFrom := range container.EnvFrom {
			if envFrom.SecretRef != nil {
				e.upsertDependency(ctx, namespace, targetKind, targetName, "Secret", envFrom.SecretRef.Name)
			}
			if envFrom.ConfigMapRef != nil {
				e.upsertDependency(ctx, namespace, targetKind, targetName, "ConfigMap", envFrom.ConfigMapRef.Name)
			}
		}
		for _, env := range container.Env {
			if env.ValueFrom != nil && env.ValueFrom.SecretKeyRef != nil {
				e.upsertDependency(ctx, namespace, targetKind, targetName, "Secret", env.ValueFrom.SecretKeyRef.Name)
			}
			if env.ValueFrom != nil && env.ValueFrom.ConfigMapKeyRef != nil {
				e.upsertDependency(ctx, namespace, targetKind, targetName, "ConfigMap", env.ValueFrom.ConfigMapKeyRef.Name)
			}
		}
	}
}

func (e *EventStreamer) upsertDependency(ctx context.Context, namespace, targetKind, targetName, sourceKind, sourceName string) {
	query := `
		INSERT INTO dependency_graph (namespace, target_kind, target_name, source_kind, source_name)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (namespace, target_kind, target_name, source_kind, source_name) DO NOTHING
	`
	_, err := e.db.ExecContext(ctx, query, namespace, targetKind, targetName, sourceKind, sourceName)
	if err != nil {
		slog.Error("Failed to upsert dependency", "target", targetName, "source", sourceName, "error", err)
	}
}

func (e *EventStreamer) removePod(pod *corev1.Pod) {
	if e.db == nil {
		return
	}
	query := `DELETE FROM dependency_graph WHERE namespace = $1 AND target_kind = 'Pod' AND target_name = $2`
	_, err := e.db.ExecContext(context.Background(), query, pod.Namespace, pod.Name)
	if err != nil {
		slog.Error("Failed to remove pod dependencies", "pod", pod.Name, "error", err)
	}
}

// GetDependenciesForSource returns all target resources (e.g. Pods) that depend on a specific source (e.g. Secret)
func (e *EventStreamer) GetDependenciesForSource(ctx context.Context, namespace, sourceKind, sourceName string) ([]string, error) {
	if e.db == nil {
		return nil, nil
	}

	query := `SELECT target_name FROM dependency_graph WHERE namespace = $1 AND source_kind = $2 AND source_name = $3 AND target_kind = 'Pod'`
	rows, err := e.db.QueryContext(ctx, query, namespace, sourceKind, sourceName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var targets []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err == nil {
			targets = append(targets, name)
		}
	}
	return targets, nil
}
