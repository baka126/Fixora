package controller

import (
	"context"
	"encoding/json"
	"log/slog"
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
	crdEnabled bool
	dynClient  dynamic.Interface
}

var incidentHistoryGVR = schema.GroupVersionResource{
	Group:    "fixora.io",
	Version:  "v1alpha1",
	Resource: "incidenthistories",
}

func newHistoryCache(crdEnabled bool, dynClient dynamic.Interface) *historyCache {
	return &historyCache{
		crdEnabled: crdEnabled,
		dynClient:  dynClient,
	}
}

func (h *historyCache) getFromCRD(ctx context.Context, namespace, podName string) (*PodHistory, *unstructured.Unstructured, bool) {
	if h.dynClient == nil {
		return nil, nil, false
	}
	unstruct, err := h.dynClient.Resource(incidentHistoryGVR).Namespace(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return nil, nil, false
	}

	spec, found, err := unstructured.NestedMap(unstruct.Object, "spec")
	if !found || err != nil {
		return nil, unstruct, false
	}

	incidentsRaw, found, err := unstructured.NestedSlice(spec, "incidents")
	if !found || err != nil {
		return nil, unstruct, false
	}

	var incidents []Incident
	// Best effort mapping from unstructured slice to our struct
	bytes, _ := json.Marshal(incidentsRaw)
	json.Unmarshal(bytes, &incidents)

	if len(incidents) > 0 {
		return &PodHistory{Incidents: incidents}, unstruct, true
	}

	return nil, unstruct, false
}

func (h *historyCache) saveToCRD(ctx context.Context, namespace, podName string, ph *PodHistory, existing *unstructured.Unstructured) {
	if h.dynClient == nil {
		return
	}
	// Convert incidents to unstruct
	bytes, _ := json.Marshal(ph.Incidents)
	var incidentsRaw []interface{}
	json.Unmarshal(bytes, &incidentsRaw)

	if existing == nil {
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
		_, err := h.dynClient.Resource(incidentHistoryGVR).Namespace(namespace).Create(ctx, newCRD, metav1.CreateOptions{})
		if err != nil {
			slog.Error("Failed to create IncidentHistory CRD", "namespace", namespace, "name", podName, "error", err)
		}
	} else {
		// Update existing
		unstructured.SetNestedSlice(existing.Object, incidentsRaw, "spec", "incidents")
		_, err := h.dynClient.Resource(incidentHistoryGVR).Namespace(namespace).Update(ctx, existing, metav1.UpdateOptions{})
		if err != nil {
			slog.Error("Failed to update IncidentHistory CRD", "namespace", namespace, "name", podName, "error", err)
		}
	}
}

func (h *historyCache) Get(ctx context.Context, namespace, podName string) (*PodHistory, bool) {
	if !h.crdEnabled {
		return nil, false
	}
	ph, _, ok := h.getFromCRD(ctx, namespace, podName)
	return ph, ok
}

func (h *historyCache) Update(ctx context.Context, namespace, podName, reason, rootCause string) {
	if !h.crdEnabled {
		return
	}

	inc := Incident{
		Timestamp: time.Now(),
		Reason:    reason,
		RootCause: rootCause,
	}

	ph, unstruct, ok := h.getFromCRD(ctx, namespace, podName)
	if ok && ph != nil {
		ph.Incidents = append(ph.Incidents, inc)
	} else {
		ph = &PodHistory{
			Incidents: []Incident{inc},
		}
	}

	h.saveToCRD(ctx, namespace, podName, ph, unstruct)
}

func (h *historyCache) UpdatePatch(ctx context.Context, namespace, podName, patch string) {
	if !h.crdEnabled {
		return
	}

	ph, unstruct, ok := h.getFromCRD(ctx, namespace, podName)
	if ok && ph != nil && len(ph.Incidents) > 0 {
		ph.Incidents[len(ph.Incidents)-1].AppliedFix = patch
		h.saveToCRD(ctx, namespace, podName, ph, unstruct)
	}
}
