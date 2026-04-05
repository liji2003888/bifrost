package enterprise

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	bifrost "github.com/maximhq/bifrost/core"
	"github.com/maximhq/bifrost/framework/logstore"
)

type fakeLogSearchProvider struct {
	logs    []logstore.Log
	mcpLogs []logstore.MCPToolLog
}

func (f *fakeLogSearchProvider) Search(_ context.Context, _ *logstore.SearchFilters, pagination *logstore.PaginationOptions) (*logstore.SearchResult, error) {
	start := pagination.Offset
	if start >= len(f.logs) {
		return &logstore.SearchResult{Logs: []logstore.Log{}}, nil
	}
	end := start + pagination.Limit
	if end > len(f.logs) {
		end = len(f.logs)
	}
	return &logstore.SearchResult{
		Logs: f.logs[start:end],
	}, nil
}

func (f *fakeLogSearchProvider) SearchMCPToolLogs(_ context.Context, _ *logstore.MCPToolLogSearchFilters, pagination *logstore.PaginationOptions) (*logstore.MCPToolLogSearchResult, error) {
	start := pagination.Offset
	if start >= len(f.mcpLogs) {
		return &logstore.MCPToolLogSearchResult{Logs: []logstore.MCPToolLog{}}, nil
	}
	end := start + pagination.Limit
	if end > len(f.mcpLogs) {
		end = len(f.mcpLogs)
	}
	return &logstore.MCPToolLogSearchResult{
		Logs: f.mcpLogs[start:end],
	}, nil
}

func TestLogExportServiceWritesJSONL(t *testing.T) {
	service, err := NewLogExportService(t.TempDir(), &LogExportsConfig{
		Enabled:        true,
		Format:         "jsonl",
		MaxRowsPerFile: 10,
	}, &fakeLogSearchProvider{
		logs: []logstore.Log{
			{ID: "log-1", Provider: "openai", Model: "gpt-4o", Status: "success", Timestamp: time.Now().UTC()},
			{ID: "log-2", Provider: "anthropic", Model: "claude", Status: "error", Timestamp: time.Now().UTC()},
		},
	}, nil, bifrost.NewNoOpLogger())
	if err != nil {
		t.Fatalf("NewLogExportService() error = %v", err)
	}

	job, err := service.Submit(context.Background(), LogExportRequest{
		Scope: ExportScopeLogs,
	}, "tester")
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	var finalJob *ExportJob
	for range 50 {
		time.Sleep(20 * time.Millisecond)
		current, ok := service.GetJob(job.ID)
		if !ok {
			t.Fatalf("expected export job to exist")
		}
		if current.Status == ExportJobCompleted || current.Status == ExportJobFailed {
			finalJob = current
			break
		}
	}

	if finalJob == nil {
		t.Fatal("timed out waiting for export job completion")
	}
	if finalJob.Status != ExportJobCompleted {
		t.Fatalf("expected completed job, got %+v", finalJob)
	}
	if finalJob.RowsExported != 2 {
		t.Fatalf("expected 2 exported rows, got %d", finalJob.RowsExported)
	}

	data, err := os.ReadFile(finalJob.FilePath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "\"id\":\"log-1\"") || !strings.Contains(content, "\"id\":\"log-2\"") {
		t.Fatalf("unexpected export contents: %s", content)
	}
}

func TestLogExportServiceReloadsPersistedJobs(t *testing.T) {
	baseDir := t.TempDir()
	provider := &fakeLogSearchProvider{
		logs: []logstore.Log{
			{ID: "log-1", Provider: "openai", Model: "gpt-4o", Status: "success", Timestamp: time.Now().UTC()},
		},
	}

	service, err := NewLogExportService(baseDir, &LogExportsConfig{
		Enabled:        true,
		Format:         "jsonl",
		MaxRowsPerFile: 10,
	}, provider, nil, bifrost.NewNoOpLogger())
	if err != nil {
		t.Fatalf("NewLogExportService() error = %v", err)
	}

	job, err := service.Submit(context.Background(), LogExportRequest{
		Scope: ExportScopeLogs,
	}, "tester")
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	var finalJob *ExportJob
	for range 50 {
		time.Sleep(20 * time.Millisecond)
		current, ok := service.GetJob(job.ID)
		if !ok {
			t.Fatalf("expected export job to exist")
		}
		if current.Status == ExportJobCompleted || current.Status == ExportJobFailed {
			finalJob = current
			break
		}
	}

	if finalJob == nil {
		t.Fatal("timed out waiting for export job completion")
	}
	if finalJob.Status != ExportJobCompleted {
		t.Fatalf("expected completed job, got %+v", finalJob)
	}

	reloaded, err := NewLogExportService(baseDir, &LogExportsConfig{
		Enabled:        true,
		Format:         "jsonl",
		MaxRowsPerFile: 10,
	}, provider, nil, bifrost.NewNoOpLogger())
	if err != nil {
		t.Fatalf("NewLogExportService() reload error = %v", err)
	}

	reloadedJob, ok := reloaded.GetJob(job.ID)
	if !ok {
		t.Fatalf("expected persisted export job %s to be reloaded", job.ID)
	}
	if reloadedJob.Status != ExportJobCompleted {
		t.Fatalf("expected reloaded completed status, got %+v", reloadedJob)
	}
	if reloadedJob.FilePath != finalJob.FilePath {
		t.Fatalf("expected reloaded file path %q, got %q", finalJob.FilePath, reloadedJob.FilePath)
	}
}
