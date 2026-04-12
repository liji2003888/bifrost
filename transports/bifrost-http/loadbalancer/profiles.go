package loadbalancer

import (
	"fmt"
	"math"
	"math/rand"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	bifrost "github.com/maximhq/bifrost/core"
	"github.com/maximhq/bifrost/core/schemas"
)

type HealthState string

const (
	HealthStateHealthy    HealthState = "healthy"
	HealthStateDegraded   HealthState = "degraded"
	HealthStateFailed     HealthState = "failed"
	HealthStateRecovering HealthState = "recovering"
)

type routeProfile struct {
	State                HealthState
	Score                float64
	Weight               int
	ExpectedTrafficShare float64
	ActualTrafficShare   float64
	UpdatedAt            time.Time
}

type directionProfile struct {
	State                HealthState
	Score                float64
	Weight               int
	ExpectedTrafficShare float64
	ActualTrafficShare   float64
	UpdatedAt            time.Time
}

func newTracker(cfg trackerConfig) *Tracker {
	tracker := &Tracker{
		cfg:    cfg,
		stopCh: make(chan struct{}),
	}
	atomic.StoreUint32(&tracker.dirty, 1)
	tracker.recomputeProfiles(time.Now(), true)
	if cfg.recomputeInterval > 0 {
		tracker.wg.Add(1)
		go tracker.recomputeLoop()
	}
	return tracker
}

func (t *Tracker) cleanup() error {
	if t == nil {
		return nil
	}
	t.stopOnce.Do(func() {
		close(t.stopCh)
	})
	t.wg.Wait()
	return nil
}

func (t *Tracker) copyInto(target *Tracker) {
	if t == nil || target == nil {
		return
	}

	t.routes.Range(func(key, value any) bool {
		route, ok := key.(routeKey)
		if !ok {
			return true
		}
		stats, ok := value.(*routeStats)
		if !ok {
			return true
		}
		cloned := cloneRouteStats(stats)
		target.routes.Store(route, cloned)
		return true
	})

	t.directions.Range(func(key, value any) bool {
		direction, ok := key.(directionKey)
		if !ok {
			return true
		}
		stats, ok := value.(*routeStats)
		if !ok {
			return true
		}
		cloned := cloneRouteStats(stats)
		target.directions.Store(direction, cloned)
		return true
	})

	atomic.StoreUint32(&target.dirty, 1)
}

func cloneRouteStats(source *routeStats) *routeStats {
	if source == nil {
		return &routeStats{}
	}
	source.mu.RLock()
	defer source.mu.RUnlock()

	return &routeStats{
		samples:             source.samples,
		successes:           source.successes,
		failures:            source.failures,
		consecutiveFailures: source.consecutiveFailures,
		recoverySuccesses:   source.recoverySuccesses,
		errorEWMA:           source.errorEWMA,
		latencyEWMA:         source.latencyEWMA,
		lastSuccess:         source.lastSuccess,
		lastFailure:         source.lastFailure,
		recoveryStarted:     source.recoveryStarted,
		lastUpdated:         source.lastUpdated,
	}
}

func (t *Tracker) recomputeLoop() {
	defer t.wg.Done()
	ticker := time.NewTicker(t.cfg.recomputeInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if atomic.LoadUint32(&t.dirty) == 0 {
				continue
			}
			t.recomputeProfiles(time.Now(), false)
		case <-t.stopCh:
			return
		}
	}
}

func (t *Tracker) ensureProfilesFresh(force bool) {
	if t == nil {
		return
	}
	now := time.Now()
	last := atomic.LoadInt64(&t.lastRecomputeUnixNs)
	if !force && last > 0 && t.cfg.recomputeInterval > 0 && now.Sub(time.Unix(0, last)) < t.cfg.recomputeInterval {
		return
	}
	t.recomputeProfiles(now, force)
}

