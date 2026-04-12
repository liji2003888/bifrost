package handlers

import (
	"context"
	"time"

	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework/configstore"
	configstoreTables "github.com/maximhq/bifrost/framework/configstore/tables"
	enterprisecfg "github.com/maximhq/bifrost/transports/bifrost-http/enterprise"
)

const ClusterConfigReloadEndpoint = "/_cluster/config/reload"

type ClusterConfigScope string

const (
	ClusterConfigScopeClient             ClusterConfigScope = "client"
	ClusterConfigScopeAuth               ClusterConfigScope = "auth"
	ClusterConfigScopeCustomer           ClusterConfigScope = "customer"
	ClusterConfigScopeFolder             ClusterConfigScope = "folder"
	ClusterConfigScopeFramework          ClusterConfigScope = "framework"
	ClusterConfigScopeLoadBalancer       ClusterConfigScope = "load_balancer"
	ClusterConfigScopeMCPClient          ClusterConfigScope = "mcp_client"
	ClusterConfigScopeModelConfig        ClusterConfigScope = "model_config"
	ClusterConfigScopeOAuthConfig        ClusterConfigScope = "oauth_config"
	ClusterConfigScopeOAuthToken         ClusterConfigScope = "oauth_token"
	ClusterConfigScopePlugin             ClusterConfigScope = "plugin"
	ClusterConfigScopeProviderGovernance ClusterConfigScope = "provider_governance"
	ClusterConfigScopeProxy              ClusterConfigScope = "proxy"
	ClusterConfigScopeProvider           ClusterConfigScope = "provider"
	ClusterConfigScopePrompt             ClusterConfigScope = "prompt"
	ClusterConfigScopePromptSession      ClusterConfigScope = "prompt_session"
	ClusterConfigScopePromptVersion      ClusterConfigScope = "prompt_version"
	ClusterConfigScopeGuardrailProvider  ClusterConfigScope = "guardrail_provider"
	ClusterConfigScopeGuardrailRule      ClusterConfigScope = "guardrail_rule"
	ClusterConfigScopeRoutingRule        ClusterConfigScope = "routing_rule"
	ClusterConfigScopeRbac               ClusterConfigScope = "rbac"
	ClusterConfigScopeSession            ClusterConfigScope = "session"
	ClusterConfigScopeTeam               ClusterConfigScope = "team"
	ClusterConfigScopeVirtualKey         ClusterConfigScope = "virtual_key"
)

