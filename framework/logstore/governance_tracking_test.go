package logstore

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/maximhq/bifrost/core/schemas"
)

type governanceTrackingTestLogger struct{}

func (governanceTrackingTestLogger) Debug(string, ...any)                   {}
func (governanceTrackingTestLogger) Info(string, ...any)                    {}
func (governanceTrackingTestLogger) Warn(string, ...any)                    {}
func (governanceTrackingTestLogger) Error(string, ...any)                   {}
func (governanceTrackingTestLogger) Fatal(string, ...any)                   {}
func (governanceTrackingTestLogger) SetLevel(schemas.LogLevel)              {}
func (governanceTrackingTestLogger) SetOutputType(schemas.LoggerOutputType) {}
func (governanceTrackingTestLogger) LogHTTPRequest(schemas.LogLevel, string) schemas.LogEventBuilder {
	return schemas.NoopLogEvent
}

func newGovernanceTrackingTestStore(t *testing.T) *RDBLogStore {
	t.Helper()

	store, err := newSqliteLogStore(context.Background(), &SQLiteConfig{
		Path: filepath.Join(t.TempDir(), "logstore.db"),
	}, governanceTrackingTestLogger{})
	if err != nil {
		t.Fatalf("newSqliteLogStore() error = %v", err)
	}
	return store
}

func TestGovernanceTrackingMigrationAddsColumns(t *testing.T) {
	store := newGovernanceTrackingTestStore(t)
	migrator := store.db.Migrator()

	for _, fieldName := range []string{"UserID", "TeamID", "TeamName", "CustomerID", "CustomerName"} {
		if !migrator.HasColumn(&Log{}, fieldName) {
			t.Fatalf("expected migration to add %s column", fieldName)
		}
	}
}

func TestSearchLogsFiltersByGovernanceTrackingFields(t *testing.T) {
	store := newGovernanceTrackingTestStore(t)
	now := time.Now().UTC()

	userID := "user-a"
	teamID := "team-a"
	teamName := "Alpha Team"
	customerID := "customer-a"
	customerName := "Acme"
	if err := store.CreateIfNotExists(context.Background(), &Log{
		ID:           "log-a",
		Timestamp:    now,
		Object:       "chat_completion",
		Provider:     "openai",
		Model:        "gpt-4o-mini",
		Status:       "success",
		UserID:       &userID,
		TeamID:       &teamID,
		TeamName:     &teamName,
		CustomerID:   &customerID,
		CustomerName: &customerName,
		CreatedAt:    now,
	}); err != nil {
		t.Fatalf("CreateIfNotExists(log-a) error = %v", err)
	}

	userIDB := "user-b"
	teamIDB := "team-b"
	teamNameB := "Beta Team"
	customerIDB := "customer-b"
	customerNameB := "Beta Corp"
	if err := store.CreateIfNotExists(context.Background(), &Log{
		ID:           "log-b",
		Timestamp:    now.Add(time.Second),
		Object:       "chat_completion",
		Provider:     "openai",
		Model:        "gpt-4o-mini",
		Status:       "success",
		UserID:       &userIDB,
		TeamID:       &teamIDB,
		TeamName:     &teamNameB,
		CustomerID:   &customerIDB,
		CustomerName: &customerNameB,
		CreatedAt:    now.Add(time.Second),
	}); err != nil {
		t.Fatalf("CreateIfNotExists(log-b) error = %v", err)
	}

	result, err := store.SearchLogs(context.Background(), SearchFilters{
		TeamIDs: []string{"team-a"},
	}, PaginationOptions{
		Limit:  20,
		Offset: 0,
		SortBy: "timestamp",
		Order:  "desc",
	})
	if err != nil {
		t.Fatalf("SearchLogs(team) error = %v", err)
	}
	if len(result.Logs) != 1 || result.Logs[0].ID != "log-a" {
		t.Fatalf("expected team filter to return log-a, got %+v", result.Logs)
	}

	result, err = store.SearchLogs(context.Background(), SearchFilters{
		CustomerIDs: []string{"customer-b"},
	}, PaginationOptions{
		Limit:  20,
		Offset: 0,
		SortBy: "timestamp",
		Order:  "desc",
	})
	if err != nil {
		t.Fatalf("SearchLogs(customer) error = %v", err)
	}
	if len(result.Logs) != 1 || result.Logs[0].ID != "log-b" {
		t.Fatalf("expected customer filter to return log-b, got %+v", result.Logs)
	}

	result, err = store.SearchLogs(context.Background(), SearchFilters{
		UserIDs: []string{"user-a"},
	}, PaginationOptions{
		Limit:  20,
		Offset: 0,
		SortBy: "timestamp",
		Order:  "desc",
	})
	if err != nil {
		t.Fatalf("SearchLogs(user) error = %v", err)
	}
	if len(result.Logs) != 1 || result.Logs[0].ID != "log-a" {
		t.Fatalf("expected user filter to return log-a, got %+v", result.Logs)
	}
}

func TestCanUseMatViewRejectsGovernanceFilters(t *testing.T) {
	if canUseMatView(SearchFilters{TeamIDs: []string{"team-a"}}) {
		t.Fatal("expected team filters to bypass materialized views")
	}
	if canUseMatView(SearchFilters{CustomerIDs: []string{"customer-a"}}) {
		t.Fatal("expected customer filters to bypass materialized views")
	}
	if canUseMatView(SearchFilters{UserIDs: []string{"user-a"}}) {
		t.Fatal("expected user filters to bypass materialized views")
	}
}
