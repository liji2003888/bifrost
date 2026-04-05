package loadbalancer

import (
	"context"
	"testing"

	bifrost "github.com/maximhq/bifrost/core"
	"github.com/maximhq/bifrost/core/schemas"
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
