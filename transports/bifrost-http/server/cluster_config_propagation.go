package server

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	bifrost "github.com/maximhq/bifrost/core"
	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework"
	"github.com/maximhq/bifrost/framework/configstore"
	configstoreTables "github.com/maximhq/bifrost/framework/configstore/tables"
	"github.com/maximhq/bifrost/framework/modelcatalog"
	"github.com/maximhq/bifrost/plugins/litellmcompat"
	"github.com/maximhq/bifrost/transports/bifrost-http/handlers"
	"github.com/maximhq/bifrost/transports/bifrost-http/lib"
)

const clusterConfigPropagationTimeout = 3 * time.Second

func (s *BifrostHTTPServer) PropagateClusterConfigChange(ctx context.Context, change *handlers.ClusterConfigChange) error {
	if s == nil || change == nil {
		return nil
	}
	if change.SourceNodeID == "" && s.ClusterService != nil {
		change.SourceNodeID = s.ClusterService.NodeID()
	}
	s.broadcastClusterConfigStoreUpdates(change)
	if s.ClusterService == nil {
		return nil
	}

	addresses := s.ClusterService.FanoutAddresses(ctx)
	if len(addresses) == 0 {
		return nil
	}

	var (
		wg    sync.WaitGroup
		errMu sync.Mutex
		errs  []error
	)

	for _, address := range addresses {
		wg.Add(1)
		go func(address string) {
			defer wg.Done()

			requestCtx := ctx
			if requestCtx == nil {
				requestCtx = context.Background()
			}
			requestCtx, cancel := context.WithTimeout(requestCtx, clusterConfigPropagationTimeout)
			defer cancel()

			if err := s.ClusterService.PostJSON(requestCtx, address, handlers.ClusterConfigReloadEndpoint, change, nil); err != nil {
				errMu.Lock()
				errs = append(errs, fmt.Errorf("%s: %w", address, err))
				errMu.Unlock()
			}
		}(address)
	}

	wg.Wait()
	return errors.Join(errs...)
}

