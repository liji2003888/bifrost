package enterprise

import (
	"bufio"
	"compress/gzip"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/bytedance/sonic"
	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework/logstore"
)

type ExportScope string

const (
	ExportScopeLogs    ExportScope = "logs"
	ExportScopeMCPLogs ExportScope = "mcp_logs"
)

type ExportJobStatus string

const (
	ExportJobPending   ExportJobStatus = "pending"
	ExportJobRunning   ExportJobStatus = "running"
	ExportJobCompleted ExportJobStatus = "completed"
	ExportJobFailed    ExportJobStatus = "failed"
)

type LogSearchProvider interface {
	Search(ctx context.Context, filters *logstore.SearchFilters, pagination *logstore.PaginationOptions) (*logstore.SearchResult, error)
	SearchMCPToolLogs(ctx context.Context, filters *logstore.MCPToolLogSearchFilters, pagination *logstore.PaginationOptions) (*logstore.MCPToolLogSearchResult, error)
}

type LogExportRequest struct {
	Scope       ExportScope                       `json:"scope"`
	Format      string                            `json:"format,omitempty"`
	Compression string                            `json:"compression,omitempty"`
	MaxRows     int                               `json:"max_rows,omitempty"`
	LogFilters  *logstore.SearchFilters           `json:"log_filters,omitempty"`
	MCPFilters  *logstore.MCPToolLogSearchFilters `json:"mcp_filters,omitempty"`
}

type ExportJob struct {
	ID           string          `json:"id"`
	Status       ExportJobStatus `json:"status"`
	Scope        ExportScope     `json:"scope"`
	Format       string          `json:"format"`
	Compression  string          `json:"compression,omitempty"`
	FilePath     string          `json:"file_path,omitempty"`
	RowsExported int             `json:"rows_exported"`
	CreatedAt    time.Time       `json:"created_at"`
	CompletedAt  *time.Time      `json:"completed_at,omitempty"`
	Error        string          `json:"error,omitempty"`
}

type LogExportService struct {
	cfg      *LogExportsConfig
	provider LogSearchProvider
	audit    *AuditService
	logger   schemas.Logger
	basePath string

	mu    sync.RWMutex
	jobs  map[string]*ExportJob
	order []string
}

func NewLogExportService(baseDir string, cfg *LogExportsConfig, provider LogSearchProvider, audit *AuditService, logger schemas.Logger) (*LogExportService, error) {
	if cfg == nil || !cfg.Enabled || provider == nil {
		return nil, nil
	}

	basePath := strings.TrimSpace(cfg.StoragePath)
	if basePath == "" {
		basePath = filepath.Join(baseDir, "exports")
	}
	if err := os.MkdirAll(basePath, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create export directory: %w", err)
	}

	return &LogExportService{
		cfg:      cfg,
		provider: provider,
		audit:    audit,
		logger:   logger,
		basePath: basePath,
		jobs:     make(map[string]*ExportJob),
		order:    make([]string, 0),
	}, nil
}

func (s *LogExportService) Submit(ctx context.Context, req LogExportRequest, actorID string) (*ExportJob, error) {
	if s == nil {
		return nil, fmt.Errorf("log export service is not enabled")
	}

	scope := req.Scope
	if scope == "" {
		scope = ExportScopeLogs
	}
	if scope != ExportScopeLogs && scope != ExportScopeMCPLogs {
		return nil, fmt.Errorf("unsupported export scope: %s", scope)
	}

	format := strings.ToLower(strings.TrimSpace(req.Format))
	if format == "" {
		format = strings.ToLower(strings.TrimSpace(s.cfg.Format))
	}
	if format == "" {
		format = "jsonl"
	}
	if format != "jsonl" && format != "csv" {
		return nil, fmt.Errorf("unsupported export format: %s", format)
	}

	compression := strings.ToLower(strings.TrimSpace(req.Compression))
	if compression == "" {
		compression = strings.ToLower(strings.TrimSpace(s.cfg.Compression))
	}
	if compression != "" && compression != "gzip" {
		return nil, fmt.Errorf("unsupported compression: %s", compression)
	}

	job := &ExportJob{
		ID:          fmt.Sprintf("export_%d", time.Now().UnixNano()),
		Status:      ExportJobPending,
		Scope:       scope,
		Format:      format,
		Compression: compression,
		CreatedAt:   time.Now().UTC(),
	}

	s.mu.Lock()
	s.jobs[job.ID] = job
	s.order = append(s.order, job.ID)
	s.mu.Unlock()

	if s.audit != nil {
		_ = s.audit.Append(&AuditEvent{
			Timestamp:    time.Now().UTC(),
			Category:     AuditCategoryExport,
			Action:       "submit",
			ResourceType: string(scope),
			ResourceID:   job.ID,
			ActorID:      actorID,
			Message:      "log export job submitted",
			Metadata: map[string]any{
				"format":      format,
				"compression": compression,
			},
		})
	}

	go s.execute(context.WithoutCancel(ctx), job.ID, req)
	return s.cloneJob(job), nil
}