func (t *Tracker) recomputeProfiles(now time.Time, force bool) {
	if t == nil {
		return
	}
	if !force && atomic.LoadUint32(&t.dirty) == 0 {
		return
	}

	t.recomputeMu.Lock()
	defer t.recomputeMu.Unlock()

	if !force {
		last := atomic.LoadInt64(&t.lastRecomputeUnixNs)
		if last > 0 && t.cfg.recomputeInterval > 0 && now.Sub(time.Unix(0, last)) < t.cfg.recomputeInterval {
			return
		}
		if atomic.LoadUint32(&t.dirty) == 0 {
			return
		}
	}

	routeInputs := make([]struct {
		key      routeKey
		stats    *routeStats
		snapshot RouteSnapshot
	}, 0)
	t.routes.Range(func(key, value any) bool {
		route, ok := key.(routeKey)
		if !ok {
			return true
		}
		stats, ok := value.(*routeStats)
		if !ok {
			return true
		}
		stats.mu.RLock()
		snapshot := RouteSnapshot{
			Samples:             stats.samples,
			Successes:           stats.successes,
			Failures:            stats.failures,
			ConsecutiveFailures: stats.consecutiveFailures,
			ErrorEWMA:           stats.errorEWMA,
			LatencyEWMA:         stats.latencyEWMA,
			LastUpdated:         stats.lastUpdated,
		}
		stats.mu.RUnlock()
		routeInputs = append(routeInputs, struct {
			key      routeKey
			stats    *routeStats
			snapshot RouteSnapshot
		}{
			key:      route,
			stats:    stats,
			snapshot: snapshot,
		})
		return true
	})

	directionInputs := make([]struct {
		key      directionKey
		stats    *routeStats
		snapshot DirectionSnapshot
	}, 0)
	t.directions.Range(func(key, value any) bool {
		direction, ok := key.(directionKey)
		if !ok {
			return true
		}
		stats, ok := value.(*routeStats)
		if !ok {
			return true
		}
		stats.mu.RLock()
		snapshot := DirectionSnapshot{
			Samples:             stats.samples,
			Successes:           stats.successes,
			Failures:            stats.failures,
			ConsecutiveFailures: stats.consecutiveFailures,
			ErrorEWMA:           stats.errorEWMA,
			LatencyEWMA:         stats.latencyEWMA,
			LastUpdated:         stats.lastUpdated,
		}
		stats.mu.RUnlock()
		directionInputs = append(directionInputs, struct {
			key      directionKey
			stats    *routeStats
			snapshot DirectionSnapshot
		}{
			key:      direction,
			stats:    stats,
			snapshot: snapshot,
		})
		return true
	})

	t.rebuildRouteProfiles(routeInputs, now)
	t.rebuildDirectionProfiles(directionInputs, now)

	atomic.StoreUint32(&t.dirty, 0)
	atomic.StoreInt64(&t.lastRecomputeUnixNs, now.UnixNano())
}

func (t *Tracker) rebuildRouteProfiles(inputs []struct {
	key      routeKey
	stats    *routeStats
	snapshot RouteSnapshot
}, now time.Time) {
	type groupedRoute struct {
		key      routeKey
		stats    *routeStats
		snapshot RouteSnapshot
	}

	grouped := make(map[string][]groupedRoute)
	for _, input := range inputs {
		groupID := fmt.Sprintf("%s/%s", input.key.provider, input.key.model)
		grouped[groupID] = append(grouped[groupID], groupedRoute{
			key:      input.key,
			stats:    input.stats,
			snapshot: input.snapshot,
		})
	}

	for _, routes := range grouped {
		baselineLatency := 0.0
		totalSamples := int64(0)
		for _, route := range routes {
			totalSamples += route.snapshot.Samples
			if route.snapshot.LatencyEWMA <= 0 {
				continue
			}
			if baselineLatency == 0 || route.snapshot.LatencyEWMA < baselineLatency {
				baselineLatency = route.snapshot.LatencyEWMA
			}
		}

		precomputed := make([]routeProfile, len(routes))
		totalWeight := 0
		for i, route := range routes {
			precomputed[i] = t.buildRouteProfile(route.stats, route.snapshot, baselineLatency, totalSamples, 0, now)
			totalWeight += precomputed[i].Weight
		}

		for i, route := range routes {
			actualShare := 0.0
			if totalSamples > 0 {
				actualShare = float64(route.snapshot.Samples) / float64(totalSamples)
			}
			expectedShare := 0.0
			if totalWeight > 0 {
				expectedShare = float64(precomputed[i].Weight) / float64(totalWeight)
			}
			utilPenalty := t.utilizationPenalty(actualShare, expectedShare)
			score := clamp(precomputed[i].Score+(utilPenalty*0.05), 0, 1)
			weight := t.weightForScore(score, precomputed[i].State)
			profile := routeProfile{
				State:                precomputed[i].State,
				Score:                score,
				Weight:               weight,
				ExpectedTrafficShare: expectedShare,
				ActualTrafficShare:   actualShare,
				UpdatedAt:            now,
			}
			t.routeProfiles.Store(route.key, profile)
		}
	}
}

