package enterprise

import (
	"bufio"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/bytedance/sonic"
	"github.com/maximhq/bifrost/core/schemas"
	"github.com/valyala/fasthttp"
)

const (
	DefaultAuditFileName       = "audit-logs.jsonl"
	auditRetentionPruneWindow = time.Hour
)

type AuditCategory string

const (
	AuditCategoryAuthentication      AuditCategory = "authentication"
	AuditCategoryConfigurationChange AuditCategory = "configuration_change"
	AuditCategoryDataAccess          AuditCategory = "data_access"
	AuditCategoryExport              AuditCategory = "export"
	AuditCategoryCluster             AuditCategory = "cluster"
	AuditCategorySecurityEvent       AuditCategory = "security_event"
	AuditCategorySystem              AuditCategory = "system"
)

type AuditEvent struct {
	ID            string         `json:"id"`
	Timestamp     time.Time      `json:"timestamp"`
	Category      AuditCategory  `json:"category"`
	Action        string         `json:"action"`
	ResourceType  string         `json:"resource_type,omitempty"`
	ResourceID    string         `json:"resource_id,omitempty"`
	ActorID       string         `json:"actor_id,omitempty"`
	Method        string         `json:"method,omitempty"`
	Path          string         `json:"path,omitempty"`
	StatusCode    int            `json:"status_code,omitempty"`
	RemoteAddr    string         `json:"remote_addr,omitempty"`
	RequestID     string         `json:"request_id,omitempty"`
	Message       string         `json:"message,omitempty"`
	Metadata      map[string]any `json:"metadata,omitempty"`
	PreviousHash  string         `json:"previous_hash,omitempty"`
	IntegrityHash string         `json:"integrity_hash,omitempty"`
}

type AuditSearchFilters struct {
	Category     AuditCategory
	Action       string
	ResourceType string
	ActorID      string
	StartTime    *time.Time
	EndTime      *time.Time
	Limit        int
	Offset       int
}

type AuditSearchResult struct {
	Events []AuditEvent `json:"events"`
	Total  int          `json:"total"`
}

type AuditService struct {
	path          string
	hmacKey       []byte
	logger        schemas.Logger
	retentionDays int
	mu            sync.Mutex
	lastHash      string
	lastPruneAt   time.Time
}

func NewAuditService(baseDir string, cfg *AuditLogsConfig, logger schemas.Logger) (*AuditService, error) {
	if cfg == nil || cfg.Disabled {
		return nil, nil
	}

	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create audit base directory: %w", err)
	}

	service := &AuditService{
		path:          filepath.Join(baseDir, DefaultAuditFileName),
		logger:        logger,
		retentionDays: max(cfg.RetentionDays, 0),
	}
	if cfg.HMACKey != nil {
		service.hmacKey = []byte(cfg.HMACKey.GetValue())
	}
	if err := service.pruneExpired(time.Now().UTC(), true); err != nil {
		return nil, err
	}
	if err := service.loadLastHash(); err != nil {
		return nil, err
	}
	return service, nil
}

func (s *AuditService) Path() string {
	if s == nil {
		return ""
	}
	return s.path
}

