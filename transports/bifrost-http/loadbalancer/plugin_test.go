package loadbalancer

import (
	"context"
	"testing"
	"time"

	"github.com/bytedance/sonic"
	bifrost "github.com/maximhq/bifrost/core"
	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework/configstore"
	"github.com/maximhq/bifrost/framework/modelcatalog"
	enterprisecfg "github.com/maximhq/bifrost/transports/bifrost-http/enterprise"
)

func TestSelectorFallsBackToStaticWeightsWithoutMetrics(t *testing.T) {
	plugin, err := Init(&enterprisecfg.LoadBalancerConfig{
		Enabled: true,
		TrackerConfig: &enterprisecfg.LoadBalancerTrackerConfig{
			ExplorationRatio: 0,
			JitterRatio:      0,
		},
	}, bifrost.NewNoOpLogger())
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	t.Cleanup(func() { _ = plugin.Cleanup() })

	keys := []schemas.Key{
		{ID: "key-a", Name: "key-a", Weight: 9},
		{ID: "key-b", Name: "key-b", Weight: 1},
	}

	counts := map[string]int{}
	for range 2000 {
		selected, selErr := plugin.GetKeySelector()(nil, keys, schemas.OpenAI, "gpt-4o")
		if selErr != nil {
			t.Fatalf("GetKeySelector() error = %v", selErr)
		}
		counts[selected.ID]++
	}

	if counts["key-a"] <= counts["key-b"] {
		t.Fatalf("expected heavier key to win more often, got counts=%v", counts)
	}
}

func TestSelectorPrefersHealthyKeyAfterFailures(t *testing.T) {
	plugin, err := Init(&enterprisecfg.LoadBalancerConfig{
		Enabled: true,
		TrackerConfig: &enterprisecfg.LoadBalancerTrackerConfig{
			ExplorationRatio: 0,
			JitterRatio:      0,
			MinimumSamples:   1,
		},
	}, bifrost.NewNoOpLogger())
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	t.Cleanup(func() { _ = plugin.Cleanup() })

	keys := []schemas.Key{
		{ID: "key-a", Name: "key-a", Weight: 1},
		{ID: "key-b", Name: "key-b", Weight: 1},
	}

	for range 20 {
		plugin.tracker.Observe(schemas.OpenAI, "gpt-4o", "key-a", 1200, false)
		plugin.tracker.Observe(schemas.OpenAI, "gpt-4o", "key-b", 120, true)
	}

	counts := map[string]int{}
	for range 2000 {
		selected, selErr := plugin.GetKeySelector()(nil, keys, schemas.OpenAI, "gpt-4o")
		if selErr != nil {
			t.Fatalf("GetKeySelector() error = %v", selErr)
		}
		counts[selected.ID]++
	}

	if counts["key-b"] <= counts["key-a"] {
		t.Fatalf("expected healthy key to be preferred, got counts=%v", counts)
	}
}

func TestPostLLMHookRecordsRouteMetrics(t *testing.T) {
	plugin, err := Init(&enterprisecfg.LoadBalancerConfig{Enabled: true}, bifrost.NewNoOpLogger())
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	t.Cleanup(func() { _ = plugin.Cleanup() })

	ctx := schemas.NewBifrostContext(context.Background(), schemas.NoDeadline)
	ctx.SetValue(schemas.BifrostContextKeySelectedKeyID, "key-a")
	ctx.SetValue(schemas.BifrostContextKeySelectedKeyName, "key-a")

	req := &schemas.BifrostRequest{
		RequestType: schemas.ChatCompletionRequest,
		ChatRequest: &schemas.BifrostChatRequest{
			Provider: schemas.OpenAI,
			Model:    "gpt-4o",
		},
	}

	if _, _, err = plugin.PreLLMHook(ctx, req); err != nil {
		t.Fatalf("PreLLMHook() error = %v", err)
	}

	resp := &schemas.BifrostResponse{
		ChatResponse: &schemas.BifrostChatResponse{
			Model: "gpt-4o",
			ExtraFields: schemas.BifrostResponseExtraFields{
				RequestType:    schemas.ChatCompletionRequest,
				Provider:       schemas.OpenAI,
				ModelRequested: "gpt-4o",
				Latency:        180,
			},
		},
	}

	if _, _, err = plugin.PostLLMHook(ctx, resp, nil); err != nil {
		t.Fatalf("PostLLMHook() error = %v", err)
	}

	snapshot, ok := plugin.Snapshot(schemas.OpenAI, "gpt-4o", "key-a")
	if !ok {
		t.Fatal("expected route snapshot to be recorded")
	}
	if snapshot.Samples != 1 {
		t.Fatalf("expected 1 sample, got %d", snapshot.Samples)
	}
	if snapshot.Successes != 1 || snapshot.Failures != 0 {
		t.Fatalf("unexpected counters: %+v", snapshot)
	}
	if snapshot.LatencyEWMA != 180 {
		t.Fatalf("expected latency EWMA to be recorded, got %v", snapshot.LatencyEWMA)
	}

	direction, ok := plugin.DirectionSnapshot(schemas.OpenAI, "gpt-4o")
	if !ok {
		t.Fatal("expected direction snapshot to be recorded")
	}
	if direction.Samples != 1 || direction.Successes != 1 {
		t.Fatalf("unexpected direction counters: %+v", direction)
	}
}