func (t *Tracker) rebuildDirectionProfiles(inputs []struct {
	key      directionKey
	stats    *routeStats
	snapshot DirectionSnapshot
}, now time.Time) {
	type groupedDirection struct {
		key      directionKey
		stats    *routeStats
		snapshot DirectionSnapshot
	}

	grouped := make(map[string][]groupedDirection)
	for _, input := range inputs {
		grouped[input.key.model] = append(grouped[input.key.model], groupedDirection{
			key:      input.key,
			stats:    input.stats,
			snapshot: input.snapshot,
		})
	}

	for _, directions := range grouped {
		baselineLatency := 0.0
		totalSamples := int64(0)
		for _, direction := range directions {
			totalSamples += direction.snapshot.Samples
			if direction.snapshot.LatencyEWMA <= 0 {
				continue
			}
			if baselineLatency == 0 || direction.snapshot.LatencyEWMA < baselineLatency {
				baselineLatency = direction.snapshot.LatencyEWMA
			}
		}

		precomputed := make([]directionProfile, len(directions))
		totalWeight := 0
		for i, direction := range directions {
			precomputed[i] = t.buildDirectionProfile(direction.stats, direction.snapshot, baselineLatency, totalSamples, 0, now)
			totalWeight += precomputed[i].Weight
		}

		for i, direction := range directions {
			actualShare := 0.0
			if totalSamples > 0 {
				actualShare = float64(direction.snapshot.Samples) / float64(totalSamples)
			}
			expectedShare := 0.0
			if totalWeight > 0 {
				expectedShare = float64(precomputed[i].Weight) / float64(totalWeight)
			}
			utilPenalty := t.utilizationPenalty(actualShare, expectedShare)
			score := clamp(precomputed[i].Score+(utilPenalty*0.05), 0, 1)
			weight := t.weightForScore(score, precomputed[i].State)
			profile := directionProfile{
				State:                precomputed[i].State,
				Score:                score,
				Weight:               weight,
				ExpectedTrafficShare: expectedShare,
				ActualTrafficShare:   actualShare,
				UpdatedAt:            now,
			}
			t.directionProfiles.Store(direction.key, profile)
		}
	}
}

func (t *Tracker) buildRouteProfile(stats *routeStats, snapshot RouteSnapshot, baselineLatency float64, totalSamples int64, utilizationPenalty float64, now time.Time) routeProfile {
	latencyRatio := 1.0
	if baselineLatency > 0 && snapshot.LatencyEWMA > 0 {
		latencyRatio = snapshot.LatencyEWMA / baselineLatency
	}
	state := t.determineHealthState(stats, snapshot.ErrorEWMA, snapshot.ConsecutiveFailures, latencyRatio, now)
	score := t.scoreSnapshot(snapshot.Samples, snapshot.ErrorEWMA, snapshot.ConsecutiveFailures, latencyRatio, utilizationPenalty, t.recoveryMomentum(stats, now))
	actualShare := 0.0
	if totalSamples > 0 {
		actualShare = float64(snapshot.Samples) / float64(totalSamples)
	}
	return routeProfile{
		State:              state,
		Score:              score,
		Weight:             t.weightForScore(score, state),
		ActualTrafficShare: actualShare,
		UpdatedAt:          now,
	}
}

func (t *Tracker) buildDirectionProfile(stats *routeStats, snapshot DirectionSnapshot, baselineLatency float64, totalSamples int64, utilizationPenalty float64, now time.Time) directionProfile {
	latencyRatio := 1.0
	if baselineLatency > 0 && snapshot.LatencyEWMA > 0 {
		latencyRatio = snapshot.LatencyEWMA / baselineLatency
	}
	state := t.determineHealthState(stats, snapshot.ErrorEWMA, snapshot.ConsecutiveFailures, latencyRatio, now)
	score := t.scoreSnapshot(snapshot.Samples, snapshot.ErrorEWMA, snapshot.ConsecutiveFailures, latencyRatio, utilizationPenalty, t.recoveryMomentum(stats, now))
	actualShare := 0.0
	if totalSamples > 0 {
		actualShare = float64(snapshot.Samples) / float64(totalSamples)
	}
	return directionProfile{
		State:              state,
		Score:              score,
		Weight:             t.weightForScore(score, state),
		ActualTrafficShare: actualShare,
		UpdatedAt:          now,
	}
}

