package enterprise

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/bytedance/sonic"
	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework/kvstore"
)

const (
	clusterStatusEndpoint = "/_cluster/status"
	clusterSetEndpoint    = "/_cluster/kv/set"
	clusterDeleteEndpoint = "/_cluster/kv/delete"
	ClusterAuthHeader     = "X-Bifrost-Cluster-Token"
)

type ClusterMutation struct {
	Key       string `json:"key"`
	ValueJSON []byte `json:"value_json,omitempty"`
	WrittenAt int64  `json:"written_at,omitempty"`
	ExpiresAt int64  `json:"expires_at,omitempty"`
	DeletedAt int64  `json:"deleted_at,omitempty"`
	NodeID    string `json:"node_id,omitempty"`
}

type ClusterPeerStatus struct {
	Address              string     `json:"address"`
	Healthy              bool       `json:"healthy"`
	LastSeen             *time.Time `json:"last_seen,omitempty"`
	LastError            string     `json:"last_error,omitempty"`
	ConsecutiveSuccesses int        `json:"consecutive_successes"`
	ConsecutiveFailures  int        `json:"consecutive_failures"`
}

type ClusterStatus struct {
	NodeID    string              `json:"node_id"`
	StartedAt time.Time           `json:"started_at"`
	Healthy   bool                `json:"healthy"`
	KVKeys    int                 `json:"kv_keys"`
	Peers     []ClusterPeerStatus `json:"peers"`
}

type clusterEvent struct {
	path    string
	payload ClusterMutation
}

type peerState struct {
	address              string
	healthy              bool
	lastSeen             *time.Time
	lastError            string
	consecutiveSuccesses int
	consecutiveFailures  int
}

type ClusterService struct {
	cfg     *ClusterConfig
	logger  schemas.Logger
	nodeID  string
	kvStore *kvstore.Store
	client  *http.Client
	auth    string

	startedAt time.Time
	queue     chan clusterEvent
	stopCh    chan struct{}
	stopOnce  sync.Once
	wg        sync.WaitGroup

	mu    sync.RWMutex
	peers map[string]*peerState
}

func NewClusterService(cfg *ClusterConfig, store *kvstore.Store, nodeID string, logger schemas.Logger) (*ClusterService, error) {
	if cfg == nil || !cfg.Enabled || store == nil {
		return nil, nil
	}

	service := &ClusterService{
		cfg:       cfg,
		logger:    logger,
		nodeID:    nodeID,
		kvStore:   store,
		client:    &http.Client{Timeout: clusterTimeout(cfg)},
		auth:      clusterAuthToken(cfg),
		startedAt: time.Now().UTC(),
		queue:     make(chan clusterEvent, 512),
		stopCh:    make(chan struct{}),
		peers:     make(map[string]*peerState),
	}

	for _, peer := range cfg.Peers {
		address := normalizeClusterAddress(peer)
		if address == "" || address == normalizeClusterAddress(nodeID) {
			continue
		}
		service.peers[address] = &peerState{address: address}
	}

	store.SetDelegate(service)
	service.start()
	return service, nil
}

func (s *ClusterService) start() {
	if s == nil {
		return
	}

	s.wg.Add(2)
	go func() {
		defer s.wg.Done()
		s.dispatchLoop()
	}()
	go func() {
		defer s.wg.Done()
		s.healthLoop()
	}()
}

func (s *ClusterService) Close() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() {
		close(s.stopCh)
	})
	s.wg.Wait()
}

func (s *ClusterService) OnSet(key string, valueJSON []byte, writtenAt int64, expiresAt int64) {
	if s == nil {
		return
	}
	select {
	case s.queue <- clusterEvent{
		path: clusterSetEndpoint,
		payload: ClusterMutation{
			Key:       key,
			ValueJSON: valueJSON,
			WrittenAt: writtenAt,
			ExpiresAt: expiresAt,
			NodeID:    s.nodeID,
		},
	}:
	default:
		if s.logger != nil {
			s.logger.Warn("cluster queue full, dropping set mutation for key %s", key)
		}
	}
}