func (s *LogExportService) GetJob(id string) (*ExportJob, bool) {
	if s == nil {
		return nil, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	job, ok := s.jobs[id]
	if !ok {
		return nil, false
	}
	return s.cloneJob(job), true
}

func (s *LogExportService) ListJobs() []ExportJob {
	if s == nil {
		return nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]ExportJob, 0, len(s.order))
	for _, id := range s.order {
		if job, ok := s.jobs[id]; ok && job != nil {
			result = append(result, *s.cloneJob(job))
		}
	}
	slices.Reverse(result)
	return result
}

func (s *LogExportService) execute(ctx context.Context, jobID string, req LogExportRequest) {
	s.updateJob(jobID, func(job *ExportJob) {
		job.Status = ExportJobRunning
	})

	job, ok := s.GetJob(jobID)
	if !ok {
		return
	}

	filePath := filepath.Join(s.basePath, s.buildFileName(job))
	file, err := os.Create(filePath)
	if err != nil {
		s.failJob(jobID, err)
		return
	}
	defer file.Close()

	writer, closer, err := s.wrapWriter(file, job.Compression)
	if err != nil {
		s.failJob(jobID, err)
		return
	}
	defer closer()

	buffered := bufio.NewWriter(writer)
	defer buffered.Flush()

	var rows int
	switch job.Scope {
	case ExportScopeLogs:
		rows, err = s.exportLogs(ctx, buffered, job.Format, req)
	case ExportScopeMCPLogs:
		rows, err = s.exportMCPLogs(ctx, buffered, job.Format, req)
	default:
		err = fmt.Errorf("unsupported export scope: %s", job.Scope)
	}
	if err != nil {
		s.failJob(jobID, err)
		_ = os.Remove(filePath)
		return
	}

	completedAt := time.Now().UTC()
	s.updateJob(jobID, func(job *ExportJob) {
		job.Status = ExportJobCompleted
		job.FilePath = filePath
		job.RowsExported = rows
		job.CompletedAt = &completedAt
	})
}

func (s *LogExportService) exportLogs(ctx context.Context, writer io.Writer, format string, req LogExportRequest) (int, error) {
	filters := req.LogFilters
	if filters == nil {
		filters = &logstore.SearchFilters{}
	}

	maxRows := req.MaxRows
	if maxRows <= 0 {
		maxRows = s.cfg.MaxRowsPerFile
	}
	if maxRows <= 0 {
		maxRows = 10000
	}

	switch format {
	case "csv":
		return s.exportLogsCSV(ctx, writer, filters, maxRows)
	default:
		return s.exportLogsJSONL(ctx, writer, filters, maxRows)
	}
}

func (s *LogExportService) exportMCPLogs(ctx context.Context, writer io.Writer, format string, req LogExportRequest) (int, error) {
	filters := req.MCPFilters
	if filters == nil {
		filters = &logstore.MCPToolLogSearchFilters{}
	}

	maxRows := req.MaxRows
	if maxRows <= 0 {
		maxRows = s.cfg.MaxRowsPerFile
	}
	if maxRows <= 0 {
		maxRows = 10000
	}

	switch format {
	case "csv":
		return s.exportMCPLogsCSV(ctx, writer, filters, maxRows)
	default:
		return s.exportMCPLogsJSONL(ctx, writer, filters, maxRows)
	}
}

func (s *LogExportService) exportLogsJSONL(ctx context.Context, writer io.Writer, filters *logstore.SearchFilters, maxRows int) (int, error) {
	total := 0
	offset := 0
	for total < maxRows {
		limit := min(500, maxRows-total)
		result, err := s.provider.Search(ctx, filters, &logstore.PaginationOptions{
			Limit:  limit,
			Offset: offset,
			SortBy: "timestamp",
			Order:  "desc",
		})
		if err != nil {
			return total, err
		}
		if result == nil || len(result.Logs) == 0 {
			return total, nil
		}
		for _, item := range result.Logs {
			payload, err := sonic.Marshal(item)
			if err != nil {
				return total, err
			}
			if _, err := writer.Write(append(payload, '\n')); err != nil {
				return total, err
			}
			total++
		}
		offset += len(result.Logs)
		if len(result.Logs) < limit {
			break
		}
	}
	return total, nil
}

