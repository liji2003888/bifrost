package tables

import (
	"strings"
	"time"

	"github.com/bytedance/sonic"
	bifrost "github.com/maximhq/bifrost/core"
	"gorm.io/gorm"
)

// TableGuardrailProvider represents an external moderation service configuration in the database.
// Supported provider types: bedrock, azure_content_moderation, patronus, mistral_moderation, pangea
type TableGuardrailProvider struct {
	ID             string    `gorm:"primaryKey;type:varchar(255)" json:"id"`
	Name           string    `gorm:"type:varchar(255);not null" json:"name"`
	ProviderType   string    `gorm:"type:varchar(100);not null" json:"provider_type"` // bedrock, azure_content_moderation, patronus, mistral_moderation, pangea
	Enabled        bool      `gorm:"not null;default:true" json:"enabled"`
	TimeoutSeconds int       `gorm:"not null;default:30" json:"timeout_seconds"`
	Config         *string   `gorm:"type:text" json:"-"`
	ParsedConfig   map[string]any `gorm:"-" json:"config,omitempty"`
	CreatedAt      time.Time `gorm:"index;not null" json:"created_at"`
	UpdatedAt      time.Time `gorm:"index;not null" json:"updated_at"`
}

// TableName for TableGuardrailProvider
func (TableGuardrailProvider) TableName() string { return "guardrail_providers" }

// BeforeSave hook for TableGuardrailProvider to serialize JSON fields
func (g *TableGuardrailProvider) BeforeSave(tx *gorm.DB) error {
	if g.ParsedConfig != nil {
		data, err := sonic.Marshal(g.ParsedConfig)
		if err != nil {
			return err
		}
		g.Config = bifrost.Ptr(string(data))
	} else {
		g.Config = nil
	}
	return nil
}

// AfterFind hook for TableGuardrailProvider to deserialize JSON fields
func (g *TableGuardrailProvider) AfterFind(tx *gorm.DB) error {
	if g.Config != nil && strings.TrimSpace(*g.Config) != "" {
		if err := sonic.Unmarshal([]byte(*g.Config), &g.ParsedConfig); err != nil {
			return err
		}
	}
	return nil
}

// TableGuardrailRule represents a guardrail rule with CEL matching and provider binding in the database.
type TableGuardrailRule struct {
	ID               string         `gorm:"primaryKey;type:varchar(255)" json:"id"`
	Name             string         `gorm:"type:varchar(255);not null" json:"name"`
	Description      string         `gorm:"type:text" json:"description"`
	Enabled          bool           `gorm:"not null;default:true" json:"enabled"`
	ApplyOn          string         `gorm:"type:varchar(50);not null;default:both" json:"apply_on"` // input, output, both
	ProfileIDs       *string        `gorm:"type:text" json:"-"`                                     // JSON array of guardrail provider IDs
	ParsedProfileIDs []string       `gorm:"-" json:"profile_ids,omitempty"`
	SamplingRate     int            `gorm:"not null;default:100" json:"sampling_rate"` // 0-100
	TimeoutSeconds   int            `gorm:"not null;default:60" json:"timeout_seconds"`
	CelExpression    string         `gorm:"type:text" json:"cel_expression"`
	Query            *string        `gorm:"type:text" json:"-"`
	ParsedQuery      map[string]any `gorm:"-" json:"query,omitempty"`
	Scope            string         `gorm:"type:varchar(50);not null;default:global" json:"scope"` // "global" | "team" | "customer" | "virtual_key"
	ScopeID          *string        `gorm:"type:varchar(255)" json:"scope_id"`
	Priority         int            `gorm:"type:int;not null;default:0;index" json:"priority"`
	CreatedAt        time.Time      `gorm:"index;not null" json:"created_at"`
	UpdatedAt        time.Time      `gorm:"index;not null" json:"updated_at"`
}

// TableName for TableGuardrailRule
func (TableGuardrailRule) TableName() string { return "guardrail_rules" }

// BeforeSave hook for TableGuardrailRule to serialize JSON fields
func (g *TableGuardrailRule) BeforeSave(tx *gorm.DB) error {
	if len(g.ParsedProfileIDs) > 0 {
		data, err := sonic.Marshal(g.ParsedProfileIDs)
		if err != nil {
			return err
		}
		g.ProfileIDs = bifrost.Ptr(string(data))
	} else {
		g.ProfileIDs = nil
	}
	if g.ParsedQuery != nil {
		data, err := sonic.Marshal(g.ParsedQuery)
		if err != nil {
			return err
		}
		g.Query = bifrost.Ptr(string(data))
	} else {
		g.Query = nil
	}
	if g.ApplyOn == "" {
		g.ApplyOn = "both"
	}
	if g.Scope == "" {
		g.Scope = "global"
	}
	return nil
}

// AfterFind hook for TableGuardrailRule to deserialize JSON fields
func (g *TableGuardrailRule) AfterFind(tx *gorm.DB) error {
	if g.ProfileIDs != nil && strings.TrimSpace(*g.ProfileIDs) != "" {
		if err := sonic.Unmarshal([]byte(*g.ProfileIDs), &g.ParsedProfileIDs); err != nil {
			return err
		}
	}
	if g.Query != nil && strings.TrimSpace(*g.Query) != "" {
		if err := sonic.Unmarshal([]byte(*g.Query), &g.ParsedQuery); err != nil {
			return err
		}
	}
	if g.ApplyOn == "" {
		g.ApplyOn = "both"
	}
	if g.Scope == "" {
		g.Scope = "global"
	}
	return nil
}
