package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	neturl "net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework/configstore"
	configstoreTables "github.com/maximhq/bifrost/framework/configstore/tables"
	"github.com/tidwall/gjson"
	"github.com/valyala/fasthttp"
)

const (
	defaultHostedMCPToolTimeout     = 30 * time.Second
	defaultHostedMCPToolPreviewSize = 64 * 1024
)

var hostedMCPToolNameSanitizer = regexp.MustCompile(`[^a-zA-Z0-9_]+`)

func normalizeHostedMCPToolName(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	name = strings.ReplaceAll(name, "-", "_")
	name = strings.ReplaceAll(name, " ", "_")
	name = hostedMCPToolNameSanitizer.ReplaceAllString(name, "_")
	name = strings.Trim(name, "_")
	if name == "" {
		return "hosted_tool"
	}
	if len(name) > 0 && name[0] >= '0' && name[0] <= '9' {
		name = "tool_" + name
	}
	return name
}

func (s *BifrostHTTPServer) getHostedMCPToolHTTPClient() *fasthttp.Client {
	if s.hostedMCPToolHTTPClient == nil {
		s.hostedMCPToolHTTPClient = &fasthttp.Client{
			MaxConnsPerHost:     512,
			MaxIdleConnDuration: 30 * time.Second,
			ReadTimeout:         defaultHostedMCPToolTimeout,
			WriteTimeout:        defaultHostedMCPToolTimeout,
		}
	}
	return s.hostedMCPToolHTTPClient
}

func stringifyHostedToolValue(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	case json.Number:
		return v.String()
	case float64, float32, int, int64, int32, uint, uint64, uint32, bool:
		return fmt.Sprintf("%v", v)
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(data)
	}
}

func cloneHostedToolSchemaMap(value map[string]any) map[string]any {
	if len(value) == 0 {
		return nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		cloned := make(map[string]any, len(value))
		for key, item := range value {
			cloned[key] = item
		}
		return cloned
	}
	var cloned map[string]any
	if err := json.Unmarshal(data, &cloned); err != nil {
		cloned = make(map[string]any, len(value))
		for key, item := range value {
			cloned[key] = item
		}
	}
	return cloned
}

func truncateHostedToolPreviewOutput(output string, limit int) (string, bool) {
	if limit <= 0 || len(output) <= limit {
		return output, false
	}
	return output[:limit], true
}

func shouldIncludeHostedToolResponseRaw(template string) bool {
	return strings.Contains(template, "{{response.raw")
}

func lookupHostedToolValue(path string, root any) string {
	path = strings.TrimSpace(path)
	if path == "" || root == nil {
		return ""
	}
	parts := strings.Split(path, ".")
	var current any = root
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return ""
		}
		switch typed := current.(type) {
		case map[string]any:
			next, ok := typed[part]
			if !ok {
				return ""
			}
			current = next
		case []any:
			index, err := strconv.Atoi(part)
			if err != nil || index < 0 || index >= len(typed) {
				return ""
			}
			current = typed[index]
		default:
			return ""
		}
	}
	return stringifyHostedToolValue(current)
}

func buildHostedToolResponseTemplateContext(rawBody []byte, includeRaw bool) any {
	result := map[string]any{}
	if includeRaw {
		result["raw"] = string(rawBody)
	}
	if !gjson.ValidBytes(rawBody) {
		return result
	}
	var parsed any
	if err := json.Unmarshal(rawBody, &parsed); err != nil {
		return result
	}
	switch typed := parsed.(type) {
	case map[string]any:
		for key, value := range typed {
			result[key] = value
		}
	case []any:
		result["items"] = typed
	default:
		result["value"] = typed
	}
	return result
}

func extractHostedToolJSONPath(rawBody []byte, path string) string {
	path = strings.TrimSpace(path)
	if path == "" || !gjson.ValidBytes(rawBody) {
		return ""
	}
	value := gjson.GetBytes(rawBody, path)
	if !value.Exists() {
		return ""
	}
	if value.IsObject() || value.IsArray() {
		var pretty bytes.Buffer
		if err := json.Indent(&pretty, []byte(value.Raw), "", "  "); err == nil {
			return pretty.String()
		}
		return value.Raw
	}
	return stringifyHostedToolValue(value.Value())
}

