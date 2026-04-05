package server

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/maximhq/bifrost/framework/configstore"
	configstoreTables "github.com/maximhq/bifrost/framework/configstore/tables"
	"github.com/maximhq/bifrost/transports/bifrost-http/lib"
	"gorm.io/gorm"
)

func (s *BifrostHTTPServer) ApplyClusterCustomerConfig(ctx context.Context, id string, cfg *configstoreTables.TableCustomer, deleteCustomer bool) error {
	if s == nil || s.Config == nil || s.Config.ConfigStore == nil {
		return fmt.Errorf("config store not found")
	}

	id = normalizeClusterGovernanceID(id, func() string {
		if cfg == nil {
			return ""
		}
		return cfg.ID
	}(), "customer")
	if id == "" {
		return fmt.Errorf("customer id is required")
	}

	if deleteCustomer {
		if err := s.Config.ConfigStore.DeleteCustomer(ctx, id); err != nil && !errors.Is(err, configstore.ErrNotFound) {
			return fmt.Errorf("failed to delete customer config: %w", err)
		}
		return s.RemoveCustomer(ctx, id)
	}
	if cfg == nil {
		return fmt.Errorf("customer config is required")
	}
	if err := ensureClusterBudgetPayload("customer", id, cfg.BudgetID, cfg.Budget); err != nil {
		return err
	}
	if err := ensureClusterRateLimitPayload("customer", id, cfg.RateLimitID, cfg.RateLimit); err != nil {
		return err
	}
	if err := s.applyClusterCustomerRecord(ctx, cfg); err != nil {
		return err
	}
	_, err := s.ReloadCustomer(ctx, id)
	return err
}

func (s *BifrostHTTPServer) ApplyClusterTeamConfig(ctx context.Context, id string, cfg *configstoreTables.TableTeam, deleteTeam bool) error {
	if s == nil || s.Config == nil || s.Config.ConfigStore == nil {
		return fmt.Errorf("config store not found")
	}

	id = normalizeClusterGovernanceID(id, func() string {
		if cfg == nil {
			return ""
		}
		return cfg.ID
	}(), "team")
	if id == "" {
		return fmt.Errorf("team id is required")
	}

	if deleteTeam {
		if err := s.Config.ConfigStore.DeleteTeam(ctx, id); err != nil && !errors.Is(err, configstore.ErrNotFound) {
			return fmt.Errorf("failed to delete team config: %w", err)
		}
		return s.RemoveTeam(ctx, id)
	}
	if cfg == nil {
		return fmt.Errorf("team config is required")
	}
	if err := s.waitForClusterCustomer(ctx, cfg.CustomerID); err != nil {
		return err
	}
	if err := ensureClusterBudgetPayload("team", id, cfg.BudgetID, cfg.Budget); err != nil {
		return err
	}
	if err := ensureClusterRateLimitPayload("team", id, cfg.RateLimitID, cfg.RateLimit); err != nil {
		return err
	}
	if err := s.applyClusterTeamRecord(ctx, cfg); err != nil {
		return err
	}
	_, err := s.ReloadTeam(ctx, id)
	return err
}

