package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	bifrost "github.com/maximhq/bifrost/core"
	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework/configstore"
	configstoreTables "github.com/maximhq/bifrost/framework/configstore/tables"
	"github.com/maximhq/bifrost/transports/bifrost-http/lib"
)

type hostedToolPreviewStore struct {
	configstore.ConfigStore
	tool *configstoreTables.TableMCPHostedTool
}

func (s *hostedToolPreviewStore) GetMCPHostedToolByID(_ context.Context, id string) (*configstoreTables.TableMCPHostedTool, error) {
	if s.tool == nil || s.tool.ToolID != id {
		return nil, configstore.ErrNotFound
	}
	cloned := *s.tool
	return &cloned, nil
}

func TestExecuteHostedMCPToolAppliesQueryParamsAndExtractsJSONPath(t *testing.T) {
	SetLogger(bifrost.NewNoOpLogger())

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("tenant_id"); got != "tenant-1" {
			t.Fatalf("expected tenant_id query param to be forwarded, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"summary": "tenant-1 summary",
			},
		})
	}))
	defer upstream.Close()

	responsePath := "data.summary"
	server := &BifrostHTTPServer{}
	tool := &configstoreTables.TableMCPHostedTool{
		Name:   "search_catalog",
		Method: http.MethodGet,
		URL:    upstream.URL + "/search",
		QueryParams: map[string]string{
			"tenant_id": "{{args.tenant_id}}",
		},
		ResponseJSONPath: &responsePath,
	}

	result, err := server.executeHostedMCPTool(context.Background(), tool, map[string]any{
		"tenant_id": "tenant-1",
	})
	if err != nil {
		t.Fatalf("executeHostedMCPTool returned error: %v", err)
	}
	if result != "tenant-1 summary" {
		t.Fatalf("expected extracted summary, got %q", result)
	}
}

func TestExecuteHostedMCPToolAppliesResponseTemplate(t *testing.T) {
	SetLogger(bifrost.NewNoOpLogger())

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"user": map[string]any{
				"name": "Alice",
			},
			"team": "platform",
		})
	}))
	defer upstream.Close()

	responseTemplate := "User {{response.user.name}} belongs to {{response.team}}"
	server := &BifrostHTTPServer{}
	tool := &configstoreTables.TableMCPHostedTool{
		Name:             "get_user_profile",
		Method:           http.MethodGet,
		URL:              upstream.URL + "/profile",
		ResponseTemplate: &responseTemplate,
	}

	result, err := server.executeHostedMCPTool(context.Background(), tool, map[string]any{})
	if err != nil {
		t.Fatalf("executeHostedMCPTool returned error: %v", err)
	}
	if result != "User Alice belongs to platform" {
		t.Fatalf("expected structured response template output, got %q", result)
	}
}

func TestExecuteHostedMCPToolSupportsResponseRawTemplate(t *testing.T) {
	SetLogger(bifrost.NewNoOpLogger())

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer upstream.Close()

	responseTemplate := "Raw => {{response.raw}}"
	server := &BifrostHTTPServer{}
	tool := &configstoreTables.TableMCPHostedTool{
		Name:             "inspect_raw_response",
		Method:           http.MethodGet,
		URL:              upstream.URL + "/raw",
		ResponseTemplate: &responseTemplate,
	}

	result, err := server.executeHostedMCPTool(context.Background(), tool, map[string]any{})
	if err != nil {
		t.Fatalf("executeHostedMCPTool returned error: %v", err)
	}
	if result != `Raw => {"ok":true}` {
		t.Fatalf("expected raw response template output, got %q", result)
	}
}

