package enterprise

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/bytedance/sonic"
	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework/configstore"
	configstoreTables "github.com/maximhq/bifrost/framework/configstore/tables"
	"github.com/maximhq/bifrost/framework/logstore"
)

// ExportScheduler runs scheduled log export jobs based on TableLogExportConfig records.
type ExportScheduler struct {
	store    configstore.ConfigStore
	exporter *LogExportService
	logger   schemas.Logger

	stopCh chan struct{}
	wg     sync.WaitGroup
}

// NewExportScheduler creates a new ExportScheduler.
func NewExportScheduler(store configstore.ConfigStore, exporter *LogExportService, logger schemas.Logger) *ExportScheduler {
	return &ExportScheduler{
		store:    store,
		exporter: exporter,
		logger:   logger,
		stopCh:   make(chan struct{}),
	}
}

// Start launches the scheduler background goroutine.
// It ticks every 60 seconds and checks whether any enabled export config is due to run.
func (s *ExportScheduler) Start() {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()

		// Run an initial check immediately on startup.
		s.checkAndRun(context.Background())

		for {
			select {
			case <-s.stopCh:
				return
			case <-ticker.C:
				s.checkAndRun(context.Background())
			}
		}
	}()
}

// Stop signals the scheduler to exit and waits for the goroutine to finish.
func (s *ExportScheduler) Stop() {
	close(s.stopCh)
	s.wg.Wait()
}

// checkAndRun loads all enabled export configs and triggers execution for any that are due.
func (s *ExportScheduler) checkAndRun(ctx context.Context) {
	if s.store == nil || s.exporter == nil {
		return
	}

	configs, err := s.store.GetLogExportConfigs(ctx)
	if err != nil {
		if s.logger != nil {
			s.logger.Warn("export scheduler: failed to load configs: %v", err)
		}
		return
	}

	now := time.Now().UTC()
	for i := range configs {
		cfg := &configs[i]
		if !cfg.Enabled {
			continue
		}

		// Calculate NextRunAt on first encounter or if it's zero.
		if cfg.NextRunAt == nil || cfg.NextRunAt.IsZero() {
			next := s.calculateNextRun(cfg, now)
			cfg.NextRunAt = &next
			if err := s.store.UpdateLogExportConfig(ctx, cfg); err != nil && s.logger != nil {
				s.logger.Warn("export scheduler: failed to persist next_run_at for config %s: %v", cfg.ID, err)
			}
			continue
		}

		if now.Before(*cfg.NextRunAt) {
			continue
		}

		// Don't double-trigger if already running.
		if cfg.LastRunStatus == "running" {
			continue
		}

		s.wg.Add(1)
		go func(c configstoreTables.TableLogExportConfig) {
			defer s.wg.Done()
			s.executeExport(context.WithoutCancel(ctx), &c)
		}(*cfg)
	}
}

