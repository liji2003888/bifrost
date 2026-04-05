package enterprise

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"slices"
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

type clusterDiscoveryResolver interface {
	Discover(ctx context.Context, cfg *ClusterConfig, nodeID string) ([]string, error)
}

type dnsClusterDiscoveryResolver struct {
	lookupHost func(ctx context.Context, host string) ([]string, error)
}

type ClusterMutation struct {
	Key       string `json:"key"`
	ValueJSON []byte `json:"value_json,omitempty"`
	WrittenAt int64  `json:"written_at,omitempty"`
	ExpiresAt int64  `json:"expires_at,omitempty"`
	DeletedAt int64  `json:"deleted_at,omitempty"`
	NodeID    string `json:"node_id,omitempty"`
}

type ClusterPeerStatus struct {
	Address              string                   `json:"address"`
	Healthy              bool                     `json:"healthy"`
	ReportedHealthy      *bool                    `json:"reported_healthy,omitempty"`
	NodeID               string                   `json:"node_id,omitempty"`
	StartedAt            *time.Time               `json:"started_at,omitempty"`
	KVKeys               int                      `json:"kv_keys,omitempty"`
	DiscoveryPeerCount   int                      `json:"discovery_peer_count,omitempty"`
	ConfigSync           *ClusterConfigSyncStatus `json:"config_sync,omitempty"`
	LastSeen             *time.Time               `json:"last_seen,omitempty"`
	LastError            string                   `json:"last_error,omitempty"`
	ConsecutiveSuccesses int                      `json:"consecutive_successes"`
	ConsecutiveFailures  int                      `json:"consecutive_failures"`
}

type ClusterStatus struct {
	NodeID     string                   `json:"node_id"`
	StartedAt  time.Time                `json:"started_at"`
	Healthy    bool                     `json:"healthy"`
	KVKeys     int                      `json:"kv_keys"`
	ConfigSync *ClusterConfigSyncStatus `json:"config_sync,omitempty"`
	Peers      []ClusterPeerStatus      `json:"peers"`
	Discovery  *ClusterDiscoveryStatus  `json:"discovery,omitempty"`
}

type ClusterConfigSyncStatus struct {
	StoreConnected   bool     `json:"store_connected"`
	StoreKind        string   `json:"store_kind,omitempty"`
	RuntimeHash      string   `json:"runtime_hash,omitempty"`
	StoreHash        string   `json:"store_hash,omitempty"`
	InSync           *bool    `json:"in_sync,omitempty"`
	DriftDomains     []string `json:"drift_domains,omitempty"`
	CustomerCount    int      `json:"customer_count,omitempty"`
	ModelConfigCount int      `json:"model_config_count,omitempty"`
	ProviderCount    int      `json:"provider_count,omitempty"`
	RoutingRuleCount int      `json:"routing_rule_count,omitempty"`
	TeamCount        int      `json:"team_count,omitempty"`
	VirtualKeyCount  int      `json:"virtual_key_count,omitempty"`
	MCPClientCount   int      `json:"mcp_client_count,omitempty"`
	PluginCount      int      `json:"plugin_count,omitempty"`
	LastError        string   `json:"last_error,omitempty"`
}

type ClusterDiscoveryStatus struct {
	Enabled     bool                 `json:"enabled"`
	Type        ClusterDiscoveryType `json:"type,omitempty"`
	LastRefresh *time.Time           `json:"last_refresh,omitempty"`
	LastError   string               `json:"last_error,omitempty"`
	PeerCount   int                  `json:"peer_count"`
}

type clusterEvent struct {
	path    string
	payload ClusterMutation
}

type peerState struct {
	address              string
	healthy              bool
	reportedHealthy      *bool
	nodeID               string
	startedAt            *time.Time
	kvKeys               int
	discoveryPeerCount   int
	configSync           *ClusterConfigSyncStatus
	lastSeen             *time.Time
	lastError            string
	consecutiveSuccesses int
	consecutiveFailures  int
	static               bool
	discovered           bool
}