func TestExecuteHostedMCPToolUsesRequestHeadersInQueryAndBodyTemplates(t *testing.T) {
	SetLogger(bifrost.NewNoOpLogger())

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("tenant"); got != "tenant-42" {
			t.Fatalf("expected request header mapped into query param, got %q", got)
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		if payload["request_id"] != "req-9" {
			t.Fatalf("expected request header mapped into body template, got %+v", payload)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}))
	defer upstream.Close()

	bodyTemplate := `{"request_id":"{{req.header.x-request-id}}"}`
	server := &BifrostHTTPServer{}
	tool := &configstoreTables.TableMCPHostedTool{
		Name:   "sync_request",
		Method: http.MethodPost,
		URL:    upstream.URL + "/sync",
		QueryParams: map[string]string{
			"tenant": "{{req.header.x-tenant-id}}",
		},
		BodyTemplate: &bodyTemplate,
	}

	bfCtx := schemas.NewBifrostContext(context.Background(), schemas.NoDeadline)
	bfCtx.SetValue(schemas.BifrostContextKeyRequestHeaders, map[string]string{
		"x-tenant-id":  "tenant-42",
		"x-request-id": "req-9",
	})

	result, err := server.executeHostedMCPTool(bfCtx, tool, map[string]any{})
	if err != nil {
		t.Fatalf("executeHostedMCPTool returned error: %v", err)
	}
	if result == "" {
		t.Fatal("expected non-empty response")
	}
}

func TestExecuteHostedMCPToolRejectsMissingRequiredArgs(t *testing.T) {
	SetLogger(bifrost.NewNoOpLogger())

	server := &BifrostHTTPServer{}
	tool := &configstoreTables.TableMCPHostedTool{
		Name:   "required_args_tool",
		Method: http.MethodPost,
		URL:    "http://example.com/unused",
		ToolSchema: schemas.ChatTool{
			Type: schemas.ChatToolTypeFunction,
			Function: &schemas.ChatToolFunction{
				Name: "required_args_tool",
				Parameters: &schemas.ToolFunctionParameters{
					Type: "object",
					Properties: schemas.NewOrderedMapFromPairs(
						schemas.KV("tenant_id", map[string]any{"type": "string"}),
					),
					Required: []string{"tenant_id"},
				},
			},
		},
	}

	_, err := server.executeHostedMCPTool(context.Background(), tool, map[string]any{})
	if err == nil || !strings.Contains(err.Error(), "args.tenant_id is required") {
		t.Fatalf("expected required arg validation error, got %v", err)
	}
}

func TestExecuteHostedMCPToolRejectsWrongArgType(t *testing.T) {
	SetLogger(bifrost.NewNoOpLogger())

	server := &BifrostHTTPServer{}
	tool := &configstoreTables.TableMCPHostedTool{
		Name:   "typed_args_tool",
		Method: http.MethodPost,
		URL:    "http://example.com/unused",
		ToolSchema: schemas.ChatTool{
			Type: schemas.ChatToolTypeFunction,
			Function: &schemas.ChatToolFunction{
				Name: "typed_args_tool",
				Parameters: &schemas.ToolFunctionParameters{
					Type: "object",
					Properties: schemas.NewOrderedMapFromPairs(
						schemas.KV("limit", map[string]any{"type": "integer"}),
					),
					Required: []string{"limit"},
				},
			},
		},
	}

	_, err := server.executeHostedMCPTool(context.Background(), tool, map[string]any{
		"limit": "10",
	})
	if err == nil || !strings.Contains(err.Error(), "args.limit must be an integer") {
		t.Fatalf("expected integer validation error, got %v", err)
	}
}

func TestExecuteHostedMCPToolRejectsUnknownArgsWhenAdditionalPropertiesDisabled(t *testing.T) {
	SetLogger(bifrost.NewNoOpLogger())

	additionalProps := false
	server := &BifrostHTTPServer{}
	tool := &configstoreTables.TableMCPHostedTool{
		Name:   "strict_args_tool",
		Method: http.MethodPost,
		URL:    "http://example.com/unused",
		ToolSchema: schemas.ChatTool{
			Type: schemas.ChatToolTypeFunction,
			Function: &schemas.ChatToolFunction{
				Name: "strict_args_tool",
				Parameters: &schemas.ToolFunctionParameters{
					Type: "object",
					Properties: schemas.NewOrderedMapFromPairs(
						schemas.KV("query", map[string]any{"type": "string"}),
					),
					AdditionalProperties: &schemas.AdditionalPropertiesStruct{
						AdditionalPropertiesBool: &additionalProps,
					},
				},
			},
		},
	}

	_, err := server.executeHostedMCPTool(context.Background(), tool, map[string]any{
		"query":   "hello",
		"unknown": "value",
	})
	if err == nil || !strings.Contains(err.Error(), "args.unknown is not allowed") {
		t.Fatalf("expected additional properties validation error, got %v", err)
	}
}