func (s *BifrostHTTPServer) ApplyClusterConfigChange(ctx context.Context, change *handlers.ClusterConfigChange) error {
	if change == nil {
		return fmt.Errorf("cluster config change is required")
	}

	var err error
	switch change.Scope {
	case handlers.ClusterConfigScopeClient:
		err = s.ApplyClusterClientConfig(ctx, change.ClientConfig)
	case handlers.ClusterConfigScopeAuth:
		err = s.ApplyClusterAuthConfig(ctx, change.AuthConfig, change.FlushSessions)
	case handlers.ClusterConfigScopeCustomer:
		err = s.ApplyClusterCustomerConfig(ctx, change.CustomerID, change.CustomerConfig, change.Delete)
	case handlers.ClusterConfigScopeFolder:
		err = s.ApplyClusterFolderConfig(ctx, change.FolderID, change.FolderConfig, change.Delete)
	case handlers.ClusterConfigScopeFramework:
		err = s.ApplyClusterFrameworkConfig(ctx, change.FrameworkConfig)
	case handlers.ClusterConfigScopeLoadBalancer:
		err = s.ApplyClusterLoadBalancerConfig(ctx, change.LoadBalancerConfig)
	case handlers.ClusterConfigScopeMCPClient:
		err = s.ApplyClusterMCPClientConfig(ctx, change.MCPClientID, change.MCPClientConfig, change.Delete)
	case handlers.ClusterConfigScopeModelConfig:
		err = s.ApplyClusterModelConfig(ctx, change.ModelConfigID, change.ModelConfig, change.Delete)
	case handlers.ClusterConfigScopeOAuthConfig:
		err = s.ApplyClusterOAuthConfig(ctx, change.OAuthConfigID, change.OAuthConfig)
	case handlers.ClusterConfigScopeOAuthToken:
		err = s.ApplyClusterOAuthToken(ctx, change.OAuthTokenID, change.OAuthToken, change.Delete)
	case handlers.ClusterConfigScopePlugin:
		err = s.ApplyClusterPluginConfig(ctx, change.PluginName, change.PluginConfig, change.Delete)
	case handlers.ClusterConfigScopeProviderGovernance:
		err = s.ApplyClusterProviderGovernanceConfig(ctx, change.Provider, change.ProviderGovernance)
	case handlers.ClusterConfigScopeProxy:
		err = s.ApplyClusterProxyConfig(ctx, change.ProxyConfig)
	case handlers.ClusterConfigScopeProvider:
		if change.Provider == "" {
			return fmt.Errorf("provider is required for provider cluster config changes")
		}
		if change.Delete {
			err = s.RemoveProvider(ctx, change.Provider)
			break
		}
		err = s.ApplyClusterProviderConfig(ctx, change.Provider, change.ProviderConfig)
	case handlers.ClusterConfigScopePrompt:
		err = s.ApplyClusterPromptConfig(ctx, change.PromptID, change.PromptConfig, change.Delete)
	case handlers.ClusterConfigScopePromptSession:
		err = s.ApplyClusterPromptSessionConfig(ctx, change.PromptSessionID, change.PromptSession, change.Delete)
	case handlers.ClusterConfigScopePromptVersion:
		err = s.ApplyClusterPromptVersionConfig(ctx, change.PromptVersionID, change.PromptVersion, change.Delete)
	case handlers.ClusterConfigScopeRoutingRule:
		err = s.ApplyClusterRoutingRuleConfig(ctx, change.RoutingRuleID, change.RoutingRule, change.Delete)
	case handlers.ClusterConfigScopeSession:
		err = s.ApplyClusterSessionConfig(ctx, change.SessionToken, change.SessionConfig, change.Delete)
	case handlers.ClusterConfigScopeTeam:
		err = s.ApplyClusterTeamConfig(ctx, change.TeamID, change.TeamConfig, change.Delete)
	case handlers.ClusterConfigScopeVirtualKey:
		err = s.ApplyClusterVirtualKeyConfig(ctx, change.VirtualKeyID, change.VirtualKeyConfig, change.Delete)
	default:
		return fmt.Errorf("unsupported cluster config scope: %s", change.Scope)
	}

	if err == nil {
		s.broadcastClusterConfigStoreUpdates(change)
	}
	return err
}

func (s *BifrostHTTPServer) broadcastClusterConfigStoreUpdates(change *handlers.ClusterConfigChange) {
	if s == nil || s.WebSocketHandler == nil || change == nil {
		return
	}
	tags := clusterConfigChangeTags(change)
	if len(tags) == 0 {
		return
	}
	s.WebSocketHandler.BroadcastUpdatesToClients(tags)
}