type ClusterService struct {
	cfg     *ClusterConfig
	logger  schemas.Logger
	nodeID  string
	kvStore *kvstore.Store
	client  *http.Client
	auth    string
	self    map[string]struct{}

	startedAt          time.Time
	queue              chan clusterEvent
	stopCh             chan struct{}
	stopOnce           sync.Once
	wg                 sync.WaitGroup
	resolver           clusterDiscoveryResolver
	configSyncReporter func() ClusterConfigSyncStatus

	mu                   sync.RWMutex
	peers                map[string]*peerState
	discoveryLastRefresh *time.Time
	discoveryLastError   string
}

func NewClusterService(cfg *ClusterConfig, store *kvstore.Store, nodeID string, logger schemas.Logger) (*ClusterService, error) {
	return newClusterService(cfg, store, nodeID, logger, defaultClusterDiscoveryResolver())
}

func newClusterService(cfg *ClusterConfig, store *kvstore.Store, nodeID string, logger schemas.Logger, resolver clusterDiscoveryResolver) (*ClusterService, error) {
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
		resolver:  resolver,
		self:      clusterSelfAddresses(nodeID, cfg),
	}

	for _, peer := range cfg.Peers {
		service.upsertPeer(normalizeClusterAddress(peer), true, false)
	}

	if clusterDiscoveryEnabled(cfg) {
		service.refreshDiscoveredPeers(context.Background())
	}

	store.SetDelegate(service)
	service.start()
	return service, nil
}

func (s *ClusterService) start() {
	if s == nil {
		return
	}

	goroutines := 2
	if clusterDiscoveryEnabled(s.cfg) && s.resolver != nil {
		goroutines++
	}
	s.wg.Add(goroutines)
	go func() {
		defer s.wg.Done()
		s.dispatchLoop()
	}()
	go func() {
		defer s.wg.Done()
		s.healthLoop()
	}()
	if clusterDiscoveryEnabled(s.cfg) && s.resolver != nil {
		go func() {
			defer s.wg.Done()
			s.discoveryLoop()
		}()
	}
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

	var localConfigSync *ClusterConfigSyncStatus
	if s.configSyncReporter != nil {
		status := s.configSyncReporter()
		localConfigSync = cloneClusterConfigSyncStatus(&status)
	}

	s.mu.RLock()
	peers := make([]ClusterPeerStatus, 0, len(s.peers))
	discoveredPeers := 0
	for _, peer := range s.peers {
		var lastSeen *time.Time
		if peer.lastSeen != nil {
			t := *peer.lastSeen
			lastSeen = &t
		}
		var reportedHealthy *bool
		if peer.reportedHealthy != nil {
			value := *peer.reportedHealthy
			reportedHealthy = &value
		}
		var startedAt *time.Time
		if peer.startedAt != nil {
			t := *peer.startedAt
			startedAt = &t
		}
		peers = append(peers, ClusterPeerStatus{
			Address:              peer.address,
			Healthy:              peer.healthy,
			ReportedHealthy:      reportedHealthy,
			NodeID:               peer.nodeID,
			StartedAt:            startedAt,
			KVKeys:               peer.kvKeys,
			DiscoveryPeerCount:   peer.discoveryPeerCount,
			ConfigSync:           cloneClusterConfigSyncStatus(peer.configSync),
			LastSeen:             lastSeen,
			LastError:            peer.lastError,
			ConsecutiveSuccesses: peer.consecutiveSuccesses,
			ConsecutiveFailures:  peer.consecutiveFailures,
		})
		if peer.discovered {
			discoveredPeers++
		}
	}
	var lastRefresh *time.Time
	if s.discoveryLastRefresh != nil {
		t := *s.discoveryLastRefresh
		lastRefresh = &t
	}
	discoveryLastError := s.discoveryLastError
	s.mu.RUnlock()

	slices.SortFunc(peers, func(a, b ClusterPeerStatus) int {
		return strings.Compare(a.Address, b.Address)
	})

	healthy := true
	failureThreshold := clusterFailureThreshold(s.cfg)
	for _, peer := range peers {
		if peer.ConsecutiveFailures >= failureThreshold {
			healthy = false
			break
		}
		if peer.ReportedHealthy != nil && !*peer.ReportedHealthy {
			healthy = false
			break
		}
	}

	return ClusterStatus{
		NodeID:     s.nodeID,
		StartedAt:  s.startedAt,
		Healthy:    healthy,
		KVKeys:     s.kvStore.Len(),
		ConfigSync: localConfigSync,
		Peers:      peers,
		Discovery:  clusterDiscoveryStatus(s.cfg, lastRefresh, discoveryLastError, discoveredPeers),
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
		s.markPeerSuccess(peer, nil)
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

func (s *ClusterService) discoveryLoop() {
	ticker := time.NewTicker(clusterDiscoveryInterval(s.cfg))
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.refreshDiscoveredPeers(context.Background())
		case <-s.stopCh:
			return
		}
	}
}