func (t *Tracker) determineHealthState(stats *routeStats, errorEWMA float64, consecutiveFailures int64, latencyRatio float64, now time.Time) HealthState {
	if stats != nil && !stats.recoveryStarted.IsZero() && stats.lastSuccess.After(stats.lastFailure) {
		if errorEWMA < t.cfg.degradedErrorThreshold && stats.recoverySuccesses >= t.cfg.recoverySuccessThreshold {
			return HealthStateHealthy
		}
		return HealthStateRecovering
	}
	if consecutiveFailures >= t.cfg.failedConsecutiveFailures || errorEWMA >= t.cfg.failedErrorThreshold {
		return HealthStateFailed
	}
	if errorEWMA >= t.cfg.degradedErrorThreshold || latencyRatio >= 1.75 {
		return HealthStateDegraded
	}
	return HealthStateHealthy
}

func (t *Tracker) scoreSnapshot(samples int64, errorEWMA float64, consecutiveFailures int64, latencyRatio float64, utilizationPenalty float64, momentum float64) float64 {
	confidence := 1.0
	if t.cfg.minimumSamples > 0 {
		confidence = math.Min(1, float64(samples)/float64(t.cfg.minimumSamples))
	}

	errorComponent := clamp(errorEWMA/max(t.cfg.failedErrorThreshold, 0.001), 0, 1)
	latencyComponent := 0.0
	if latencyRatio > 1 {
		latencyComponent = clamp((latencyRatio-1)/2, 0, 1)
	}
	failureComponent := 0.0
	if t.cfg.failedConsecutiveFailures > 0 {
		failureComponent = clamp(float64(consecutiveFailures)/float64(t.cfg.failedConsecutiveFailures), 0, 1)
	}
	score := (errorComponent * 0.50) + (latencyComponent * 0.20) + (failureComponent * 0.25) + (utilizationPenalty * 0.05)
	score = ((1 - confidence) * 0.5) + (confidence * score)
	score -= momentum * 0.15
	return clamp(score, 0, 1)
}

func (t *Tracker) utilizationPenalty(actualShare, expectedShare float64) float64 {
	if actualShare <= 0 || expectedShare <= 0 || actualShare <= expectedShare {
		return 0
	}
	return clamp((actualShare-expectedShare)/expectedShare, 0, 1)
}

func (t *Tracker) recoveryMomentum(stats *routeStats, now time.Time) float64 {
	if stats == nil || stats.recoveryStarted.IsZero() || !stats.lastSuccess.After(stats.lastFailure) {
		return 0
	}
	if t.cfg.recoveryHalfLife <= 0 {
		return 0
	}
	elapsed := now.Sub(stats.recoveryStarted)
	if elapsed < 0 {
		elapsed = 0
	}
	// Reach ~90% recovery after one configured half-life window.
	timeProgress := 1 - math.Exp(-math.Ln10*float64(elapsed)/float64(t.cfg.recoveryHalfLife))
	sampleProgress := 0.0
	if t.cfg.recoverySuccessThreshold > 0 {
		sampleProgress = math.Min(1, float64(stats.recoverySuccesses)/float64(t.cfg.recoverySuccessThreshold))
	}
	return clamp(math.Max(timeProgress, sampleProgress), 0, 0.9)
}

func (t *Tracker) weightForScore(score float64, state HealthState) int {
	if state == HealthStateFailed {
		return 0
	}
	weight := int(math.Round(float64(t.cfg.weightFloor) + ((1 - score) * float64(t.cfg.weightCeiling-t.cfg.weightFloor))))
	if state == HealthStateRecovering {
		weight = maxInt(weight, int(math.Round(float64(t.cfg.weightFloor)*2)))
	}
	return clampInt(weight, t.cfg.weightFloor, t.cfg.weightCeiling)
}

func (t *Tracker) routeProfile(route routeKey) (routeProfile, bool) {
	value, ok := t.routeProfiles.Load(route)
	if !ok {
		return routeProfile{}, false
	}
	profile, ok := value.(routeProfile)
	return profile, ok
}

func (t *Tracker) directionProfile(direction directionKey) (directionProfile, bool) {
	value, ok := t.directionProfiles.Load(direction)
	if !ok {
		return directionProfile{}, false
	}
	profile, ok := value.(directionProfile)
	return profile, ok
}

