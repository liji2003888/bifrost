package loadbalancer

import (
	"fmt"
	"math"
	"math/rand"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	bifrost "github.com/maximhq/bifrost/core"
	"github.com/maximhq/bifrost/core/schemas"
	configstoreTables "github.com/maximhq/bifrost/framework/configstore/tables"
	enterprisecfg "github.com/maximhq/bifrost/transports/bifrost-http/enterprise"
)

const (
	PluginName = "loadbalancer"
)

const startTimeKey schemas.BifrostContextKey = "bf-loadbalancer-start-time"

type Config = enterprisecfg.LoadBalancerConfig

type Plugin struct {
	mu             sync.RWMutex
	cfg            trackerConfig
	policy         runtimePolicy
	tracker        *Tracker
	logger         schemas.Logger
	catalog        modelsCatalog
	providerLookup providerConfigLookup
}

type runtimePolicy struct {
	enabled                       bool
	keyBalancingEnabled           bool
	directionRoutingEnabled       bool
	directionRoutingForVirtualKey bool
	providerAllowlist             map[schemas.ModelProvider]struct{}
	modelAllowlist                map[string]struct{}
}

type directionTarget struct {
	Provider schemas.ModelProvider
	Model    string
}

type requestPolicy struct {
	source                  string
	enabled                 bool
	keyBalancingEnabled     bool
	directionRoutingEnabled bool
	directionTargets        []directionTarget
	additionalFallbacks     []string
}

type RouteSnapshot struct {
	Samples             int64     `json:"samples"`
	Successes           int64     `json:"successes"`
	Failures            int64     `json:"failures"`
	ConsecutiveFailures int64     `json:"consecutive_failures"`
	ErrorEWMA           float64   `json:"error_ewma"`
	LatencyEWMA         float64   `json:"latency_ewma"`
	LastUpdated         time.Time `json:"last_updated"`
}

type RouteStatus struct {
	Provider             schemas.ModelProvider `json:"provider"`
	Model                string                `json:"model"`
	KeyID                string                `json:"key_id"`
	State                HealthState           `json:"state"`
	Score                float64               `json:"score"`
	Weight               int                   `json:"weight"`
	ExpectedTrafficShare float64               `json:"expected_traffic_share"`
	ActualTrafficShare   float64               `json:"actual_traffic_share"`
	RouteSnapshot
}

type DirectionSnapshot struct {
	Samples             int64     `json:"samples"`
	Successes           int64     `json:"successes"`
	Failures            int64     `json:"failures"`
	ConsecutiveFailures int64     `json:"consecutive_failures"`
	ErrorEWMA           float64   `json:"error_ewma"`
	LatencyEWMA         float64   `json:"latency_ewma"`
	LastUpdated         time.Time `json:"last_updated"`
}

type DirectionStatus struct {
	Provider             schemas.ModelProvider `json:"provider"`
	Model                string                `json:"model"`
	State                HealthState           `json:"state"`
	Score                float64               `json:"score"`
	Weight               int                   `json:"weight"`
	ExpectedTrafficShare float64               `json:"expected_traffic_share"`
	ActualTrafficShare   float64               `json:"actual_traffic_share"`
	DirectionSnapshot
}

type routeKey struct {
	provider schemas.ModelProvider
	model    string
	keyID    string
}

type directionKey struct {
	provider schemas.ModelProvider
	model    string
}

type routeStats struct {
	mu                  sync.RWMutex
	samples             int64
	successes           int64
	failures            int64
	consecutiveFailures int64
	recoverySuccesses   int64
	errorEWMA           float64
	latencyEWMA         float64
	lastSuccess         time.Time
	lastFailure         time.Time
	recoveryStarted     time.Time
	lastUpdated         time.Time
}

type trackerConfig struct {
	ewmaAlpha                 float64
	errorPenalty              float64
	latencyPenalty            float64
	consecutiveFailurePenalty float64
	minimumSamples            int
	explorationRatio          float64
	jitterRatio               float64
	minWeightMultiplier       float64
	maxWeightMultiplier       float64
	recomputeInterval         time.Duration
	degradedErrorThreshold    float64
	failedErrorThreshold      float64
	failedConsecutiveFailures int64
	recoveryHalfLife          time.Duration
	weightFloor               int
	weightCeiling             int
	explorationFloorRatio     float64
	recoverySuccessThreshold  int64
}

