package logging

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework/configstore"
	"github.com/maximhq/bifrost/framework/logstore"
	"github.com/maximhq/bifrost/framework/streaming"
)

type testLogger struct{}

func (testLogger) Debug(string, ...any)                   {}
func (testLogger) Info(string, ...any)                    {}
func (testLogger) Warn(string, ...any)                    {}
func (testLogger) Error(string, ...any)                   {}
func (testLogger) Fatal(string, ...any)                   {}
func (testLogger) SetLevel(schemas.LogLevel)              {}
func (testLogger) SetOutputType(schemas.LoggerOutputType) {}
func (testLogger) LogHTTPRequest(schemas.LogLevel, string) schemas.LogEventBuilder {
	return schemas.NoopLogEvent
}

func newTestStore(t *testing.T) logstore.LogStore {
	t.Helper()

	store, err := logstore.NewLogStore(context.Background(), &logstore.Config{
		Enabled: true,
		Type:    logstore.LogStoreTypeSQLite,
		Config: &logstore.SQLiteConfig{
			Path: filepath.Join(t.TempDir(), "logging.db"),
		},
	}, testLogger{})
	if err != nil {
		t.Fatalf("NewLogStore() error = %v", err)
	}
	return store
}

func TestUpdateLogEntryPreservesResponsesInputContentSummary(t *testing.T) {
	store := newTestStore(t)
	plugin := &LoggerPlugin{
		store:  store,
		logger: testLogger{},
	}

	requestID := "req-1"
	now := time.Now().UTC()
	inputText := "request-side text"
	initial := &InitialLogData{
		Object:   "responses",
		Provider: "openai",
		Model:    "gpt-4o-mini",
		ResponsesInputHistory: []schemas.ResponsesMessage{{
			Content: &schemas.ResponsesMessageContent{
				ContentStr: &inputText,
			},
		}},
	}

	if err := plugin.insertInitialLogEntry(context.Background(), requestID, "", now, 0, nil, initial); err != nil {
		t.Fatalf("insertInitialLogEntry() error = %v", err)
	}

	responsesText := "responses output"
	update := &UpdateLogData{
		Status: "success",
		ResponsesOutput: []schemas.ResponsesMessage{{
			Content: &schemas.ResponsesMessageContent{
				ContentStr: &responsesText,
			},
		}},
	}

	if err := plugin.updateLogEntry(context.Background(), requestID, "", "", 10, "", "", "", "", 0, nil, "", update); err != nil {
		t.Fatalf("updateLogEntry() error = %v", err)
	}

	logEntry, err := store.FindByID(context.Background(), requestID)
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}
	if !strings.Contains(logEntry.ContentSummary, inputText) {
		t.Fatalf("expected content summary to preserve responses input, got %q", logEntry.ContentSummary)
	}
	if strings.Contains(logEntry.ContentSummary, responsesText) {
		t.Fatalf("expected content summary to avoid overwriting with responses output-only data, got %q", logEntry.ContentSummary)
	}
}

func TestUpdateLogEntryUpdatesContentSummaryForChatOutput(t *testing.T) {
	store := newTestStore(t)
	plugin := &LoggerPlugin{
		store:  store,
		logger: testLogger{},
	}

	requestID := "req-chat"
	now := time.Now().UTC()
	initial := &InitialLogData{
		Object:   "chat_completion",
		Provider: "openai",
		Model:    "gpt-4o-mini",
	}

	if err := plugin.insertInitialLogEntry(context.Background(), requestID, "", now, 0, nil, initial); err != nil {
		t.Fatalf("insertInitialLogEntry() error = %v", err)
	}

	chatText := "assistant output"
	update := &UpdateLogData{
		Status: "success",
		ChatOutput: &schemas.ChatMessage{
			Role: schemas.ChatMessageRoleAssistant,
			Content: &schemas.ChatMessageContent{
				ContentStr: &chatText,
			},
		},
	}

	if err := plugin.updateLogEntry(context.Background(), requestID, "", "", 10, "", "", "", "", 0, nil, "", update); err != nil {
		t.Fatalf("updateLogEntry() error = %v", err)
	}

	logEntry, err := store.FindByID(context.Background(), requestID)
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}
	if !strings.Contains(logEntry.ContentSummary, chatText) {
		t.Fatalf("expected content summary to include chat output, got %q", logEntry.ContentSummary)
	}
}