func (s *AuditService) Append(event *AuditEvent) error {
	if s == nil || event == nil {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if event.ID == "" {
		event.ID = fmt.Sprintf("audit_%d", time.Now().UnixNano())
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}
	event.PreviousHash = s.lastHash

	hash, err := s.computeHash(*event)
	if err != nil {
		return err
	}
	event.IntegrityHash = hash

	payload, err := sonic.Marshal(event)
	if err != nil {
		return err
	}

	file, err := os.OpenFile(s.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()

	if _, err := file.Write(append(payload, '\n')); err != nil {
		return err
	}

	s.lastHash = event.IntegrityHash
	if err := s.pruneExpiredLocked(time.Now().UTC(), false); err != nil {
		return err
	}
	return nil
}

func (s *AuditService) Search(filters AuditSearchFilters) (*AuditSearchResult, error) {
	if s == nil {
		return &AuditSearchResult{Events: []AuditEvent{}, Total: 0}, nil
	}

	file, err := os.Open(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return &AuditSearchResult{Events: []AuditEvent{}, Total: 0}, nil
		}
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	events := make([]AuditEvent, 0)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var event AuditEvent
		if err := sonic.Unmarshal([]byte(line), &event); err != nil {
			continue
		}
		if !matchesAuditFilters(event, filters) {
			continue
		}
		events = append(events, event)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	slices.Reverse(events)

	total := len(events)
	offset := max(filters.Offset, 0)
	if offset >= total {
		return &AuditSearchResult{Events: []AuditEvent{}, Total: total}, nil
	}

	limit := filters.Limit
	if limit <= 0 {
		limit = 100
	}
	end := min(offset+limit, total)

	return &AuditSearchResult{
		Events: events[offset:end],
		Total:  total,
	}, nil
}

func (s *AuditService) Middleware() schemas.BifrostHTTPMiddleware {
	return func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
		return func(ctx *fasthttp.RequestCtx) {
			next(ctx)

			path := string(ctx.Path())
			method := string(ctx.Method())
			statusCode := ctx.Response.StatusCode()
			if !shouldAuditRequest(path, method, statusCode) {
				return
			}

			requestID := string(ctx.Response.Header.Peek("X-Request-Id"))
			if requestID == "" {
				requestID = string(ctx.Request.Header.Peek("X-Request-Id"))
			}
			actorID := resolveAuditActorID(ctx)

			event := &AuditEvent{
				Timestamp:  time.Now().UTC(),
				Category:   classifyAuditCategory(path, method, statusCode),
				Action:     strings.ToLower(method),
				Method:     method,
				Path:       path,
				StatusCode: statusCode,
				RemoteAddr: ctx.RemoteIP().String(),
				RequestID:  requestID,
				ActorID:    actorID,
				Metadata: map[string]any{
					"query": string(ctx.URI().QueryString()),
				},
			}
			if err := s.Append(event); err != nil && s.logger != nil {
				s.logger.Warn("failed to append audit log: %v", err)
			}
		}
	}
}

func (s *AuditService) loadLastHash() error {
	file, err := os.Open(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var lastLine string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			lastLine = line
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if lastLine == "" {
		return nil
	}

	var event AuditEvent
	if err := sonic.Unmarshal([]byte(lastLine), &event); err != nil {
		return nil
	}
	s.lastHash = event.IntegrityHash
	return nil
}

func (s *AuditService) pruneExpired(now time.Time, force bool) error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.pruneExpiredLocked(now, force)
}

func (s *AuditService) pruneExpiredLocked(now time.Time, force bool) error {
	if s == nil || s.retentionDays <= 0 {
		return nil
	}

	pruneAt := now.UTC()
	if !force && !s.lastPruneAt.IsZero() && pruneAt.Sub(s.lastPruneAt) < auditRetentionPruneWindow {
		return nil
	}

	file, err := os.Open(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			s.lastPruneAt = pruneAt
			return nil
		}
		return err
	}
	defer file.Close()

	cutoff := pruneAt.AddDate(0, 0, -s.retentionDays)
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	retainedLines := make([]string, 0)
	lastRetainedLine := ""
	removedCount := 0

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var event AuditEvent
		if err := sonic.Unmarshal([]byte(line), &event); err != nil {
			retainedLines = append(retainedLines, line)
			lastRetainedLine = line
			continue
		}
		if event.Timestamp.Before(cutoff) {
			removedCount++
			continue
		}
		retainedLines = append(retainedLines, line)
		lastRetainedLine = line
	}
	if err := scanner.Err(); err != nil {
		return err
	}

	s.lastPruneAt = pruneAt
	if removedCount == 0 {
		return nil
	}

	tmpPath := s.path + ".tmp"
	var contents []byte
	if len(retainedLines) > 0 {
		contents = []byte(strings.Join(retainedLines, "\n") + "\n")
	}
	if err := os.WriteFile(tmpPath, contents, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, s.path); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}

	if lastRetainedLine == "" {
		s.lastHash = ""
	} else {
		var event AuditEvent
		if err := sonic.Unmarshal([]byte(lastRetainedLine), &event); err != nil {
			s.lastHash = ""
		} else {
			s.lastHash = event.IntegrityHash
		}
	}
	if s.logger != nil {
		s.logger.Info("pruned %d expired audit log entries", removedCount)
	}
	return nil
}

