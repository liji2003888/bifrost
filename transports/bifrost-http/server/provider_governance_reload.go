package server

import (
	"context"
	"fmt"

	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework/configstore/tables"
)

func (s *BifrostHTTPServer) ReloadProviderGovernance(ctx context.Context, provider schemas.ModelProvider) (*tables.TableProvider, error) {
	if s == nil || s.Config == nil || s.Config.ConfigStore == nil {
		return nil, fmt.Errorf("config store not found")
	}

	providerInfo, err := s.Config.ConfigStore.GetProvider(ctx, provider)
	if err != nil {
		logger.Error("failed to load provider governance: %v", err)
		return nil, err
	}

	return s.syncProviderGovernanceInMemory(ctx, providerInfo)
}

func (s *BifrostHTTPServer) syncProviderGovernanceInMemory(ctx context.Context, providerInfo *tables.TableProvider) (*tables.TableProvider, error) {
	if providerInfo == nil {
		return nil, fmt.Errorf("provider config is required")
	}

	governancePlugin, err := s.getGovernancePlugin()
	if err != nil {
		return nil, err
	}

	updatedProvider := governancePlugin.GetGovernanceStore().UpdateProviderInMemory(providerInfo)
	if updatedProvider == nil {
		return providerInfo, nil
	}

	if updatedProvider.Budget != nil && providerInfo.Budget != nil {
		if updatedProvider.Budget.CurrentUsage != providerInfo.Budget.CurrentUsage {
			if err := s.Config.ConfigStore.UpdateBudgetUsage(ctx, updatedProvider.Budget.ID, updatedProvider.Budget.CurrentUsage); err != nil {
				logger.Error("failed to sync provider governance budget usage to database: %v", err)
			}
		}
	}
	if updatedProvider.RateLimit != nil && providerInfo.RateLimit != nil {
		tokenUsageChanged := updatedProvider.RateLimit.TokenCurrentUsage != providerInfo.RateLimit.TokenCurrentUsage
		requestUsageChanged := updatedProvider.RateLimit.RequestCurrentUsage != providerInfo.RateLimit.RequestCurrentUsage
		if tokenUsageChanged || requestUsageChanged {
			if err := s.Config.ConfigStore.UpdateRateLimitUsage(ctx, updatedProvider.RateLimit.ID, updatedProvider.RateLimit.TokenCurrentUsage, updatedProvider.RateLimit.RequestCurrentUsage); err != nil {
				logger.Error("failed to sync provider governance rate limit usage to database: %v", err)
			}
		}
	}

	return updatedProvider, nil
}