func (s *ClusterService) checkPeers() {
	for _, peer := range s.peerAddresses() {
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
		if resp.StatusCode >= http.StatusBadRequest {
			resp.Body.Close()
			s.markPeerFailure(peer, fmt.Errorf("peer returned status %d", resp.StatusCode))
			continue
		}
		status, err := readClusterStatus(resp.Body)
		resp.Body.Close()
		if err != nil {
			s.markPeerFailure(peer, fmt.Errorf("invalid cluster status response: %w", err))
			continue
		}
		s.markPeerSuccess(peer, status)
	}
}

func (s *ClusterService) refreshDiscoveredPeers(ctx context.Context) {
	if s == nil || s.resolver == nil || !clusterDiscoveryEnabled(s.cfg) {
		return
	}

	discoveryCtx := ctx
	if discoveryCtx == nil {
		discoveryCtx = context.Background()
	}
	if discoveryTimeout := clusterDiscoveryTimeout(s.cfg); discoveryTimeout > 0 {
		var cancel context.CancelFunc
		discoveryCtx, cancel = context.WithTimeout(discoveryCtx, discoveryTimeout)
		defer cancel()
	}

	addresses, err := s.resolver.Discover(discoveryCtx, s.cfg, s.nodeID)
	now := time.Now().UTC()

	s.mu.Lock()
	s.discoveryLastRefresh = &now
	if err != nil {
		s.discoveryLastError = err.Error()
		s.mu.Unlock()
		if s.logger != nil {
			s.logger.Warn("cluster discovery refresh failed: %v", err)
		}
		return
	}
	s.discoveryLastError = ""
	s.mu.Unlock()

	s.syncDiscoveredPeers(addresses)
}

func (s *ClusterService) syncDiscoveredPeers(addresses []string) {
	if s == nil {
		return
	}

	discovered := make(map[string]struct{}, len(addresses))
	for _, address := range addresses {
		normalized := normalizeClusterAddress(address)
		if normalized == "" || s.isSelfAddress(normalized) {
			continue
		}
		discovered[normalized] = struct{}{}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for address := range discovered {
		peer, ok := s.peers[address]
		if !ok {
			s.peers[address] = &peerState{address: address, discovered: true}
			continue
		}
		peer.discovered = true
	}

	for address, peer := range s.peers {
		if !peer.discovered {
			continue
		}
		if _, ok := discovered[address]; ok {
			continue
		}
		if peer.static {
			peer.discovered = false
			continue
		}
		delete(s.peers, address)
	}
}

func (s *ClusterService) markPeerSuccess(address string, status *ClusterStatus) {
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
	if status != nil {
		reportedHealthy := status.Healthy
		peer.reportedHealthy = &reportedHealthy
		peer.nodeID = strings.TrimSpace(status.NodeID)
		peer.kvKeys = status.KVKeys
		peer.configSync = cloneClusterConfigSyncStatus(status.ConfigSync)
		if status.StartedAt.IsZero() {
			peer.startedAt = nil
		} else {
			startedAt := status.StartedAt
			peer.startedAt = &startedAt
		}
		if status.Discovery != nil {
			peer.discoveryPeerCount = status.Discovery.PeerCount
		} else {
			peer.discoveryPeerCount = 0
		}
	}
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
	peer.reportedHealthy = nil
	peer.configSync = nil
	if peer.consecutiveFailures >= failureThreshold {
		peer.healthy = false
	}
}

func readClusterStatus(reader io.Reader) (*ClusterStatus, error) {
	if reader == nil {
		return nil, nil
	}

	body, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return nil, nil
	}

	var status ClusterStatus
	if err := sonic.Unmarshal(body, &status); err != nil {
		return nil, err
	}
	return &status, nil
}