func (t *Tracker) ListSnapshots(provider schemas.ModelProvider, model string) []RouteStatus {
	if t == nil {
		return nil
	}
	t.ensureProfilesFresh(false)

	snapshots := make([]RouteStatus, 0)
	t.routes.Range(func(key, value any) bool {
		route, ok := key.(routeKey)
		if !ok {
			return true
		}
		if provider != "" && route.provider != provider {
			return true
		}
		if model != "" && route.model != model {
			return true
		}

		stats, ok := value.(*routeStats)
		if !ok {
			return true
		}
		stats.mu.RLock()
		status := RouteStatus{
			Provider: route.provider,
			Model:    route.model,
			KeyID:    route.keyID,
			RouteSnapshot: RouteSnapshot{
				Samples:             stats.samples,
				Successes:           stats.successes,
				Failures:            stats.failures,
				ConsecutiveFailures: stats.consecutiveFailures,
				ErrorEWMA:           stats.errorEWMA,
				LatencyEWMA:         stats.latencyEWMA,
				LastUpdated:         stats.lastUpdated,
			},
		}
		stats.mu.RUnlock()
		if profile, ok := t.routeProfile(route); ok {
			status.State = profile.State
			status.Score = profile.Score
			status.Weight = profile.Weight
			status.ExpectedTrafficShare = profile.ExpectedTrafficShare
			status.ActualTrafficShare = profile.ActualTrafficShare
		}
		snapshots = append(snapshots, status)
		return true
	})

	slices.SortFunc(snapshots, func(a, b RouteStatus) int {
		if cmp := strings.Compare(string(a.Provider), string(b.Provider)); cmp != 0 {
			return cmp
		}
		if cmp := strings.Compare(a.Model, b.Model); cmp != 0 {
			return cmp
		}
		return strings.Compare(a.KeyID, b.KeyID)
	})
	return snapshots
}

func (t *Tracker) ListDirectionSnapshots(provider schemas.ModelProvider, model string) []DirectionStatus {
	if t == nil {
		return nil
	}
	t.ensureProfilesFresh(false)

	statuses := make([]DirectionStatus, 0)
	t.directions.Range(func(key, value any) bool {
		direction, ok := key.(directionKey)
		if !ok {
			return true
		}
		if provider != "" && direction.provider != provider {
			return true
		}
		if model != "" && direction.model != model {
			return true
		}

		stats, ok := value.(*routeStats)
		if !ok {
			return true
		}
		stats.mu.RLock()
		status := DirectionStatus{
			Provider: direction.provider,
			Model:    direction.model,
			DirectionSnapshot: DirectionSnapshot{
				Samples:             stats.samples,
				Successes:           stats.successes,
				Failures:            stats.failures,
				ConsecutiveFailures: stats.consecutiveFailures,
				ErrorEWMA:           stats.errorEWMA,
				LatencyEWMA:         stats.latencyEWMA,
				LastUpdated:         stats.lastUpdated,
			},
		}
		stats.mu.RUnlock()
		if profile, ok := t.directionProfile(direction); ok {
			status.State = profile.State
			status.Score = profile.Score
			status.Weight = profile.Weight
			status.ExpectedTrafficShare = profile.ExpectedTrafficShare
			status.ActualTrafficShare = profile.ActualTrafficShare
		}
		statuses = append(statuses, status)
		return true
	})

	slices.SortFunc(statuses, func(a, b DirectionStatus) int {
		if cmp := strings.Compare(string(a.Provider), string(b.Provider)); cmp != 0 {
			return cmp
		}
		return strings.Compare(a.Model, b.Model)
	})
	return statuses
}

func (t *Tracker) SelectKey(ctx *schemas.BifrostContext, keys []schemas.Key, providerKey schemas.ModelProvider, model string) (schemas.Key, error) {
	if len(keys) == 0 {
		return schemas.Key{}, fmt.Errorf("no keys available for provider %s", providerKey)
	}
	if len(keys) == 1 {
		return keys[0], nil
	}

	t.ensureProfilesFresh(false)
	exploration := rand.Float64() < t.cfg.explorationRatio
	weightedKeys := t.weightedKeys(providerKey, model, keys, exploration)
	if len(weightedKeys) == 0 {
		return bifrost.WeightedRandomKeySelector(ctx, keys, providerKey, model)
	}
	return bifrost.WeightedRandomKeySelector(ctx, weightedKeys, providerKey, model)
}

