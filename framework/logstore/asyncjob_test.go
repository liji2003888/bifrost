package logstore

import (
	"context"
	"testing"
	"time"

	"github.com/maximhq/bifrost/core/schemas"
)

func TestAsyncJobExecutorPropagatesUserValuesIntoBackgroundContext(t *testing.T) {
	store := newGovernanceTrackingTestStore(t)
	executor := NewAsyncJobExecutor(store, nil, governanceTrackingTestLogger{})

	contextKey := schemas.BifrostContextKey("test-user-value")
	seenValues := make(chan map[string]any, 1)

	job, err := executor.SubmitJob(
		nil,
		5,
		func(ctx *schemas.BifrostContext) (interface{}, *schemas.BifrostError) {
			seenValues <- map[string]any{
				"user_value": ctx.Value(contextKey),
				"is_async":   ctx.Value(schemas.BifrostIsAsyncRequest),
			}
			return map[string]string{"status": "ok"}, nil
		},
		schemas.ChatCompletionRequest,
		map[any]any{
			contextKey: "preserved-value",
		},
	)
	if err != nil {
		t.Fatalf("SubmitJob() error = %v", err)
	}

	select {
	case seen := <-seenValues:
		if seen["user_value"] != "preserved-value" {
			t.Fatalf("expected async context to preserve user value, got %+v", seen["user_value"])
		}
		if seen["is_async"] != true {
			t.Fatalf("expected async marker to be true, got %+v", seen["is_async"])
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for async job execution")
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		storedJob, findErr := store.FindAsyncJobByID(context.Background(), job.ID)
		if findErr == nil && storedJob.Status == schemas.AsyncJobStatusCompleted {
			if storedJob.Response == "" {
				t.Fatal("expected async job response to be stored after completion")
			}
			return
		}
		time.Sleep(25 * time.Millisecond)
	}

	t.Fatalf("async job %s did not reach completed state", job.ID)
}
