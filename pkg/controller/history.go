package controller

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

type Incident struct {
	Timestamp  time.Time `json:"timestamp"`
	Reason     string    `json:"reason"`
	RootCause  string    `json:"root_cause"`
	AppliedFix string    `json:"applied_fix,omitempty"` // The patch we generated
}

type PodHistory struct {
	Incidents []Incident `json:"incidents"`
}

type historyCache struct {
	history    map[string]*PodHistory // key: namespace/podname
	mu         sync.RWMutex
	filePath   string
	crdEnabled bool
	dynClient  dynamic.Interface
}

var incidentHistoryGVR = schema.GroupVersionResource{
	Group:    "fixora.io",
	Version:  "v1alpha1",
	Resource: "incidenthistories",
}

func newHistoryCache(filePath string, crdEnabled bool, dynClient dynamic.Interface) *historyCache {
	h := &historyCache{
		history:    make(map[string]*PodHistory),
		filePath:   filePath,
		crdEnabled: crdEnabled,
		dynClient:  dynClient,
	}
	h.load()
	return h
}

func (h *historyCache) load() {
	if h.filePath == "" {
		return
	}
	data, err := os.ReadFile(h.filePath)
	if err != nil {
		if !os.IsNotExist(err) {
			slog.Warn("Failed to read history file", "error", err)
		}
		return
	}
	if err := json.Unmarshal(data, &h.history); err != nil {
		slog.Warn("Failed to unmarshal history file", "error", err)
	}
}

func (h *historyCache) saveLocal() {
	if h.filePath == "" {
		return
	}
	data, err := json.MarshalIndent(h.history, "", "  ")
	if err != nil {
		slog.Warn("Failed to marshal history data", "error", err)
		return
	}
	if err := os.WriteFile(h.filePath, data, 0644); err != nil {
		slog.Warn("Failed to write history file", "error", err)
	}
}

func (h *historyCache) getFromCRD(ctx context.Context, namespace, podName string) (*PodHistory, bool) {
	if h.dynClient == nil {
		return nil, false
	}
	unstruct, err := h.dynClient.Resource(incidentHistoryGVR).Namespace(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return nil, false
	}

	spec, found, err := unstructured.NestedMap(unstruct.Object, "spec")
	if !found || err != nil {
		return nil, false
	}

	incidentsRaw, found, err := unstructured.NestedSlice(spec, "incidents")
	if !found || err != nil {
		return nil, false
	}

	var incidents []Incident
	// Best effort mapping from unstructured slice to our struct
	bytes, _ := json.Marshal(incidentsRaw)
	json.Unmarshal(bytes, &incidents)

	if len(incidents) > 0 {
		return &PodHistory{Incidents: incidents}, true
	}

	return nil, false
}

func (h *historyCache) saveToCRD(ctx context.Context, namespace, podName string, ph *PodHistory) {
	if h.dynClient == nil {
		return
	}
	// Convert incidents to unstruct
	bytes, _ := json.Marshal(ph.Incidents)
	var incidentsRaw []interface{}
	json.Unmarshal(bytes, &incidentsRaw)

	unstruct, err := h.dynClient.Resource(incidentHistoryGVR).Namespace(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		// Create new
		newCRD := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "fixora.io/v1alpha1",
				"kind":       "IncidentHistory",
				"metadata": map[string]interface{}{
					"name":      podName,
					"namespace": namespace,
				},
				"spec": map[string]interface{}{
					"incidents": incidentsRaw,
				},
			},
		}
		_, err = h.dynClient.Resource(incidentHistoryGVR).Namespace(namespace).Create(ctx, newCRD, metav1.CreateOptions{})
		if err != nil {
			slog.Error("Failed to create IncidentHistory CRD", "namespace", namespace, "name", podName, "error", err)
		}
	} else {
		// Update existing
		unstructured.SetNestedSlice(unstruct.Object, incidentsRaw, "spec", "incidents")
		_, err = h.dynClient.Resource(incidentHistoryGVR).Namespace(namespace).Update(ctx, unstruct, metav1.UpdateOptions{})
		if err != nil {
			slog.Error("Failed to update IncidentHistory CRD", "namespace", namespace, "name", podName, "error", err)
		}
	}
}

func (h *historyCache) Get(ctx context.Context, namespace, podName string) (*PodHistory, bool) {
	if h.crdEnabled {
		if ph, ok := h.getFromCRD(ctx, namespace, podName); ok {
			return ph, true
		}
	}

	h.mu.RLock()
	defer h.mu.RUnlock()
	prev, exists := h.history[namespace+"/"+podName]
	if exists && len(prev.Incidents) > 0 {
		return prev, true
	}
	return nil, false
}

func (h *historyCache) Update(ctx context.Context, namespace, podName, reason, rootCause string) {
	h.mu.Lock()
	key := namespace + "/" + podName

	inc := Incident{
		Timestamp: time.Now(),
		Reason:    reason,
		RootCause: rootCause,
	}

	var ph *PodHistory
	if prev, exists := h.history[key]; exists {
		prev.Incidents = append(prev.Incidents, inc)
		ph = prev
	} else {
		ph = &PodHistory{
			Incidents: []Incident{inc},
		}
		h.history[key] = ph
	}
	h.mu.Unlock()

	if h.crdEnabled {
		h.saveToCRD(ctx, namespace, podName, ph)
	} else {
		h.mu.Lock()
		h.saveLocal()
		h.mu.Unlock()
	}
}

func (h *historyCache) UpdatePatch(ctx context.Context, namespace, podName, patch string) {
	h.mu.Lock()
	key := namespace + "/" + podName
	var ph *PodHistory

	if prev, exists := h.history[key]; exists {
		if len(prev.Incidents) > 0 {
			prev.Incidents[len(prev.Incidents)-1].AppliedFix = patch
			ph = prev
		}
	}
	h.mu.Unlock()

	if ph != nil {
		if h.crdEnabled {
			h.saveToCRD(ctx, namespace, podName, ph)
		} else {
			h.mu.Lock()
			h.saveLocal()
			h.mu.Unlock()
		}
	}
}
