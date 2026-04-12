package loadbalancer

import (
	"fmt"
	"math"
	"math/rand"
	"slices"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/maximhq/bifrost/core/network"
	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework/configstore"
)

type modelsCatalog interface {
	GetProvidersForModel(model string) []schemas.ModelProvider
	RefineModelForProvider(provider schemas.ModelProvider, model string) (string, error)
	IsModelAllowedForProvider(provider schemas.ModelProvider, model string, allowedModels []string) bool
	IsSameModel(model1, model2 string) bool
}

type providerConfigLookup func(provider schemas.ModelProvider) (configstore.ProviderConfig, bool)

func (p *Plugin) BindRoutingSources(catalog modelsCatalog, lookup providerConfigLookup) {
	if p == nil {
		return
	}
	p.catalog = catalog
	p.providerLookup = lookup
}

func (p *Plugin) HTTPTransportPreHook(ctx *schemas.BifrostContext, req *schemas.HTTPRequest) (*schemas.HTTPResponse, error) {
	if p == nil || p.catalog == nil || p.providerLookup == nil || req == nil || len(req.Body) == 0 {
		return nil, nil
	}
	policy := p.policySnapshot()

	contentType := strings.ToLower(req.CaseInsensitiveHeaderLookup("Content-Type"))
	isMultipart := strings.HasPrefix(contentType, "multipart/form-data")
	if !isMultipart && !strings.Contains(contentType, "json") {
		return nil, nil
	}

	var payload map[string]any
	var err error
	if isMultipart {
		payload, err = network.ParseMultipartFormFields(contentType, req.Body)
	} else {
		err = sonic.Unmarshal(req.Body, &payload)
	}
	if err != nil {
		p.logger.Warn("failed to parse request body for adaptive routing: %v", err)
		return nil, nil
	}

	modelValue, ok := payload["model"].(string)
	modelValue = strings.TrimSpace(modelValue)
	if !ok || modelValue == "" {
		return nil, nil
	}

	provider, modelName := schemas.ParseModelString(modelValue, "")
	if modelName == "" {
		return nil, nil
	}

	requestPolicy, scoped := requestPolicyFromContext(ctx)
	if scoped {
		if provider != "" {
			if applyRuleFallbacks(payload, requestPolicy.additionalFallbacks) {
				if err := network.SerializePayloadToRequest(req, payload, isMultipart, contentType); err != nil {
					return nil, err
				}
			}
			return nil, nil
		}
		if !requestPolicy.shouldUseDirectionRouting() || hasExplicitFallbacks(payload["fallbacks"]) {
			return nil, nil
		}

		selectedModel, fallbacks, changed := p.selectProviderAndFallbacksForRequestPolicy(ctx, requestPolicy, modelName)
		if !changed {
			return nil, nil
		}
		payload["model"] = selectedModel
		if len(fallbacks) > 0 {
			payload["fallbacks"] = fallbacks
		}
		if err := network.SerializePayloadToRequest(req, payload, isMultipart, contentType); err != nil {
			return nil, err
		}
		return nil, nil
	}

	if provider != "" || !policy.shouldUseDirectionRouting(ctx, modelValue) || hasExplicitFallbacks(payload["fallbacks"]) {
		return nil, nil
	}

	selectedModel, fallbacks, changed := p.selectProviderAndFallbacks(ctx, policy, modelName)
	if !changed {
		return nil, nil
	}

	payload["model"] = selectedModel
	if len(fallbacks) > 0 {
		payload["fallbacks"] = fallbacks
	}
	if err := network.SerializePayloadToRequest(req, payload, isMultipart, contentType); err != nil {
		return nil, err
	}
	return nil, nil
}

func (p *Plugin) HTTPTransportPostHook(_ *schemas.BifrostContext, _ *schemas.HTTPRequest, _ *schemas.HTTPResponse) error {
	return nil
}

func (p *Plugin) HTTPTransportStreamChunkHook(_ *schemas.BifrostContext, _ *schemas.HTTPRequest, chunk *schemas.BifrostStreamChunk) (*schemas.BifrostStreamChunk, error) {
	return chunk, nil
}

type directionCandidate struct {
	Provider schemas.ModelProvider
	Model    string
	Profile  directionProfile
}

func (p *Plugin) selectProviderAndFallbacks(ctx *schemas.BifrostContext, policy runtimePolicy, model string) (string, []string, bool) {
	candidates := p.directionCandidates(policy, model)
	return p.selectProviderAndFallbacksFromCandidates(ctx, model, candidates, nil)
}

func (p *Plugin) selectProviderAndFallbacksForRequestPolicy(ctx *schemas.BifrostContext, policy requestPolicy, model string) (string, []string, bool) {
	candidates := p.directionCandidatesForRequestPolicy(policy, model)
	return p.selectProviderAndFallbacksFromCandidates(ctx, model, candidates, policy.additionalFallbacks)
}