// executeExport runs a single scheduled export job for the given config.
func (s *ExportScheduler) executeExport(ctx context.Context, cfg *configstoreTables.TableLogExportConfig) {
	if s.logger != nil {
		s.logger.Info("export scheduler: starting scheduled export for config %s (%s)", cfg.ID, cfg.Name)
	}

	// Mark as running.
	now := time.Now().UTC()
	cfg.LastRunAt = &now
	cfg.LastRunStatus = "running"
	cfg.LastRunError = ""
	next := s.calculateNextRun(cfg, now)
	cfg.NextRunAt = &next
	if err := s.store.UpdateLogExportConfig(ctx, cfg); err != nil && s.logger != nil {
		s.logger.Warn("export scheduler: failed to mark config %s as running: %v", cfg.ID, err)
	}

	// Build the export request from the config.
	req := LogExportRequest{
		Scope:       ExportScope(cfg.DataScope),
		Format:      cfg.Format,
		Compression: cfg.Compression,
		MaxRows:     cfg.MaxRows,
	}

	// Apply parsed filters if present.
	if cfg.ParsedFilters != nil {
		if scope, ok := cfg.ParsedFilters["scope"].(string); ok && scope != "" {
			req.Scope = ExportScope(scope)
		}
		// Serialize back and decode into typed filter structs via JSON round-trip.
		if req.Scope == ExportScopeMCPLogs {
			var mcpFilters logstore.MCPToolLogSearchFilters
			if err := mapToStruct(cfg.ParsedFilters, &mcpFilters); err == nil {
				req.MCPFilters = &mcpFilters
			}
		} else {
			var logFilters logstore.SearchFilters
			if err := mapToStruct(cfg.ParsedFilters, &logFilters); err == nil {
				req.LogFilters = &logFilters
			}
		}
	}

	job, err := s.exporter.Submit(ctx, req, fmt.Sprintf("scheduler:%s", cfg.ID))
	if err != nil {
		s.finishExport(ctx, cfg, "", fmt.Errorf("failed to submit export job: %w", err))
		return
	}

	// Poll until the job completes.
	deadline := time.Now().Add(2 * time.Hour)
	for time.Now().Before(deadline) {
		select {
		case <-s.stopCh:
			// Scheduler is stopping; leave the job in-flight.
			return
		case <-time.After(5 * time.Second):
		}

		current, ok := s.exporter.GetJob(job.ID)
		if !ok {
			s.finishExport(ctx, cfg, "", fmt.Errorf("export job %s disappeared", job.ID))
			return
		}

		switch current.Status {
		case ExportJobCompleted:
			// Attempt to upload to the configured destination.
			uploadErr := s.uploadToDestination(ctx, cfg, current)
			if uploadErr != nil {
				s.finishExport(ctx, cfg, current.FilePath, uploadErr)
				return
			}
			s.finishExport(ctx, cfg, current.FilePath, nil)
			return
		case ExportJobFailed:
			s.finishExport(ctx, cfg, "", fmt.Errorf("export job failed: %s", current.Error))
			return
		}
		// Still pending/running — keep polling.
	}

	s.finishExport(ctx, cfg, "", fmt.Errorf("export job timed out after 2 hours"))
}

// uploadToDestination creates the appropriate ExportDestination and uploads the completed file.
func (s *ExportScheduler) uploadToDestination(ctx context.Context, cfg *configstoreTables.TableLogExportConfig, job *ExportJob) error {
	dest, err := NewDestination(cfg.DestinationType, cfg.ParsedDestinationConfig)
	if err != nil {
		return fmt.Errorf("failed to create destination: %w", err)
	}

	if cfg.DestinationType == "local" || cfg.DestinationType == "" {
		// Nothing to upload; file is already on disk.
		return nil
	}

	objectKey := filepath.Base(job.FilePath)
	if err := dest.Upload(ctx, job.FilePath, objectKey); err != nil {
		return fmt.Errorf("upload to %s failed: %w", dest.Name(), err)
	}

	if s.logger != nil {
		s.logger.Info("export scheduler: uploaded %s to %s destination", objectKey, dest.Name())
	}
	return nil
}

// finishExport persists the final run state back to the config store.
func (s *ExportScheduler) finishExport(ctx context.Context, cfg *configstoreTables.TableLogExportConfig, filePath string, runErr error) {
	if runErr != nil {
		cfg.LastRunStatus = "failed"
		cfg.LastRunError = runErr.Error()
		if s.logger != nil {
			s.logger.Warn("export scheduler: config %s (%s) failed: %v", cfg.ID, cfg.Name, runErr)
		}
	} else {
		cfg.LastRunStatus = "success"
		cfg.LastRunError = ""
		if s.logger != nil {
			s.logger.Info("export scheduler: config %s (%s) succeeded (file: %s)", cfg.ID, cfg.Name, filePath)
		}
	}

	if err := s.store.UpdateLogExportConfig(ctx, cfg); err != nil && s.logger != nil {
		s.logger.Warn("export scheduler: failed to persist final state for config %s: %v", cfg.ID, err)
	}
}

// shouldRunNow returns true if the config's NextRunAt is at or before now and the
// config is enabled and not already running.
func (s *ExportScheduler) shouldRunNow(cfg *configstoreTables.TableLogExportConfig) bool {
	if !cfg.Enabled {
		return false
	}
	if cfg.LastRunStatus == "running" {
		return false
	}
	if cfg.NextRunAt == nil || cfg.NextRunAt.IsZero() {
		return false
	}
	return !time.Now().UTC().Before(*cfg.NextRunAt)
}

