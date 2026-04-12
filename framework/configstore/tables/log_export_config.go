package tables

import (
	"strings"
	"time"

	"github.com/bytedance/sonic"
	bifrost "github.com/maximhq/bifrost/core"
	"gorm.io/gorm"
)

// TableLogExportConfig represents a named log export configuration with schedule and destination
type TableLogExportConfig struct {
	ID          string `gorm:"primaryKey;type:varchar(255)" json:"id"`
	Name        string `gorm:"type:varchar(255);not null;uniqueIndex" json:"name"`
	Description string `gorm:"type:text" json:"description"`
	Enabled     bool   `gorm:"not null;default:true" json:"enabled"`

	// Schedule
	Frequency    string `gorm:"type:varchar(50);not null;default:daily" json:"frequency"`    // daily, weekly, monthly
	ScheduleTime string `gorm:"type:varchar(10);not null;default:02:00" json:"schedule_time"` // HH:MM format
	ScheduleDay  string `gorm:"type:varchar(20)" json:"schedule_day,omitempty"`              // day of week for weekly, day of month for monthly
	Timezone     string `gorm:"type:varchar(100);not null;default:UTC" json:"timezone"`

	// Destination
	DestinationType       string         `gorm:"type:varchar(50);not null" json:"destination_type"` // local, s3, gcs, azure_blob
	DestinationConfig     *string        `gorm:"type:text" json:"-"`                                // JSON serialized
	ParsedDestinationConfig map[string]any `gorm:"-" json:"destination_config,omitempty"`

	// Data config
	Format      string `gorm:"type:varchar(20);not null;default:jsonl" json:"format"`       // jsonl, csv
	Compression string `gorm:"type:varchar(20);default:gzip" json:"compression"`            // none, gzip
	MaxRows     int    `gorm:"not null;default:100000" json:"max_rows"`
	DataScope   string `gorm:"type:varchar(50);not null;default:logs" json:"data_scope"` // logs, mcp_logs

	// Filters (JSON)
	Filters       *string        `gorm:"type:text" json:"-"`
	ParsedFilters map[string]any `gorm:"-" json:"filters,omitempty"`

	// Execution state
	LastRunAt     *time.Time `gorm:"" json:"last_run_at,omitempty"`
	LastRunStatus string     `gorm:"type:varchar(50)" json:"last_run_status,omitempty"` // success, failed, running
	LastRunError  string     `gorm:"type:text" json:"last_run_error,omitempty"`
	NextRunAt     *time.Time `gorm:"" json:"next_run_at,omitempty"`

	CreatedAt time.Time `gorm:"index;not null" json:"created_at"`
	UpdatedAt time.Time `gorm:"index;not null" json:"updated_at"`
}

// TableName returns the table name for TableLogExportConfig.
func (TableLogExportConfig) TableName() string { return "log_export_configs" }

// BeforeSave hook for TableLogExportConfig to serialize JSON fields.
func (c *TableLogExportConfig) BeforeSave(tx *gorm.DB) error {
	if len(c.ParsedDestinationConfig) > 0 {
		data, err := sonic.Marshal(c.ParsedDestinationConfig)
		if err != nil {
			return err
		}
		c.DestinationConfig = bifrost.Ptr(string(data))
	} else {
		c.DestinationConfig = nil
	}
	if len(c.ParsedFilters) > 0 {
		data, err := sonic.Marshal(c.ParsedFilters)
		if err != nil {
			return err
		}
		c.Filters = bifrost.Ptr(string(data))
	} else {
		c.Filters = nil
	}
	return nil
}

// AfterFind hook for TableLogExportConfig to deserialize JSON fields.
func (c *TableLogExportConfig) AfterFind(tx *gorm.DB) error {
	if c.DestinationConfig != nil && strings.TrimSpace(*c.DestinationConfig) != "" {
		if err := sonic.Unmarshal([]byte(*c.DestinationConfig), &c.ParsedDestinationConfig); err != nil {
			return err
		}
	}
	if c.Filters != nil && strings.TrimSpace(*c.Filters) != "" {
		if err := sonic.Unmarshal([]byte(*c.Filters), &c.ParsedFilters); err != nil {
			return err
		}
	}
	return nil
}