func (s *LogExportService) exportLogsCSV(ctx context.Context, writer io.Writer, filters *logstore.SearchFilters, maxRows int) (int, error) {
	csvWriter := csv.NewWriter(writer)
	if err := csvWriter.Write([]string{
		"id", "timestamp", "object", "provider", "model", "status", "latency_ms", "total_tokens", "cost", "selected_key_id", "virtual_key_id",
	}); err != nil {
		return 0, err
	}

	total := 0
	offset := 0
	for total < maxRows {
		limit := min(500, maxRows-total)
		result, err := s.provider.Search(ctx, filters, &logstore.PaginationOptions{
			Limit:  limit,
			Offset: offset,
			SortBy: "timestamp",
			Order:  "desc",
		})
		if err != nil {
			return total, err
		}
		if result == nil || len(result.Logs) == 0 {
			break
		}
		for _, item := range result.Logs {
			row := []string{
				item.ID,
				item.Timestamp.Format(time.RFC3339),
				item.Object,
				item.Provider,
				item.Model,
				item.Status,
				formatFloatPtr(item.Latency),
				fmt.Sprintf("%d", item.TotalTokens),
				formatFloatPtr(item.Cost),
				item.SelectedKeyID,
				derefString(item.VirtualKeyID),
			}
			if err := csvWriter.Write(row); err != nil {
				return total, err
			}
			total++
		}
		offset += len(result.Logs)
		if len(result.Logs) < limit {
			break
		}
	}
	csvWriter.Flush()
	return total, csvWriter.Error()
}

func (s *LogExportService) exportMCPLogsJSONL(ctx context.Context, writer io.Writer, filters *logstore.MCPToolLogSearchFilters, maxRows int) (int, error) {
	total := 0
	offset := 0
	for total < maxRows {
		limit := min(500, maxRows-total)
		result, err := s.provider.SearchMCPToolLogs(ctx, filters, &logstore.PaginationOptions{
			Limit:  limit,
			Offset: offset,
			SortBy: "timestamp",
			Order:  "desc",
		})
		if err != nil {
			return total, err
		}
		if result == nil || len(result.Logs) == 0 {
			return total, nil
		}
		for _, item := range result.Logs {
			payload, err := sonic.Marshal(item)
			if err != nil {
				return total, err
			}
			if _, err := writer.Write(append(payload, '\n')); err != nil {
				return total, err
			}
			total++
		}
		offset += len(result.Logs)
		if len(result.Logs) < limit {
			break
		}
	}
	return total, nil
}

func (s *LogExportService) exportMCPLogsCSV(ctx context.Context, writer io.Writer, filters *logstore.MCPToolLogSearchFilters, maxRows int) (int, error) {
	csvWriter := csv.NewWriter(writer)
	if err := csvWriter.Write([]string{
		"id", "timestamp", "tool_name", "server_label", "status", "latency_ms", "cost", "virtual_key_id", "llm_request_id",
	}); err != nil {
		return 0, err
	}

	total := 0
	offset := 0
	for total < maxRows {
		limit := min(500, maxRows-total)
		result, err := s.provider.SearchMCPToolLogs(ctx, filters, &logstore.PaginationOptions{
			Limit:  limit,
			Offset: offset,
			SortBy: "timestamp",
			Order:  "desc",
		})
		if err != nil {
			return total, err
		}
		if result == nil || len(result.Logs) == 0 {
			break
		}
		for _, item := range result.Logs {
			row := []string{
				item.ID,
				item.Timestamp.Format(time.RFC3339),
				item.ToolName,
				item.ServerLabel,
				item.Status,
				formatFloatPtr(item.Latency),
				formatFloatPtr(item.Cost),
				derefString(item.VirtualKeyID),
				derefString(item.LLMRequestID),
			}
			if err := csvWriter.Write(row); err != nil {
				return total, err
			}
			total++
		}
		offset += len(result.Logs)
		if len(result.Logs) < limit {
			break
		}
	}
	csvWriter.Flush()
	return total, csvWriter.Error()
}

func (s *LogExportService) buildFileName(job *ExportJob) string {
	ext := job.Format
	if ext == "" {
		ext = "jsonl"
	}
	if job.Compression == "gzip" {
		return fmt.Sprintf("%s_%s.%s.gz", job.Scope, job.ID, ext)
	}
	return fmt.Sprintf("%s_%s.%s", job.Scope, job.ID, ext)
}

func (s *LogExportService) wrapWriter(writer io.Writer, compression string) (io.Writer, func() error, error) {
	if compression != "gzip" {
		return writer, func() error { return nil }, nil
	}

	gzipWriter := gzip.NewWriter(writer)
	return gzipWriter, gzipWriter.Close, nil
}

func (s *LogExportService) failJob(jobID string, err error) {
	completedAt := time.Now().UTC()
	s.updateJob(jobID, func(job *ExportJob) {
		job.Status = ExportJobFailed
		job.Error = err.Error()
		job.CompletedAt = &completedAt
	})
	if s.logger != nil {
		s.logger.Warn("log export job %s failed: %v", jobID, err)
	}
}

func (s *LogExportService) updateJob(jobID string, mutate func(*ExportJob)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[jobID]
	if !ok || job == nil {
		return
	}
	mutate(job)
}

func (s *LogExportService) cloneJob(job *ExportJob) *ExportJob {
	if job == nil {
		return nil
	}
	copyJob := *job
	return &copyJob
}

func formatFloatPtr(value *float64) string {
	if value == nil {
		return ""
	}
	return fmt.Sprintf("%.4f", *value)
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