func (s *ClusterService) addAuthHeader(req *http.Request) {
	if s == nil || req == nil || strings.TrimSpace(s.auth) == "" {
		return
	}
	req.Header.Set(ClusterAuthHeader, s.auth)
}

func (s *ClusterService) peerAddresses() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	peers := make([]string, 0, len(s.peers))
	for address := range s.peers {
		peers = append(peers, address)
	}
	return peers
}

func (s *ClusterService) upsertPeer(address string, isStatic bool, isDiscovered bool) {
	if s == nil || address == "" || s.isSelfAddress(address) {
		return
	}

	peer, ok := s.peers[address]
	if !ok {
		s.peers[address] = &peerState{
			address:    address,
			static:     isStatic,
			discovered: isDiscovered,
		}
		return
	}
	peer.static = peer.static || isStatic
	peer.discovered = peer.discovered || isDiscovered
}

func (s *ClusterService) isSelfAddress(address string) bool {
	if s == nil {
		return false
	}
	_, ok := s.self[normalizeClusterAddress(address)]
	return ok
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

func (s *ClusterService) SetConfigSyncReporter(reporter func() ClusterConfigSyncStatus) {
	if s == nil {
		return
	}
	s.configSyncReporter = reporter
}

func (s *ClusterService) NodeID() string {
	if s == nil {
		return ""
	}
	return s.nodeID
}

func (s *ClusterService) PeerStatuses() []ClusterPeerStatus {
	if s == nil {
		return nil
	}
	return s.Status().Peers
}

func (s *ClusterService) GetJSON(ctx context.Context, address, path string, out any) error {
	if s == nil {
		return fmt.Errorf("cluster service is not enabled")
	}

	address = normalizeClusterAddress(address)
	if address == "" {
		return fmt.Errorf("cluster peer address is required")
	}

	requestPath := strings.TrimSpace(path)
	if requestPath == "" {
		return fmt.Errorf("cluster request path is required")
	}
	if !strings.HasPrefix(requestPath, "/") {
		requestPath = "/" + requestPath
	}

	requestCtx := ctx
	if requestCtx == nil {
		requestCtx = context.Background()
	}

	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, address+requestPath, nil)
	if err != nil {
		return err
	}
	s.addAuthHeader(req)

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		message := strings.TrimSpace(string(body))
		if message == "" {
			message = fmt.Sprintf("peer returned status %d", resp.StatusCode)
		}
		return fmt.Errorf("%s", message)
	}

	if out == nil {
		return nil
	}
	return sonic.ConfigDefault.NewDecoder(resp.Body).Decode(out)
}