func TestListSnapshotsFiltersByProviderAndModel(t *testing.T) {
	plugin, err := Init(&enterprisecfg.LoadBalancerConfig{Enabled: true}, bifrost.NewNoOpLogger())
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	t.Cleanup(func() { _ = plugin.Cleanup() })

	plugin.tracker.Observe(schemas.OpenAI, "gpt-4o", "key-a", 100, true)
	plugin.tracker.Observe(schemas.Anthropic, "claude-sonnet", "key-b", 200, true)

	openAISnapshots := plugin.ListSnapshots(schemas.OpenAI, "")
	if len(openAISnapshots) != 1 {
		t.Fatalf("expected one filtered snapshot, got %d", len(openAISnapshots))
	}
	if openAISnapshots[0].Provider != schemas.OpenAI || openAISnapshots[0].KeyID != "key-a" {
		t.Fatalf("unexpected snapshot: %+v", openAISnapshots[0])
	}

	modelSnapshots := plugin.ListSnapshots("", "claude-sonnet")
	if len(modelSnapshots) != 1 || modelSnapshots[0].Provider != schemas.Anthropic {
		t.Fatalf("unexpected model filtered snapshots: %+v", modelSnapshots)
	}
}

func TestPreLLMHookReordersFallbacksUsingDirectionMetrics(t *testing.T) {
	plugin, err := Init(&enterprisecfg.LoadBalancerConfig{
		Enabled:                 true,
		DirectionRoutingEnabled: schemas.Ptr(true),
		TrackerConfig: &enterprisecfg.LoadBalancerTrackerConfig{
			MinimumSamples:   1,
			ExplorationRatio: 0,
			JitterRatio:      0,
		},
	}, bifrost.NewNoOpLogger())
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	t.Cleanup(func() { _ = plugin.Cleanup() })

	for range 10 {
		plugin.tracker.ObserveDirection(schemas.OpenAI, "gpt-4.1", 1200, false)
		plugin.tracker.ObserveDirection(schemas.Anthropic, "claude-sonnet-4", 140, true)
	}

	req := &schemas.BifrostRequest{
		RequestType: schemas.ChatCompletionRequest,
		ChatRequest: &schemas.BifrostChatRequest{
			Provider: schemas.OpenAI,
			Model:    "gpt-4o",
			Fallbacks: []schemas.Fallback{
				{Provider: schemas.OpenAI, Model: "gpt-4.1"},
				{Provider: schemas.Anthropic, Model: "claude-sonnet-4"},
			},
		},
	}
	ctx := schemas.NewBifrostContext(context.Background(), schemas.NoDeadline)

	if _, _, err = plugin.PreLLMHook(ctx, req); err != nil {
		t.Fatalf("PreLLMHook() error = %v", err)
	}

	_, _, fallbacks := req.GetRequestFields()
	if len(fallbacks) != 2 {
		t.Fatalf("expected 2 fallbacks, got %+v", fallbacks)
	}
	if fallbacks[0].Provider != schemas.Anthropic || fallbacks[0].Model != "claude-sonnet-4" {
		t.Fatalf("expected healthiest fallback to move to the front, got %+v", fallbacks)
	}
}