func (p *Plugin) selectProviderAndFallbacksFromCandidates(ctx *schemas.BifrostContext, model string, candidates []directionCandidate, additionalFallbacks []string) (string, []string, bool) {
	if len(candidates) == 0 {
		return "", nil, false
	}
	if len(candidates) == 1 {
		selected := fmt.Sprintf("%s/%s", candidates[0].Provider, candidates[0].Model)
		fallbacks := mergeAdaptiveFallbacks(selected, nil, additionalFallbacks)
		return selected, fallbacks, selected != model || len(fallbacks) > 0
	}
	tracker := p.currentTracker()
	if tracker == nil {
		return "", nil, false
	}
	cfg := p.currentTrackerConfig()
	tracker.ensureProfilesFresh(false)
	exploration := rand.Float64() < cfg.explorationRatio
	selectedIndex := p.chooseDirectionCandidate(candidates, cfg, exploration)
	selected := candidates[selectedIndex]

	remaining := make([]directionCandidate, 0, len(candidates)-1)
	for i, candidate := range candidates {
		if i == selectedIndex {
			continue
		}
		remaining = append(remaining, candidate)
	}
	slices.SortFunc(remaining, func(a, b directionCandidate) int {
		if a.Profile.Weight == b.Profile.Weight {
			if a.Profile.Score == b.Profile.Score {
				return strings.Compare(string(a.Provider), string(b.Provider))
			}
			if a.Profile.Score > b.Profile.Score {
				return -1
			}
			return 1
		}
		if a.Profile.Weight > b.Profile.Weight {
			return -1
		}
		return 1
	})

	fallbacks := make([]string, 0, len(remaining))
	for _, candidate := range remaining {
		fallbacks = append(fallbacks, fmt.Sprintf("%s/%s", candidate.Provider, candidate.Model))
	}
	fallbacks = mergeAdaptiveFallbacks(fmt.Sprintf("%s/%s", selected.Provider, selected.Model), fallbacks, additionalFallbacks)

	if ctx != nil {
		ctx.AppendRoutingEngineLog(
			schemas.RoutingEngineLoadbalancing,
			fmt.Sprintf(
				"Adaptive routing selected provider %s for model %s (state=%s weight=%d score=%.2f, exploration=%t) with %d fallback(s)",
				selected.Provider,
				model,
				selected.Profile.State,
				selected.Profile.Weight,
				selected.Profile.Score,
				exploration,
				len(fallbacks),
			),
		)
	}

	return fmt.Sprintf("%s/%s", selected.Provider, selected.Model), fallbacks, true
}

func (p *Plugin) directionCandidates(policy runtimePolicy, model string) []directionCandidate {
	if p == nil || p.catalog == nil || p.providerLookup == nil {
		return nil
	}
	cfg := p.currentTrackerConfig()
	providers := p.catalog.GetProvidersForModel(model)
	if len(providers) == 0 {
		return nil
	}

	candidates := make([]directionCandidate, 0, len(providers))
	for _, provider := range providers {
		if !policy.allowsProvider(provider) {
			continue
		}
		config, ok := p.providerLookup(provider)
		if !ok || !providerHasEligibleKey(provider, model, config, p.catalog) {
			continue
		}
		refinedModel, err := p.catalog.RefineModelForProvider(provider, model)
		if err != nil || refinedModel == "" {
			continue
		}
		if !policy.allowsModel(refinedModel) {
			continue
		}
		tracker := p.currentTracker()
		if tracker == nil {
			continue
		}
		profile, ok := tracker.directionProfile(directionKey{provider: provider, model: refinedModel})
		if !ok {
			profile = directionProfile{
				State:  HealthStateHealthy,
				Score:  0.5,
				Weight: (cfg.weightFloor + cfg.weightCeiling) / 2,
			}
		}
		candidates = append(candidates, directionCandidate{
			Provider: provider,
			Model:    refinedModel,
			Profile:  profile,
		})
	}
	return candidates
}

func (p *Plugin) directionCandidatesForRequestPolicy(policy requestPolicy, model string) []directionCandidate {
	if p == nil || p.providerLookup == nil {
		return nil
	}
	cfg := p.currentTrackerConfig()
	tracker := p.currentTracker()
	if tracker == nil {
		return nil
	}

	candidates := make([]directionCandidate, 0, len(policy.directionTargets))
	seen := make(map[string]struct{}, len(policy.directionTargets))
	for _, target := range policy.directionTargets {
		provider := schemas.ModelProvider(strings.TrimSpace(string(target.Provider)))
		if provider == "" {
			continue
		}

		candidateModel := strings.TrimSpace(target.Model)
		if candidateModel == "" {
			candidateModel = model
		}
		if candidateModel == "" {
			continue
		}

		refinedModel, ok := p.resolveCandidateModel(provider, candidateModel)
		if !ok {
			continue
		}

		config, ok := p.providerLookup(provider)
		if !ok || !providerHasEligibleKey(provider, refinedModel, config, p.catalog) {
			continue
		}

		key := fmt.Sprintf("%s/%s", provider, refinedModel)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}

		profile, ok := tracker.directionProfile(directionKey{provider: provider, model: refinedModel})
		if !ok {
			profile = directionProfile{
				State:  HealthStateHealthy,
				Score:  0.5,
				Weight: (cfg.weightFloor + cfg.weightCeiling) / 2,
			}
		}
		candidates = append(candidates, directionCandidate{
			Provider: provider,
			Model:    refinedModel,
			Profile:  profile,
		})
	}
	return candidates
}