func clusterConfigChangeTags(change *handlers.ClusterConfigChange) []string {
	if change == nil {
		return nil
	}

	var tags []string
	switch change.Scope {
	case handlers.ClusterConfigScopeClient, handlers.ClusterConfigScopeAuth, handlers.ClusterConfigScopeFramework, handlers.ClusterConfigScopeProxy:
		tags = append(tags, "Config")
	case handlers.ClusterConfigScopeLoadBalancer:
		tags = append(tags, "Config", "AdaptiveRouting")
	case handlers.ClusterConfigScopeCustomer, handlers.ClusterConfigScopeTeam, handlers.ClusterConfigScopeVirtualKey, handlers.ClusterConfigScopeModelConfig, handlers.ClusterConfigScopeProviderGovernance, handlers.ClusterConfigScopeRoutingRule:
		tags = append(tags, "Customers", "Teams", "VirtualKeys", "Budgets", "RateLimits", "UsageStats", "DebugStats", "HealthCheck", "ModelConfigs", "ProviderGovernance", "RoutingRules")
	case handlers.ClusterConfigScopeMCPClient:
		tags = append(tags, "MCPClients", "OAuth2Config")
	case handlers.ClusterConfigScopeOAuthConfig, handlers.ClusterConfigScopeOAuthToken:
		tags = append(tags, "OAuth2Config", "MCPClients")
	case handlers.ClusterConfigScopePlugin:
		tags = append(tags, "Plugins")
	case handlers.ClusterConfigScopeProvider:
		tags = append(tags, "Providers", "DBKeys", "Models", "BaseModels", "ProviderGovernance")
	case handlers.ClusterConfigScopePrompt:
		tags = append(tags, "Prompts", "Folders", "Versions", "Sessions")
	case handlers.ClusterConfigScopeFolder:
		tags = append(tags, "Folders", "Prompts")
	case handlers.ClusterConfigScopePromptVersion:
		tags = append(tags, "Versions", "Prompts")
	case handlers.ClusterConfigScopePromptSession:
		tags = append(tags, "Sessions", "Prompts", "Versions")
	case handlers.ClusterConfigScopeSession:
		tags = append(tags, "SessionState")
	}

	tags = append(tags, "ClusterNodes")
	return dedupeStoreUpdateTags(tags)
}

func dedupeStoreUpdateTags(tags []string) []string {
	if len(tags) == 0 {
		return nil
	}
	result := make([]string, 0, len(tags))
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" || slices.Contains(result, tag) {
			continue
		}
		result = append(result, tag)
	}
	return result
}

func (s *BifrostHTTPServer) ApplyClusterAuthConfig(ctx context.Context, cfg *configstore.AuthConfig, flushSessions bool) error {
	if s == nil || s.Config == nil || s.Config.ConfigStore == nil {
		return fmt.Errorf("config store not found")
	}
	if cfg == nil {
		return fmt.Errorf("auth config is required")
	}

	if err := s.UpdateAuthConfig(ctx, cfg); err != nil {
		return fmt.Errorf("failed to persist auth config: %w", err)
	}
	if flushSessions {
		if err := s.Config.ConfigStore.FlushSessions(ctx); err != nil {
			return fmt.Errorf("failed to flush sessions after auth config update: %w", err)
		}
	}
	return nil
}

func (s *BifrostHTTPServer) ApplyClusterClientConfig(ctx context.Context, cfg *configstore.ClientConfig) error {
	if s == nil || s.Config == nil || s.Config.ConfigStore == nil {
		return fmt.Errorf("config store not found")
	}
	if cfg == nil {
		return fmt.Errorf("client config is required")
	}

	previousEnableLiteLLM := false
	if s.Config.ClientConfig != nil {
		previousEnableLiteLLM = s.Config.ClientConfig.EnableLiteLLMFallbacks
	}

	if err := s.Config.ConfigStore.UpdateClientConfig(ctx, cfg); err != nil {
		return fmt.Errorf("failed to persist client config: %w", err)
	}
	if err := s.ReloadClientConfigFromConfigStore(ctx); err != nil {
		return err
	}
	if err := s.ReloadHeaderFilterConfig(ctx, s.Config.ClientConfig.HeaderFilterConfig); err != nil {
		return err
	}
	if s.Config.MCPConfig != nil {
		if err := s.UpdateMCPToolManagerConfig(ctx, s.Config.ClientConfig.MCPAgentDepth, s.Config.ClientConfig.MCPToolExecutionTimeout, s.Config.ClientConfig.MCPCodeModeBindingLevel); err != nil {
			return err
		}
	}

	currentEnableLiteLLM := s.Config.ClientConfig.EnableLiteLLMFallbacks
	if currentEnableLiteLLM != previousEnableLiteLLM {
		if currentEnableLiteLLM {
			if err := s.ReloadPlugin(ctx, "litellmcompat", nil, &litellmcompat.Config{Enabled: true}, nil, nil); err != nil {
				return err
			}
		} else {
			disabledCtx := context.WithValue(ctx, handlers.PluginDisabledKey, true)
			if err := s.RemovePlugin(disabledCtx, "litellmcompat"); err != nil {
				return err
			}
		}
	}

	return nil
}