func TestListDirectionSnapshotsFiltersByProviderAndModel(t *testing.T) {
	plugin, err := Init(&enterprisecfg.LoadBalancerConfig{Enabled: true}, bifrost.NewNoOpLogger())
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	t.Cleanup(func() { _ = plugin.Cleanup() })

	plugin.tracker.ObserveDirection(schemas.OpenAI, "gpt-4o", 100, true)
	plugin.tracker.ObserveDirection(schemas.Anthropic, "claude-sonnet", 200, true)

	openAISnapshots := plugin.ListDirectionSnapshots(schemas.OpenAI, "")
	if len(openAISnapshots) != 1 {
		t.Fatalf("expected one filtered direction snapshot, got %d", len(openAISnapshots))
	}
	if openAISnapshots[0].Provider != schemas.OpenAI || openAISnapshots[0].Model != "gpt-4o" {
		t.Fatalf("unexpected direction snapshot: %+v", openAISnapshots[0])
	}

	modelSnapshots := plugin.ListDirectionSnapshots("", "claude-sonnet")
	if len(modelSnapshots) != 1 || modelSnapshots[0].Provider != schemas.Anthropic {
		t.Fatalf("unexpected direction filtered snapshots: %+v", modelSnapshots)
	}
}

func TestListSnapshotsExposePrecomputedStateAndWeights(t *testing.T) {
	plugin, err := Init(&enterprisecfg.LoadBalancerConfig{
		Enabled: true,
		TrackerConfig: &enterprisecfg.LoadBalancerTrackerConfig{
			MinimumSamples:            1,
			ExplorationRatio:          0,
			JitterRatio:               0,
			FailedConsecutiveFailures: 3,
		},
	}, bifrost.NewNoOpLogger())
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	t.Cleanup(func() { _ = plugin.Cleanup() })

	for range 6 {
		plugin.tracker.Observe(schemas.OpenAI, "gpt-4o", "key-a", 950, false)
	}
	plugin.tracker.recomputeProfiles(time.Now(), true)

	statuses := plugin.ListSnapshots(schemas.OpenAI, "gpt-4o")
	if len(statuses) != 1 {
		t.Fatalf("expected one route status, got %+v", statuses)
	}
	if statuses[0].State != HealthStateFailed {
		t.Fatalf("expected route to be marked failed, got %+v", statuses[0])
	}
	if statuses[0].Weight != 0 {
		t.Fatalf("expected failed route weight to trip circuit breaker, got %+v", statuses[0])
	}
	if statuses[0].Score <= 0 {
		t.Fatalf("expected failed route score to be populated, got %+v", statuses[0])
	}
}

func TestHTTPTransportPreHookSelectsProviderForBareModel(t *testing.T) {
	plugin, err := Init(&enterprisecfg.LoadBalancerConfig{
		Enabled:                 true,
		DirectionRoutingEnabled: schemas.Ptr(true),
		TrackerConfig: &enterprisecfg.LoadBalancerTrackerConfig{
			MinimumSamples:   1,
			ExplorationRatio: 0,
			JitterRatio:      0,
		},
	}, bifrost.NewNoOpLogger())
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	t.Cleanup(func() { _ = plugin.Cleanup() })

	catalog := modelcatalog.NewTestCatalog(map[string]string{"gpt-4o": "gpt-4o"})
	catalog.UpsertModelDataForProvider(schemas.OpenAI, &schemas.BifrostListModelsResponse{
		Data: []schemas.Model{{ID: "openai/gpt-4o"}},
	}, nil, nil)
	catalog.UpsertModelDataForProvider(schemas.Anthropic, &schemas.BifrostListModelsResponse{
		Data: []schemas.Model{{ID: "anthropic/gpt-4o"}},
	}, nil, nil)

	plugin.BindRoutingSources(catalog, func(provider schemas.ModelProvider) (configstore.ProviderConfig, bool) {
		return configstore.ProviderConfig{
			Keys: []schemas.Key{{ID: string(provider) + "-key", Name: string(provider) + "-key", Weight: 1}},
		}, true
	})

	for range 10 {
		plugin.tracker.ObserveDirection(schemas.OpenAI, "gpt-4o", 900, false)
		plugin.tracker.ObserveDirection(schemas.Anthropic, "gpt-4o", 120, true)
	}
	plugin.tracker.recomputeProfiles(time.Now(), true)

	body, err := sonic.Marshal(map[string]any{
		"model": "gpt-4o",
		"messages": []map[string]any{
			{"role": "user", "content": "hello"},
		},
	})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	req := &schemas.HTTPRequest{
		Method: "POST",
		Path:   "/v1/chat/completions",
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: body,
	}
	ctx := schemas.NewBifrostContext(context.Background(), schemas.NoDeadline)
	if _, err := plugin.HTTPTransportPreHook(ctx, req); err != nil {
		t.Fatalf("HTTPTransportPreHook() error = %v", err)
	}

	var mutated map[string]any
	if err := sonic.Unmarshal(req.Body, &mutated); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if mutated["model"] != "anthropic/gpt-4o" {
		t.Fatalf("expected adaptive routing to select healthiest provider, got %v", mutated["model"])
	}
	fallbacks, ok := mutated["fallbacks"].([]any)
	if !ok || len(fallbacks) != 1 || fallbacks[0] != "openai/gpt-4o" {
		t.Fatalf("expected fallback list to be generated, got %#v", mutated["fallbacks"])
	}
}