func TestUpdateLogEntrySuppressesChatOutputWhenContentLoggingDisabled(t *testing.T) {
	store := newTestStore(t)
	disableContentLogging := true
	plugin := &LoggerPlugin{
		store:                 store,
		logger:                testLogger{},
		disableContentLogging: &disableContentLogging,
	}

	requestID := "req-chat-disabled"
	now := time.Now().UTC()
	initial := &InitialLogData{
		Object:   "chat_completion",
		Provider: "openai",
		Model:    "gpt-4o-mini",
	}

	if err := plugin.insertInitialLogEntry(context.Background(), requestID, "", now, 0, nil, initial); err != nil {
		t.Fatalf("insertInitialLogEntry() error = %v", err)
	}

	chatText := "assistant output should not be logged"
	update := &UpdateLogData{
		Status: "success",
		ChatOutput: &schemas.ChatMessage{
			Role: schemas.ChatMessageRoleAssistant,
			Content: &schemas.ChatMessageContent{
				ContentStr: &chatText,
			},
		},
	}

	if err := plugin.updateLogEntry(context.Background(), requestID, "", "", 10, "", "", "", "", 0, nil, "", update); err != nil {
		t.Fatalf("updateLogEntry() error = %v", err)
	}

	logEntry, err := store.FindByID(context.Background(), requestID)
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}
	if logEntry.OutputMessage != "" {
		t.Fatalf("expected output_message to be suppressed, got %q", logEntry.OutputMessage)
	}
	if strings.Contains(logEntry.ContentSummary, chatText) {
		t.Fatalf("expected content summary to suppress chat output, got %q", logEntry.ContentSummary)
	}
}

func TestBindClientConfigRebindsContentLoggingBehavior(t *testing.T) {
	store := newTestStore(t)

	staleClientConfig := &configstore.ClientConfig{
		DisableContentLogging: true,
		LoggingHeaders:        []string{"x-stale"},
	}
	plugin, err := Init(context.Background(), &Config{
		DisableContentLogging: &staleClientConfig.DisableContentLogging,
		LoggingHeaders:        &staleClientConfig.LoggingHeaders,
	}, testLogger{}, store, nil, nil)
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer plugin.Cleanup()

	requestID := "req-chat-rebind"
	now := time.Now().UTC()
	initial := &InitialLogData{
		Object:   "chat_completion",
		Provider: "openai",
		Model:    "gpt-4o-mini",
	}

	if err := plugin.insertInitialLogEntry(context.Background(), requestID, "", now, 0, nil, initial); err != nil {
		t.Fatalf("insertInitialLogEntry() error = %v", err)
	}

	updatedClientConfig := &configstore.ClientConfig{
		DisableContentLogging: false,
		LoggingHeaders:        []string{"x-updated"},
	}
	plugin.BindClientConfig(updatedClientConfig)

	chatText := "assistant output after rebind"
	update := &UpdateLogData{
		Status: "success",
		ChatOutput: &schemas.ChatMessage{
			Role: schemas.ChatMessageRoleAssistant,
			Content: &schemas.ChatMessageContent{
				ContentStr: &chatText,
			},
		},
	}

	if err := plugin.updateLogEntry(context.Background(), requestID, "", "", 10, "", "", "", "", 0, nil, "", update); err != nil {
		t.Fatalf("updateLogEntry() error = %v", err)
	}

	logEntry, err := store.FindByID(context.Background(), requestID)
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}
	if logEntry.OutputMessage == "" {
		t.Fatal("expected output_message to be logged after rebinding client config")
	}
	if !strings.Contains(logEntry.ContentSummary, chatText) {
		t.Fatalf("expected content summary to include chat output after rebind, got %q", logEntry.ContentSummary)
	}

	enableLogging, disableContentLogging, loggingHeaders := plugin.CurrentClientConfigBindings()
	if !enableLogging {
		t.Fatal("expected enable_logging to remain true after rebinding client config")
	}
	if disableContentLogging {
		t.Fatal("expected disable_content_logging to be false after rebinding client config")
	}
	if !strings.EqualFold(strings.Join(loggingHeaders, ","), strings.Join(updatedClientConfig.LoggingHeaders, ",")) {
		t.Fatalf("expected logging headers %v, got %v", updatedClientConfig.LoggingHeaders, loggingHeaders)
	}
}

