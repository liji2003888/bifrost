package enterprise

import (
	"os"
	"strings"
	"testing"
	"time"

	bifrost "github.com/maximhq/bifrost/core"
	"github.com/maximhq/bifrost/core/schemas"
	"github.com/valyala/fasthttp"
)

func TestAuditServiceAppendAndSearch(t *testing.T) {
	service, err := NewAuditService(t.TempDir(), &AuditLogsConfig{}, bifrost.NewNoOpLogger())
	if err != nil {
		t.Fatalf("NewAuditService() error = %v", err)
	}

	first := &AuditEvent{
		Timestamp:    time.Now().UTC().Add(-time.Minute),
		Category:     AuditCategoryConfigurationChange,
		Action:       "update",
		ResourceType: "config",
	}
	second := &AuditEvent{
		Timestamp:    time.Now().UTC(),
		Category:     AuditCategoryExport,
		Action:       "submit",
		ResourceType: "logs",
	}

	if err := service.Append(first); err != nil {
		t.Fatalf("Append(first) error = %v", err)
	}
	if err := service.Append(second); err != nil {
		t.Fatalf("Append(second) error = %v", err)
	}

	if second.PreviousHash == "" || second.PreviousHash != first.IntegrityHash {
		t.Fatalf("expected hash chain to be linked, got previous=%q first=%q", second.PreviousHash, first.IntegrityHash)
	}

	result, err := service.Search(AuditSearchFilters{
		Category: AuditCategoryExport,
		Limit:    10,
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if result.Total != 1 {
		t.Fatalf("expected 1 event, got %d", result.Total)
	}
	if len(result.Events) != 1 || result.Events[0].Category != AuditCategoryExport {
		t.Fatalf("unexpected search result: %+v", result.Events)
	}
}

func TestResolveAuditActorIDPrefersUserID(t *testing.T) {
	ctx := &fasthttp.RequestCtx{}
	ctx.SetUserValue(schemas.BifrostContextKeyUserID, "admin")
	ctx.SetUserValue(schemas.BifrostContextKeySessionToken, "secret-session")

	if actorID := resolveAuditActorID(ctx); actorID != "admin" {
		t.Fatalf("expected actor id to prefer explicit user id, got %q", actorID)
	}
}

func TestAuditServicePrunesExpiredEntriesOnStartup(t *testing.T) {
	dir := t.TempDir()

	writer, err := NewAuditService(dir, &AuditLogsConfig{}, bifrost.NewNoOpLogger())
	if err != nil {
		t.Fatalf("NewAuditService(writer) error = %v", err)
	}

	expired := &AuditEvent{
		ID:         "expired",
		Timestamp:  time.Now().UTC().Add(-72 * time.Hour),
		Category:   AuditCategorySystem,
		Action:     "expired",
		Message:    "too old",
		ResourceID: "old",
	}
	retained := &AuditEvent{
		ID:         "retained",
		Timestamp:  time.Now().UTC().Add(-2 * time.Hour),
		Category:   AuditCategorySystem,
		Action:     "retained",
		Message:    "keep me",
		ResourceID: "new",
	}

	if err := writer.Append(expired); err != nil {
		t.Fatalf("Append(expired) error = %v", err)
	}
	if err := writer.Append(retained); err != nil {
		t.Fatalf("Append(retained) error = %v", err)
	}

	service, err := NewAuditService(dir, &AuditLogsConfig{RetentionDays: 1}, bifrost.NewNoOpLogger())
	if err != nil {
		t.Fatalf("NewAuditService(retention) error = %v", err)
	}

	result, err := service.Search(AuditSearchFilters{Limit: 10})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if result.Total != 1 {
		t.Fatalf("expected 1 retained event after prune, got %d", result.Total)
	}
	if len(result.Events) != 1 || result.Events[0].ID != retained.ID {
		t.Fatalf("unexpected retained events after prune: %+v", result.Events)
	}

	payload, err := os.ReadFile(service.Path())
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if strings.Contains(string(payload), expired.ID) {
		t.Fatalf("expected expired audit entry to be removed, file contents: %s", string(payload))
	}
	if !strings.Contains(string(payload), retained.ID) {
		t.Fatalf("expected retained audit entry to remain, file contents: %s", string(payload))
	}
}

func TestAuditServicePrunesExpiredEntriesDuringAppend(t *testing.T) {
	service, err := NewAuditService(t.TempDir(), &AuditLogsConfig{RetentionDays: 1}, bifrost.NewNoOpLogger())
	if err != nil {
		t.Fatalf("NewAuditService() error = %v", err)
	}

	expired := &AuditEvent{
		ID:        "expired-on-append",
		Timestamp: time.Now().UTC().Add(-48 * time.Hour),
		Category:  AuditCategorySystem,
		Action:    "expired",
	}
	if err := service.Append(expired); err != nil {
		t.Fatalf("Append(expired) error = %v", err)
	}

	service.mu.Lock()
	service.lastPruneAt = time.Now().UTC().Add(-2 * auditRetentionPruneWindow)
	service.mu.Unlock()

	current := &AuditEvent{
		ID:        "current",
		Timestamp: time.Now().UTC(),
		Category:  AuditCategorySystem,
		Action:    "current",
	}
	if err := service.Append(current); err != nil {
		t.Fatalf("Append(current) error = %v", err)
	}

	result, err := service.Search(AuditSearchFilters{Limit: 10})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if result.Total != 1 {
		t.Fatalf("expected only current event after append-triggered prune, got %d", result.Total)
	}
	if len(result.Events) != 1 || result.Events[0].ID != current.ID {
		t.Fatalf("unexpected events after append-triggered prune: %+v", result.Events)
	}
}