func (s *BifrostHTTPServer) ApplyClusterVirtualKeyConfig(ctx context.Context, id string, cfg *configstoreTables.TableVirtualKey, deleteVirtualKey bool) error {
	if s == nil || s.Config == nil || s.Config.ConfigStore == nil {
		return fmt.Errorf("config store not found")
	}

	id = normalizeClusterGovernanceID(id, func() string {
		if cfg == nil {
			return ""
		}
		return cfg.ID
	}(), "virtual key")
	if id == "" {
		return fmt.Errorf("virtual key id is required")
	}

	if deleteVirtualKey {
		if err := s.Config.ConfigStore.DeleteVirtualKey(ctx, id); err != nil && !errors.Is(err, configstore.ErrNotFound) {
			return fmt.Errorf("failed to delete virtual key config: %w", err)
		}
		return s.RemoveVirtualKey(ctx, id)
	}
	if cfg == nil {
		return fmt.Errorf("virtual key config is required")
	}
	if err := s.waitForClusterTeam(ctx, cfg.TeamID); err != nil {
		return err
	}
	if err := s.waitForClusterCustomer(ctx, cfg.CustomerID); err != nil {
		return err
	}
	if err := s.waitForClusterVirtualKeyDependencies(ctx, cfg); err != nil {
		return err
	}
	if err := ensureClusterBudgetPayload("virtual key", id, cfg.BudgetID, cfg.Budget); err != nil {
		return err
	}
	if err := ensureClusterRateLimitPayload("virtual key", id, cfg.RateLimitID, cfg.RateLimit); err != nil {
		return err
	}
	if err := s.applyClusterVirtualKeyRecord(ctx, cfg); err != nil {
		return err
	}
	_, err := s.ReloadVirtualKey(ctx, id)
	return err
}

func (s *BifrostHTTPServer) applyClusterCustomerRecord(ctx context.Context, cfg *configstoreTables.TableCustomer) error {
	store := s.Config.ConfigStore
	return store.ExecuteTransaction(ctx, func(tx *gorm.DB) error {
		existing, err := store.GetCustomer(ctx, cfg.ID)
		if err != nil && !errors.Is(err, configstore.ErrNotFound) {
			return fmt.Errorf("failed to get existing customer config: %w", err)
		}
		if errors.Is(err, configstore.ErrNotFound) {
			existing = nil
		}

		if err := upsertClusterBudget(ctx, store, tx, cfg.Budget); err != nil {
			return fmt.Errorf("failed to sync customer budget: %w", err)
		}
		if err := upsertClusterRateLimit(ctx, store, tx, cfg.RateLimit); err != nil {
			return fmt.Errorf("failed to sync customer rate limit: %w", err)
		}

		row := clusterCustomerRecord(cfg)
		if existing == nil {
			if err := store.CreateCustomer(ctx, row, tx); err != nil {
				return fmt.Errorf("failed to create customer config: %w", err)
			}
		} else {
			if err := store.UpdateCustomer(ctx, row, tx); err != nil {
				return fmt.Errorf("failed to update customer config: %w", err)
			}
		}

		if existing != nil {
			if err := deleteOrphanedBudget(ctx, store, tx, existing.BudgetID, row.BudgetID); err != nil {
				return err
			}
			if err := deleteOrphanedRateLimit(ctx, store, tx, existing.RateLimitID, row.RateLimitID); err != nil {
				return err
			}
		}

		return nil
	})
}

func (s *BifrostHTTPServer) applyClusterTeamRecord(ctx context.Context, cfg *configstoreTables.TableTeam) error {
	store := s.Config.ConfigStore
	return store.ExecuteTransaction(ctx, func(tx *gorm.DB) error {
		existing, err := store.GetTeam(ctx, cfg.ID)
		if err != nil && !errors.Is(err, configstore.ErrNotFound) {
			return fmt.Errorf("failed to get existing team config: %w", err)
		}
		if errors.Is(err, configstore.ErrNotFound) {
			existing = nil
		}

		if err := upsertClusterBudget(ctx, store, tx, cfg.Budget); err != nil {
			return fmt.Errorf("failed to sync team budget: %w", err)
		}
		if err := upsertClusterRateLimit(ctx, store, tx, cfg.RateLimit); err != nil {
			return fmt.Errorf("failed to sync team rate limit: %w", err)
		}

		row := clusterTeamRecord(cfg)
		if existing == nil {
			if err := store.CreateTeam(ctx, row, tx); err != nil {
				return fmt.Errorf("failed to create team config: %w", err)
			}
		} else {
			if err := store.UpdateTeam(ctx, row, tx); err != nil {
				return fmt.Errorf("failed to update team config: %w", err)
			}
		}

		if existing != nil {
			if err := deleteOrphanedBudget(ctx, store, tx, existing.BudgetID, row.BudgetID); err != nil {
				return err
			}
			if err := deleteOrphanedRateLimit(ctx, store, tx, existing.RateLimitID, row.RateLimitID); err != nil {
				return err
			}
		}

		return nil
	})
}

