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

	disableContentLogging, loggingHeaders := plugin.CurrentClientConfigBindings()
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