func TestUpdateStreamingLogEntryPreservesResponsesInputContentSummary(t *testing.T) {
	store := newTestStore(t)
	plugin := &LoggerPlugin{
		store:  store,
		logger: testLogger{},
	}

	requestID := "req-stream"
	now := time.Now().UTC()
	inputText := "stream request-side text"
	initial := &InitialLogData{
		Object:   "responses_stream",
		Provider: "openai",
		Model:    "gpt-4o-mini",
		ResponsesInputHistory: []schemas.ResponsesMessage{{
			Content: &schemas.ResponsesMessageContent{
				ContentStr: &inputText,
			},
		}},
	}

	if err := plugin.insertInitialLogEntry(context.Background(), requestID, "", now, 0, nil, initial); err != nil {
		t.Fatalf("insertInitialLogEntry() error = %v", err)
	}

	responsesText := "streamed response text"
	streamResponse := &streaming.ProcessedStreamResponse{
		Data: &streaming.AccumulatedData{
			Latency: 25,
			TokenUsage: &schemas.BifrostLLMUsage{
				PromptTokens:     10,
				CompletionTokens: 5,
				TotalTokens:      15,
			},
			OutputMessages: []schemas.ResponsesMessage{{
				Content: &schemas.ResponsesMessageContent{
					ContentStr: &responsesText,
				},
			}},
		},
	}

	if err := plugin.updateStreamingLogEntry(context.Background(), requestID, "", "", "", "", "", "", 0, nil, "", streamResponse, true, false, false); err != nil {
		t.Fatalf("updateStreamingLogEntry() error = %v", err)
	}

	logEntry, err := store.FindByID(context.Background(), requestID)
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}
	if logEntry.TokenUsageParsed == nil || logEntry.TokenUsageParsed.TotalTokens != 15 {
		t.Fatalf("expected token usage to be updated, got %+v", logEntry.TokenUsageParsed)
	}
	if !strings.Contains(logEntry.ContentSummary, inputText) {
		t.Fatalf("expected content summary to preserve responses input, got %q", logEntry.ContentSummary)
	}
	if strings.Contains(logEntry.ContentSummary, responsesText) {
		t.Fatalf("expected content summary to avoid overwriting with streamed responses output-only data, got %q", logEntry.ContentSummary)
	}
}

func TestApplyNonStreamingOutputToEntryCapturesRerankUsageAndOutput(t *testing.T) {
	plugin := &LoggerPlugin{
		logger: testLogger{},
	}

	entry := &logstore.Log{}
	result := &schemas.BifrostResponse{
		RerankResponse: &schemas.BifrostRerankResponse{
			Usage: &schemas.BifrostLLMUsage{
				TotalTokens: 43,
			},
			Results: []schemas.RerankResult{
				{Index: 0, RelevanceScore: 0.9678807854652405},
				{Index: 2, RelevanceScore: 0.9021470546722412},
				{Index: 1, RelevanceScore: 0.8326528072357178},
			},
		},
	}

	plugin.applyNonStreamingOutputToEntry(entry, result)

	if entry.TokenUsageParsed == nil || entry.TokenUsageParsed.TotalTokens != 43 {
		t.Fatalf("expected rerank total tokens to be logged, got %+v", entry.TokenUsageParsed)
	}
	if entry.TotalTokens != 43 {
		t.Fatalf("expected total_tokens=43 on log entry, got %d", entry.TotalTokens)
	}
	if len(entry.RerankOutputParsed) != 3 {
		t.Fatalf("expected rerank output to be logged, got %+v", entry.RerankOutputParsed)
	}

	if err := entry.SerializeFields(); err != nil {
		t.Fatalf("SerializeFields() error = %v", err)
	}
	if entry.TokenUsage == "" {
		t.Fatal("expected serialized token_usage to be populated for rerank logs")
	}
	if entry.RerankOutput == "" {
		t.Fatal("expected serialized rerank_output to be populated")
	}

	roundTrip := &logstore.Log{
		TokenUsage:   entry.TokenUsage,
		RerankOutput: entry.RerankOutput,
	}
	if err := roundTrip.DeserializeFields(); err != nil {
		t.Fatalf("DeserializeFields() error = %v", err)
	}
	if roundTrip.TokenUsageParsed == nil || roundTrip.TokenUsageParsed.TotalTokens != 43 {
		t.Fatalf("expected round-tripped rerank total tokens to be preserved, got %+v", roundTrip.TokenUsageParsed)
	}
	if len(roundTrip.RerankOutputParsed) != 3 {
		t.Fatalf("expected round-tripped rerank output to be preserved, got %+v", roundTrip.RerankOutputParsed)
	}
}

