package controller

import (
	"sync"
	"time"
)

type incidentHistory struct {
	LastSeen  time.Time
	RootCause string
	Count     int
}

type historyCache struct {
	history map[string]*incidentHistory // key: namespace/podname
	mu      sync.RWMutex
}

func newHistoryCache() *historyCache {
	return &historyCache{
		history: make(map[string]*incidentHistory),
	}
}

func (h *historyCache) Get(namespace, podName string) (*incidentHistory, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	prev, exists := h.history[namespace+"/"+podName]
	return prev, exists
}

func (h *historyCache) Update(namespace, podName, rootCause string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	key := namespace + "/" + podName
	if prev, exists := h.history[key]; exists {
		prev.LastSeen = time.Now()
		prev.RootCause = rootCause
		prev.Count++
	} else {
		h.history[key] = &incidentHistory{
			LastSeen:  time.Now(),
			RootCause: rootCause,
			Count:     1,
		}
	}
}