func (s *BifrostHTTPServer) ReloadFrameworkConfigFromConfigStore(ctx context.Context) error {
	if s == nil || s.Config == nil || s.Config.ConfigStore == nil {
		return fmt.Errorf("config store not found")
	}

	frameworkConfig, err := s.Config.ConfigStore.GetFrameworkConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to get framework config: %w", err)
	}
	normalizedFrameworkConfig, pricingConfig, _ := lib.ResolveFrameworkPricingConfig(frameworkConfig, nil)
	if normalizedFrameworkConfig != nil {
		frameworkConfig = normalizedFrameworkConfig
	}
	s.Config.FrameworkConfig = &framework.FrameworkConfig{
		Pricing: pricingConfig,
	}
	if s.Config.ModelCatalog == nil && pricingConfig != nil {
		modelCatalog, initErr := modelcatalog.Init(ctx, pricingConfig, s.Config.ConfigStore, nil, logger)
		if initErr != nil {
			return fmt.Errorf("failed to initialize pricing manager: %w", initErr)
		}
		s.Config.ModelCatalog = modelCatalog
	}
	return s.ReloadPricingManager(ctx)
}

func (s *BifrostHTTPServer) ApplyClusterFrameworkConfig(ctx context.Context, cfg *configstoreTables.TableFrameworkConfig) error {
	if s == nil || s.Config == nil || s.Config.ConfigStore == nil {
		return fmt.Errorf("config store not found")
	}
	if cfg == nil {
		return fmt.Errorf("framework config is required")
	}
	if err := s.Config.ConfigStore.UpdateFrameworkConfig(ctx, cfg); err != nil {
		return fmt.Errorf("failed to persist framework config: %w", err)
	}
	return s.ReloadFrameworkConfigFromConfigStore(ctx)
}

func (s *BifrostHTTPServer) ApplyClusterMCPClientConfig(ctx context.Context, id string, cfg *schemas.MCPClientConfig, deleteClient bool) error {
	if s == nil || s.Config == nil || s.Config.ConfigStore == nil {
		return fmt.Errorf("config store not found")
	}
	if strings.TrimSpace(id) == "" && cfg != nil {
		id = strings.TrimSpace(cfg.ID)
	}
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("mcp client id is required")
	}

	if deleteClient {
		if err := s.Config.ConfigStore.DeleteMCPClientConfig(ctx, id); err != nil {
			return fmt.Errorf("failed to delete mcp client config: %w", err)
		}
		return s.RemoveMCPClient(ctx, id)
	}
	if cfg == nil {
		return fmt.Errorf("mcp client config is required")
	}

	normalized := *cfg
	normalized.ID = id
	if _, err := s.Config.GetMCPClient(id); err != nil {
		if err := s.Config.ConfigStore.CreateMCPClientConfig(ctx, &normalized); err != nil {
			return fmt.Errorf("failed to create mcp client config: %w", err)
		}
		if err := s.AddMCPClient(ctx, &normalized); err != nil {
			if rollbackErr := s.Config.ConfigStore.DeleteMCPClientConfig(ctx, id); rollbackErr != nil {
				logger.Warn("failed to rollback mcp client config after add error: %v", rollbackErr)
			}
			return fmt.Errorf("failed to add mcp client: %w", err)
		}
		return nil
	}

	oldDBConfig, err := s.Config.ConfigStore.GetMCPClientByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get existing mcp client config: %w", err)
	}
	if err := s.Config.ConfigStore.UpdateMCPClientConfig(ctx, id, clusterMCPClientToTable(&normalized)); err != nil {
		return fmt.Errorf("failed to update mcp client config: %w", err)
	}
	if err := s.UpdateMCPClient(ctx, id, &normalized); err != nil {
		if oldDBConfig != nil {
			if rollbackErr := s.Config.ConfigStore.UpdateMCPClientConfig(ctx, id, oldDBConfig); rollbackErr != nil {
				logger.Warn("failed to rollback mcp client config after update error: %v", rollbackErr)
			}
		}
		return fmt.Errorf("failed to update mcp client: %w", err)
	}
	return nil
}