func resolveHostedToolTemplate(value string, args map[string]any, requestHeaders map[string]string, responseData any) string {
	if !strings.Contains(value, "{{") {
		return value
	}

	result := value
	for {
		start := strings.Index(result, "{{")
		if start == -1 {
			break
		}
		end := strings.Index(result[start:], "}}")
		if end == -1 {
			break
		}
		end += start + 2

		template := strings.TrimSpace(result[start+2 : end-2])
		replacement := ""

		switch {
		case strings.HasPrefix(template, "req.header."):
			headerKey := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(template, "req.header.")))
			if requestHeaders != nil {
				replacement = requestHeaders[headerKey]
			}
		case strings.HasPrefix(template, "env."):
			replacement, _ = os.LookupEnv(strings.TrimSpace(strings.TrimPrefix(template, "env.")))
		case strings.HasPrefix(template, "args."):
			replacement = lookupHostedToolValue(strings.TrimPrefix(template, "args."), args)
		case strings.HasPrefix(template, "req.body."):
			replacement = lookupHostedToolValue(strings.TrimPrefix(template, "req.body."), args)
		case strings.HasPrefix(template, "req.query."):
			replacement = lookupHostedToolValue(strings.TrimPrefix(template, "req.query."), args)
		case strings.HasPrefix(template, "response."):
			replacement = lookupHostedToolValue(strings.TrimPrefix(template, "response."), responseData)
		}

		result = result[:start] + replacement + result[end:]
	}
	return result
}

func resolveHostedToolURL(baseURL string, args map[string]any, requestHeaders map[string]string, queryParams map[string]string) (string, error) {
	resolvedURL := resolveHostedToolTemplate(baseURL, args, requestHeaders, nil)
	if len(queryParams) == 0 {
		return resolvedURL, nil
	}
	parsed, err := neturl.Parse(resolvedURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse hosted tool URL: %w", err)
	}
	values := parsed.Query()
	for key, value := range queryParams {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		resolved := resolveHostedToolTemplate(value, args, requestHeaders, nil)
		if strings.TrimSpace(resolved) == "" {
			continue
		}
		values.Set(key, resolved)
	}
	parsed.RawQuery = values.Encode()
	return parsed.String(), nil
}

func formatHostedToolResponse(rawBody []byte, tool *configstoreTables.TableMCPHostedTool, args map[string]any, requestHeaders map[string]string) string {
	if tool == nil {
		return string(rawBody)
	}
	if tool.ResponseTemplate != nil && strings.TrimSpace(*tool.ResponseTemplate) != "" {
		responseData := buildHostedToolResponseTemplateContext(rawBody, shouldIncludeHostedToolResponseRaw(*tool.ResponseTemplate))
		resolved := resolveHostedToolTemplate(*tool.ResponseTemplate, args, requestHeaders, responseData)
		if strings.TrimSpace(resolved) != "" {
			return resolved
		}
	}
	if tool.ResponseJSONPath != nil && strings.TrimSpace(*tool.ResponseJSONPath) != "" {
		if extracted := extractHostedToolJSONPath(rawBody, *tool.ResponseJSONPath); strings.TrimSpace(extracted) != "" {
			return extracted
		}
	}
	return string(rawBody)
}