type Tracker struct {
	cfg                 trackerConfig
	routes              sync.Map
	directions          sync.Map
	routeProfiles       sync.Map
	directionProfiles   sync.Map
	remoteMu            sync.RWMutex
	remoteRoutes        map[string]map[routeKey]RouteSnapshot
	remoteDirections    map[string]map[directionKey]DirectionSnapshot
	remoteSnapshotTTL   time.Duration
	recomputeMu         sync.Mutex
	lastRecomputeUnixNs int64
	dirty               uint32
	stopCh              chan struct{}
	stopOnce            sync.Once
	wg                  sync.WaitGroup
}

func Init(config *enterprisecfg.LoadBalancerConfig, logger schemas.Logger) (*Plugin, error) {
	normalizedConfig := enterprisecfg.NormalizeLoadBalancerConfig(config)
	if normalizedConfig == nil {
		return nil, fmt.Errorf("load balancer config is required")
	}

	cfg := normalizeTrackerConfig(normalizedConfig.TrackerConfig)
	tracker := newTracker(cfg)
	seedBootstrapMetrics(tracker, normalizedConfig.Bootstrap)
	tracker.recomputeProfiles(time.Now(), true)

	return &Plugin{
		cfg:     cfg,
		policy:  normalizeRuntimePolicy(normalizedConfig),
		tracker: tracker,
		logger:  logger,
	}, nil
}

func (p *Plugin) GetName() string {
	return PluginName
}

func (p *Plugin) Cleanup() error {
	tracker := p.currentTracker()
	if tracker == nil {
		return nil
	}
	return tracker.cleanup()
}

func (p *Plugin) PreLLMHook(ctx *schemas.BifrostContext, req *schemas.BifrostRequest) (*schemas.BifrostRequest, *schemas.LLMPluginShortCircuit, error) {
	if ctx != nil {
		ctx.SetValue(startTimeKey, time.Now())
	}
	p.reorderFallbacks(ctx, req)
	return req, nil, nil
}

func (p *Plugin) PostLLMHook(ctx *schemas.BifrostContext, resp *schemas.BifrostResponse, bifrostErr *schemas.BifrostError) (*schemas.BifrostResponse, *schemas.BifrostError, error) {
	if ctx == nil {
		return resp, bifrostErr, nil
	}

	requestType, provider, model := bifrost.GetResponseFields(resp, bifrostErr)
	if bifrost.IsStreamRequestType(requestType) && !bifrost.IsFinalChunk(ctx) {
		return resp, bifrostErr, nil
	}
	if provider == "" {
		return resp, bifrostErr, nil
	}

	keyID := strings.TrimSpace(bifrost.GetStringFromContext(ctx, schemas.BifrostContextKeySelectedKeyID))
	latencyMs := extractLatencyMillis(ctx, resp)
	tracker := p.currentTracker()
	if tracker == nil {
		return resp, bifrostErr, nil
	}
	tracker.ObserveDirection(provider, model, latencyMs, bifrostErr == nil)
	if keyID != "" {
		tracker.Observe(provider, model, keyID, latencyMs, bifrostErr == nil)
	}

	return resp, bifrostErr, nil
}

func (p *Plugin) GetKeySelector() schemas.KeySelector {
	return p.selectKey
}

func (p *Plugin) Enabled() bool {
	return p.policySnapshot().enabled
}

func (p *Plugin) UpdateConfig(config *enterprisecfg.LoadBalancerConfig) error {
	if p == nil {
		return nil
	}

	normalizedConfig := enterprisecfg.NormalizeLoadBalancerConfig(config)
	if normalizedConfig == nil {
		return fmt.Errorf("load balancer config is required")
	}

	cfg := normalizeTrackerConfig(normalizedConfig.TrackerConfig)
	newTracker := newTracker(cfg)
	if previous := p.currentTracker(); previous != nil {
		previous.copyInto(newTracker)
	}
	seedBootstrapMetrics(newTracker, normalizedConfig.Bootstrap)
	newTracker.recomputeProfiles(time.Now(), true)

	p.mu.Lock()
	previousTracker := p.tracker
	p.cfg = cfg
	p.policy = normalizeRuntimePolicy(normalizedConfig)
	p.tracker = newTracker
	p.mu.Unlock()

	if previousTracker != nil {
		return previousTracker.cleanup()
	}
	return nil
}

func (p *Plugin) Snapshot(provider schemas.ModelProvider, model, keyID string) (RouteSnapshot, bool) {
	tracker := p.currentTracker()
	if tracker == nil {
		return RouteSnapshot{}, false
	}
	return tracker.Snapshot(provider, model, keyID)
}

func (p *Plugin) ListSnapshots(provider schemas.ModelProvider, model string) []RouteStatus {
	tracker := p.currentTracker()
	if tracker == nil {
		return nil
	}
	return tracker.ListSnapshots(provider, model)
}