func (s *BifrostHTTPServer) applyClusterVirtualKeyRecord(ctx context.Context, cfg *configstoreTables.TableVirtualKey) error {
	store := s.Config.ConfigStore
	return store.ExecuteTransaction(ctx, func(tx *gorm.DB) error {
		existing, err := store.GetVirtualKey(ctx, cfg.ID)
		if err != nil && !errors.Is(err, configstore.ErrNotFound) {
			return fmt.Errorf("failed to get existing virtual key config: %w", err)
		}
		if errors.Is(err, configstore.ErrNotFound) {
			existing = nil
		}

		if err := upsertClusterBudget(ctx, store, tx, cfg.Budget); err != nil {
			return fmt.Errorf("failed to sync virtual key budget: %w", err)
		}
		if err := upsertClusterRateLimit(ctx, store, tx, cfg.RateLimit); err != nil {
			return fmt.Errorf("failed to sync virtual key rate limit: %w", err)
		}

		row := clusterVirtualKeyRecord(cfg)
		if existing == nil {
			if err := store.CreateVirtualKey(ctx, row, tx); err != nil {
				return fmt.Errorf("failed to create virtual key config: %w", err)
			}
		} else {
			if err := store.UpdateVirtualKey(ctx, row, tx); err != nil {
				return fmt.Errorf("failed to update virtual key config: %w", err)
			}
			if err := deleteOrphanedBudget(ctx, store, tx, existing.BudgetID, row.BudgetID); err != nil {
				return err
			}
			if err := deleteOrphanedRateLimit(ctx, store, tx, existing.RateLimitID, row.RateLimitID); err != nil {
				return err
			}
		}

		existingProviderConfigs, err := store.GetVirtualKeyProviderConfigs(ctx, cfg.ID)
		if err != nil {
			return fmt.Errorf("failed to get existing virtual key provider configs: %w", err)
		}
		for _, providerConfig := range existingProviderConfigs {
			if err := store.DeleteVirtualKeyProviderConfig(ctx, providerConfig.ID, tx); err != nil && !errors.Is(err, configstore.ErrNotFound) {
				return fmt.Errorf("failed to delete stale virtual key provider config %d: %w", providerConfig.ID, err)
			}
		}

		existingMCPConfigs, err := store.GetVirtualKeyMCPConfigs(ctx, cfg.ID)
		if err != nil {
			return fmt.Errorf("failed to get existing virtual key mcp configs: %w", err)
		}
		for _, mcpConfig := range existingMCPConfigs {
			if err := store.DeleteVirtualKeyMCPConfig(ctx, mcpConfig.ID, tx); err != nil && !errors.Is(err, configstore.ErrNotFound) {
				return fmt.Errorf("failed to delete stale virtual key mcp config %d: %w", mcpConfig.ID, err)
			}
		}

		for i := range cfg.ProviderConfigs {
			providerConfig := cfg.ProviderConfigs[i]
			if err := ensureClusterBudgetPayload("virtual key provider config", providerConfig.Provider, providerConfig.BudgetID, providerConfig.Budget); err != nil {
				return err
			}
			if err := ensureClusterRateLimitPayload("virtual key provider config", providerConfig.Provider, providerConfig.RateLimitID, providerConfig.RateLimit); err != nil {
				return err
			}
			if err := upsertClusterBudget(ctx, store, tx, providerConfig.Budget); err != nil {
				return fmt.Errorf("failed to sync virtual key provider budget for %s: %w", providerConfig.Provider, err)
			}
			if err := upsertClusterRateLimit(ctx, store, tx, providerConfig.RateLimit); err != nil {
				return fmt.Errorf("failed to sync virtual key provider rate limit for %s: %w", providerConfig.Provider, err)
			}
			record := clusterVirtualKeyProviderConfigRecord(cfg.ID, &providerConfig)
			if err := store.CreateVirtualKeyProviderConfig(ctx, record, tx); err != nil {
				return fmt.Errorf("failed to create virtual key provider config for %s: %w", providerConfig.Provider, err)
			}
		}

		for i := range cfg.MCPConfigs {
			resolvedMCPClientID, err := s.resolveClusterMCPClientPrimaryKey(ctx, &cfg.MCPConfigs[i])
			if err != nil {
				return fmt.Errorf("failed to resolve virtual key mcp client dependency: %w", err)
			}
			record := clusterVirtualKeyMCPConfigRecord(cfg.ID, &cfg.MCPConfigs[i], resolvedMCPClientID)
			if err := store.CreateVirtualKeyMCPConfig(ctx, record, tx); err != nil {
				return fmt.Errorf("failed to create virtual key mcp config for client %d: %w", record.MCPClientID, err)
			}
		}

		return nil
	})
}