func (p *Plugin) resolveCandidateModel(provider schemas.ModelProvider, model string) (string, bool) {
	if strings.TrimSpace(model) == "" {
		return "", false
	}
	if p == nil || p.catalog == nil {
		return model, true
	}
	refinedModel, err := p.catalog.RefineModelForProvider(provider, model)
	if err == nil && strings.TrimSpace(refinedModel) != "" {
		return refinedModel, true
	}
	return model, true
}

func mergeAdaptiveFallbacks(selected string, preferred []string, additional []string) []string {
	if len(preferred) == 0 && len(additional) == 0 {
		return nil
	}

	fallbacks := make([]string, 0, len(preferred)+len(additional))
	seen := make(map[string]struct{}, len(preferred)+len(additional)+1)
	if selected != "" {
		seen[selected] = struct{}{}
	}

	appendUnique := func(values []string) {
		for _, value := range values {
			trimmed := strings.TrimSpace(value)
			if trimmed == "" {
				continue
			}
			if _, ok := seen[trimmed]; ok {
				continue
			}
			seen[trimmed] = struct{}{}
			fallbacks = append(fallbacks, trimmed)
		}
	}

	appendUnique(preferred)
	appendUnique(additional)
	return fallbacks
}

func applyRuleFallbacks(payload map[string]any, fallbacks []string) bool {
	if len(fallbacks) == 0 || payload == nil || hasExplicitFallbacks(payload["fallbacks"]) {
		return false
	}
	payload["fallbacks"] = append([]string(nil), fallbacks...)
	return true
}

func (p *Plugin) chooseDirectionCandidate(candidates []directionCandidate, cfg trackerConfig, exploration bool) int {
	if len(candidates) == 0 {
		return -1
	}
	if len(candidates) == 1 {
		return 0
	}

	maxWeight := cfg.weightFloor
	for _, candidate := range candidates {
		if candidate.Profile.Weight > maxWeight {
			maxWeight = candidate.Profile.Weight
		}
	}
	explorationFloor := clampInt(int(math.Round(float64(maxWeight)*cfg.explorationFloorRatio)), cfg.weightFloor, cfg.weightCeiling)

	weights := make([]float64, len(candidates))
	totalWeight := 0.0
	for i, candidate := range candidates {
		effective := candidate.Profile.Weight
		if effective == 0 {
			if !exploration {
				continue
			}
			effective = explorationFloor
		} else if exploration {
			effective = maxInt(effective, explorationFloor)
		}
		weight := float64(maxInt(effective, 1))
		if cfg.jitterRatio > 0 {
			jitter := 1 + ((rand.Float64()*2 - 1) * cfg.jitterRatio)
			weight = math.Max(weight*jitter, 1)
		}
		weights[i] = weight
		totalWeight += weight
	}
	if totalWeight <= 0 {
		return rand.Intn(len(candidates))
	}

	threshold := rand.Float64() * totalWeight
	current := 0.0
	for i, weight := range weights {
		current += weight
		if threshold <= current {
			return i
		}
	}
	return len(candidates) - 1
}

func providerHasEligibleKey(provider schemas.ModelProvider, model string, config configstore.ProviderConfig, catalog modelsCatalog) bool {
	if len(config.Keys) == 0 {
		return false
	}
	for _, key := range config.Keys {
		if key.Enabled != nil && !*key.Enabled {
			continue
		}
		if keyAllowsModel(provider, model, key, catalog) {
			return true
		}
	}
	return false
}

func keyAllowsModel(provider schemas.ModelProvider, model string, key schemas.Key, catalog modelsCatalog) bool {
	if len(key.BlacklistedModels) > 0 && keyModelAllowsModel(provider, model, key.BlacklistedModels, catalog) {
		return false
	}
	if len(key.Models) > 0 {
		return keyModelAllowsModel(provider, model, key.Models, catalog)
	}
	return true
}

func keyModelAllowsModel(provider schemas.ModelProvider, model string, allowedModels []string, catalog modelsCatalog) bool {
	if len(allowedModels) == 0 {
		return false
	}
	if catalog == nil {
		return slices.Contains(allowedModels, model)
	}
	if catalog.IsModelAllowedForProvider(provider, model, allowedModels) {
		return true
	}
	for _, allowedModel := range allowedModels {
		if strings.Contains(allowedModel, "/") {
			continue
		}
		if catalog.IsSameModel(allowedModel, model) {
			return true
		}
	}
	return false
}

func hasExplicitFallbacks(value any) bool {
	switch typed := value.(type) {
	case []string:
		return len(typed) > 0
	case []any:
		return len(typed) > 0
	case string:
		return strings.TrimSpace(typed) != ""
	default:
		return false
	}
}
