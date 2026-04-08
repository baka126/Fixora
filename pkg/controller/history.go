package controller

import (
	"encoding/json"
	"log/slog"
	"os"
	"sync"
	"time"
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
	history  map[string]*PodHistory // key: namespace/podname
	mu       sync.RWMutex
	filePath string
}

func newHistoryCache(filePath string) *historyCache {
	h := &historyCache{
		history:  make(map[string]*PodHistory),
		filePath: filePath,
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

func (h *historyCache) save() {
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

func (h *historyCache) Get(namespace, podName string) (*PodHistory, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	prev, exists := h.history[namespace+"/"+podName]
	if exists && len(prev.Incidents) > 0 {
		return prev, true
	}
	return nil, false
}

func (h *historyCache) Update(namespace, podName, reason, rootCause string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	key := namespace + "/" + podName

	inc := Incident{
		Timestamp: time.Now(),
		Reason:    reason,
		RootCause: rootCause,
	}

	if prev, exists := h.history[key]; exists {
		prev.Incidents = append(prev.Incidents, inc)
	} else {
		h.history[key] = &PodHistory{
			Incidents: []Incident{inc},
		}
	}
	h.save()
}

func (h *historyCache) UpdatePatch(namespace, podName, patch string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	key := namespace + "/" + podName
	if prev, exists := h.history[key]; exists {
		if len(prev.Incidents) > 0 {
			prev.Incidents[len(prev.Incidents)-1].AppliedFix = patch
			h.save()
		}
	}
}