func upsertClusterBudget(ctx context.Context, store configstore.ConfigStore, tx *gorm.DB, budget *configstoreTables.TableBudget) error {
	if budget == nil {
		return nil
	}
	existing, err := store.GetBudget(ctx, budget.ID, tx)
	if err != nil && !errors.Is(err, configstore.ErrNotFound) {
		return err
	}
	budgetCopy := *budget
	if errors.Is(err, configstore.ErrNotFound) || existing == nil {
		return store.CreateBudget(ctx, &budgetCopy, tx)
	}
	return store.UpdateBudget(ctx, &budgetCopy, tx)
}

func upsertClusterRateLimit(ctx context.Context, store configstore.ConfigStore, tx *gorm.DB, rateLimit *configstoreTables.TableRateLimit) error {
	if rateLimit == nil {
		return nil
	}
	existing, err := store.GetRateLimit(ctx, rateLimit.ID, tx)
	if err != nil && !errors.Is(err, configstore.ErrNotFound) {
		return err
	}
	rateLimitCopy := *rateLimit
	if errors.Is(err, configstore.ErrNotFound) || existing == nil {
		return store.CreateRateLimit(ctx, &rateLimitCopy, tx)
	}
	return store.UpdateRateLimit(ctx, &rateLimitCopy, tx)
}

func deleteOrphanedBudget(ctx context.Context, store configstore.ConfigStore, tx *gorm.DB, existingID, newID *string) error {
	id := orphanedClusterResourceID(existingID, newID)
	if id == "" {
		return nil
	}
	if err := store.DeleteBudget(ctx, id, tx); err != nil && !errors.Is(err, configstore.ErrNotFound) {
		return fmt.Errorf("failed to delete orphaned budget %s: %w", id, err)
	}
	return nil
}

func deleteOrphanedRateLimit(ctx context.Context, store configstore.ConfigStore, tx *gorm.DB, existingID, newID *string) error {
	id := orphanedClusterResourceID(existingID, newID)
	if id == "" {
		return nil
	}
	if err := store.DeleteRateLimit(ctx, id, tx); err != nil && !errors.Is(err, configstore.ErrNotFound) {
		return fmt.Errorf("failed to delete orphaned rate limit %s: %w", id, err)
	}
	return nil
}

func normalizeClusterGovernanceID(primary, fallback, _ string) string {
	id := strings.TrimSpace(primary)
	if id == "" {
		id = strings.TrimSpace(fallback)
	}
	return id
}

func orphanedClusterResourceID(existingID, newID *string) string {
	oldValue := strings.TrimSpace(derefString(existingID))
	newValue := strings.TrimSpace(derefString(newID))
	if oldValue == "" || oldValue == newValue {
		return ""
	}
	return oldValue
}

