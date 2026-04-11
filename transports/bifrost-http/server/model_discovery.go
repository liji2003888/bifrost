package server

import (
	"context"
	"errors"
	"strings"

	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework/configstore"
	"github.com/maximhq/bifrost/framework/configstore/tables"
)

func hasStaticModelsConfigured(keys []schemas.Key) bool {
	for _, key := range keys {
		if len(key.Models) > 0 {
			return true
		}
	}
	return false
}

func hasStaticModelsConfiguredInTableKeys(keys []tables.TableKey) bool {
	for _, key := range keys {
		if len(key.Models) > 0 {
			return true
		}
	}
	return false
}

func isDashScopeCompatibleModeBaseURL(baseURL string) bool {
	normalized := strings.ToLower(strings.TrimSpace(baseURL))
	if normalized == "" {
		return false
	}
	return strings.Contains(normalized, "dashscope.aliyuncs.com/compatible-mode") ||
		strings.Contains(normalized, "dashscope-intl.aliyuncs.com/compatible-mode")
}

type modelDiscoveryConfig struct {
	customProviderConfig *schemas.CustomProviderConfig
	networkConfig        *schemas.NetworkConfig
	hasStaticModels      bool
}

func modelDiscoveryConfigFromProviderConfig(config configstore.ProviderConfig) modelDiscoveryConfig {
	return modelDiscoveryConfig{
		customProviderConfig: config.CustomProviderConfig,
		networkConfig:        config.NetworkConfig,
		hasStaticModels:      hasStaticModelsConfigured(config.Keys),
	}
}

func modelDiscoveryConfigFromTableProvider(provider *tables.TableProvider) modelDiscoveryConfig {
	if provider == nil {
		return modelDiscoveryConfig{}
	}
	return modelDiscoveryConfig{
		customProviderConfig: provider.CustomProviderConfig,
		networkConfig:        provider.NetworkConfig,
		hasStaticModels:      hasStaticModelsConfiguredInTableKeys(provider.Keys),
	}
}

func shouldSkipActiveModelDiscovery(config modelDiscoveryConfig) bool {
	if config.customProviderConfig == nil {
		return false
	}
	return !config.customProviderConfig.IsOperationAllowed(schemas.ListModelsRequest)
}

func shouldFallbackToStaticModelCatalog(config modelDiscoveryConfig, bifrostErr *schemas.BifrostError) bool {
	if bifrostErr == nil || bifrostErr.StatusCode == nil || *bifrostErr.StatusCode != 404 {
		return false
	}
	if bifrostErr.ExtraFields.RequestType != schemas.ListModelsRequest {
		return false
	}
	if config.customProviderConfig == nil {
		return false
	}
	if isDashScopeCompatibleModeBaseURL(configBaseURL(config.networkConfig)) {
		return true
	}
	return config.hasStaticModels
}

func configBaseURL(networkConfig *schemas.NetworkConfig) string {
	if networkConfig == nil {
		return ""
	}
	return networkConfig.BaseURL
}

func (s *BifrostHTTPServer) clearModelDiscoveryStatus(ctx context.Context, provider schemas.ModelProvider) {
	if s == nil || s.Config == nil {
		return
	}

	s.Config.Mu.Lock()
	providerConfig, exists := s.Config.Providers[provider]
	if !exists {
		s.Config.Mu.Unlock()
		return
	}

	keys := append([]schemas.Key(nil), providerConfig.Keys...)
	providerConfig.Status = ""
	providerConfig.Description = ""
	for i := range providerConfig.Keys {
		providerConfig.Keys[i].Status = ""
		providerConfig.Keys[i].Description = ""
	}
	s.Config.Providers[provider] = providerConfig
	s.Config.Mu.Unlock()

	if s.Config.ConfigStore == nil {
		return
	}

	if len(keys) == 0 {
		if err := s.Config.ConfigStore.UpdateStatus(ctx, provider, "", "", ""); err != nil && !errors.Is(err, configstore.ErrNotFound) {
			logger.Warn("failed to clear model discovery status for provider %s: %v", provider, err)
		}
		return
	}

	for _, key := range keys {
		if strings.TrimSpace(key.ID) == "" {
			continue
		}
		if err := s.Config.ConfigStore.UpdateStatus(ctx, provider, key.ID, "", ""); err != nil && !errors.Is(err, configstore.ErrNotFound) {
			logger.Warn("failed to clear model discovery status for key %s of provider %s: %v", key.ID, provider, err)
		}
	}
}

func (s *BifrostHTTPServer) handleStaticModelCatalogFallback(
	ctx context.Context,
	provider schemas.ModelProvider,
	config modelDiscoveryConfig,
	bifrostErr *schemas.BifrostError,
) bool {
	if !shouldFallbackToStaticModelCatalog(config, bifrostErr) {
		return false
	}

	s.clearModelDiscoveryStatus(ctx, provider)
	logger.Info(
		"provider %s does not expose a compatible list-models endpoint; using static model configuration fallback",
		provider,
	)
	return true
}