func clusterMCPClientToTable(cfg *schemas.MCPClientConfig) *configstoreTables.TableMCPClient {
	if cfg == nil {
		return nil
	}

	return &configstoreTables.TableMCPClient{
		ClientID:           cfg.ID,
		Name:               cfg.Name,
		IsCodeModeClient:   cfg.IsCodeModeClient,
		ConnectionType:     string(cfg.ConnectionType),
		ConnectionString:   cfg.ConnectionString,
		StdioConfig:        cfg.StdioConfig,
		ToolsToExecute:     cfg.ToolsToExecute,
		ToolsToAutoExecute: cfg.ToolsToAutoExecute,
		Headers:            cfg.Headers,
		IsPingAvailable:    bifrost.Ptr(cfg.IsPingAvailable),
		ToolPricing:        cfg.ToolPricing,
		ToolSyncInterval:   int(cfg.ToolSyncInterval.Minutes()),
		AuthType:           string(cfg.AuthType),
		OauthConfigID:      cfg.OauthConfigID,
	}
}

func (s *BifrostHTTPServer) ReloadProxyConfigFromConfigStore(ctx context.Context) error {
	if s == nil || s.Config == nil || s.Config.ConfigStore == nil {
		return fmt.Errorf("config store not found")
	}

	proxyConfig, err := s.Config.ConfigStore.GetProxyConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to get proxy config: %w", err)
	}
	if proxyConfig == nil {
		proxyConfig = &configstoreTables.GlobalProxyConfig{}
	}
	return s.ReloadProxyConfig(ctx, proxyConfig)
}

func (s *BifrostHTTPServer) ApplyClusterProxyConfig(ctx context.Context, cfg *configstoreTables.GlobalProxyConfig) error {
	if s == nil || s.Config == nil || s.Config.ConfigStore == nil {
		return fmt.Errorf("config store not found")
	}
	if cfg == nil {
		return fmt.Errorf("proxy config is required")
	}
	if err := s.Config.ConfigStore.UpdateProxyConfig(ctx, cfg); err != nil {
		return fmt.Errorf("failed to persist proxy config: %w", err)
	}
	return s.ReloadProxyConfigFromConfigStore(ctx)
}

func (s *BifrostHTTPServer) ApplyClusterProviderConfig(ctx context.Context, provider schemas.ModelProvider, cfg *configstore.ProviderConfig) error {
	if s == nil || s.Config == nil || s.Config.ConfigStore == nil {
		return fmt.Errorf("config store not found")
	}
	if cfg == nil {
		return fmt.Errorf("provider config is required")
	}

	if _, err := s.Config.GetProviderConfigRaw(provider); err != nil {
		if !errors.Is(err, lib.ErrNotFound) {
			return err
		}
		if addErr := s.Config.AddProvider(ctx, provider, *cfg); addErr != nil && !errors.Is(addErr, lib.ErrAlreadyExists) {
			return addErr
		}
	}

	if err := s.Config.UpdateProviderConfig(ctx, provider, *cfg); err != nil {
		if errors.Is(err, lib.ErrNotFound) {
			if addErr := s.Config.AddProvider(ctx, provider, *cfg); addErr != nil && !errors.Is(addErr, lib.ErrAlreadyExists) {
				return addErr
			}
			if retryErr := s.Config.UpdateProviderConfig(ctx, provider, *cfg); retryErr != nil {
				return retryErr
			}
		} else {
			return err
		}
	}

	_, err := s.ReloadProvider(ctx, provider)
	return err
}