func ensureClusterBudgetPayload(resource, id string, budgetID *string, budget *configstoreTables.TableBudget) error {
	if strings.TrimSpace(derefString(budgetID)) == "" {
		return nil
	}
	if budget == nil {
		return fmt.Errorf("%s %s is missing budget payload for %s", resource, id, derefString(budgetID))
	}
	return nil
}

func ensureClusterRateLimitPayload(resource, id string, rateLimitID *string, rateLimit *configstoreTables.TableRateLimit) error {
	if strings.TrimSpace(derefString(rateLimitID)) == "" {
		return nil
	}
	if rateLimit == nil {
		return fmt.Errorf("%s %s is missing rate limit payload for %s", resource, id, derefString(rateLimitID))
	}
	return nil
}

func (s *BifrostHTTPServer) waitForClusterCustomer(ctx context.Context, customerID *string) error {
	id := strings.TrimSpace(derefString(customerID))
	if id == "" {
		return nil
	}
	_, err := s.Config.ConfigStore.RetryOnNotFound(ctx, func(ctx context.Context) (any, error) {
		return s.Config.ConfigStore.GetCustomer(ctx, id)
	}, lib.DBLookupMaxRetries, lib.DBLookupDelay)
	if err != nil {
		return fmt.Errorf("customer dependency %s is not available on this node: %w", id, err)
	}
	return nil
}

func (s *BifrostHTTPServer) waitForClusterTeam(ctx context.Context, teamID *string) error {
	id := strings.TrimSpace(derefString(teamID))
	if id == "" {
		return nil
	}
	_, err := s.Config.ConfigStore.RetryOnNotFound(ctx, func(ctx context.Context) (any, error) {
		return s.Config.ConfigStore.GetTeam(ctx, id)
	}, lib.DBLookupMaxRetries, lib.DBLookupDelay)
	if err != nil {
		return fmt.Errorf("team dependency %s is not available on this node: %w", id, err)
	}
	return nil
}

func (s *BifrostHTTPServer) waitForClusterVirtualKeyDependencies(ctx context.Context, cfg *configstoreTables.TableVirtualKey) error {
	if cfg == nil {
		return nil
	}

	for i := range cfg.ProviderConfigs {
		if err := s.waitForClusterProviderKeys(ctx, cfg.ProviderConfigs[i].Keys); err != nil {
			return fmt.Errorf("provider config %s dependencies are not ready: %w", cfg.ProviderConfigs[i].Provider, err)
		}
	}

	for i := range cfg.MCPConfigs {
		resolvedMCPClientID, err := s.resolveClusterMCPClientPrimaryKey(ctx, &cfg.MCPConfigs[i])
		if err != nil {
			return err
		}
		cfg.MCPConfigs[i].MCPClientID = resolvedMCPClientID
	}

	return nil
}

func (s *BifrostHTTPServer) waitForClusterProviderKeys(ctx context.Context, keys []configstoreTables.TableKey) error {
	if len(keys) == 0 {
		return nil
	}

	ids := make([]string, 0, len(keys))
	seen := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		keyID := strings.TrimSpace(key.KeyID)
		if keyID == "" {
			continue
		}
		if _, ok := seen[keyID]; ok {
			continue
		}
		seen[keyID] = struct{}{}
		ids = append(ids, keyID)
	}
	if len(ids) == 0 {
		return nil
	}

	_, err := s.Config.ConfigStore.RetryOnNotFound(ctx, func(ctx context.Context) (any, error) {
		resolvedKeys, err := s.Config.ConfigStore.GetKeysByIDs(ctx, ids)
		if err != nil {
			return nil, err
		}
		if len(resolvedKeys) != len(ids) {
			return nil, configstore.ErrNotFound
		}
		return resolvedKeys, nil
	}, lib.DBLookupMaxRetries, lib.DBLookupDelay)
	if err != nil {
		return err
	}
	return nil
}