func (s *ClusterService) OnDelete(key string, deletedAt int64) {
	if s == nil {
		return
	}
	select {
	case s.queue <- clusterEvent{
		path: clusterDeleteEndpoint,
		payload: ClusterMutation{
			Key:       key,
			DeletedAt: deletedAt,
			NodeID:    s.nodeID,
		},
	}:
	default:
		if s.logger != nil {
			s.logger.Warn("cluster queue full, dropping delete mutation for key %s", key)
		}
	}
}

func (s *ClusterService) ApplySet(mutation ClusterMutation) error {
	if s == nil {
		return nil
	}
	return s.kvStore.SetRemote(mutation.Key, mutation.ValueJSON, mutation.WrittenAt, mutation.ExpiresAt)
}

func (s *ClusterService) ApplyDelete(mutation ClusterMutation) error {
	if s == nil {
		return nil
	}
	return s.kvStore.DeleteRemote(mutation.Key, mutation.DeletedAt)
}

func (s *ClusterService) Status() ClusterStatus {
	if s == nil {
		return ClusterStatus{}
	}

	s.mu.RLock()
	peers := make([]ClusterPeerStatus, 0, len(s.peers))
	for _, peer := range s.peers {
		var lastSeen *time.Time
		if peer.lastSeen != nil {
			t := *peer.lastSeen
			lastSeen = &t
		}
		peers = append(peers, ClusterPeerStatus{
			Address:              peer.address,
			Healthy:              peer.healthy,
			LastSeen:             lastSeen,
			LastError:            peer.lastError,
			ConsecutiveSuccesses: peer.consecutiveSuccesses,
			ConsecutiveFailures:  peer.consecutiveFailures,
		})
	}
	s.mu.RUnlock()

	healthy := true
	failureThreshold := clusterFailureThreshold(s.cfg)
	for _, peer := range peers {
		if peer.ConsecutiveFailures >= failureThreshold {
			healthy = false
			break
		}
	}

	return ClusterStatus{
		NodeID:    s.nodeID,
		StartedAt: s.startedAt,
		Healthy:   healthy,
		KVKeys:    s.kvStore.Len(),
		Peers:     peers,
	}
}

func (s *ClusterService) dispatchLoop() {
	for {
		select {
		case event := <-s.queue:
			s.broadcast(event)
		case <-s.stopCh:
			return
		}
	}
}

func (s *ClusterService) broadcast(event clusterEvent) {
	payload, err := sonic.Marshal(event.payload)
	if err != nil {
		if s.logger != nil {
			s.logger.Warn("failed to marshal cluster mutation: %v", err)
		}
		return
	}

	s.mu.RLock()
	peers := make([]string, 0, len(s.peers))
	for address := range s.peers {
		peers = append(peers, address)
	}
	s.mu.RUnlock()

	for _, peer := range peers {
		req, err := http.NewRequest(http.MethodPost, peer+event.path, bytes.NewReader(payload))
		if err != nil {
			s.markPeerFailure(peer, err)
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		s.addAuthHeader(req)

		resp, err := s.client.Do(req)
		if err != nil {
			s.markPeerFailure(peer, err)
			continue
		}
		resp.Body.Close()
		if resp.StatusCode >= http.StatusBadRequest {
			s.markPeerFailure(peer, fmt.Errorf("peer returned status %d", resp.StatusCode))
			continue
		}
		s.markPeerSuccess(peer)
	}
}

func (s *ClusterService) healthLoop() {
	ticker := time.NewTicker(clusterHealthInterval(s.cfg))
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.checkPeers()
		case <-s.stopCh:
			return
		}
	}
}

