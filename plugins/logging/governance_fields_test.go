package logging

import (
	"context"
	"testing"
	"time"

	"github.com/maximhq/bifrost/core/schemas"
)

func TestPostLLMHookPersistsGovernanceTrackingFields(t *testing.T) {
	store := newTestStore(t)
	plugin, err := Init(context.Background(), &Config{}, testLogger{}, store, nil, nil)
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer func() {
		if cleanupErr := plugin.Cleanup(); cleanupErr != nil {
			t.Fatalf("Cleanup() error = %v", cleanupErr)
		}
	}()

	requestID := "req-governance-fields"
	bifrostCtx := schemas.NewBifrostContext(context.Background(), schemas.NoDeadline)
	bifrostCtx.SetValue(schemas.BifrostContextKeyRequestID, requestID)
	bifrostCtx.SetValue(schemas.BifrostContextKeyGovernanceUserID, "user-42")
	bifrostCtx.SetValue(schemas.BifrostContextKeyGovernanceTeamID, "team-7")
	bifrostCtx.SetValue(schemas.BifrostContextKeyGovernanceTeamName, "Core Team")
	bifrostCtx.SetValue(schemas.BifrostContextKeyGovernanceCustomerID, "customer-3")
	bifrostCtx.SetValue(schemas.BifrostContextKeyGovernanceCustomerName, "Acme Corp")

	req := &schemas.BifrostRequest{
		RequestType: schemas.RerankRequest,
		RerankRequest: &schemas.BifrostRerankRequest{
			Provider: schemas.VLLM,
			Model:    "qwen3-8b-reranker",
			Query:    "什么是深度学习？",
			Documents: []schemas.RerankDocument{
				{Text: "深度学习是机器学习的一个子集，基于人工神经网络。"},
				{Text: "今天中午吃红烧肉。"},
			},
		},
	}

	if _, _, err := plugin.PreLLMHook(bifrostCtx, req); err != nil {
		t.Fatalf("PreLLMHook() error = %v", err)
	}

	result := &schemas.BifrostResponse{
		RerankResponse: &schemas.BifrostRerankResponse{
			Model: "qwen3-8b-reranker",
			Usage: &schemas.BifrostLLMUsage{TotalTokens: 12},
			Results: []schemas.RerankResult{
				{Index: 0, RelevanceScore: 0.98},
			},
			ExtraFields: schemas.BifrostResponseExtraFields{
				RequestType: schemas.RerankRequest,
				Provider:    schemas.VLLM,
				Latency:     32,
			},
		},
	}

	if _, _, err := plugin.PostLLMHook(bifrostCtx, result, nil); err != nil {
		t.Fatalf("PostLLMHook() error = %v", err)
	}

	var entryUserID, entryTeamID, entryTeamName, entryCustomerID, entryCustomerName string
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		entry, findErr := store.FindByID(context.Background(), requestID)
		if findErr == nil {
			if entry.UserID != nil {
				entryUserID = *entry.UserID
			}
			if entry.TeamID != nil {
				entryTeamID = *entry.TeamID
			}
			if entry.TeamName != nil {
				entryTeamName = *entry.TeamName
			}
			if entry.CustomerID != nil {
				entryCustomerID = *entry.CustomerID
			}
			if entry.CustomerName != nil {
				entryCustomerName = *entry.CustomerName
			}
			break
		}
		time.Sleep(25 * time.Millisecond)
	}

	if entryUserID != "user-42" {
		t.Fatalf("expected user id to persist, got %q", entryUserID)
	}
	if entryTeamID != "team-7" || entryTeamName != "Core Team" {
		t.Fatalf("expected team data to persist, got id=%q name=%q", entryTeamID, entryTeamName)
	}
	if entryCustomerID != "customer-3" || entryCustomerName != "Acme Corp" {
		t.Fatalf("expected customer data to persist, got id=%q name=%q", entryCustomerID, entryCustomerName)
	}
}