func (t *Tracker) weightedKeys(provider schemas.ModelProvider, model string, keys []schemas.Key, exploration bool) []schemas.Key {
	weightedKeys := make([]schemas.Key, 0, len(keys))
	maxProfileWeight := t.cfg.weightFloor
	for _, key := range keys {
		profile, ok := t.routeProfile(routeKey{provider: provider, model: model, keyID: key.ID})
		if ok && profile.Weight > maxProfileWeight {
			maxProfileWeight = profile.Weight
		}
	}
	explorationFloor := int(math.Round(float64(maxProfileWeight) * t.cfg.explorationFloorRatio))
	explorationFloor = clampInt(explorationFloor, t.cfg.weightFloor, t.cfg.weightCeiling)

	for _, key := range keys {
		weighted := key
		baseWeight := key.Weight
		if baseWeight <= 0 {
			baseWeight = 1
		}

		profile, ok := t.routeProfile(routeKey{provider: provider, model: model, keyID: key.ID})
		if !ok {
			weighted.Weight = t.adjustedWeight(provider, model, key, t.findBaselineLatency(provider, model, keys))
		} else {
			effectiveWeight := profile.Weight
			if profile.Weight == 0 {
				if !exploration {
					continue
				}
				effectiveWeight = explorationFloor
			} else if exploration {
				effectiveWeight = maxInt(effectiveWeight, explorationFloor)
			}
			weighted.Weight = baseWeight * (float64(effectiveWeight) / float64(max(t.cfg.weightCeiling, 1)))
		}

		if weighted.Weight <= 0 {
			continue
		}
		if t.cfg.jitterRatio > 0 {
			jitter := 1 + ((rand.Float64()*2 - 1) * t.cfg.jitterRatio)
			weighted.Weight = math.Max(weighted.Weight*jitter, 0.01)
		}
		weightedKeys = append(weightedKeys, weighted)
	}
	return weightedKeys
}

func (t *Tracker) ReorderFallbacks(fallbacks []schemas.Fallback) ([]schemas.Fallback, bool) {
	if t == nil || len(fallbacks) < 2 {
		return fallbacks, false
	}
	t.ensureProfilesFresh(false)

	type fallbackCandidate struct {
		index    int
		fallback schemas.Fallback
		profile  directionProfile
		raw      DirectionSnapshot
		known    bool
	}

	candidates := make([]fallbackCandidate, 0, len(fallbacks))
	for i, fallback := range fallbacks {
		profile, ok := t.directionProfile(directionKey{provider: fallback.Provider, model: fallback.Model})
		raw, rawOK := t.DirectionSnapshot(fallback.Provider, fallback.Model)
		candidates = append(candidates, fallbackCandidate{
			index:    i,
			fallback: fallback,
			profile:  profile,
			raw:      raw,
			known:    ok || rawOK,
		})
	}

	knownCount := 0
	for _, candidate := range candidates {
		if candidate.known {
			knownCount++
		}
	}
	if knownCount < 2 {
		return fallbacks, false
	}

	slices.SortStableFunc(candidates, func(a, b fallbackCandidate) int {
		if a.known != b.known {
			if a.known {
				return -1
			}
			return 1
		}
		if !a.known {
			return a.index - b.index
		}
		aWeight, aScore := a.profile.Weight, a.profile.Score
		bWeight, bScore := b.profile.Weight, b.profile.Score
		if aWeight == 0 && a.raw.Samples > 0 {
			aScore = t.directionScore(a.raw, 0)
			aWeight = t.weightForScore(aScore, t.determineHealthState(nil, a.raw.ErrorEWMA, a.raw.ConsecutiveFailures, 1, time.Now()))
		}
		if bWeight == 0 && b.raw.Samples > 0 {
			bScore = t.directionScore(b.raw, 0)
			bWeight = t.weightForScore(bScore, t.determineHealthState(nil, b.raw.ErrorEWMA, b.raw.ConsecutiveFailures, 1, time.Now()))
		}
		if aWeight == bWeight {
			if aScore == bScore {
				return a.index - b.index
			}
			if aScore > bScore {
				return -1
			}
			return 1
		}
		if aWeight > bWeight {
			return -1
		}
		return 1
	})

	reordered := make([]schemas.Fallback, len(candidates))
	changed := false
	for i, candidate := range candidates {
		reordered[i] = candidate.fallback
		if candidate.index != i {
			changed = true
		}
	}
	return reordered, changed
}

func clampInt(value, minValue, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func max[T ~int | ~int64 | ~float64](a, b T) T {
	if a > b {
		return a
	}
	return b
}
