package enterprise

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/smtp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bytedance/sonic"
	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework/configstore/tables"
	"github.com/maximhq/bifrost/framework/logstore"
	"github.com/maximhq/bifrost/plugins/governance"
)

type LogStatsProvider interface {
	GetStats(ctx context.Context, filters *logstore.SearchFilters) (*logstore.SearchStats, error)
}

type GovernanceDataProvider interface {
	GetGovernanceData() *governance.GovernanceData
}

type AlertSeverity string

const (
	AlertSeverityInfo     AlertSeverity = "info"
	AlertSeverityWarning  AlertSeverity = "warning"
	AlertSeverityCritical AlertSeverity = "critical"
)

type AlertRecord struct {
	ID          string         `json:"id"`
	Key         string         `json:"key"`
	Type        string         `json:"type"`
	Severity    AlertSeverity  `json:"severity"`
	Title       string         `json:"title"`
	Message     string         `json:"message"`
	TriggeredAt time.Time      `json:"triggered_at"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

type AlertManager struct {
	cfg        *AlertsConfig
	stats      LogStatsProvider
	governance GovernanceDataProvider
	audit      *AuditService
	logger     schemas.Logger
	client     *http.Client

	mu       sync.RWMutex
	active   map[string]AlertRecord
	stopCh   chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup
}

func NewAlertManager(cfg *AlertsConfig, stats LogStatsProvider, governance GovernanceDataProvider, audit *AuditService, logger schemas.Logger) *AlertManager {
	if cfg == nil || !cfg.Enabled {
		return nil
	}

	return &AlertManager{
		cfg:        normalizeAlertsConfig(cfg),
		stats:      stats,
		governance: governance,
		audit:      audit,
		logger:     logger,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		active: make(map[string]AlertRecord),
		stopCh: make(chan struct{}),
	}
}

func (m *AlertManager) Start() {
	if m == nil {
		return
	}
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		interval := time.Duration(m.cfg.EvaluationIntervalSeconds) * time.Second
		if interval <= 0 {
			interval = 30 * time.Second
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		m.evaluate()
		for {
			select {
			case <-ticker.C:
				m.evaluate()
			case <-m.stopCh:
				return
			}
		}
	}()
}

func (m *AlertManager) Stop() {
	if m == nil {
		return
	}
	m.stopOnce.Do(func() {
		close(m.stopCh)
	})
	m.wg.Wait()
}

func (m *AlertManager) ListActive() []AlertRecord {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]AlertRecord, 0, len(m.active))
	for _, alert := range m.active {
		result = append(result, alert)
	}
	slices.SortFunc(result, func(a, b AlertRecord) int {
		return b.TriggeredAt.Compare(a.TriggeredAt)
	})
	return result
}

func (m *AlertManager) evaluate() {
	if m == nil {
		return
	}

	current := make(map[string]AlertRecord)
	m.evaluateHealthAlerts(current)
	m.evaluateBudgetAlerts(current)

	m.mu.Lock()
	defer m.mu.Unlock()

	for key, alert := range current {
		if _, exists := m.active[key]; exists {
			continue
		}
		m.active[key] = alert
		go m.dispatch(alert)
	}

	for key := range m.active {
		if _, exists := current[key]; exists {
			continue
		}
		delete(m.active, key)
	}
}

func (m *AlertManager) evaluateHealthAlerts(current map[string]AlertRecord) {
	if m.stats == nil {
		return
	}

	now := time.Now().UTC()
	start := now.Add(-time.Duration(m.cfg.LookbackMinutes) * time.Minute)
	stats, err := m.stats.GetStats(context.Background(), &logstore.SearchFilters{
		StartTime: &start,
		EndTime:   &now,
	})
	if err != nil {
		if m.logger != nil {
			m.logger.Warn("failed to evaluate health alerts: %v", err)
		}
		return
	}
	if stats == nil || stats.TotalRequests < int64(m.cfg.MinimumRequests) {
		return
	}

	errorRate := 100 - stats.SuccessRate
	if errorRate >= m.cfg.ErrorRateThresholdPercent {
		alert := newAlertRecord(
			"alert-health-error-rate",
			"health.error_rate",
			AlertSeverityCritical,
			"High error rate detected",
			fmt.Sprintf("Error rate reached %.2f%% over the last %d minutes.", errorRate, m.cfg.LookbackMinutes),
			now,
			map[string]any{
				"error_rate_percent": errorRate,
				"lookback_minutes":   m.cfg.LookbackMinutes,
				"total_requests":     stats.TotalRequests,
			},
		)
		current[alert.Key] = alert
	}

	if stats.AverageLatency >= m.cfg.AverageLatencyThresholdMs {
		alert := newAlertRecord(
			"alert-health-latency",
			"health.average_latency",
			AlertSeverityWarning,
			"Average latency threshold exceeded",
			fmt.Sprintf("Average latency reached %.2f ms over the last %d minutes.", stats.AverageLatency, m.cfg.LookbackMinutes),
			now,
			map[string]any{
				"average_latency_ms": stats.AverageLatency,
				"lookback_minutes":   m.cfg.LookbackMinutes,
				"total_requests":     stats.TotalRequests,
			},
		)
		current[alert.Key] = alert
	}
}

func (m *AlertManager) evaluateBudgetAlerts(current map[string]AlertRecord) {
	if m.governance == nil || len(m.cfg.BudgetThresholdsPercent) == 0 {
		return
	}

	data := m.governance.GetGovernanceData()
	if data == nil {
		return
	}

	thresholds := append([]float64(nil), m.cfg.BudgetThresholdsPercent...)
	slices.Sort(thresholds)

	now := time.Now().UTC()
	for _, budget := range data.Budgets {
		if budget == nil || budget.MaxLimit <= 0 {
			continue
		}
		percentUsed := (budget.CurrentUsage / budget.MaxLimit) * 100
		for _, threshold := range thresholds {
			if percentUsed < threshold {
				continue
			}
			alertKey := fmt.Sprintf("budget.%s.%.2f", budget.ID, threshold)
			alert := newAlertRecord(
				fmt.Sprintf("alert-budget-%s-%0.f", budget.ID, threshold),
				alertKey,
				AlertSeverityWarning,
				"Budget threshold exceeded",
				fmt.Sprintf("Budget %s is at %.2f%% of its configured limit.", budget.ID, percentUsed),
				now,
				map[string]any{
					"budget_id":           budget.ID,
					"budget_percent_used": percentUsed,
					"budget_threshold":    threshold,
					"max_limit":           budget.MaxLimit,
					"current_usage":       budget.CurrentUsage,
				},
			)
			current[alert.Key] = alert
		}
	}
}

func (m *AlertManager) dispatch(alert AlertRecord) {
	if m == nil {
		return
	}

	if m.audit != nil {
		_ = m.audit.Append(&AuditEvent{
			Timestamp:    time.Now().UTC(),
			Category:     AuditCategorySecurityEvent,
			Action:       "trigger",
			ResourceType: "alert",
			ResourceID:   alert.ID,
			Message:      alert.Message,
			Metadata:     alert.Metadata,
		})
	}

	if err := m.sendToChannels(alert); err != nil && m.logger != nil {
		m.logger.Warn("failed to dispatch alert %s: %v", alert.ID, err)
	}
}

func (m *AlertManager) sendToChannels(alert AlertRecord) error {
	var errs []string

	if cfg := m.cfg.Channels; cfg != nil {
		if cfg.Webhook != nil && cfg.Webhook.Enabled {
			if err := m.sendWebhookAlert(alert, cfg.Webhook); err != nil {
				errs = append(errs, err.Error())
			}
		}
		if cfg.Feishu != nil && cfg.Feishu.Enabled {
			if err := m.sendFeishuAlert(alert, cfg.Feishu); err != nil {
				errs = append(errs, err.Error())
			}
		}
		if cfg.Email != nil && cfg.Email.Enabled {
			if err := m.sendEmailAlert(alert, cfg.Email); err != nil {
				errs = append(errs, err.Error())
			}
		}
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

func (m *AlertManager) sendWebhookAlert(alert AlertRecord, cfg *WebhookAlertConfig) error {
	if cfg == nil || cfg.URL == nil || cfg.URL.GetValue() == "" {
		return fmt.Errorf("webhook alert url is not configured")
	}

	payload, err := sonic.Marshal(map[string]any{
		"alert": alert,
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, cfg.URL.GetValue(), bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	for key, value := range cfg.Headers {
		req.Header.Set(key, value)
	}

	resp, err := m.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("webhook alert returned status %d", resp.StatusCode)
	}
	return nil
}

func (m *AlertManager) sendFeishuAlert(alert AlertRecord, cfg *FeishuAlertConfig) error {
	if cfg == nil || cfg.WebhookURL == nil || cfg.WebhookURL.GetValue() == "" {
		return fmt.Errorf("feishu webhook url is not configured")
	}

	body := map[string]any{
		"msg_type": "text",
		"content": map[string]string{
			"text": fmt.Sprintf("%s\n%s", alert.Title, alert.Message),
		},
	}

	if cfg.Secret != nil && cfg.Secret.GetValue() != "" {
		timestamp := strconv.FormatInt(time.Now().Unix(), 10)
		signature := signFeishuSecret(timestamp, cfg.Secret.GetValue())
		body["timestamp"] = timestamp
		body["sign"] = signature
	}

	payload, err := sonic.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, cfg.WebhookURL.GetValue(), bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("feishu alert returned status %d", resp.StatusCode)
	}
	return nil
}

func (m *AlertManager) sendEmailAlert(alert AlertRecord, cfg *EmailAlertConfig) error {
	if cfg == nil || cfg.SMTPHost == "" || cfg.From == "" || len(cfg.To) == 0 {
		return fmt.Errorf("email alert config is incomplete")
	}

	addr := fmt.Sprintf("%s:%d", cfg.SMTPHost, cfg.SMTPPort)
	var auth smtp.Auth
	if cfg.Username != nil && cfg.Username.GetValue() != "" {
		auth = smtp.PlainAuth("", cfg.Username.GetValue(), cfg.Password.GetValue(), cfg.SMTPHost)
	}

	message := buildSMTPMessage(cfg.From, cfg.To, alert.Title, alert.Message)
	return smtp.SendMail(addr, auth, cfg.From, cfg.To, []byte(message))
}

func normalizeAlertsConfig(cfg *AlertsConfig) *AlertsConfig {
	copyCfg := *cfg
	if copyCfg.EvaluationIntervalSeconds <= 0 {
		copyCfg.EvaluationIntervalSeconds = 30
	}
	if copyCfg.LookbackMinutes <= 0 {
		copyCfg.LookbackMinutes = 5
	}
	if copyCfg.MinimumRequests <= 0 {
		copyCfg.MinimumRequests = 20
	}
	if copyCfg.ErrorRateThresholdPercent <= 0 {
		copyCfg.ErrorRateThresholdPercent = 10
	}
	if copyCfg.AverageLatencyThresholdMs <= 0 {
		copyCfg.AverageLatencyThresholdMs = 5000
	}
	return &copyCfg
}

func newAlertRecord(id, kind string, severity AlertSeverity, title, message string, triggeredAt time.Time, metadata map[string]any) AlertRecord {
	return AlertRecord{
		ID:          id,
		Key:         kind,
		Type:        kind,
		Severity:    severity,
		Title:       title,
		Message:     message,
		TriggeredAt: triggeredAt,
		Metadata:    metadata,
	}
}

func signFeishuSecret(timestamp, secret string) string {
	stringToSign := timestamp + "\n" + secret
	mac := hmac.New(sha256.New, []byte(stringToSign))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func buildSMTPMessage(from string, to []string, subject string, body string) string {
	var builder strings.Builder
	builder.WriteString("To: " + strings.Join(to, ",") + "\r\n")
	builder.WriteString("Subject: " + subject + "\r\n")
	builder.WriteString("MIME-Version: 1.0\r\n")
	builder.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	builder.WriteString("\r\n")
	builder.WriteString(body)
	builder.WriteString("\r\n")
	return builder.String()
}

func budgetPercent(budget *tables.TableBudget) float64 {
	if budget == nil || budget.MaxLimit <= 0 {
		return 0
	}
	return (budget.CurrentUsage / budget.MaxLimit) * 100
}