func TestExecuteHostedMCPToolAcceptsValidArgsAgainstSchema(t *testing.T) {
	SetLogger(bifrost.NewNoOpLogger())

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer upstream.Close()

	additionalProps := false
	server := &BifrostHTTPServer{}
	tool := &configstoreTables.TableMCPHostedTool{
		Name:   "valid_schema_tool",
		Method: http.MethodPost,
		URL:    upstream.URL + "/lookup",
		ToolSchema: schemas.ChatTool{
			Type: schemas.ChatToolTypeFunction,
			Function: &schemas.ChatToolFunction{
				Name: "valid_schema_tool",
				Parameters: &schemas.ToolFunctionParameters{
					Type: "object",
					Properties: schemas.NewOrderedMapFromPairs(
						schemas.KV("query", map[string]any{"type": "string"}),
						schemas.KV("limit", map[string]any{"type": "integer", "minimum": 1.0}),
					),
					Required: []string{"query", "limit"},
					AdditionalProperties: &schemas.AdditionalPropertiesStruct{
						AdditionalPropertiesBool: &additionalProps,
					},
				},
			},
		},
	}

	result, err := server.executeHostedMCPTool(context.Background(), tool, map[string]any{
		"query": "hello",
		"limit": 5,
	})
	if err != nil {
		t.Fatalf("expected schema-valid execution to succeed, got %v", err)
	}
	if result == "" {
		t.Fatal("expected non-empty response")
	}
}

func TestExecuteHostedMCPToolAppliesBearerPassthroughAuthProfile(t *testing.T) {
	SetLogger(bifrost.NewNoOpLogger())

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer tenant-token" {
			t.Fatalf("expected authorization header passthrough, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer upstream.Close()

	server := &BifrostHTTPServer{}
	tool := &configstoreTables.TableMCPHostedTool{
		Name:   "secured_lookup",
		Method: http.MethodGet,
		URL:    upstream.URL + "/lookup",
		AuthProfile: &configstoreTables.MCPHostedToolAuthProfile{
			Mode: configstoreTables.MCPHostedToolAuthModeBearerPassthrough,
		},
	}

	bfCtx := schemas.NewBifrostContext(context.Background(), schemas.NoDeadline)
	bfCtx.SetValue(schemas.BifrostContextKeyRequestHeaders, map[string]string{
		"authorization": "Bearer tenant-token",
	})

	result, err := server.executeHostedMCPTool(bfCtx, tool, map[string]any{})
	if err != nil {
		t.Fatalf("executeHostedMCPTool returned error: %v", err)
	}
	if result == "" {
		t.Fatal("expected non-empty response")
	}
}

func TestExecuteHostedMCPToolAppliesHeaderPassthroughAuthProfile(t *testing.T) {
	SetLogger(bifrost.NewNoOpLogger())

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Tenant-ID"); got != "tenant-42" {
			t.Fatalf("expected tenant passthrough header, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer upstream.Close()

	server := &BifrostHTTPServer{}
	tool := &configstoreTables.TableMCPHostedTool{
		Name:   "header_mapped_lookup",
		Method: http.MethodGet,
		URL:    upstream.URL + "/lookup",
		AuthProfile: &configstoreTables.MCPHostedToolAuthProfile{
			Mode: configstoreTables.MCPHostedToolAuthModeHeaderPassthrough,
			HeaderMappings: map[string]string{
				"X-Tenant-ID": "x-tenant-id",
			},
		},
	}

	bfCtx := schemas.NewBifrostContext(context.Background(), schemas.NoDeadline)
	bfCtx.SetValue(schemas.BifrostContextKeyRequestHeaders, map[string]string{
		"x-tenant-id": "tenant-42",
	})

	result, err := server.executeHostedMCPTool(bfCtx, tool, map[string]any{})
	if err != nil {
		t.Fatalf("executeHostedMCPTool returned error: %v", err)
	}
	if result == "" {
		t.Fatal("expected non-empty response")
	}
}

func TestExecuteHostedMCPToolRespectsExecutionProfileMaxResponseBodyBytes(t *testing.T) {
	SetLogger(bifrost.NewNoOpLogger())

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"payload":"1234567890"}`))
	}))
	defer upstream.Close()

	maxResponseBodyBytes := 8
	server := &BifrostHTTPServer{}
	tool := &configstoreTables.TableMCPHostedTool{
		Name:   "bounded_response",
		Method: http.MethodGet,
		URL:    upstream.URL + "/payload",
		ExecutionProfile: &configstoreTables.MCPHostedToolExecutionProfile{
			MaxResponseBodyBytes: &maxResponseBodyBytes,
		},
	}

	_, err := server.executeHostedMCPTool(context.Background(), tool, map[string]any{})
	if err == nil || !strings.Contains(err.Error(), "body size exceeds the given limit") {
		t.Fatalf("expected max response body size error, got %v", err)
	}
}

func TestExecuteHostedMCPToolWithMetadataCapturesExecutionDetails(t *testing.T) {
	SetLogger(bifrost.NewNoOpLogger())

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"summary":"ok"}`))
	}))
	defer upstream.Close()

	responseSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"summary": map[string]any{"type": "string"},
		},
	}
	server := &BifrostHTTPServer{}
	tool := &configstoreTables.TableMCPHostedTool{
		Name:           "inspect_metadata",
		Method:         http.MethodGet,
		URL:            upstream.URL + "/metadata",
		ResponseSchema: responseSchema,
	}

	result, err := server.executeHostedMCPToolWithMetadata(context.Background(), tool, map[string]any{})
	if err != nil {
		t.Fatalf("executeHostedMCPToolWithMetadata returned error: %v", err)
	}
	if result.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %+v", result)
	}
	if result.ContentType != "application/json" || result.ResponseBytes == 0 {
		t.Fatalf("expected execution metadata to be populated, got %+v", result)
	}
	if result.ResolvedURL != upstream.URL+"/metadata" {
		t.Fatalf("expected resolved URL, got %+v", result.ResolvedURL)
	}
	if result.ResponseSchema["type"] != "object" {
		t.Fatalf("expected response schema to be cloned, got %+v", result.ResponseSchema)
	}
}