type ClusterConfigChange struct {
	Scope              ClusterConfigScope                      `json:"scope"`
	SourceNodeID       string                                  `json:"source_node_id,omitempty"`
	CustomerID         string                                  `json:"customer_id,omitempty"`
	CustomerConfig     *configstoreTables.TableCustomer        `json:"customer_config,omitempty"`
	FolderID           string                                  `json:"folder_id,omitempty"`
	FolderConfig       *configstoreTables.TableFolder          `json:"folder_config,omitempty"`
	ModelConfigID      string                                  `json:"model_config_id,omitempty"`
	ModelConfig        *configstoreTables.TableModelConfig     `json:"model_config,omitempty"`
	OAuthConfigID      string                                  `json:"oauth_config_id,omitempty"`
	OAuthConfig        *ClusterOAuthConfig                     `json:"oauth_config,omitempty"`
	OAuthTokenID       string                                  `json:"oauth_token_id,omitempty"`
	OAuthToken         *ClusterOAuthToken                      `json:"oauth_token,omitempty"`
	PluginName         string                                  `json:"plugin_name,omitempty"`
	PluginConfig       *configstoreTables.TablePlugin          `json:"plugin_config,omitempty"`
	Provider           schemas.ModelProvider                   `json:"provider,omitempty"`
	ProviderGovernance *configstoreTables.TableProvider        `json:"provider_governance,omitempty"`
	MCPClientID        string                                  `json:"mcp_client_id,omitempty"`
	PromptID           string                                  `json:"prompt_id,omitempty"`
	PromptConfig       *configstoreTables.TablePrompt          `json:"prompt_config,omitempty"`
	PromptSessionID    uint                                    `json:"prompt_session_id,omitempty"`
	PromptSession      *configstoreTables.TablePromptSession   `json:"prompt_session,omitempty"`
	PromptVersionID    uint                                    `json:"prompt_version_id,omitempty"`
	PromptVersion      *configstoreTables.TablePromptVersion   `json:"prompt_version,omitempty"`
	GuardrailProviderID string                                        `json:"guardrail_provider_id,omitempty"`
	GuardrailProvider   *configstoreTables.TableGuardrailProvider     `json:"guardrail_provider,omitempty"`
	GuardrailRuleID     string                                        `json:"guardrail_rule_id,omitempty"`
	GuardrailRule       *configstoreTables.TableGuardrailRule         `json:"guardrail_rule,omitempty"`
	RoutingRuleID      string                                  `json:"routing_rule_id,omitempty"`
	RoutingRule        *configstoreTables.TableRoutingRule     `json:"routing_rule,omitempty"`
	RbacRoleID         string                                  `json:"rbac_role_id,omitempty"`
	RbacRole           *configstoreTables.TableRbacRole        `json:"rbac_role,omitempty"`
	SessionToken       string                                  `json:"session_token,omitempty"`
	SessionConfig      *configstoreTables.SessionsTable        `json:"session_config,omitempty"`
	TeamID             string                                  `json:"team_id,omitempty"`
	VirtualKeyID       string                                  `json:"virtual_key_id,omitempty"`
	Delete             bool                                    `json:"delete,omitempty"`
	FlushSessions      bool                                    `json:"flush_sessions,omitempty"`
	ClientConfig       *configstore.ClientConfig               `json:"client_config,omitempty"`
	AuthConfig         *configstore.AuthConfig                 `json:"auth_config,omitempty"`
	FrameworkConfig    *configstoreTables.TableFrameworkConfig `json:"framework_config,omitempty"`
	LoadBalancerConfig *enterprisecfg.LoadBalancerConfig       `json:"load_balancer_config,omitempty"`
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

type ClusterOAuthConfig struct {
	ID                  string    `json:"id"`
	ClientID            string    `json:"client_id"`
	ClientSecret        string    `json:"client_secret,omitempty"`
	AuthorizeURL        string    `json:"authorize_url"`
	TokenURL            string    `json:"token_url"`
	RegistrationURL     *string   `json:"registration_url,omitempty"`
	RedirectURI         string    `json:"redirect_uri"`
	Scopes              string    `json:"scopes"`
	State               string    `json:"state,omitempty"`
	CodeVerifier        string    `json:"code_verifier,omitempty"`
	CodeChallenge       string    `json:"code_challenge,omitempty"`
	Status              string    `json:"status"`
	TokenID             *string   `json:"token_id,omitempty"`
	ServerURL           string    `json:"server_url"`
	UseDiscovery        bool      `json:"use_discovery"`
	MCPClientConfigJSON *string   `json:"mcp_client_config_json,omitempty"`
	EncryptionStatus    string    `json:"encryption_status,omitempty"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
	ExpiresAt           time.Time `json:"expires_at"`
}

type ClusterOAuthToken struct {
	ID               string     `json:"id"`
	AccessToken      string     `json:"access_token,omitempty"`
	RefreshToken     string     `json:"refresh_token,omitempty"`
	TokenType        string     `json:"token_type"`
	ExpiresAt        time.Time  `json:"expires_at"`
	Scopes           string     `json:"scopes"`
	LastRefreshedAt  *time.Time `json:"last_refreshed_at,omitempty"`
	EncryptionStatus string     `json:"encryption_status,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

func CloneClusterOAuthConfig(config *configstoreTables.TableOauthConfig) *ClusterOAuthConfig {
	if config == nil {
		return nil
	}
	cloned := &ClusterOAuthConfig{
		ID:               config.ID,
		ClientID:         config.ClientID,
		ClientSecret:     config.ClientSecret,
		AuthorizeURL:     config.AuthorizeURL,
		TokenURL:         config.TokenURL,
		RedirectURI:      config.RedirectURI,
		Scopes:           config.Scopes,
		State:            config.State,
		CodeVerifier:     config.CodeVerifier,
		CodeChallenge:    config.CodeChallenge,
		Status:           config.Status,
		ServerURL:        config.ServerURL,
		UseDiscovery:     config.UseDiscovery,
		EncryptionStatus: config.EncryptionStatus,
		CreatedAt:        config.CreatedAt,
		UpdatedAt:        config.UpdatedAt,
		ExpiresAt:        config.ExpiresAt,
	}
	if config.RegistrationURL != nil {
		registrationURL := *config.RegistrationURL
		cloned.RegistrationURL = &registrationURL
	}
	if config.TokenID != nil {
		tokenID := *config.TokenID
		cloned.TokenID = &tokenID
	}
	if config.MCPClientConfigJSON != nil {
		mcpClientConfigJSON := *config.MCPClientConfigJSON
		cloned.MCPClientConfigJSON = &mcpClientConfigJSON
	}
	return cloned
}

func (config *ClusterOAuthConfig) ToTable() *configstoreTables.TableOauthConfig {
	if config == nil {
		return nil
	}
	table := &configstoreTables.TableOauthConfig{
		ID:               config.ID,
		ClientID:         config.ClientID,
		ClientSecret:     config.ClientSecret,
		AuthorizeURL:     config.AuthorizeURL,
		TokenURL:         config.TokenURL,
		RedirectURI:      config.RedirectURI,
		Scopes:           config.Scopes,
		State:            config.State,
		CodeVerifier:     config.CodeVerifier,
		CodeChallenge:    config.CodeChallenge,
		Status:           config.Status,
		ServerURL:        config.ServerURL,
		UseDiscovery:     config.UseDiscovery,
		EncryptionStatus: config.EncryptionStatus,
		CreatedAt:        config.CreatedAt,
		UpdatedAt:        config.UpdatedAt,
		ExpiresAt:        config.ExpiresAt,
	}
	if config.RegistrationURL != nil {
		registrationURL := *config.RegistrationURL
		table.RegistrationURL = &registrationURL
	}
	if config.TokenID != nil {
		tokenID := *config.TokenID
		table.TokenID = &tokenID
	}
	if config.MCPClientConfigJSON != nil {
		mcpClientConfigJSON := *config.MCPClientConfigJSON
		table.MCPClientConfigJSON = &mcpClientConfigJSON
	}
	return table
}

func CloneClusterOAuthToken(token *configstoreTables.TableOauthToken) *ClusterOAuthToken {
	if token == nil {
		return nil
	}
	cloned := &ClusterOAuthToken{
		ID:               token.ID,
		AccessToken:      token.AccessToken,
		RefreshToken:     token.RefreshToken,
		TokenType:        token.TokenType,
		ExpiresAt:        token.ExpiresAt,
		Scopes:           token.Scopes,
		EncryptionStatus: token.EncryptionStatus,
		CreatedAt:        token.CreatedAt,
		UpdatedAt:        token.UpdatedAt,
	}
	if token.LastRefreshedAt != nil {
		lastRefreshedAt := *token.LastRefreshedAt
		cloned.LastRefreshedAt = &lastRefreshedAt
	}
	return cloned
}

func (token *ClusterOAuthToken) ToTable() *configstoreTables.TableOauthToken {
	if token == nil {
		return nil
	}
	table := &configstoreTables.TableOauthToken{
		ID:               token.ID,
		AccessToken:      token.AccessToken,
		RefreshToken:     token.RefreshToken,
		TokenType:        token.TokenType,
		ExpiresAt:        token.ExpiresAt,
		Scopes:           token.Scopes,
		EncryptionStatus: token.EncryptionStatus,
		CreatedAt:        token.CreatedAt,
		UpdatedAt:        token.UpdatedAt,
	}
	if token.LastRefreshedAt != nil {
		lastRefreshedAt := *token.LastRefreshedAt
		table.LastRefreshedAt = &lastRefreshedAt
	}
	return table
}