func (s *ClusterService) PostJSON(ctx context.Context, address, path string, payload any, out any) error {
	if s == nil {
		return fmt.Errorf("cluster service is not enabled")
	}

	address = normalizeClusterAddress(address)
	if address == "" {
		return fmt.Errorf("cluster peer address is required")
	}

	requestPath := strings.TrimSpace(path)
	if requestPath == "" {
		return fmt.Errorf("cluster request path is required")
	}
	if !strings.HasPrefix(requestPath, "/") {
		requestPath = "/" + requestPath
	}

	body, err := sonic.Marshal(payload)
	if err != nil {
		return err
	}

	requestCtx := ctx
	if requestCtx == nil {
		requestCtx = context.Background()
	}

	req, err := http.NewRequestWithContext(requestCtx, http.MethodPost, address+requestPath, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	s.addAuthHeader(req)

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		message := strings.TrimSpace(string(body))
		if message == "" {
			message = fmt.Sprintf("peer returned status %d", resp.StatusCode)
		}
		return fmt.Errorf("%s", message)
	}

	if out == nil {
		return nil
	}
	return sonic.ConfigDefault.NewDecoder(resp.Body).Decode(out)
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

func cloneClusterConfigSyncStatus(status *ClusterConfigSyncStatus) *ClusterConfigSyncStatus {
	if status == nil {
		return nil
	}
	cloned := *status
	if len(status.DriftDomains) > 0 {
		cloned.DriftDomains = append([]string(nil), status.DriftDomains...)
	}
	if status.InSync != nil {
		value := *status.InSync
		cloned.InSync = &value
	}
	return &cloned
}

func defaultClusterDiscoveryResolver() clusterDiscoveryResolver {
	return &dnsClusterDiscoveryResolver{
		lookupHost: net.DefaultResolver.LookupHost,
	}
}

func (r *dnsClusterDiscoveryResolver) Discover(ctx context.Context, cfg *ClusterConfig, nodeID string) ([]string, error) {
	if r == nil || cfg == nil || !clusterDiscoveryEnabled(cfg) || cfg.Discovery == nil {
		return nil, nil
	}
	if cfg.Discovery.Type != ClusterDiscoveryDNS && cfg.Discovery.Type != ClusterDiscoveryKubernetes {
		return nil, nil
	}

	targets := clusterDiscoveryTargets(cfg)
	if len(targets) == 0 {
		return nil, nil
	}

	allowedNetworks, err := parseAllowedAddressSpaces(cfg.Discovery.AllowedAddressSpace)
	if err != nil {
		return nil, err
	}

	defaultPort := clusterAdvertisePort(nodeID, cfg)
	addresses := make([]string, 0, len(targets))
	seen := make(map[string]struct{})
	var firstErr error

	for _, target := range targets {
		host, port, ok := parseDiscoveryTarget(target, defaultPort)
		if !ok {
			continue
		}
		if ip := net.ParseIP(host); ip != nil {
			if clusterHostAllowed(ip, allowedNetworks) {
				addDiscoveredPeer(seen, &addresses, net.JoinHostPort(ip.String(), port))
			}
			continue
		}

		resolvedHosts, lookupErr := r.lookupHost(ctx, host)
		if lookupErr != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("failed to resolve %s: %w", host, lookupErr)
			}
			continue
		}
		for _, resolvedHost := range resolvedHosts {
			ip := net.ParseIP(resolvedHost)
			if ip == nil || !clusterHostAllowed(ip, allowedNetworks) {
				continue
			}
			addDiscoveredPeer(seen, &addresses, net.JoinHostPort(ip.String(), port))
		}
	}

	if len(addresses) == 0 && firstErr != nil {
		return nil, firstErr
	}
	return addresses, nil
}

func clusterDiscoveryTargets(cfg *ClusterConfig) []string {
	if cfg == nil || cfg.Discovery == nil || !cfg.Discovery.Enabled {
		return nil
	}

	targets := make([]string, 0, len(cfg.Discovery.DNSNames)+2)
	for _, name := range cfg.Discovery.DNSNames {
		name = strings.TrimSpace(name)
		if name != "" {
			targets = append(targets, name)
		}
	}

	serviceName := strings.TrimSpace(cfg.Discovery.ServiceName)
	if serviceName == "" {
		return dedupeStrings(targets)
	}

	switch cfg.Discovery.Type {
	case ClusterDiscoveryKubernetes:
		namespace := strings.TrimSpace(cfg.Discovery.K8sNamespace)
		if namespace == "" {
			namespace = "default"
		}
		targets = append(targets,
			fmt.Sprintf("%s.%s.svc", serviceName, namespace),
			fmt.Sprintf("%s.%s.svc.cluster.local", serviceName, namespace),
		)
	default:
		targets = append(targets, serviceName)
	}

	return dedupeStrings(targets)
}

func parseDiscoveryTarget(target, defaultPort string) (host string, port string, ok bool) {
	target = strings.TrimSpace(target)
	if target == "" {
		return "", "", false
	}

	if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
		parsed, err := url.Parse(target)
		if err != nil || parsed.Hostname() == "" {
			return "", "", false
		}
		host = parsed.Hostname()
		port = parsed.Port()
		if port == "" {
			port = defaultPort
		}
		return host, port, port != ""
	}

	if splitHost, splitPort, err := net.SplitHostPort(target); err == nil {
		return strings.TrimSpace(splitHost), strings.TrimSpace(splitPort), strings.TrimSpace(splitPort) != ""
	}

	trimmedIP := strings.Trim(target, "[]")
	if ip := net.ParseIP(trimmedIP); ip != nil {
		return ip.String(), defaultPort, defaultPort != ""
	}

	if defaultPort == "" {
		return "", "", false
	}
	return target, defaultPort, true
}