// calculateNextRun computes the next scheduled run time based on the config's
// frequency, schedule_time, schedule_day, and timezone.
func (s *ExportScheduler) calculateNextRun(cfg *configstoreTables.TableLogExportConfig, from time.Time) time.Time {
	loc, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		loc = time.UTC
	}

	// Parse HH:MM
	schedHour, schedMin := parseHHMM(cfg.ScheduleTime)

	local := from.In(loc)

	switch strings.ToLower(cfg.Frequency) {
	case "weekly":
		targetWeekday := parseWeekday(cfg.ScheduleDay)
		// Advance day by day until we find the target weekday.
		candidate := time.Date(local.Year(), local.Month(), local.Day(), schedHour, schedMin, 0, 0, loc)
		for i := 0; i < 8; i++ {
			if candidate.Weekday() == targetWeekday && candidate.After(from) {
				return candidate.UTC()
			}
			candidate = candidate.AddDate(0, 0, 1)
		}
		// Fallback: one week from now.
		return from.Add(7 * 24 * time.Hour)

	case "monthly":
		targetDay := parseDayOfMonth(cfg.ScheduleDay)
		if targetDay < 1 || targetDay > 31 {
			targetDay = 1
		}
		// Try this month, then next month.
		for i := 0; i < 3; i++ {
			year := local.Year()
			month := local.Month() + time.Month(i)
			// Clamp day to the last day of the month.
			maxDay := daysInMonth(year, month)
			day := targetDay
			if day > maxDay {
				day = maxDay
			}
			candidate := time.Date(year, month, day, schedHour, schedMin, 0, 0, loc)
			if candidate.After(from) {
				return candidate.UTC()
			}
		}
		return from.AddDate(0, 1, 0)

	default: // "daily"
		candidate := time.Date(local.Year(), local.Month(), local.Day(), schedHour, schedMin, 0, 0, loc)
		if candidate.After(from) {
			return candidate.UTC()
		}
		return candidate.AddDate(0, 0, 1).UTC()
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Scheduler helpers
// ─────────────────────────────────────────────────────────────────────────────

// parseHHMM parses a "HH:MM" string into hour and minute ints.
func parseHHMM(s string) (hour, min int) {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return 2, 0 // default 02:00
	}
	hour = parseIntOr(parts[0], 2)
	min = parseIntOr(parts[1], 0)
	if hour < 0 || hour > 23 {
		hour = 2
	}
	if min < 0 || min > 59 {
		min = 0
	}
	return
}

func parseIntOr(s string, fallback int) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return fallback
	}
	n := 0
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return fallback
		}
		n = n*10 + int(ch-'0')
	}
	return n
}

// parseWeekday maps common English weekday names to time.Weekday.
func parseWeekday(s string) time.Weekday {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "sunday", "sun", "0":
		return time.Sunday
	case "monday", "mon", "1":
		return time.Monday
	case "tuesday", "tue", "2":
		return time.Tuesday
	case "wednesday", "wed", "3":
		return time.Wednesday
	case "thursday", "thu", "4":
		return time.Thursday
	case "friday", "fri", "5":
		return time.Friday
	case "saturday", "sat", "6":
		return time.Saturday
	default:
		return time.Sunday
	}
}

// parseDayOfMonth converts a string like "15" or "1" to an integer day of month.
func parseDayOfMonth(s string) int {
	return parseIntOr(strings.TrimSpace(s), 1)
}

// daysInMonth returns the number of days in a given month/year.
func daysInMonth(year int, month time.Month) int {
	// time.Date normalises month overflows, so day 0 of next month == last day of this month.
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

// mapToStruct marshals a map[string]any into a typed struct via JSON round-trip.
func mapToStruct(src map[string]any, dst any) error {
	data, err := sonic.Marshal(src)
	if err != nil {
		return err
	}
	return sonic.Unmarshal(data, dst)
}