func (s *ClusterService) checkPeers() {
	s.mu.RLock()
	peers := make([]string, 0, len(s.peers))
	for address := range s.peers {
		peers = append(peers, address)
	}
	s.mu.RUnlock()

	for _, peer := range peers {
		req, err := http.NewRequest(http.MethodGet, peer+clusterStatusEndpoint, nil)
		if err != nil {
			s.markPeerFailure(peer, err)
			continue
		}
		s.addAuthHeader(req)
		resp, err := s.client.Do(req)
		if err != nil {
			s.markPeerFailure(peer, err)
			continue
		}
		resp.Body.Close()
		if resp.StatusCode >= http.StatusBadRequest {
			s.markPeerFailure(peer, fmt.Errorf("peer returned status %d", resp.StatusCode))
			continue
		}
		s.markPeerSuccess(peer)
	}
}

func (s *ClusterService) markPeerSuccess(address string) {
	now := time.Now().UTC()
	successThreshold := clusterSuccessThreshold(s.cfg)

	s.mu.Lock()
	defer s.mu.Unlock()

	peer, ok := s.peers[address]
	if !ok {
		return
	}
	peer.consecutiveSuccesses++
	peer.consecutiveFailures = 0
	peer.lastError = ""
	peer.lastSeen = &now
	if peer.consecutiveSuccesses >= successThreshold {
		peer.healthy = true
	}
}

func (s *ClusterService) markPeerFailure(address string, err error) {
	failureThreshold := clusterFailureThreshold(s.cfg)

	s.mu.Lock()
	defer s.mu.Unlock()

	peer, ok := s.peers[address]
	if !ok {
		return
	}
	peer.consecutiveFailures++
	peer.consecutiveSuccesses = 0
	if err != nil {
		peer.lastError = err.Error()
	}
	if peer.consecutiveFailures >= failureThreshold {
		peer.healthy = false
	}
}

func (s *ClusterService) addAuthHeader(req *http.Request) {
	if s == nil || req == nil || strings.TrimSpace(s.auth) == "" {
		return
	}
	req.Header.Set(ClusterAuthHeader, s.auth)
}

func (s *ClusterService) IsInternalTokenValid(token string) bool {
	if s == nil {
		return false
	}
	expected := strings.TrimSpace(s.auth)
	if expected == "" {
		return true
	}
	return strings.TrimSpace(token) == expected
}

func normalizeClusterAddress(value string) string {
	address := strings.TrimSpace(value)
	if address == "" {
		return ""
	}
	if strings.HasPrefix(address, "http://") || strings.HasPrefix(address, "https://") {
		return strings.TrimRight(address, "/")
	}
	return "http://" + strings.TrimRight(address, "/")
}

func clusterAuthToken(cfg *ClusterConfig) string {
	if cfg == nil || cfg.AuthToken == nil {
		return ""
	}
	return strings.TrimSpace(cfg.AuthToken.GetValue())
}

func clusterTimeout(cfg *ClusterConfig) time.Duration {
	timeout := 10 * time.Second
	if cfg != nil && cfg.Gossip != nil && cfg.Gossip.Config != nil && cfg.Gossip.Config.TimeoutSeconds > 0 {
		timeout = time.Duration(cfg.Gossip.Config.TimeoutSeconds) * time.Second
	}
	return timeout
}

func clusterHealthInterval(cfg *ClusterConfig) time.Duration {
	timeout := clusterTimeout(cfg)
	interval := timeout / 2
	if interval < 5*time.Second {
		interval = 5 * time.Second
	}
	return interval
}

func clusterSuccessThreshold(cfg *ClusterConfig) int {
	if cfg != nil && cfg.Gossip != nil && cfg.Gossip.Config != nil && cfg.Gossip.Config.SuccessThreshold > 0 {
		return cfg.Gossip.Config.SuccessThreshold
	}
	return 1
}

func clusterFailureThreshold(cfg *ClusterConfig) int {
	if cfg != nil && cfg.Gossip != nil && cfg.Gossip.Config != nil && cfg.Gossip.Config.FailureThreshold > 0 {
		return cfg.Gossip.Config.FailureThreshold
	}
	return 3
}