func (p *Plugin) ListLocalSnapshots(provider schemas.ModelProvider, model string) []RouteStatus {
	tracker := p.currentTracker()
	if tracker == nil {
		return nil
	}
	return tracker.ListLocalSnapshots(provider, model)
}

func (p *Plugin) DirectionSnapshot(provider schemas.ModelProvider, model string) (DirectionSnapshot, bool) {
	tracker := p.currentTracker()
	if tracker == nil {
		return DirectionSnapshot{}, false
	}
	return tracker.DirectionSnapshot(provider, model)
}

func (p *Plugin) ListDirectionSnapshots(provider schemas.ModelProvider, model string) []DirectionStatus {
	tracker := p.currentTracker()
	if tracker == nil {
		return nil
	}
	return tracker.ListDirectionSnapshots(provider, model)
}

func (p *Plugin) ListLocalDirectionSnapshots(provider schemas.ModelProvider, model string) []DirectionStatus {
	tracker := p.currentTracker()
	if tracker == nil {
		return nil
	}
	return tracker.ListLocalDirectionSnapshots(provider, model)
}

func (p *Plugin) UpdateRemoteSnapshots(nodeID string, routes []RouteStatus, directions []DirectionStatus) {
	tracker := p.currentTracker()
	if tracker == nil {
		return
	}
	tracker.UpdateRemoteSnapshots(nodeID, routes, directions)
}

func (p *Plugin) PruneRemoteNode(nodeID string) {
	tracker := p.currentTracker()
	if tracker == nil {
		return
	}
	tracker.PruneRemoteNode(nodeID)
}

