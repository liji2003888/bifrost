package handlers

import (
	"context"

	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework/configstore"
	configstoreTables "github.com/maximhq/bifrost/framework/configstore/tables"
)

const ClusterConfigReloadEndpoint = "/_cluster/config/reload"

type ClusterConfigScope string

const (
	ClusterConfigScopeClient             ClusterConfigScope = "client"
	ClusterConfigScopeAuth               ClusterConfigScope = "auth"
	ClusterConfigScopeCustomer           ClusterConfigScope = "customer"
	ClusterConfigScopeFramework          ClusterConfigScope = "framework"
	ClusterConfigScopeMCPClient          ClusterConfigScope = "mcp_client"
	ClusterConfigScopeModelConfig        ClusterConfigScope = "model_config"
	ClusterConfigScopePlugin             ClusterConfigScope = "plugin"
	ClusterConfigScopeProviderGovernance ClusterConfigScope = "provider_governance"
	ClusterConfigScopeProxy              ClusterConfigScope = "proxy"
	ClusterConfigScopeProvider           ClusterConfigScope = "provider"
	ClusterConfigScopeRoutingRule        ClusterConfigScope = "routing_rule"
	ClusterConfigScopeTeam               ClusterConfigScope = "team"
	ClusterConfigScopeVirtualKey         ClusterConfigScope = "virtual_key"
)

type ClusterConfigChange struct {
	Scope              ClusterConfigScope                      `json:"scope"`
	CustomerID         string                                  `json:"customer_id,omitempty"`
	CustomerConfig     *configstoreTables.TableCustomer        `json:"customer_config,omitempty"`
	ModelConfigID      string                                  `json:"model_config_id,omitempty"`
	ModelConfig        *configstoreTables.TableModelConfig     `json:"model_config,omitempty"`
	PluginName         string                                  `json:"plugin_name,omitempty"`
	PluginConfig       *configstoreTables.TablePlugin          `json:"plugin_config,omitempty"`
	Provider           schemas.ModelProvider                   `json:"provider,omitempty"`
	ProviderGovernance *configstoreTables.TableProvider        `json:"provider_governance,omitempty"`
	MCPClientID        string                                  `json:"mcp_client_id,omitempty"`
	RoutingRuleID      string                                  `json:"routing_rule_id,omitempty"`
	RoutingRule        *configstoreTables.TableRoutingRule     `json:"routing_rule,omitempty"`
	TeamID             string                                  `json:"team_id,omitempty"`
	VirtualKeyID       string                                  `json:"virtual_key_id,omitempty"`
	Delete             bool                                    `json:"delete,omitempty"`
	FlushSessions      bool                                    `json:"flush_sessions,omitempty"`
	ClientConfig       *configstore.ClientConfig               `json:"client_config,omitempty"`
	AuthConfig         *configstore.AuthConfig                 `json:"auth_config,omitempty"`
	FrameworkConfig    *configstoreTables.TableFrameworkConfig `json:"framework_config,omitempty"`
	MCPClientConfig    *schemas.MCPClientConfig                `json:"mcp_client_config,omitempty"`
	ProxyConfig        *configstoreTables.GlobalProxyConfig    `json:"proxy_config,omitempty"`
	ProviderConfig     *configstore.ProviderConfig             `json:"provider_config,omitempty"`
	TeamConfig         *configstoreTables.TableTeam            `json:"team_config,omitempty"`
	VirtualKeyConfig   *configstoreTables.TableVirtualKey      `json:"virtual_key_config,omitempty"`
}

type ClusterConfigPropagator interface {
	PropagateClusterConfigChange(ctx context.Context, change *ClusterConfigChange) error
}

type ClusterConfigApplier interface {
	ApplyClusterConfigChange(ctx context.Context, change *ClusterConfigChange) error
}