func applyHostedToolAuthProfile(req *fasthttp.Request, profile *configstoreTables.MCPHostedToolAuthProfile, requestHeaders map[string]string) {
	if req == nil || profile == nil || requestHeaders == nil {
		return
	}
	switch profile.Mode {
	case configstoreTables.MCPHostedToolAuthModeBearerPassthrough:
		if token := strings.TrimSpace(requestHeaders["authorization"]); token != "" {
			req.Header.Set("Authorization", token)
		}
	case configstoreTables.MCPHostedToolAuthModeHeaderPassthrough:
		for targetHeader, sourceHeader := range profile.HeaderMappings {
			targetHeader = strings.TrimSpace(targetHeader)
			sourceHeader = strings.TrimSpace(strings.ToLower(sourceHeader))
			if targetHeader == "" || sourceHeader == "" {
				continue
			}
			if value := strings.TrimSpace(requestHeaders[sourceHeader]); value != "" {
				req.Header.Set(targetHeader, value)
			}
		}
	}
}

func hostedToolClientForExecution(base *fasthttp.Client, profile *configstoreTables.MCPHostedToolExecutionProfile) *fasthttp.Client {
	if base == nil {
		return nil
	}
	if profile == nil {
		return base
	}
	client := *base
	if profile.MaxResponseBodyBytes != nil && *profile.MaxResponseBodyBytes > 0 {
		client.MaxResponseBodySize = *profile.MaxResponseBodyBytes
	}
	if profile.TimeoutSeconds != nil && *profile.TimeoutSeconds > 0 {
		timeout := time.Duration(*profile.TimeoutSeconds) * time.Second
		client.ReadTimeout = timeout
		client.WriteTimeout = timeout
	}
	return &client
}

func (s *BifrostHTTPServer) executeHostedMCPToolWithMetadata(ctx context.Context, tool *configstoreTables.TableMCPHostedTool, args map[string]any) (*configstoreTables.MCPHostedToolExecutionResult, error) {
	if tool == nil {
		return nil, fmt.Errorf("hosted MCP tool config is required")
	}
	startedAt := time.Now()

	var requestHeaders map[string]string
	if bfCtx, ok := ctx.(*schemas.BifrostContext); ok && bfCtx != nil {
		if headers, ok := bfCtx.Value(schemas.BifrostContextKeyRequestHeaders).(map[string]string); ok {
			requestHeaders = headers
		}
	}

	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)

	req.Header.SetMethod(strings.ToUpper(strings.TrimSpace(tool.Method)))
	resolvedURL, err := resolveHostedToolURL(tool.URL, args, requestHeaders, tool.QueryParams)
	if err != nil {
		return nil, err
	}
	req.SetRequestURI(resolvedURL)

	for key, value := range tool.Headers {
		resolved := resolveHostedToolTemplate(value, args, requestHeaders, nil)
		if strings.TrimSpace(resolved) == "" {
			continue
		}
		req.Header.Set(key, resolved)
	}
	applyHostedToolAuthProfile(req, tool.AuthProfile, requestHeaders)

	if tool.BodyTemplate != nil && strings.TrimSpace(*tool.BodyTemplate) != "" {
		body := resolveHostedToolTemplate(*tool.BodyTemplate, args, requestHeaders, nil)
		req.SetBodyString(body)
		if len(req.Header.ContentType()) == 0 {
			req.Header.SetContentType("application/json")
		}
	}

	timeout := defaultHostedMCPToolTimeout
	if tool.ExecutionProfile != nil && tool.ExecutionProfile.TimeoutSeconds != nil && *tool.ExecutionProfile.TimeoutSeconds > 0 {
		timeout = time.Duration(*tool.ExecutionProfile.TimeoutSeconds) * time.Second
	}
	if deadline, ok := ctx.Deadline(); ok {
		if remaining := time.Until(deadline); remaining > 0 {
			if remaining < timeout {
				timeout = remaining
			}
		}
	}

	client := hostedToolClientForExecution(s.getHostedMCPToolHTTPClient(), tool.ExecutionProfile)
	if err := client.DoTimeout(req, resp, timeout); err != nil {
		return nil, fmt.Errorf("failed to call hosted API endpoint: %w", err)
	}

	statusCode := resp.StatusCode()
	body := string(resp.Body())
	if statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices {
		if body == "" {
			body = http.StatusText(statusCode)
		}
		return nil, fmt.Errorf("hosted API endpoint returned status %d: %s", statusCode, body)
	}

	return &configstoreTables.MCPHostedToolExecutionResult{
		Output:         formatHostedToolResponse(resp.Body(), tool, args, requestHeaders),
		StatusCode:     statusCode,
		LatencyMS:      time.Since(startedAt).Milliseconds(),
		ResponseBytes:  len(resp.Body()),
		ContentType:    string(resp.Header.ContentType()),
		ResolvedURL:    resolvedURL,
		ResponseSchema: cloneHostedToolSchemaMap(tool.ResponseSchema),
	}, nil
}