func parseAllowedAddressSpaces(values []string) ([]*net.IPNet, error) {
	if len(values) == 0 {
		return nil, nil
	}

	networks := make([]*net.IPNet, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		_, network, err := net.ParseCIDR(value)
		if err != nil {
			return nil, fmt.Errorf("invalid allowed_address_space %q: %w", value, err)
		}
		networks = append(networks, network)
	}
	return networks, nil
}

func clusterHostAllowed(ip net.IP, networks []*net.IPNet) bool {
	if ip == nil {
		return false
	}
	if len(networks) == 0 {
		return true
	}
	for _, network := range networks {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

func addDiscoveredPeer(seen map[string]struct{}, addresses *[]string, address string) {
	normalized := normalizeClusterAddress(address)
	if normalized == "" {
		return
	}
	if _, ok := seen[normalized]; ok {
		return
	}
	seen[normalized] = struct{}{}
	*addresses = append(*addresses, normalized)
}

func clusterSelfAddresses(nodeID string, cfg *ClusterConfig) map[string]struct{} {
	self := make(map[string]struct{})
	addSelfAddress := func(address string) {
		normalized := normalizeClusterAddress(address)
		if normalized == "" {
			return
		}
		self[normalized] = struct{}{}
	}

	addSelfAddress(nodeID)

	port := clusterAdvertisePort(nodeID, cfg)
	if port == "" {
		return self
	}

	addSelfAddress(net.JoinHostPort("127.0.0.1", port))
	addSelfAddress(net.JoinHostPort("localhost", port))
	addSelfAddress(net.JoinHostPort("::1", port))

	interfaceAddrs, err := net.InterfaceAddrs()
	if err != nil {
		return self
	}
	for _, address := range interfaceAddrs {
		ipNet, ok := address.(*net.IPNet)
		if !ok || ipNet.IP == nil {
			continue
		}
		addSelfAddress(net.JoinHostPort(ipNet.IP.String(), port))
	}
	return self
}

func dedupeStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func clusterDiscoveryStatus(cfg *ClusterConfig, lastRefresh *time.Time, lastError string, peerCount int) *ClusterDiscoveryStatus {
	if !clusterDiscoveryEnabled(cfg) || cfg == nil || cfg.Discovery == nil {
		return nil
	}
	return &ClusterDiscoveryStatus{
		Enabled:     true,
		Type:        cfg.Discovery.Type,
		LastRefresh: lastRefresh,
		LastError:   lastError,
		PeerCount:   peerCount,
	}
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

func clusterDiscoveryTimeout(cfg *ClusterConfig) time.Duration {
	if cfg != nil && cfg.Discovery != nil && cfg.Discovery.DialTimeout > 0 {
		return time.Duration(cfg.Discovery.DialTimeout)
	}
	return clusterTimeout(cfg)
}

func clusterDiscoveryInterval(cfg *ClusterConfig) time.Duration {
	interval := 30 * time.Second
	if healthInterval := clusterHealthInterval(cfg); healthInterval > 0 && healthInterval < interval {
		interval = healthInterval
	}
	if interval < 5*time.Second {
		return 5 * time.Second
	}
	return interval
}

func clusterAdvertisePort(nodeID string, cfg *ClusterConfig) string {
	if cfg != nil && cfg.Discovery != nil && cfg.Discovery.BindPort > 0 {
		return fmt.Sprintf("%d", cfg.Discovery.BindPort)
	}
	if _, port, err := net.SplitHostPort(strings.TrimSpace(nodeID)); err == nil {
		return port
	}
	if cfg != nil && cfg.Gossip != nil && cfg.Gossip.Port > 0 {
		return fmt.Sprintf("%d", cfg.Gossip.Port)
	}
	return ""
}

func clusterDiscoveryEnabled(cfg *ClusterConfig) bool {
	return cfg != nil && cfg.Discovery != nil && cfg.Discovery.Enabled
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