func TestApplyNonStreamingOutputToEntryKeepsRerankUsageWhenContentLoggingDisabled(t *testing.T) {
	disableContentLogging := true
	plugin := &LoggerPlugin{
		logger:                testLogger{},
		disableContentLogging: &disableContentLogging,
	}

	entry := &logstore.Log{}
	result := &schemas.BifrostResponse{
		RerankResponse: &schemas.BifrostRerankResponse{
			Usage: &schemas.BifrostLLMUsage{
				TotalTokens: 43,
			},
			Results: []schemas.RerankResult{
				{Index: 0, RelevanceScore: 0.9678807854652405},
			},
		},
	}

	plugin.applyNonStreamingOutputToEntry(entry, result)

	if entry.TokenUsageParsed == nil || entry.TokenUsageParsed.TotalTokens != 43 {
		t.Fatalf("expected rerank total tokens to be logged even when content logging is disabled, got %+v", entry.TokenUsageParsed)
	}
	if len(entry.RerankOutputParsed) != 0 {
		t.Fatalf("expected rerank output to be suppressed when content logging is disabled, got %+v", entry.RerankOutputParsed)
	}
}

func TestApplyNonStreamingOutputToEntryKeepsRerankUsageWhenFullLoggingDisabled(t *testing.T) {
	enableLogging := false
	plugin := &LoggerPlugin{
		logger:        testLogger{},
		enableLogging: &enableLogging,
	}

	entry := &logstore.Log{}
	result := &schemas.BifrostResponse{
		RerankResponse: &schemas.BifrostRerankResponse{
			Usage: &schemas.BifrostLLMUsage{
				TotalTokens: 43,
			},
			Results: []schemas.RerankResult{
				{Index: 0, RelevanceScore: 0.9678807854652405},
			},
		},
	}

	plugin.applyNonStreamingOutputToEntry(entry, result)

	if entry.TokenUsageParsed == nil || entry.TokenUsageParsed.TotalTokens != 43 {
		t.Fatalf("expected rerank total tokens to be logged even when full logging is disabled, got %+v", entry.TokenUsageParsed)
	}
	if len(entry.RerankOutputParsed) != 0 {
		t.Fatalf("expected rerank output to be suppressed when full logging is disabled, got %+v", entry.RerankOutputParsed)
	}
}

func TestApplyNonStreamingOutputToEntryCapturesModelAlias(t *testing.T) {
	plugin := &LoggerPlugin{
		logger: testLogger{},
	}

	entry := &logstore.Log{
		Model: "gpt-4o",
	}
	result := &schemas.BifrostResponse{
		ChatResponse: &schemas.BifrostChatResponse{
			Model: "openai/gpt-4o-2024-08-06",
		},
	}
	result.PopulateExtraFields(schemas.ChatCompletionRequest, schemas.OpenAI, "gpt-4o", "openai/gpt-4o-2024-08-06")

	plugin.applyNonStreamingOutputToEntry(entry, result)

	if entry.Model != "openai/gpt-4o-2024-08-06" {
		t.Fatalf("expected resolved model to be stored on log entry, got %q", entry.Model)
	}
	if entry.Alias == nil || *entry.Alias != "gpt-4o" {
		t.Fatalf("expected requested model alias to be stored, got %+v", entry.Alias)
	}
}

