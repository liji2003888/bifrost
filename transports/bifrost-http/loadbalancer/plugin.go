package loadbalancer

import (
	"fmt"
	"math"
	"math/rand"
	"strings"
	"sync"
	"time"

	bifrost "github.com/maximhq/bifrost/core"
	"github.com/maximhq/bifrost/core/schemas"
	enterprisecfg "github.com/maximhq/bifrost/transports/bifrost-http/enterprise"
)

const (
	PluginName = "loadbalancer"
)

const startTimeKey schemas.BifrostContextKey = "bf-loadbalancer-start-time"

type Config = enterprisecfg.LoadBalancerConfig

type Plugin struct {
	cfg     trackerConfig
	tracker *Tracker
	logger  schemas.Logger
}

type RouteSnapshot struct {
	Samples             int64
	Successes           int64
	Failures            int64
	ConsecutiveFailures int64
	ErrorEWMA           float64
	LatencyEWMA         float64
	LastUpdated         time.Time
}

type routeKey struct {
	provider schemas.ModelProvider
	model    string
	keyID    string
}

type routeStats struct {
	mu                  sync.RWMutex
	samples             int64
	successes           int64
	failures            int64
	consecutiveFailures int64
	errorEWMA           float64
	latencyEWMA         float64
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
}

type Tracker struct {
	cfg    trackerConfig
	routes sync.Map
}

func Init(config *enterprisecfg.LoadBalancerConfig, logger schemas.Logger) (*Plugin, error) {
	if config == nil {
		return nil, fmt.Errorf("load balancer config is required")
	}

	cfg := normalizeTrackerConfig(config.TrackerConfig)
	tracker := &Tracker{cfg: cfg}
	seedBootstrapMetrics(tracker, config.Bootstrap)

	return &Plugin{
		cfg:     cfg,
		tracker: tracker,
		logger:  logger,
	}, nil
}

func (p *Plugin) GetName() string {
	return PluginName
}

func (p *Plugin) Cleanup() error {
	return nil
}

func (p *Plugin) PreLLMHook(ctx *schemas.BifrostContext, req *schemas.BifrostRequest) (*schemas.BifrostRequest, *schemas.LLMPluginShortCircuit, error) {
	if ctx != nil {
		ctx.SetValue(startTimeKey, time.Now())
	}
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

	keyID := strings.TrimSpace(bifrost.GetStringFromContext(ctx, schemas.BifrostContextKeySelectedKeyID))
	if keyID == "" || provider == "" {
		return resp, bifrostErr, nil
	}

	latencyMs := extractLatencyMillis(ctx, resp)
	p.tracker.Observe(provider, model, keyID, latencyMs, bifrostErr == nil)

	return resp, bifrostErr, nil
}

func (p *Plugin) GetKeySelector() schemas.KeySelector {
	return p.tracker.SelectKey
}

func (p *Plugin) Snapshot(provider schemas.ModelProvider, model, keyID string) (RouteSnapshot, bool) {
	return p.tracker.Snapshot(provider, model, keyID)
}

func (t *Tracker) Observe(provider schemas.ModelProvider, model, keyID string, latencyMs float64, success bool) {
	if provider == "" || strings.TrimSpace(keyID) == "" {
		return
	}

	stats := t.getOrCreate(routeKey{provider: provider, model: model, keyID: keyID})
	stats.mu.Lock()
	defer stats.mu.Unlock()

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
		stats.errorEWMA = ewma(stats.errorEWMA, errorSample, t.cfg.ewmaAlpha)
		stats.latencyEWMA = ewma(stats.latencyEWMA, latencyMs, t.cfg.ewmaAlpha)
	}

	stats.samples++
	if success {
		stats.successes++
		stats.consecutiveFailures = 0
	} else {
		stats.failures++
		stats.consecutiveFailures++
	}
	stats.lastUpdated = time.Now()
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

func (t *Tracker) SelectKey(ctx *schemas.BifrostContext, keys []schemas.Key, providerKey schemas.ModelProvider, model string) (schemas.Key, error) {
	if len(keys) == 0 {
		return schemas.Key{}, fmt.Errorf("no keys available for provider %s", providerKey)
	}
	if len(keys) == 1 {
		return keys[0], nil
	}

	if rand.Float64() < t.cfg.explorationRatio {
		return bifrost.WeightedRandomKeySelector(ctx, keys, providerKey, model)
	}

	weightedKeys := make([]schemas.Key, len(keys))
	copy(weightedKeys, keys)

	baselineLatency := t.findBaselineLatency(providerKey, model, keys)
	for i := range weightedKeys {
		weightedKeys[i].Weight = t.adjustedWeight(providerKey, model, keys[i], baselineLatency)
	}

	return bifrost.WeightedRandomKeySelector(ctx, weightedKeys, providerKey, model)
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

func normalizeTrackerConfig(cfg *enterprisecfg.LoadBalancerTrackerConfig) trackerConfig {
	normalized := trackerConfig{
		ewmaAlpha:                 0.25,
		errorPenalty:              1.5,
		latencyPenalty:            0.6,
		consecutiveFailurePenalty: 0.15,
		minimumSamples:            10,
		explorationRatio:          0.15,
		jitterRatio:               0.05,
		minWeightMultiplier:       0.1,
		maxWeightMultiplier:       4.0,
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