func TestPreviewMCPHostedToolTruncatesLargeOutput(t *testing.T) {
	SetLogger(bifrost.NewNoOpLogger())

	largePayload := strings.Repeat("x", defaultHostedMCPToolPreviewSize+128)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte(largePayload))
	}))
	defer upstream.Close()

	store := &hostedToolPreviewStore{
		tool: &configstoreTables.TableMCPHostedTool{
			ToolID: "tool-1",
			Name:   "large_preview",
			Method: http.MethodGet,
			URL:    upstream.URL + "/preview",
		},
	}
	server := &BifrostHTTPServer{
		Config: &lib.Config{
			ConfigStore: store,
		},
	}

	result, err := server.PreviewMCPHostedTool(context.Background(), "tool-1", map[string]any{})
	if err != nil {
		t.Fatalf("PreviewMCPHostedTool returned error: %v", err)
	}
	if !result.Truncated || len(result.Output) != defaultHostedMCPToolPreviewSize {
		t.Fatalf("expected preview output to be truncated, got %+v", result)
	}
}

func TestExecuteHostedMCPToolRespectsExecutionProfileTimeout(t *testing.T) {
	SetLogger(bifrost.NewNoOpLogger())

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(1500 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer upstream.Close()

	timeoutSeconds := 1
	server := &BifrostHTTPServer{}
	tool := &configstoreTables.TableMCPHostedTool{
		Name:   "slow_tool",
		Method: http.MethodGet,
		URL:    upstream.URL + "/slow",
		ExecutionProfile: &configstoreTables.MCPHostedToolExecutionProfile{
			TimeoutSeconds: &timeoutSeconds,
		},
	}

	_, err := server.executeHostedMCPTool(context.Background(), tool, map[string]any{})
	if err == nil {
		t.Fatal("expected timeout error")
	}
}