func (s *BifrostHTTPServer) executeHostedMCPTool(ctx context.Context, tool *configstoreTables.TableMCPHostedTool, args map[string]any) (string, error) {
	result, err := s.executeHostedMCPToolWithMetadata(ctx, tool, args)
	if err != nil {
		return "", err
	}
	return result.Output, nil
}

func (s *BifrostHTTPServer) PreviewMCPHostedTool(ctx context.Context, id string, args map[string]any) (*configstoreTables.MCPHostedToolExecutionResult, error) {
	if s == nil || s.Config == nil || s.Config.ConfigStore == nil {
		return nil, fmt.Errorf("config store not found")
	}
	tool, err := s.Config.ConfigStore.GetMCPHostedToolByID(ctx, id)
	if err != nil {
		return nil, err
	}
	result, err := s.executeHostedMCPToolWithMetadata(ctx, tool, args)
	if err != nil {
		return nil, err
	}
	result.Output, result.Truncated = truncateHostedToolPreviewOutput(result.Output, defaultHostedMCPToolPreviewSize)
	return result, nil
}

func (s *BifrostHTTPServer) registerHostedMCPTool(tool *configstoreTables.TableMCPHostedTool) error {
	if s == nil || s.Client == nil {
		return fmt.Errorf("bifrost client not initialized")
	}
	if tool == nil {
		return fmt.Errorf("hosted tool config is required")
	}
	tool.Name = normalizeHostedMCPToolName(tool.Name)
	if tool.ToolSchema.Function != nil {
		tool.ToolSchema.Function.Name = tool.Name
	}

	_ = s.Client.RemoveMCPTool(tool.Name)

	description := ""
	if tool.Description != nil {
		description = *tool.Description
	}

	return s.Client.RegisterMCPToolWithContext(tool.Name, description, func(ctx context.Context, args any) (string, error) {
		argMap, _ := args.(map[string]any)
		if argMap == nil {
			argMap = map[string]any{}
		}
		return s.executeHostedMCPTool(ctx, tool, argMap)
	}, tool.ToolSchema)
}

func (s *BifrostHTTPServer) unregisterHostedMCPTool(name string) error {
	if s == nil || s.Client == nil {
		return fmt.Errorf("bifrost client not initialized")
	}
	if strings.TrimSpace(name) == "" {
		return nil
	}
	if err := s.Client.RemoveMCPTool(name); err != nil && !strings.Contains(err.Error(), "is not registered") {
		return err
	}
	return nil
}

func (s *BifrostHTTPServer) syncHostedMCPToolsFromStore(ctx context.Context) error {
	if s == nil || s.Config == nil || s.Config.ConfigStore == nil {
		return nil
	}
	tools, err := s.Config.ConfigStore.GetMCPHostedTools(ctx)
	if err != nil {
		return err
	}
	for i := range tools {
		if err := s.registerHostedMCPTool(&tools[i]); err != nil {
			return err
		}
	}
	return nil
}

func (s *BifrostHTTPServer) AddMCPHostedTool(ctx context.Context, tool *configstoreTables.TableMCPHostedTool) error {
	if s == nil || s.Config == nil || s.Config.ConfigStore == nil {
		return fmt.Errorf("config store not found")
	}
	if tool == nil {
		return fmt.Errorf("hosted tool config is required")
	}
	tool.Name = normalizeHostedMCPToolName(tool.Name)
	if tool.ToolSchema.Function != nil {
		tool.ToolSchema.Function.Name = tool.Name
	}
	if err := s.registerHostedMCPTool(tool); err != nil {
		return err
	}
	if err := s.Config.ConfigStore.CreateMCPHostedTool(ctx, tool); err != nil {
		_ = s.unregisterHostedMCPTool(tool.Name)
		return err
	}
	return nil
}

