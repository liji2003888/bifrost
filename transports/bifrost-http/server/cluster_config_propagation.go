package server

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

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
	if s == nil || s.ClusterService == nil || change == nil {
		return nil
	}

	peers := s.ClusterService.PeerStatuses()
	if len(peers) == 0 {
		return nil
	}

	var (
		wg    sync.WaitGroup
		errMu sync.Mutex
		errs  []error
	)

	for _, peer := range peers {
		if peer.Address == "" {
			continue
		}
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
		}(peer.Address)
	}

	wg.Wait()
	return errors.Join(errs...)
}

func (s *BifrostHTTPServer) ApplyClusterConfigChange(ctx context.Context, change *handlers.ClusterConfigChange) error {
	if change == nil {
		return fmt.Errorf("cluster config change is required")
	}

	switch change.Scope {
	case handlers.ClusterConfigScopeClient:
		return s.ApplyClusterClientConfig(ctx, change.ClientConfig)
	case handlers.ClusterConfigScopeFramework:
		return s.ApplyClusterFrameworkConfig(ctx, change.FrameworkConfig)
	case handlers.ClusterConfigScopeProxy:
		return s.ApplyClusterProxyConfig(ctx, change.ProxyConfig)
	case handlers.ClusterConfigScopeProvider:
		if change.Provider == "" {
			return fmt.Errorf("provider is required for provider cluster config changes")
		}
		if change.Delete {
			return s.RemoveProvider(ctx, change.Provider)
		}
		return s.ApplyClusterProviderConfig(ctx, change.Provider, change.ProviderConfig)
	default:
		return fmt.Errorf("unsupported cluster config scope: %s", change.Scope)
	}
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
