package controller

import (
	"context"
	"log/slog"
	"time"

	v1 "k8s.io/api/core/v1"
)

const (
	worldSnapshotCacheTTL     = 30 * time.Second
	worldSnapshotRefreshLimit = 8 * time.Second
)

func (c *Controller) CachedWorldSnapshot(ctx context.Context) (*WorldSnapshot, []v1.Pod) {
	if c == nil {
		return nil, nil
	}
	c.worldCacheMu.RLock()
	cached := c.worldCache
	cachedPods := append([]v1.Pod(nil), c.worldPods...)
	cachedAt := c.worldCacheAt
	refreshing := c.worldRefreshing
	c.worldCacheMu.RUnlock()

	if cached != nil && time.Since(cachedAt) < worldSnapshotCacheTTL {
		return cached, cachedPods
	}

	if !refreshing {
		c.startWorldSnapshotRefresh()
	}

	if cached != nil {
		return cached, cachedPods
	}
	cluster := c.dashboardEnvironment(ctx)
	return &WorldSnapshot{
		GeneratedAt: time.Now(),
		Cluster:     cluster,
		Workloads:   map[string]*WorldWorkload{},
		Pods:        map[string]*WorldPod{},
		Services:    map[string]*WorldService{},
		Ingresses:   map[string]*WorldIngress{},
		Nodes:       map[string]*WorldNode{},
	}, nil
}

func (c *Controller) startWorldSnapshotRefresh() {
	c.worldCacheMu.Lock()
	if c.worldRefreshing {
		c.worldCacheMu.Unlock()
		return
	}
	c.worldRefreshing = true
	c.worldCacheMu.Unlock()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), worldSnapshotRefreshLimit)
		defer cancel()

		snapshot, pods := c.BuildWorldSnapshot(ctx)
		c.worldCacheMu.Lock()
		c.worldCache = snapshot
		c.worldPods = append([]v1.Pod(nil), pods...)
		c.worldCacheAt = time.Now()
		c.worldRefreshing = false
		c.worldCacheMu.Unlock()

		if ctx.Err() != nil {
			slog.Warn("World snapshot refresh exceeded dashboard budget", "error", ctx.Err())
		}
	}()
}