func TestApplyStreamingOutputToEntryCapturesModelAlias(t *testing.T) {
	plugin := &LoggerPlugin{
		logger: testLogger{},
	}

	entry := &logstore.Log{
		Model: "claude-opus-4-5",
	}
	streamResponse := &streaming.ProcessedStreamResponse{
		Model: "bedrock/anthropic.claude-opus-4-5-20250301-v1:0",
		Data: &streaming.AccumulatedData{
			Model: "bedrock/anthropic.claude-opus-4-5-20250301-v1:0",
		},
	}

	plugin.applyStreamingOutputToEntry(entry, streamResponse)

	if entry.Model != "bedrock/anthropic.claude-opus-4-5-20250301-v1:0" {
		t.Fatalf("expected resolved streaming model to be stored on log entry, got %q", entry.Model)
	}
	if entry.Alias == nil || *entry.Alias != "claude-opus-4-5" {
		t.Fatalf("expected original requested model alias to be preserved, got %+v", entry.Alias)
	}
}

func TestRerankLogsPersistThroughPreAndPostHooks(t *testing.T) {
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

	requestID := "req-rerank-hooks"
	bifrostCtx := schemas.NewBifrostContext(context.Background(), schemas.NoDeadline)
	bifrostCtx.SetValue(schemas.BifrostContextKeyRequestID, requestID)

	topN := 3
	req := &schemas.BifrostRequest{
		RequestType: schemas.RerankRequest,
		RerankRequest: &schemas.BifrostRerankRequest{
			Provider: schemas.VLLM,
			Model:    "qwen3-8b-reranker",
			Query:    "什么是深度学习？",
			Documents: []schemas.RerankDocument{
				{Text: "深度学习是机器学习的一个子集，基于人工神经网络。"},
				{Text: "今天中午吃红烧肉。"},
				{Text: "苹果是一种水果，富含维生素。"},
			},
			Params: &schemas.RerankParameters{
				TopN: &topN,
			},
		},
	}

	if _, _, err := plugin.PreLLMHook(bifrostCtx, req); err != nil {
		t.Fatalf("PreLLMHook() error = %v", err)
	}

	result := &schemas.BifrostResponse{
		RerankResponse: &schemas.BifrostRerankResponse{
			Model: "qwen3-8b-reranker",
			Usage: &schemas.BifrostLLMUsage{
				TotalTokens: 43,
			},
			Results: []schemas.RerankResult{
				{Index: 0, RelevanceScore: 0.9678807854652405},
				{Index: 2, RelevanceScore: 0.9021470546722412},
				{Index: 1, RelevanceScore: 0.8326528072357178},
			},
			ExtraFields: schemas.BifrostResponseExtraFields{
				RequestType: schemas.RerankRequest,
				Provider:    schemas.VLLM,
				Latency:     130,
			},
		},
	}

	if _, _, err := plugin.PostLLMHook(bifrostCtx, result, nil); err != nil {
		t.Fatalf("PostLLMHook() error = %v", err)
	}

	var logEntry *logstore.Log
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		logEntry, err = store.FindByID(context.Background(), requestID)
		if err == nil {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}

	if logEntry.TokenUsageParsed == nil || logEntry.TokenUsageParsed.TotalTokens != 43 {
		t.Fatalf("expected rerank total tokens to persist through hooks, got %+v", logEntry.TokenUsageParsed)
	}
	if len(logEntry.RerankOutputParsed) != 3 {
		t.Fatalf("expected rerank output to persist through hooks, got %+v", logEntry.RerankOutputParsed)
	}
	if logEntry.Object != string(schemas.RerankRequest) {
		t.Fatalf("expected object %q, got %q", schemas.RerankRequest, logEntry.Object)
	}
}