func (p *Plugin) currentTracker() *Tracker {
	if p == nil {
		return nil
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.tracker
}

func (p *Plugin) currentTrackerConfig() trackerConfig {
	if p == nil {
		return trackerConfig{}
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.cfg
}

func (p *Plugin) policySnapshot() runtimePolicy {
	if p == nil {
		return runtimePolicy{}
	}
	p.mu.RLock()
	defer p.mu.RUnlock()

	policy := p.policy
	if len(policy.providerAllowlist) > 0 {
		policy.providerAllowlist = mapsClone(policy.providerAllowlist)
	}
	if len(policy.modelAllowlist) > 0 {
		policy.modelAllowlist = mapStringSetClone(policy.modelAllowlist)
	}
	return policy
}

func (p *Plugin) selectKey(ctx *schemas.BifrostContext, keys []schemas.Key, provider schemas.ModelProvider, model string) (schemas.Key, error) {
	policy, scoped := p.resolveRequestPolicy(ctx, provider, model)
	if scoped {
		if !policy.shouldUseKeyBalancing() {
			return bifrost.WeightedRandomKeySelector(ctx, keys, provider, model)
		}
		tracker := p.currentTracker()
		if tracker == nil {
			return bifrost.WeightedRandomKeySelector(ctx, keys, provider, model)
		}
		return tracker.SelectKey(ctx, keys, provider, model)
	}

	legacyPolicy := p.policySnapshot()
	if !legacyPolicy.shouldUseKeyBalancing(provider, model) {
		return bifrost.WeightedRandomKeySelector(ctx, keys, provider, model)
	}

	tracker := p.currentTracker()
	if tracker == nil {
		return bifrost.WeightedRandomKeySelector(ctx, keys, provider, model)
	}
	return tracker.SelectKey(ctx, keys, provider, model)
}

func (p *Plugin) resolveRequestPolicy(ctx *schemas.BifrostContext, provider schemas.ModelProvider, model string) (requestPolicy, bool) {
	if scoped, ok := requestPolicyFromContext(ctx); ok {
		return scoped, true
	}

	legacy := p.policySnapshot()
	return requestPolicy{
		source:                  "global",
		enabled:                 legacy.enabled && legacy.allowsProvider(provider) && legacy.allowsModel(model),
		keyBalancingEnabled:     legacy.keyBalancingEnabled,
		directionRoutingEnabled: legacy.shouldUseDirectionRouting(ctx, model),
	}, false
}

func requestPolicyFromContext(ctx *schemas.BifrostContext) (requestPolicy, bool) {
	if ctx == nil {
		return requestPolicy{}, false
	}

	rawConfig, _ := ctx.Value(schemas.BifrostContextKeyGovernanceAdaptiveRoutingConfig).(map[string]any)
	rawTargets, _ := ctx.Value(schemas.BifrostContextKeyGovernanceAdaptiveRoutingTargets).([]configstoreTables.TableRoutingTarget)
	rawFallbacks, _ := ctx.Value(schemas.BifrostContextKeyGovernanceAdaptiveRoutingFallbacks).([]string)
	if rawConfig == nil && len(rawTargets) == 0 && len(rawFallbacks) == 0 {
		return requestPolicy{}, false
	}

	policy := requestPolicy{
		source:                  "routing_rule",
		enabled:                 true,
		keyBalancingEnabled:     true,
		directionRoutingEnabled: len(rawTargets) > 0,
		additionalFallbacks:     append([]string(nil), rawFallbacks...),
	}
	if value, ok := adaptiveConfigBool(rawConfig, "enabled"); ok {
		policy.enabled = value
	}
	if value, ok := adaptiveConfigBool(rawConfig, "key_balancing_enabled"); ok {
		policy.keyBalancingEnabled = value
	}
	if value, ok := adaptiveConfigBool(rawConfig, "direction_routing_enabled"); ok {
		policy.directionRoutingEnabled = value
	}
	for _, target := range rawTargets {
		policy.directionTargets = append(policy.directionTargets, directionTarget{
			Provider: schemas.ModelProvider(strings.TrimSpace(derefString(target.Provider))),
			Model:    strings.TrimSpace(derefString(target.Model)),
		})
	}
	return policy, true
}

func (p requestPolicy) shouldUseKeyBalancing() bool {
	return p.enabled && p.keyBalancingEnabled
}

func (p requestPolicy) shouldUseDirectionRouting() bool {
	return p.enabled && p.directionRoutingEnabled && len(p.directionTargets) > 0
}

func adaptiveConfigBool(config map[string]any, key string) (bool, bool) {
	if len(config) == 0 {
		return false, false
	}
	value, ok := config[key]
	if !ok {
		return false, false
	}
	switch typed := value.(type) {
	case bool:
		return typed, true
	default:
		return false, false
	}
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func normalizeRuntimePolicy(cfg *enterprisecfg.LoadBalancerConfig) runtimePolicy {
	normalized := enterprisecfg.NormalizeLoadBalancerConfig(cfg)
	if normalized == nil {
		return runtimePolicy{}
	}

	policy := runtimePolicy{
		enabled:                       normalized.Enabled,
		keyBalancingEnabled:           boolValue(normalized.KeyBalancingEnabled, true),
		directionRoutingEnabled:       boolValue(normalized.DirectionRoutingEnabled, false),
		directionRoutingForVirtualKey: boolValue(normalized.DirectionRoutingForVirtualKeys, false),
	}
	if len(normalized.ProviderAllowlist) > 0 {
		policy.providerAllowlist = make(map[schemas.ModelProvider]struct{}, len(normalized.ProviderAllowlist))
		for _, provider := range normalized.ProviderAllowlist {
			trimmed := strings.TrimSpace(strings.ToLower(provider))
			if trimmed == "" {
				continue
			}
			policy.providerAllowlist[schemas.ModelProvider(trimmed)] = struct{}{}
		}
	}
	if len(normalized.ModelAllowlist) > 0 {
		policy.modelAllowlist = make(map[string]struct{}, len(normalized.ModelAllowlist))
		for _, model := range normalized.ModelAllowlist {
			_, parsedModel := schemas.ParseModelString(strings.TrimSpace(model), "")
			normalizedModel := strings.TrimSpace(parsedModel)
			if normalizedModel == "" {
				normalizedModel = strings.TrimSpace(model)
			}
			if normalizedModel == "" {
				continue
			}
			policy.modelAllowlist[normalizedModel] = struct{}{}
		}
	}
	return policy
}

func (p runtimePolicy) shouldUseKeyBalancing(provider schemas.ModelProvider, model string) bool {
	return p.enabled && p.keyBalancingEnabled && p.allowsProvider(provider) && p.allowsModel(model)
}

func (p runtimePolicy) shouldUseDirectionRouting(ctx *schemas.BifrostContext, model string) bool {
	if !p.enabled || !p.directionRoutingEnabled || !p.allowsModel(model) {
		return false
	}
	if !p.directionRoutingForVirtualKey && hasGovernanceVirtualKey(ctx) {
		return false
	}
	return true
}

func (p runtimePolicy) allowsProvider(provider schemas.ModelProvider) bool {
	if len(p.providerAllowlist) == 0 {
		return true
	}
	_, ok := p.providerAllowlist[provider]
	return ok
}

func (p runtimePolicy) allowsModel(model string) bool {
	if len(p.modelAllowlist) == 0 {
		return true
	}
	_, parsedModel := schemas.ParseModelString(strings.TrimSpace(model), "")
	normalizedModel := strings.TrimSpace(parsedModel)
	if normalizedModel == "" {
		normalizedModel = strings.TrimSpace(model)
	}
	_, ok := p.modelAllowlist[normalizedModel]
	return ok
}

func boolValue(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

func hasGovernanceVirtualKey(ctx *schemas.BifrostContext) bool {
	if ctx == nil {
		return false
	}
	if strings.TrimSpace(bifrost.GetStringFromContext(ctx, schemas.BifrostContextKeyGovernanceVirtualKeyID)) != "" {
		return true
	}
	if strings.TrimSpace(bifrost.GetStringFromContext(ctx, schemas.BifrostContextKeyVirtualKey)) != "" {
		return true
	}
	return false
}

func mapsClone[K comparable, V any](source map[K]V) map[K]V {
	if len(source) == 0 {
		return nil
	}
	cloned := make(map[K]V, len(source))
	for key, value := range source {
		cloned[key] = value
	}
	return cloned
}

func mapStringSetClone(source map[string]struct{}) map[string]struct{} {
	return mapsClone(source)
}

func (t *Tracker) Observe(provider schemas.ModelProvider, model, keyID string, latencyMs float64, success bool) {
	if provider == "" || strings.TrimSpace(keyID) == "" {
		return
	}

	stats := t.getOrCreate(routeKey{provider: provider, model: model, keyID: keyID})
	stats.mu.Lock()
	defer stats.mu.Unlock()
	observeStats(stats, t.cfg, latencyMs, success)
	atomic.StoreUint32(&t.dirty, 1)
}

func (t *Tracker) ObserveDirection(provider schemas.ModelProvider, model string, latencyMs float64, success bool) {
	if provider == "" {
		return
	}

	stats := t.getOrCreateDirection(directionKey{provider: provider, model: model})
	stats.mu.Lock()
	defer stats.mu.Unlock()

	observeStats(stats, t.cfg, latencyMs, success)
	atomic.StoreUint32(&t.dirty, 1)
}

func (t *Tracker) Snapshot(provider schemas.ModelProvider, model, keyID string) (RouteSnapshot, bool) {
	stats, ok := t.get(routeKey{provider: provider, model: model, keyID: keyID})
	if !ok {
		return RouteSnapshot{}, false
	}

	stats.mu.RLock()
	defer stats.mu.RUnlock()

	return RouteSnapshot{
		Samples:             stats.samples,
		Successes:           stats.successes,
		Failures:            stats.failures,
		ConsecutiveFailures: stats.consecutiveFailures,
		ErrorEWMA:           stats.errorEWMA,
		LatencyEWMA:         stats.latencyEWMA,
		LastUpdated:         stats.lastUpdated,
	}, true
}

func (t *Tracker) DirectionSnapshot(provider schemas.ModelProvider, model string) (DirectionSnapshot, bool) {
	stats, ok := t.getDirection(directionKey{provider: provider, model: model})
	if !ok {
		return DirectionSnapshot{}, false
	}

	stats.mu.RLock()
	defer stats.mu.RUnlock()

	return DirectionSnapshot{
		Samples:             stats.samples,
		Successes:           stats.successes,
		Failures:            stats.failures,
		ConsecutiveFailures: stats.consecutiveFailures,
		ErrorEWMA:           stats.errorEWMA,
		LatencyEWMA:         stats.latencyEWMA,
		LastUpdated:         stats.lastUpdated,
	}, true
}

func (t *Tracker) adjustedWeight(provider schemas.ModelProvider, model string, key schemas.Key, baselineLatency float64) float64 {
	baseWeight := key.Weight
	if baseWeight <= 0 {
		baseWeight = 1
	}

	snapshot, ok := t.Snapshot(provider, model, key.ID)
	if !ok {
		return baseWeight
	}

	confidence := 1.0
	if t.cfg.minimumSamples > 0 {
		confidence = math.Min(1, float64(snapshot.Samples)/float64(t.cfg.minimumSamples))
	}

	errorFactor := clamp(1-(snapshot.ErrorEWMA*t.cfg.errorPenalty), 0.15, 1)
	latencyFactor := 1.0
	if baselineLatency > 0 && snapshot.LatencyEWMA > 0 {
		latencyRatio := baselineLatency / snapshot.LatencyEWMA
		latencyFactor = clamp(math.Pow(latencyRatio, t.cfg.latencyPenalty), 0.25, 1.25)
	}
	failureFactor := clamp(1-(float64(snapshot.ConsecutiveFailures)*t.cfg.consecutiveFailurePenalty), 0.1, 1)

	dynamicMultiplier := errorFactor * latencyFactor * failureFactor
	blendedMultiplier := ((1 - confidence) * 1.0) + (confidence * dynamicMultiplier)
	adjustedWeight := baseWeight * blendedMultiplier

	minWeight := math.Max(baseWeight*t.cfg.minWeightMultiplier, 0.01)
	maxWeight := math.Max(baseWeight*t.cfg.maxWeightMultiplier, minWeight)
	adjustedWeight = clamp(adjustedWeight, minWeight, maxWeight)

	if t.cfg.jitterRatio > 0 {
		jitter := 1 + ((rand.Float64()*2 - 1) * t.cfg.jitterRatio)
		adjustedWeight *= jitter
	}

	return clamp(adjustedWeight, minWeight, maxWeight)
}

func (t *Tracker) findBaselineLatency(provider schemas.ModelProvider, model string, keys []schemas.Key) float64 {
	best := 0.0
	for _, key := range keys {
		snapshot, ok := t.Snapshot(provider, model, key.ID)
		if !ok || snapshot.LatencyEWMA <= 0 {
			continue
		}
		if best == 0 || snapshot.LatencyEWMA < best {
			best = snapshot.LatencyEWMA
		}
	}
	return best
}

func (t *Tracker) get(route routeKey) (*routeStats, bool) {
	value, ok := t.routes.Load(route)
	if !ok {
		return nil, false
	}
	stats, ok := value.(*routeStats)
	return stats, ok
}

func (t *Tracker) getDirection(direction directionKey) (*routeStats, bool) {
	value, ok := t.directions.Load(direction)
	if !ok {
		return nil, false
	}
	stats, ok := value.(*routeStats)
	return stats, ok
}

func (t *Tracker) getOrCreate(route routeKey) *routeStats {
	if value, ok := t.routes.Load(route); ok {
		if stats, ok := value.(*routeStats); ok {
			return stats
		}
	}

	stats := &routeStats{}
	actual, _ := t.routes.LoadOrStore(route, stats)
	return actual.(*routeStats)
}

func (t *Tracker) getOrCreateDirection(direction directionKey) *routeStats {
	if value, ok := t.directions.Load(direction); ok {
		if stats, ok := value.(*routeStats); ok {
			return stats
		}
	}

	stats := &routeStats{}
	actual, _ := t.directions.LoadOrStore(direction, stats)
	return actual.(*routeStats)
}

func normalizeTrackerConfig(cfg *enterprisecfg.LoadBalancerTrackerConfig) trackerConfig {
	normalized := trackerConfig{
		ewmaAlpha:                 0.25,
		errorPenalty:              1.5,
		latencyPenalty:            0.6,
		consecutiveFailurePenalty: 0.15,
		minimumSamples:            10,
		explorationRatio:          0.25,
		jitterRatio:               0.05,
		minWeightMultiplier:       0.1,
		maxWeightMultiplier:       4.0,
		recomputeInterval:         5 * time.Second,
		degradedErrorThreshold:    0.02,
		failedErrorThreshold:      0.05,
		failedConsecutiveFailures: 5,
		recoveryHalfLife:          30 * time.Second,
		weightFloor:               1,
		weightCeiling:             1000,
		explorationFloorRatio:     0.1,
		recoverySuccessThreshold:  3,
	}

	if cfg == nil {
		return normalized
	}

	if cfg.EWMAAlpha > 0 && cfg.EWMAAlpha <= 1 {
		normalized.ewmaAlpha = cfg.EWMAAlpha
	}
	if cfg.ErrorPenalty > 0 {
		normalized.errorPenalty = cfg.ErrorPenalty
	}
	if cfg.LatencyPenalty > 0 {
		normalized.latencyPenalty = cfg.LatencyPenalty
	}
	if cfg.ConsecutiveFailurePenalty > 0 {
		normalized.consecutiveFailurePenalty = cfg.ConsecutiveFailurePenalty
	}
	if cfg.MinimumSamples > 0 {
		normalized.minimumSamples = cfg.MinimumSamples
	}
	if cfg.ExplorationRatio >= 0 && cfg.ExplorationRatio <= 1 {
		normalized.explorationRatio = cfg.ExplorationRatio
	}
	if cfg.JitterRatio >= 0 && cfg.JitterRatio <= 1 {
		normalized.jitterRatio = cfg.JitterRatio
	}
	if cfg.MinWeightMultiplier > 0 {
		normalized.minWeightMultiplier = cfg.MinWeightMultiplier
	}
	if cfg.MaxWeightMultiplier > 0 {
		normalized.maxWeightMultiplier = cfg.MaxWeightMultiplier
	}
	if cfg.RecomputeIntervalSeconds > 0 {
		normalized.recomputeInterval = time.Duration(cfg.RecomputeIntervalSeconds) * time.Second
	}
	if cfg.DegradedErrorThreshold > 0 && cfg.DegradedErrorThreshold < 1 {
		normalized.degradedErrorThreshold = cfg.DegradedErrorThreshold
	}
	if cfg.FailedErrorThreshold > 0 && cfg.FailedErrorThreshold < 1 {
		normalized.failedErrorThreshold = cfg.FailedErrorThreshold
	}
	if cfg.FailedConsecutiveFailures > 0 {
		normalized.failedConsecutiveFailures = int64(cfg.FailedConsecutiveFailures)
	}
	if cfg.RecoveryHalfLifeSeconds > 0 {
		normalized.recoveryHalfLife = time.Duration(cfg.RecoveryHalfLifeSeconds) * time.Second
	}
	if cfg.WeightFloor > 0 {
		normalized.weightFloor = cfg.WeightFloor
	}
	if cfg.WeightCeiling > 0 {
		normalized.weightCeiling = cfg.WeightCeiling
	}

	return normalized
}

func seedBootstrapMetrics(tracker *Tracker, bootstrap *enterprisecfg.LoadBalancerBootstrap) {
	if tracker == nil || bootstrap == nil {
		return
	}

	for routeID, metrics := range bootstrap.RouteMetrics {
		parts := strings.SplitN(routeID, "/", 3)
		if len(parts) != 3 {
			continue
		}

		stats := tracker.getOrCreate(routeKey{
			provider: schemas.ModelProvider(parts[0]),
			model:    parts[1],
			keyID:    parts[2],
		})

		stats.mu.Lock()
		stats.samples = metrics.SampleCount
		stats.errorEWMA = clamp(metrics.ErrorRate, 0, 1)
		stats.latencyEWMA = math.Max(metrics.LatencyMs, 1)
		stats.consecutiveFailures = maxInt64(metrics.ConsecutiveFailures, 0)
		stats.lastUpdated = time.Now()
		stats.mu.Unlock()
	}

	for directionID, metrics := range bootstrap.DirectionMetrics {
		parts := strings.SplitN(directionID, "/", 2)
		if len(parts) != 2 {
			continue
		}

		snapshot, ok := bootstrapSnapshot(metrics)
		if !ok {
			continue
		}
		stats := tracker.getOrCreateDirection(directionKey{
			provider: schemas.ModelProvider(parts[0]),
			model:    parts[1],
		})
		stats.mu.Lock()
		applyBootstrapSnapshot(stats, snapshot)
		stats.mu.Unlock()
	}
}

func extractLatencyMillis(ctx *schemas.BifrostContext, resp *schemas.BifrostResponse) float64 {
	if resp != nil {
		if latency := resp.GetExtraFields().Latency; latency > 0 {
			return float64(latency)
		}
	}

	startTime, ok := ctx.Value(startTimeKey).(time.Time)
	if !ok || startTime.IsZero() {
		return 1
	}

	elapsed := time.Since(startTime).Milliseconds()
	if elapsed <= 0 {
		return 1
	}

	return float64(elapsed)
}

func ewma(current, sample, alpha float64) float64 {
	return current + (alpha * (sample - current))
}

func clamp(value, minValue, maxValue float64) float64 {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func maxInt64(value, floor int64) int64 {
	if value < floor {
		return floor
	}
	return value
}

func maxInt(value, floor int) int {
	if value < floor {
		return floor
	}
	return value
}

func observeStats(stats *routeStats, cfg trackerConfig, latencyMs float64, success bool) {
	now := time.Now()
	errorSample := 0.0
	if !success {
		errorSample = 1.0
	}
	if latencyMs <= 0 {
		latencyMs = 1
	}

	if stats.samples == 0 {
		stats.errorEWMA = errorSample
		stats.latencyEWMA = latencyMs
	} else {
		stats.errorEWMA = ewma(stats.errorEWMA, errorSample, cfg.ewmaAlpha)
		stats.latencyEWMA = ewma(stats.latencyEWMA, latencyMs, cfg.ewmaAlpha)
	}

	stats.samples++
	if success {
		stats.successes++
		stats.lastSuccess = now
		if stats.consecutiveFailures > 0 || !stats.lastFailure.IsZero() {
			if stats.recoveryStarted.IsZero() {
				stats.recoveryStarted = now
			}
			stats.recoverySuccesses++
		}
		stats.consecutiveFailures = 0
	} else {
		stats.failures++
		stats.lastFailure = now
		stats.consecutiveFailures++
		stats.recoveryStarted = time.Time{}
		stats.recoverySuccesses = 0
	}
	stats.lastUpdated = now
}

func applyBootstrapSnapshot(stats *routeStats, snapshot DirectionSnapshot) {
	stats.samples = snapshot.Samples
	stats.successes = snapshot.Successes
	stats.failures = snapshot.Failures
	stats.errorEWMA = clamp(snapshot.ErrorEWMA, 0, 1)
	stats.latencyEWMA = math.Max(snapshot.LatencyEWMA, 1)
	stats.consecutiveFailures = maxInt64(snapshot.ConsecutiveFailures, 0)
	if snapshot.LastUpdated.IsZero() {
		stats.lastUpdated = time.Now()
		return
	}
	stats.lastUpdated = snapshot.LastUpdated
}

func bootstrapSnapshot(values map[string]any) (DirectionSnapshot, bool) {
	if len(values) == 0 {
		return DirectionSnapshot{}, false
	}

	snapshot := DirectionSnapshot{
		ErrorEWMA:           getFloatFromBootstrap(values, "error_rate"),
		LatencyEWMA:         math.Max(getFloatFromBootstrap(values, "latency_ms"), 1),
		ConsecutiveFailures: getInt64FromBootstrap(values, "consecutive_failures"),
		Samples:             getInt64FromBootstrap(values, "sample_count"),
	}
	return snapshot, snapshot.Samples > 0 || snapshot.ConsecutiveFailures > 0 || snapshot.ErrorEWMA > 0 || snapshot.LatencyEWMA > 1
}

func getFloatFromBootstrap(values map[string]any, key string) float64 {
	value, ok := values[key]
	if !ok {
		return 0
	}
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case int32:
		return float64(typed)
	default:
		return 0
	}
}

func getInt64FromBootstrap(values map[string]any, key string) int64 {
	value, ok := values[key]
	if !ok {
		return 0
	}
	switch typed := value.(type) {
	case int:
		return int64(typed)
	case int64:
		return typed
	case int32:
		return int64(typed)
	case float64:
		return int64(typed)
	case float32:
		return int64(typed)
	default:
		return 0
	}
}

func (t *Tracker) directionScore(snapshot DirectionSnapshot, baselineLatency float64) float64 {
	confidence := 1.0
	if t.cfg.minimumSamples > 0 {
		confidence = math.Min(1, float64(snapshot.Samples)/float64(t.cfg.minimumSamples))
	}

	errorFactor := clamp(1-(snapshot.ErrorEWMA*t.cfg.errorPenalty), 0.15, 1)
	latencyFactor := 1.0
	if baselineLatency > 0 && snapshot.LatencyEWMA > 0 {
		latencyRatio := baselineLatency / snapshot.LatencyEWMA
		latencyFactor = clamp(math.Pow(latencyRatio, t.cfg.latencyPenalty), 0.25, 1.25)
	}
	failureFactor := clamp(1-(float64(snapshot.ConsecutiveFailures)*t.cfg.consecutiveFailurePenalty), 0.1, 1)
	dynamicMultiplier := errorFactor * latencyFactor * failureFactor
	return ((1 - confidence) * 1.0) + (confidence * dynamicMultiplier)
}

func (p *Plugin) reorderFallbacks(ctx *schemas.BifrostContext, req *schemas.BifrostRequest) {
	tracker := p.currentTracker()
	if tracker == nil || req == nil {
		return
	}
	if ctx != nil && bifrost.GetIntFromContext(ctx, schemas.BifrostContextKeyFallbackIndex) > 0 {
		return
	}
	primaryProvider, primaryModel, fallbacks := req.GetRequestFields()
	policy, scoped := p.resolveRequestPolicy(ctx, primaryProvider, primaryModel)
	if scoped {
		if !policy.shouldUseDirectionRouting() {
			return
		}
	} else if !p.policySnapshot().shouldUseDirectionRouting(ctx, primaryModel) {
		return
	}
	if len(fallbacks) < 2 {
		return
	}

	reordered, changed := tracker.ReorderFallbacks(fallbacks)
	if !changed {
		return
	}

	req.SetFallbacks(reordered)
	if ctx != nil {
		ctx.AppendRoutingEngineLog(
			schemas.RoutingEngineLoadbalancing,
			fmt.Sprintf(
				"Reordered %d fallback providers for %s/%s: %s",
				len(reordered),
				primaryProvider,
				primaryModel,
				formatFallbacks(reordered),
			),
		)
	}
}

func formatFallbacks(fallbacks []schemas.Fallback) string {
	if len(fallbacks) == 0 {
		return "[]"
	}

	formatted := make([]string, 0, len(fallbacks))
	for _, fallback := range fallbacks {
		formatted = append(formatted, fmt.Sprintf("%s/%s", fallback.Provider, fallback.Model))
	}
	return "[" + strings.Join(formatted, ", ") + "]"
}