func (s *BifrostHTTPServer) UpdateMCPHostedTool(ctx context.Context, id string, tool *configstoreTables.TableMCPHostedTool) error {
	if s == nil || s.Config == nil || s.Config.ConfigStore == nil {
		return fmt.Errorf("config store not found")
	}
	if tool == nil {
		return fmt.Errorf("hosted tool config is required")
	}

	existing, err := s.Config.ConfigStore.GetMCPHostedToolByID(ctx, id)
	if err != nil {
		return err
	}

	tool.ID = existing.ID
	tool.ToolID = existing.ToolID
	tool.CreatedAt = existing.CreatedAt
	tool.Name = normalizeHostedMCPToolName(tool.Name)
	if tool.ToolSchema.Function != nil {
		tool.ToolSchema.Function.Name = tool.Name
	}

	if existing.Name != tool.Name {
		if err := s.unregisterHostedMCPTool(existing.Name); err != nil {
			return err
		}
	}
	if err := s.registerHostedMCPTool(tool); err != nil {
		return err
	}
	if err := s.Config.ConfigStore.UpdateMCPHostedTool(ctx, tool); err != nil {
		if existing.Name != tool.Name {
			_ = s.registerHostedMCPTool(existing)
		}
		return err
	}
	return nil
}

func (s *BifrostHTTPServer) RemoveMCPHostedTool(ctx context.Context, id string) error {
	if s == nil || s.Config == nil || s.Config.ConfigStore == nil {
		return fmt.Errorf("config store not found")
	}
	existing, err := s.Config.ConfigStore.GetMCPHostedToolByID(ctx, id)
	if err != nil {
		if errors.Is(err, configstore.ErrNotFound) {
			return nil
		}
		return err
	}
	if err := s.Config.ConfigStore.DeleteMCPHostedTool(ctx, id); err != nil {
		if errors.Is(err, configstore.ErrNotFound) {
			return s.unregisterHostedMCPTool(existing.Name)
		}
		return err
	}
	return s.unregisterHostedMCPTool(existing.Name)
}

func (s *BifrostHTTPServer) ApplyClusterMCPHostedToolConfig(ctx context.Context, id string, tool *configstoreTables.TableMCPHostedTool, deleteTool bool) error {
	if s == nil || s.Config == nil || s.Config.ConfigStore == nil {
		return fmt.Errorf("config store not found")
	}
	if deleteTool {
		if tool != nil && tool.Name != "" {
			if err := s.unregisterHostedMCPTool(tool.Name); err != nil {
				return err
			}
		}
		if err := s.Config.ConfigStore.DeleteMCPHostedTool(ctx, id); err != nil && !errors.Is(err, configstore.ErrNotFound) {
			return err
		}
		return nil
	}
	if tool == nil {
		return fmt.Errorf("hosted tool config is required")
	}
	tool.Name = normalizeHostedMCPToolName(tool.Name)
	if tool.ToolSchema.Function != nil {
		tool.ToolSchema.Function.Name = tool.Name
	}

	existing, err := s.Config.ConfigStore.GetMCPHostedToolByID(ctx, id)
	if err != nil && !errors.Is(err, configstore.ErrNotFound) {
		return err
	}
	if err == nil && existing != nil {
		if existing.Name != tool.Name {
			if removeErr := s.unregisterHostedMCPTool(existing.Name); removeErr != nil {
				return removeErr
			}
		}
		if err := s.registerHostedMCPTool(tool); err != nil {
			return err
		}
		return s.Config.ConfigStore.UpdateMCPHostedTool(ctx, tool)
	}

	return s.AddMCPHostedTool(ctx, tool)
}
