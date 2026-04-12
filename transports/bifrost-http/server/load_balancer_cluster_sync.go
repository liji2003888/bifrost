package server

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/maximhq/bifrost/transports/bifrost-http/enterprise"
	"github.com/maximhq/bifrost/transports/bifrost-http/lib"
	"github.com/maximhq/bifrost/transports/bifrost-http/loadbalancer"
)

const (
	clusterAdaptiveRoutingStatusEndpoint = "/_cluster/adaptive-routing/status"
	clusterLoadBalancerSyncInterval      = 5 * time.Second
)

type clusterAdaptiveRoutingSyncer struct {
	cluster  *enterprise.ClusterService
	plugin   func() *loadbalancer.Plugin
	interval time.Duration

	mu            sync.Mutex
	lastRemoteIDs map[string]struct{}
}

func newClusterAdaptiveRoutingSyncer(server *BifrostHTTPServer) *clusterAdaptiveRoutingSyncer {
	if server == nil || server.ClusterService == nil || server.Config == nil {
		return nil
	}
	return &clusterAdaptiveRoutingSyncer{
		cluster:  server.ClusterService,
		interval: clusterLoadBalancerSyncInterval,
		plugin: func() *loadbalancer.Plugin {
			plugin, _ := lib.FindPluginAs[*loadbalancer.Plugin](server.Config, loadbalancer.PluginName)
			return plugin
		},
		lastRemoteIDs: make(map[string]struct{}),
	}
}

func (s *clusterAdaptiveRoutingSyncer) Start(ctx context.Context) {
	if s == nil || s.cluster == nil || s.plugin == nil || ctx == nil {
		return
	}
	go s.loop(ctx)
}

func (s *clusterAdaptiveRoutingSyncer) loop(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.runOnce(ctx)
		case <-ctx.Done():
			return
		}
	}
}

func (s *clusterAdaptiveRoutingSyncer) runOnce(ctx context.Context) {
	plugin := s.plugin()
	if plugin == nil || !plugin.Enabled() {
		s.pruneAll()
		return
	}

	peers := s.cluster.PeerStatuses()
	liveRemoteIDs := make(map[string]struct{}, len(peers))
	for _, peer := range peers {
		address := strings.TrimSpace(peer.Address)
		if address == "" {
			continue
		}

		requestCtx := ctx
		var cancel context.CancelFunc
		if requestCtx == nil {
			requestCtx = context.Background()
		}
		if _, hasDeadline := requestCtx.Deadline(); !hasDeadline {
			requestCtx, cancel = context.WithTimeout(requestCtx, 10*time.Second)
		}

		var remote struct {
			NodeID     string                         `json:"node_id,omitempty"`
			Routes     []loadbalancer.RouteStatus     `json:"routes"`
			Directions []loadbalancer.DirectionStatus `json:"directions"`
		}
		err := s.cluster.GetJSON(requestCtx, address, clusterAdaptiveRoutingStatusEndpoint, &remote)
		if cancel != nil {
			cancel()
		}
		if err != nil {
			continue
		}

		nodeID := strings.TrimSpace(remote.NodeID)
		if nodeID == "" {
			nodeID = strings.TrimSpace(peer.NodeID)
		}
		if nodeID == "" {
			nodeID = address
		}
		liveRemoteIDs[nodeID] = struct{}{}
		plugin.UpdateRemoteSnapshots(nodeID, remote.Routes, remote.Directions)
	}

	s.pruneMissing(plugin, liveRemoteIDs)
}

func (s *clusterAdaptiveRoutingSyncer) pruneMissing(plugin *loadbalancer.Plugin, liveRemoteIDs map[string]struct{}) {
	if s == nil || plugin == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for nodeID := range s.lastRemoteIDs {
		if _, ok := liveRemoteIDs[nodeID]; ok {
			continue
		}
		plugin.PruneRemoteNode(nodeID)
	}
	s.lastRemoteIDs = liveRemoteIDs
}

func (s *clusterAdaptiveRoutingSyncer) pruneAll() {
	if s == nil {
		return
	}
	plugin := s.plugin()
	if plugin == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for nodeID := range s.lastRemoteIDs {
		plugin.PruneRemoteNode(nodeID)
	}
	s.lastRemoteIDs = make(map[string]struct{})
}