func clusterCustomerRecord(cfg *configstoreTables.TableCustomer) *configstoreTables.TableCustomer {
	record := *cfg
	record.Budget = nil
	record.RateLimit = nil
	record.Teams = nil
	record.VirtualKeys = nil
	return &record
}

func clusterTeamRecord(cfg *configstoreTables.TableTeam) *configstoreTables.TableTeam {
	record := *cfg
	record.Customer = nil
	record.Budget = nil
	record.RateLimit = nil
	record.VirtualKeys = nil
	return &record
}

func clusterVirtualKeyRecord(cfg *configstoreTables.TableVirtualKey) *configstoreTables.TableVirtualKey {
	record := *cfg
	record.Team = nil
	record.Customer = nil
	record.Budget = nil
	record.RateLimit = nil
	record.ProviderConfigs = nil
	record.MCPConfigs = nil
	return &record
}

func clusterVirtualKeyProviderConfigRecord(virtualKeyID string, cfg *configstoreTables.TableVirtualKeyProviderConfig) *configstoreTables.TableVirtualKeyProviderConfig {
	record := *cfg
	record.VirtualKeyID = virtualKeyID
	record.Budget = nil
	record.RateLimit = nil
	if len(cfg.Keys) > 0 {
		record.Keys = append([]configstoreTables.TableKey(nil), cfg.Keys...)
	} else {
		record.Keys = nil
	}
	return &record
}

func clusterVirtualKeyMCPConfigRecord(virtualKeyID string, cfg *configstoreTables.TableVirtualKeyMCPConfig, mcpClientID uint) *configstoreTables.TableVirtualKeyMCPConfig {
	record := *cfg
	record.VirtualKeyID = virtualKeyID
	record.MCPClientID = mcpClientID
	record.MCPClient = configstoreTables.TableMCPClient{}
	return &record
}

func (s *BifrostHTTPServer) resolveClusterMCPClientPrimaryKey(ctx context.Context, cfg *configstoreTables.TableVirtualKeyMCPConfig) (uint, error) {
	if cfg == nil {
		return 0, fmt.Errorf("virtual key mcp config is required")
	}

	if clientID := strings.TrimSpace(cfg.MCPClient.ClientID); clientID != "" {
		lookup, err := s.Config.ConfigStore.RetryOnNotFound(ctx, func(ctx context.Context) (any, error) {
			return s.Config.ConfigStore.GetMCPClientByID(ctx, clientID)
		}, lib.DBLookupMaxRetries, lib.DBLookupDelay)
		if err == nil {
			if client, ok := lookup.(*configstoreTables.TableMCPClient); ok {
				return client.ID, nil
			}
		}
	}

	if name := strings.TrimSpace(cfg.MCPClient.Name); name != "" {
		lookup, err := s.Config.ConfigStore.RetryOnNotFound(ctx, func(ctx context.Context) (any, error) {
			return s.Config.ConfigStore.GetMCPClientByName(ctx, name)
		}, lib.DBLookupMaxRetries, lib.DBLookupDelay)
		if err == nil {
			if client, ok := lookup.(*configstoreTables.TableMCPClient); ok {
				return client.ID, nil
			}
		}
	}

	if cfg.MCPClientID != 0 {
		var existingMCPClient configstoreTables.TableMCPClient
		if err := s.Config.ConfigStore.DB().WithContext(ctx).First(&existingMCPClient, "id = ?", cfg.MCPClientID).Error; err == nil {
			return existingMCPClient.ID, nil
		}
	}

	return 0, fmt.Errorf("mcp client dependency is not available for virtual key config (client_id=%q name=%q db_id=%d)", cfg.MCPClient.ClientID, cfg.MCPClient.Name, cfg.MCPClientID)
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