func (s *AuditService) computeHash(event AuditEvent) (string, error) {
	clone := event
	clone.IntegrityHash = ""
	payload, err := sonic.Marshal(clone)
	if err != nil {
		return "", err
	}

	if len(s.hmacKey) == 0 {
		sum := sha256.Sum256(payload)
		return hex.EncodeToString(sum[:]), nil
	}

	mac := hmac.New(sha256.New, s.hmacKey)
	if _, err := mac.Write(payload); err != nil {
		return "", err
	}
	return hex.EncodeToString(mac.Sum(nil)), nil
}

func matchesAuditFilters(event AuditEvent, filters AuditSearchFilters) bool {
	if filters.Category != "" && event.Category != filters.Category {
		return false
	}
	if filters.Action != "" && !strings.EqualFold(event.Action, filters.Action) {
		return false
	}
	if filters.ResourceType != "" && !strings.EqualFold(event.ResourceType, filters.ResourceType) {
		return false
	}
	if filters.ActorID != "" && event.ActorID != filters.ActorID {
		return false
	}
	if filters.StartTime != nil && event.Timestamp.Before(*filters.StartTime) {
		return false
	}
	if filters.EndTime != nil && event.Timestamp.After(*filters.EndTime) {
		return false
	}
	return true
}

func resolveAuditActorID(ctx *fasthttp.RequestCtx) string {
	if ctx == nil {
		return ""
	}
	if actorID, _ := ctx.UserValue(schemas.BifrostContextKeyUserID).(string); strings.TrimSpace(actorID) != "" {
		return strings.TrimSpace(actorID)
	}
	sessionToken, _ := ctx.UserValue(schemas.BifrostContextKeySessionToken).(string)
	sessionToken = strings.TrimSpace(sessionToken)
	if sessionToken == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(sessionToken))
	return "session:" + hex.EncodeToString(sum[:8])
}

func shouldAuditRequest(path, method string, statusCode int) bool {
	if !strings.HasPrefix(path, "/api/") {
		return false
	}
	if path == "/api/session/login" || path == "/api/session/logout" {
		return true
	}
	if path == "/api/logs" || strings.HasPrefix(path, "/api/logs/") {
		return true
	}
	if path == "/api/mcp-logs" || strings.HasPrefix(path, "/api/mcp-logs/") {
		return true
	}
	if strings.HasPrefix(path, "/api/config") ||
		strings.HasPrefix(path, "/api/plugins") ||
		strings.HasPrefix(path, "/api/providers") ||
		strings.HasPrefix(path, "/api/audit-logs") ||
		strings.HasPrefix(path, "/api/log-exports") ||
		strings.HasPrefix(path, "/api/cluster") {
		return true
	}
	if method != fasthttp.MethodGet && method != fasthttp.MethodHead {
		return true
	}
	return statusCode >= fasthttp.StatusBadRequest
}

func classifyAuditCategory(path, method string, statusCode int) AuditCategory {
	switch {
	case strings.HasPrefix(path, "/api/session"):
		if statusCode >= fasthttp.StatusBadRequest {
			return AuditCategorySecurityEvent
		}
		return AuditCategoryAuthentication
	case strings.HasPrefix(path, "/api/log-exports"):
		return AuditCategoryExport
	case strings.HasPrefix(path, "/api/audit-logs"),
		strings.HasPrefix(path, "/api/logs"),
		strings.HasPrefix(path, "/api/mcp-logs"):
		return AuditCategoryDataAccess
	case strings.HasPrefix(path, "/api/cluster"):
		return AuditCategoryCluster
	case method != fasthttp.MethodGet && method != fasthttp.MethodHead:
		return AuditCategoryConfigurationChange
	case statusCode == fasthttp.StatusUnauthorized || statusCode == fasthttp.StatusForbidden:
		return AuditCategorySecurityEvent
	default:
		return AuditCategorySystem
	}
}