func TestUpdateConfigPreservesMetricsAndDisablesDirectionRoutingInPlace(t *testing.T) {
	plugin, err := Init(&enterprisecfg.LoadBalancerConfig{
		Enabled:                 true,
		DirectionRoutingEnabled: schemas.Ptr(true),
		TrackerConfig: &enterprisecfg.LoadBalancerTrackerConfig{
			MinimumSamples:   1,
			ExplorationRatio: 0,
			JitterRatio:      0,
		},
	}, bifrost.NewNoOpLogger())
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	t.Cleanup(func() { _ = plugin.Cleanup() })

	plugin.tracker.Observe(schemas.OpenAI, "gpt-4o", "key-a", 120, true)
	for range 5 {
		plugin.tracker.ObserveDirection(schemas.OpenAI, "gpt-4o", 900, false)
		plugin.tracker.ObserveDirection(schemas.Anthropic, "gpt-4o", 120, true)
	}

	if _, ok := plugin.Snapshot(schemas.OpenAI, "gpt-4o", "key-a"); !ok {
		t.Fatal("expected route snapshot before config update")
	}
	if _, ok := plugin.DirectionSnapshot(schemas.Anthropic, "gpt-4o"); !ok {
		t.Fatal("expected direction snapshot before config update")
	}

	if err := plugin.UpdateConfig(&enterprisecfg.LoadBalancerConfig{
		Enabled:                 true,
		KeyBalancingEnabled:     schemas.Ptr(true),
		DirectionRoutingEnabled: schemas.Ptr(false),
		TrackerConfig: &enterprisecfg.LoadBalancerTrackerConfig{
			MinimumSamples:   1,
			ExplorationRatio: 0,
			JitterRatio:      0,
		},
	}); err != nil {
		t.Fatalf("UpdateConfig() error = %v", err)
	}

	routeSnapshot, ok := plugin.Snapshot(schemas.OpenAI, "gpt-4o", "key-a")
	if !ok || routeSnapshot.Samples == 0 {
		t.Fatalf("expected route snapshot to survive config update, got %+v", routeSnapshot)
	}
	directionSnapshot, ok := plugin.DirectionSnapshot(schemas.Anthropic, "gpt-4o")
	if !ok || directionSnapshot.Samples == 0 {
		t.Fatalf("expected direction snapshot to survive config update, got %+v", directionSnapshot)
	}

	catalog := modelcatalog.NewTestCatalog(map[string]string{"gpt-4o": "gpt-4o"})
	catalog.UpsertModelDataForProvider(schemas.OpenAI, &schemas.BifrostListModelsResponse{
		Data: []schemas.Model{{ID: "openai/gpt-4o"}},
	}, nil, nil)
	catalog.UpsertModelDataForProvider(schemas.Anthropic, &schemas.BifrostListModelsResponse{
		Data: []schemas.Model{{ID: "anthropic/gpt-4o"}},
	}, nil, nil)
	plugin.BindRoutingSources(catalog, func(provider schemas.ModelProvider) (configstore.ProviderConfig, bool) {
		return configstore.ProviderConfig{
			Keys: []schemas.Key{{ID: string(provider) + "-key", Name: string(provider) + "-key", Weight: 1}},
		}, true
	})

	body, err := sonic.Marshal(map[string]any{
		"model": "gpt-4o",
		"messages": []map[string]any{
			{"role": "user", "content": "hello"},
		},
	})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	req := &schemas.HTTPRequest{
		Method: "POST",
		Path:   "/v1/chat/completions",
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: body,
	}
	ctx := schemas.NewBifrostContext(context.Background(), schemas.NoDeadline)
	if _, err := plugin.HTTPTransportPreHook(ctx, req); err != nil {
		t.Fatalf("HTTPTransportPreHook() error = %v", err)
	}

	var mutated map[string]any
	if err := sonic.Unmarshal(req.Body, &mutated); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if mutated["model"] != "gpt-4o" {
		t.Fatalf("expected provider direction routing to stay off after config update, got %v", mutated["model"])
	}
	if _, ok := mutated["fallbacks"]; ok {
		t.Fatalf("expected no adaptive fallback generation when direction routing is disabled, got %#v", mutated["fallbacks"])
	}
}

