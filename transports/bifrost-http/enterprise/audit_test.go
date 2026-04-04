package enterprise

import (
	"testing"
	"time"

	bifrost "github.com/maximhq/bifrost/core"
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
