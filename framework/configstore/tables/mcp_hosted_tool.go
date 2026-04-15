package tables

import (
	"encoding/json"
	"time"

	"github.com/bytedance/sonic"
	"github.com/maximhq/bifrost/core/schemas"
	"gorm.io/gorm"
)

type MCPHostedToolAuthMode string

const (
	MCPHostedToolAuthModeNone              MCPHostedToolAuthMode = "none"
	MCPHostedToolAuthModeBearerPassthrough MCPHostedToolAuthMode = "bearer_passthrough"
	MCPHostedToolAuthModeHeaderPassthrough MCPHostedToolAuthMode = "header_passthrough"
)

type MCPHostedToolAuthProfile struct {
	Mode           MCPHostedToolAuthMode `json:"mode"`
	HeaderMappings map[string]string     `json:"header_mappings,omitempty"`
}

type MCPHostedToolExecutionProfile struct {
	TimeoutSeconds       *int `json:"timeout_seconds,omitempty"`
	MaxResponseBodyBytes *int `json:"max_response_body_bytes,omitempty"`
}

type MCPHostedToolExecutionResult struct {
	Output         string         `json:"output"`
	StatusCode     int            `json:"status_code"`
	LatencyMS      int64          `json:"latency_ms"`
	ResponseBytes  int            `json:"response_bytes"`
	ContentType    string         `json:"content_type,omitempty"`
	ResolvedURL    string         `json:"resolved_url,omitempty"`
	Truncated      bool           `json:"truncated,omitempty"`
	ResponseSchema map[string]any `json:"response_schema,omitempty"`
}

// TableMCPHostedTool stores an HTTP API endpoint that Bifrost hosts as an
// in-process MCP tool. These definitions are cluster-safe because they are
// persisted in ConfigStore and re-registered on startup.
type TableMCPHostedTool struct {
	ID                   uint    `gorm:"primaryKey;autoIncrement" json:"id"`
	ToolID               string  `gorm:"type:varchar(255);uniqueIndex;not null" json:"tool_id"`
	Name                 string  `gorm:"type:varchar(255);uniqueIndex;not null" json:"name"`
	Description          *string `gorm:"type:text" json:"description,omitempty"`
	Method               string  `gorm:"type:varchar(16);not null" json:"method"`
	URL                  string  `gorm:"type:text;not null" json:"url"`
	HeadersJSON          string  `gorm:"type:text" json:"-"`
	QueryParamsJSON      string  `gorm:"type:text" json:"-"`
	AuthProfileJSON      string  `gorm:"type:text" json:"-"`
	ExecutionProfileJSON string  `gorm:"type:text" json:"-"`
	ResponseSchemaJSON   string  `gorm:"type:text" json:"-"`
	ResponseExamplesJSON string  `gorm:"type:text" json:"-"`
	BodyTemplate         *string `gorm:"type:text" json:"body_template,omitempty"`
	ResponseJSONPath     *string `gorm:"type:text" json:"response_json_path,omitempty"`
	ResponseTemplate     *string `gorm:"type:text" json:"response_template,omitempty"`
	ToolSchemaJSON       string  `gorm:"type:text;not null" json:"-"`

	CreatedAt time.Time `gorm:"index;not null" json:"created_at"`
	UpdatedAt time.Time `gorm:"index;not null" json:"updated_at"`

	Headers          map[string]string              `gorm:"-" json:"headers,omitempty"`
	QueryParams      map[string]string              `gorm:"-" json:"query_params,omitempty"`
	AuthProfile      *MCPHostedToolAuthProfile      `gorm:"-" json:"auth_profile,omitempty"`
	ExecutionProfile *MCPHostedToolExecutionProfile `gorm:"-" json:"execution_profile,omitempty"`
	ResponseSchema   map[string]any                 `gorm:"-" json:"response_schema,omitempty"`
	ResponseExamples []any                          `gorm:"-" json:"response_examples,omitempty"`
	ToolSchema       schemas.ChatTool               `gorm:"-" json:"tool_schema"`
}

func (TableMCPHostedTool) TableName() string { return "config_mcp_hosted_tools" }

func (t *TableMCPHostedTool) BeforeSave(_ *gorm.DB) error {
	if t.Headers != nil {
		data, err := json.Marshal(t.Headers)
		if err != nil {
			return err
		}
		t.HeadersJSON = string(data)
	} else {
		t.HeadersJSON = "{}"
	}
	if t.QueryParams != nil {
		data, err := json.Marshal(t.QueryParams)
		if err != nil {
			return err
		}
		t.QueryParamsJSON = string(data)
	} else {
		t.QueryParamsJSON = "{}"
	}
	if t.AuthProfile != nil {
		data, err := json.Marshal(t.AuthProfile)
		if err != nil {
			return err
		}
		t.AuthProfileJSON = string(data)
	} else {
		t.AuthProfileJSON = ""
	}
	if t.ExecutionProfile != nil {
		data, err := json.Marshal(t.ExecutionProfile)
		if err != nil {
			return err
		}
		t.ExecutionProfileJSON = string(data)
	} else {
		t.ExecutionProfileJSON = ""
	}
	if t.ResponseSchema != nil {
		data, err := json.Marshal(t.ResponseSchema)
		if err != nil {
			return err
		}
		t.ResponseSchemaJSON = string(data)
	} else {
		t.ResponseSchemaJSON = ""
	}
	if t.ResponseExamples != nil {
		data, err := json.Marshal(t.ResponseExamples)
		if err != nil {
			return err
		}
		t.ResponseExamplesJSON = string(data)
	} else {
		t.ResponseExamplesJSON = ""
	}

	data, err := json.Marshal(t.ToolSchema)
	if err != nil {
		return err
	}
	t.ToolSchemaJSON = string(data)
	return nil
}

func (t *TableMCPHostedTool) AfterFind(_ *gorm.DB) error {
	if t.HeadersJSON != "" {
		if err := sonic.Unmarshal([]byte(t.HeadersJSON), &t.Headers); err != nil {
			return err
		}
	}
	if t.QueryParamsJSON != "" {
		if err := sonic.Unmarshal([]byte(t.QueryParamsJSON), &t.QueryParams); err != nil {
			return err
		}
	}
	if t.AuthProfileJSON != "" {
		var profile MCPHostedToolAuthProfile
		if err := sonic.Unmarshal([]byte(t.AuthProfileJSON), &profile); err != nil {
			return err
		}
		t.AuthProfile = &profile
	}
	if t.ExecutionProfileJSON != "" {
		var profile MCPHostedToolExecutionProfile
		if err := sonic.Unmarshal([]byte(t.ExecutionProfileJSON), &profile); err != nil {
			return err
		}
		t.ExecutionProfile = &profile
	}
	if t.ResponseSchemaJSON != "" {
		if err := sonic.Unmarshal([]byte(t.ResponseSchemaJSON), &t.ResponseSchema); err != nil {
			return err
		}
	}
	if t.ResponseExamplesJSON != "" {
		if err := sonic.Unmarshal([]byte(t.ResponseExamplesJSON), &t.ResponseExamples); err != nil {
			return err
		}
	}
	if t.ToolSchemaJSON != "" {
		if err := sonic.Unmarshal([]byte(t.ToolSchemaJSON), &t.ToolSchema); err != nil {
			return err
		}
	}
	return nil
}