func TestRemoteSnapshotsMergeIntoClusterAwareStatusWithoutAffectingLocalStatus(t *testing.T) {
	plugin, err := Init(&enterprisecfg.LoadBalancerConfig{
		Enabled: true,
		TrackerConfig: &enterprisecfg.LoadBalancerTrackerConfig{
			MinimumSamples:   1,
			ExplorationRatio: 0,
			JitterRatio:      0,
		},
	}, bifrost.NewNoOpLogger())
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	t.Cleanup(func() { _ = plugin.Cleanup() })

	plugin.tracker.Observe(schemas.OpenAI, "gpt-4o", "key-a", 120, true)
	plugin.tracker.ObserveDirection(schemas.OpenAI, "gpt-4o", 120, true)

	remoteUpdatedAt := time.Now().UTC()
	plugin.UpdateRemoteSnapshots("peer-a", []RouteStatus{
		{
			Provider: schemas.OpenAI,
			Model:    "gpt-4o",
			KeyID:    "key-a",
			RouteSnapshot: RouteSnapshot{
				Samples:             4,
				Successes:           3,
				Failures:            1,
				ConsecutiveFailures: 1,
				ErrorEWMA:           0.2,
				LatencyEWMA:         240,
				LastUpdated:         remoteUpdatedAt,
			},
		},
	}, []DirectionStatus{
		{
			Provider: schemas.OpenAI,
			Model:    "gpt-4o",
			DirectionSnapshot: DirectionSnapshot{
				Samples:             4,
				Successes:           3,
				Failures:            1,
				ConsecutiveFailures: 1,
				ErrorEWMA:           0.2,
				LatencyEWMA:         240,
				LastUpdated:         remoteUpdatedAt,
			},
		},
	})

	localRoutes := plugin.ListLocalSnapshots(schemas.OpenAI, "gpt-4o")
	if len(localRoutes) != 1 || localRoutes[0].Samples != 1 {
		t.Fatalf("expected local-only route status to stay at 1 sample, got %+v", localRoutes)
	}

	mergedRoutes := plugin.ListSnapshots(schemas.OpenAI, "gpt-4o")
	if len(mergedRoutes) != 1 || mergedRoutes[0].Samples != 5 || mergedRoutes[0].Failures != 1 {
		t.Fatalf("expected merged route status to include remote snapshots, got %+v", mergedRoutes)
	}

	localDirections := plugin.ListLocalDirectionSnapshots(schemas.OpenAI, "gpt-4o")
	if len(localDirections) != 1 || localDirections[0].Samples != 1 {
		t.Fatalf("expected local-only direction status to stay at 1 sample, got %+v", localDirections)
	}

	mergedDirections := plugin.ListDirectionSnapshots(schemas.OpenAI, "gpt-4o")
	if len(mergedDirections) != 1 || mergedDirections[0].Samples != 5 || mergedDirections[0].Failures != 1 {
		t.Fatalf("expected merged direction status to include remote snapshots, got %+v", mergedDirections)
	}

	plugin.PruneRemoteNode("peer-a")
	postPrune := plugin.ListSnapshots(schemas.OpenAI, "gpt-4o")
	if len(postPrune) != 1 || postPrune[0].Samples != 1 {
		t.Fatalf("expected pruning remote node to restore local-only aggregate, got %+v", postPrune)
	}
}
