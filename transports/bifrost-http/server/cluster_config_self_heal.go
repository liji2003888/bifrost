package server

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework/configstore"
	configstoreTables "github.com/maximhq/bifrost/framework/configstore/tables"
	"github.com/maximhq/bifrost/plugins/governance"
	enterprisecfg "github.com/maximhq/bifrost/transports/bifrost-http/enterprise"
	"github.com/maximhq/bifrost/transports/bifrost-http/handlers"
	"github.com/maximhq/bifrost/transports/bifrost-http/lib"
)

const (
	clusterConfigSelfHealInterval = 30 * time.Second
	clusterConfigSelfHealCooldown = 1 * time.Minute
)

var clusterConfigSelfHealDomainOrder = []string{
	"client",
	"auth",
	"framework",
	"proxy",
	"providers",
	"governance",
}

type clusterConfigSelfHealer struct {
	interval time.Duration
	cooldown time.Duration
	status   func() enterprisecfg.ClusterConfigSyncStatus
	apply    func(context.Context, []string) error

	mu                   sync.Mutex
	lastAttemptSignature string
	lastAttemptAt        time.Time
}

func newClusterConfigSelfHealer(server *BifrostHTTPServer, reporter *clusterConfigSyncReporter) *clusterConfigSelfHealer {
	if server == nil || reporter == nil || server.Config == nil || server.Config.ConfigStore == nil {
		return nil
	}
	if server.ClusterService == nil {
		return nil
	}
	return &clusterConfigSelfHealer{
		interval: clusterConfigSelfHealInterval,
		cooldown: clusterConfigSelfHealCooldown,
		status:   reporter.compute,
		apply: func(ctx context.Context, domains []string) error {
			return server.ControlledSelfHealConfigFromStore(ctx, domains)
		},
	}
}

func (h *clusterConfigSelfHealer) Start(ctx context.Context) {
	if h == nil || h.status == nil || h.apply == nil || ctx == nil {
		return
	}

	go h.loop(ctx)
}

func (h *clusterConfigSelfHealer) loop(ctx context.Context) {
	ticker := time.NewTicker(h.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			h.runOnce(ctx)
		case <-ctx.Done():
			return
		}
	}
}

func (h *clusterConfigSelfHealer) runOnce(ctx context.Context) {
	if h == nil || h.status == nil || h.apply == nil {
		return
	}

	status := h.status()
	if !status.StoreConnected || status.InSync == nil {
		h.reset()
		return
	}
	if *status.InSync {
		h.reset()
		return
	}

	domains := supportedClusterConfigSelfHealDomains(status.DriftDomains)
	if len(domains) == 0 {
		return
	}

	signature := clusterConfigSelfHealSignature(status.StoreHash, domains)
	if !h.shouldAttempt(signature, time.Now()) {
		return
	}

	if logger != nil {
		logger.Warn("detected config runtime drift against ConfigStore, starting controlled self-heal: store_hash=%s domains=%s", shortClusterHash(status.StoreHash), strings.Join(domains, ","))
	}

	healCtx := ctx
	if healCtx == nil {
		healCtx = context.Background()
	}
	if _, hasDeadline := healCtx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		healCtx, cancel = context.WithTimeout(healCtx, 45*time.Second)
		defer cancel()
	}

	if err := h.apply(healCtx, domains); err != nil {
		if logger != nil {
			logger.Warn("controlled config self-heal failed: store_hash=%s domains=%s err=%v", shortClusterHash(status.StoreHash), strings.Join(domains, ","), err)
		}
		return
	}

	postStatus := h.status()
	if postStatus.InSync != nil && *postStatus.InSync {
		h.reset()
		if logger != nil {
			logger.Info("controlled config self-heal completed successfully: store_hash=%s domains=%s", shortClusterHash(postStatus.StoreHash), strings.Join(domains, ","))
		}
		return
	}

	if logger != nil {
		logger.Warn("controlled config self-heal completed with residual drift: store_hash=%s requested_domains=%s remaining_domains=%s", shortClusterHash(postStatus.StoreHash), strings.Join(domains, ","), strings.Join(postStatus.DriftDomains, ","))
	}
}

func (h *clusterConfigSelfHealer) shouldAttempt(signature string, now time.Time) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	if signature == "" {
		return false
	}
	if h.lastAttemptSignature == signature && !h.lastAttemptAt.IsZero() && now.Sub(h.lastAttemptAt) < h.cooldown {
		return false
	}
	h.lastAttemptSignature = signature
	h.lastAttemptAt = now
	return true
}

func (h *clusterConfigSelfHealer) reset() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.lastAttemptSignature = ""
	h.lastAttemptAt = time.Time{}
}

func clusterConfigSelfHealSignature(storeHash string, domains []string) string {
	return strings.TrimSpace(storeHash) + "|" + strings.Join(supportedClusterConfigSelfHealDomains(domains), ",")
}

func supportedClusterConfigSelfHealDomains(domains []string) []string {
	if len(domains) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(domains))
	for _, domain := range domains {
		normalized := strings.TrimSpace(strings.ToLower(domain))
		if !slices.Contains(clusterConfigSelfHealDomainOrder, normalized) {
			continue
		}
		seen[normalized] = struct{}{}
	}

	ordered := make([]string, 0, len(seen))
	for _, domain := range clusterConfigSelfHealDomainOrder {
		if _, ok := seen[domain]; ok {
			ordered = append(ordered, domain)
		}
	}
	return ordered
}

func shortClusterHash(hash string) string {
	hash = strings.TrimSpace(hash)
	if len(hash) <= 12 {
		return hash
	}
	return hash[:12]
}

func skipDBUpdateContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, schemas.BifrostContextKeySkipDBUpdate, true)
}

func cloneAuthConfig(cfg *configstore.AuthConfig) *configstore.AuthConfig {
	if cfg == nil {
		return nil
	}

	cloned := *cfg
	if cfg.AdminUserName != nil {
		username := *cfg.AdminUserName
		cloned.AdminUserName = &username
	}
	if cfg.AdminPassword != nil {
		password := *cfg.AdminPassword
		cloned.AdminPassword = &password
	}
	return &cloned
}

func (s *BifrostHTTPServer) ControlledSelfHealConfigFromStore(ctx context.Context, domains []string) error {
	if s == nil || s.Config == nil || s.Config.ConfigStore == nil {
		return fmt.Errorf("config store not found")
	}

	orderedDomains := supportedClusterConfigSelfHealDomains(domains)
	if len(orderedDomains) == 0 {
		return nil
	}

	for _, domain := range orderedDomains {
		switch domain {
		case "client":
			if err := s.ReloadClientRuntimeConfigFromConfigStore(ctx); err != nil {
				return fmt.Errorf("failed to self-heal client config: %w", err)
			}
		case "auth":
			if err := s.ReloadAuthConfigFromConfigStore(ctx); err != nil {
				return fmt.Errorf("failed to self-heal auth config: %w", err)
			}
		case "framework":
			if err := s.ReloadFrameworkConfigFromConfigStore(ctx); err != nil {
				return fmt.Errorf("failed to self-heal framework config: %w", err)
			}
		case "proxy":
			if err := s.ReloadProxyConfigFromConfigStore(ctx); err != nil {
				return fmt.Errorf("failed to self-heal proxy config: %w", err)
			}
		case "providers":
			if err := s.ReconcileProvidersFromConfigStore(ctx); err != nil {
				return fmt.Errorf("failed to self-heal providers: %w", err)
			}
		case "governance":
			if err := s.ReconcileGovernanceFromConfigStore(ctx); err != nil {
				return fmt.Errorf("failed to self-heal governance: %w", err)
			}
		}
	}

	s.broadcastSelfHealStoreUpdates(orderedDomains)
	return nil
}

func (s *BifrostHTTPServer) ReloadClientRuntimeConfigFromConfigStore(ctx context.Context) error {
	if err := s.ReloadClientConfigFromConfigStore(ctx); err != nil {
		return err
	}
	if s.Config != nil && s.Config.ClientConfig != nil {
		if err := s.ReloadHeaderFilterConfig(ctx, s.Config.ClientConfig.HeaderFilterConfig); err != nil {
			return err
		}
		if s.Config.MCPConfig != nil {
			if err := s.UpdateMCPToolManagerConfig(ctx, s.Config.ClientConfig.MCPAgentDepth, s.Config.ClientConfig.MCPToolExecutionTimeout, s.Config.ClientConfig.MCPCodeModeBindingLevel); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *BifrostHTTPServer) ReloadAuthConfigFromConfigStore(ctx context.Context) error {
	if s == nil || s.Config == nil || s.Config.ConfigStore == nil {
		return fmt.Errorf("config store not found")
	}

	authConfig, err := s.Config.ConfigStore.GetAuthConfig(ctx)
	if err != nil && !errors.Is(err, configstore.ErrNotFound) {
		return fmt.Errorf("failed to get auth config from store: %w", err)
	}
	if errors.Is(err, configstore.ErrNotFound) {
		authConfig = nil
	}

	if s.Config.GovernanceConfig == nil {
		s.Config.GovernanceConfig = &configstore.GovernanceConfig{}
	}
	s.Config.GovernanceConfig.AuthConfig = cloneAuthConfig(authConfig)
	if s.AuthMiddleware != nil {
		s.AuthMiddleware.UpdateAuthConfig(cloneAuthConfig(authConfig))
	}
	return nil
}

func (s *BifrostHTTPServer) ReconcileProvidersFromConfigStore(ctx context.Context) error {
	if s == nil || s.Config == nil || s.Config.ConfigStore == nil {
		return fmt.Errorf("config store not found")
	}

	storeProviders, err := s.Config.ConfigStore.GetProvidersConfig(ctx)
	if err != nil && !errors.Is(err, configstore.ErrNotFound) {
		return fmt.Errorf("failed to get providers config from store: %w", err)
	}
	if errors.Is(err, configstore.ErrNotFound) || storeProviders == nil {
		storeProviders = map[schemas.ModelProvider]configstore.ProviderConfig{}
	}

	runtimeProviders := s.Config.SnapshotProviders()
	localOnlyCtx := skipDBUpdateContext(ctx)

	for provider := range runtimeProviders {
		if _, ok := storeProviders[provider]; ok {
			continue
		}
		if err := s.RemoveProvider(localOnlyCtx, provider); err != nil {
			return fmt.Errorf("failed to remove provider %s from runtime: %w", provider, err)
		}
		if logger != nil {
			logger.Info("self-healed runtime provider removal from ConfigStore for provider %s", provider)
		}
	}

	providers := make([]schemas.ModelProvider, 0, len(storeProviders))
	for provider := range storeProviders {
		providers = append(providers, provider)
	}
	sort.Slice(providers, func(i, j int) bool { return providers[i] < providers[j] })

	for _, provider := range providers {
		storeConfig := storeProviders[provider]
		runtimeConfig, exists := runtimeProviders[provider]
		if exists {
			runtimeHash, _, err := hashProvidersConfig(map[schemas.ModelProvider]configstore.ProviderConfig{provider: runtimeConfig})
			if err != nil {
				return fmt.Errorf("failed to hash runtime provider %s: %w", provider, err)
			}
			storeHash, _, err := hashProvidersConfig(map[schemas.ModelProvider]configstore.ProviderConfig{provider: storeConfig})
			if err != nil {
				return fmt.Errorf("failed to hash store provider %s: %w", provider, err)
			}
			if runtimeHash == storeHash {
				continue
			}
			if err := s.Config.UpdateProviderConfig(localOnlyCtx, provider, storeConfig); err != nil {
				return fmt.Errorf("failed to update provider %s in runtime: %w", provider, err)
			}
		} else {
			if err := s.Config.AddProvider(localOnlyCtx, provider, storeConfig); err != nil && !errors.Is(err, lib.ErrAlreadyExists) {
				return fmt.Errorf("failed to add provider %s to runtime: %w", provider, err)
			}
		}

		if _, err := s.ReloadProvider(ctx, provider); err != nil {
			return fmt.Errorf("failed to reload provider %s from store: %w", provider, err)
		}
		if logger != nil {
			logger.Info("self-healed runtime provider from ConfigStore for provider %s", provider)
		}
	}

	return nil
}

func (s *BifrostHTTPServer) ReconcileGovernanceFromConfigStore(ctx context.Context) error {
	if s == nil || s.Config == nil || s.Config.ConfigStore == nil {
		return fmt.Errorf("config store not found")
	}

	governanceConfig, err := s.Config.ConfigStore.GetGovernanceConfig(ctx)
	if err != nil && !errors.Is(err, configstore.ErrNotFound) {
		return fmt.Errorf("failed to get governance config from store: %w", err)
	}
	if errors.Is(err, configstore.ErrNotFound) || governanceConfig == nil {
		governanceConfig = &configstore.GovernanceConfig{}
	}

	runtimeData := s.GetGovernanceData()
	if runtimeData == nil {
		return fmt.Errorf("governance runtime is not initialized")
	}

	if err := s.reconcileGovernanceCustomers(ctx, governanceConfig.Customers, runtimeData); err != nil {
		return err
	}
	if err := s.reconcileGovernanceTeams(ctx, governanceConfig.Teams, runtimeData); err != nil {
		return err
	}
	if err := s.reconcileGovernanceVirtualKeys(ctx, governanceConfig.VirtualKeys, runtimeData); err != nil {
		return err
	}
	if err := s.reconcileGovernanceModelConfigs(ctx, governanceConfig.ModelConfigs, runtimeData); err != nil {
		return err
	}
	if err := s.reconcileGovernanceRoutingRules(ctx, governanceConfig.RoutingRules, runtimeData); err != nil {
		return err
	}

	// Provider governance can drift independently of core provider config, so we
	// refresh it against all provider names visible in either runtime or ConfigStore.
	providerNames := make(map[schemas.ModelProvider]struct{})
	for _, provider := range governanceConfig.Providers {
		name := schemas.ModelProvider(strings.TrimSpace(provider.Name))
		if name != "" {
			providerNames[name] = struct{}{}
		}
	}
	for provider := range s.Config.SnapshotProviders() {
		providerNames[provider] = struct{}{}
	}
	for _, provider := range runtimeData.Providers {
		if provider == nil {
			continue
		}
		name := schemas.ModelProvider(strings.TrimSpace(provider.Name))
		if name != "" {
			providerNames[name] = struct{}{}
		}
	}

	orderedProviders := make([]schemas.ModelProvider, 0, len(providerNames))
	for provider := range providerNames {
		orderedProviders = append(orderedProviders, provider)
	}
	sort.Slice(orderedProviders, func(i, j int) bool { return orderedProviders[i] < orderedProviders[j] })
	for _, provider := range orderedProviders {
		if _, err := s.ReloadProviderGovernance(ctx, provider); err != nil && !errors.Is(err, configstore.ErrNotFound) {
			return fmt.Errorf("failed to self-heal provider governance for %s: %w", provider, err)
		}
	}

	return nil
}

func (s *BifrostHTTPServer) reconcileGovernanceCustomers(ctx context.Context, storeCustomers []configstoreTables.TableCustomer, runtimeData *governance.GovernanceData) error {
	storeIDs := make(map[string]struct{}, len(storeCustomers))
	for i := range storeCustomers {
		customer := storeCustomers[i]
		if strings.TrimSpace(customer.ID) == "" {
			continue
		}
		storeIDs[customer.ID] = struct{}{}
		runtimeCustomer := runtimeData.Customers[customer.ID]
		storeHash, err := configstore.GenerateCustomerHash(customer)
		if err != nil {
			return err
		}
		runtimeHash, err := hashRuntimeGovernanceResource(runtimeCustomer, configstore.GenerateCustomerHash)
		if err != nil {
			return err
		}
		if runtimeHash == storeHash {
			continue
		}
		if _, err := s.ReloadCustomer(ctx, customer.ID); err != nil {
			return fmt.Errorf("failed to reload customer %s: %w", customer.ID, err)
		}
	}
	for id := range runtimeData.Customers {
		if _, ok := storeIDs[id]; ok {
			continue
		}
		if err := s.RemoveCustomer(ctx, id); err != nil {
			return fmt.Errorf("failed to remove stale customer %s: %w", id, err)
		}
	}
	return nil
}

func (s *BifrostHTTPServer) reconcileGovernanceTeams(ctx context.Context, storeTeams []configstoreTables.TableTeam, runtimeData *governance.GovernanceData) error {
	storeIDs := make(map[string]struct{}, len(storeTeams))
	for i := range storeTeams {
		team := storeTeams[i]
		if strings.TrimSpace(team.ID) == "" {
			continue
		}
		storeIDs[team.ID] = struct{}{}
		runtimeTeam := runtimeData.Teams[team.ID]
		storeHash, err := configstore.GenerateTeamHash(team)
		if err != nil {
			return err
		}
		runtimeHash, err := hashRuntimeGovernanceResource(runtimeTeam, configstore.GenerateTeamHash)
		if err != nil {
			return err
		}
		if runtimeHash == storeHash {
			continue
		}
		if _, err := s.ReloadTeam(ctx, team.ID); err != nil {
			return fmt.Errorf("failed to reload team %s: %w", team.ID, err)
		}
	}
	for id := range runtimeData.Teams {
		if _, ok := storeIDs[id]; ok {
			continue
		}
		if err := s.RemoveTeam(ctx, id); err != nil {
			return fmt.Errorf("failed to remove stale team %s: %w", id, err)
		}
	}
	return nil
}

func (s *BifrostHTTPServer) reconcileGovernanceVirtualKeys(ctx context.Context, storeVirtualKeys []configstoreTables.TableVirtualKey, runtimeData *governance.GovernanceData) error {
	storeIDs := make(map[string]struct{}, len(storeVirtualKeys))
	for i := range storeVirtualKeys {
		virtualKey := storeVirtualKeys[i]
		if strings.TrimSpace(virtualKey.ID) == "" {
			continue
		}
		storeIDs[virtualKey.ID] = struct{}{}
		runtimeVirtualKey := findRuntimeVirtualKeyByID(runtimeData, virtualKey.ID)
		storeHash, err := configstore.GenerateVirtualKeyHash(virtualKey)
		if err != nil {
			return err
		}
		runtimeHash, err := hashRuntimeGovernanceResource(runtimeVirtualKey, configstore.GenerateVirtualKeyHash)
		if err != nil {
			return err
		}
		if runtimeHash == storeHash {
			continue
		}
		if _, err := s.ReloadVirtualKey(ctx, virtualKey.ID); err != nil {
			return fmt.Errorf("failed to reload virtual key %s: %w", virtualKey.ID, err)
		}
	}
	for _, virtualKey := range runtimeData.VirtualKeys {
		if virtualKey == nil || strings.TrimSpace(virtualKey.ID) == "" {
			continue
		}
		if _, ok := storeIDs[virtualKey.ID]; ok {
			continue
		}
		if err := s.RemoveVirtualKey(ctx, virtualKey.ID); err != nil {
			return fmt.Errorf("failed to remove stale virtual key %s: %w", virtualKey.ID, err)
		}
	}
	return nil
}

func (s *BifrostHTTPServer) reconcileGovernanceModelConfigs(ctx context.Context, storeModelConfigs []configstoreTables.TableModelConfig, runtimeData *governance.GovernanceData) error {
	storeIDs := make(map[string]struct{}, len(storeModelConfigs))
	for i := range storeModelConfigs {
		modelConfig := storeModelConfigs[i]
		if strings.TrimSpace(modelConfig.ID) == "" {
			continue
		}
		storeIDs[modelConfig.ID] = struct{}{}
		runtimeModelConfig := findRuntimeModelConfigByID(runtimeData, modelConfig.ID)
		storeHash := hashNamedResourceByID(hashStoreModelConfigs([]configstoreTables.TableModelConfig{modelConfig}), modelConfig.ID)
		runtimeHash := hashNamedResourceByID(hashRuntimeModelConfigs([]*configstoreTables.TableModelConfig{runtimeModelConfig}), modelConfig.ID)
		if runtimeHash == storeHash {
			continue
		}
		if _, err := s.ReloadModelConfig(ctx, modelConfig.ID); err != nil {
			return fmt.Errorf("failed to reload model config %s: %w", modelConfig.ID, err)
		}
	}
	for _, modelConfig := range runtimeData.ModelConfigs {
		if modelConfig == nil || strings.TrimSpace(modelConfig.ID) == "" {
			continue
		}
		if _, ok := storeIDs[modelConfig.ID]; ok {
			continue
		}
		if err := s.RemoveModelConfig(ctx, modelConfig.ID); err != nil {
			return fmt.Errorf("failed to remove stale model config %s: %w", modelConfig.ID, err)
		}
	}
	return nil
}

func (s *BifrostHTTPServer) reconcileGovernanceRoutingRules(ctx context.Context, storeRules []configstoreTables.TableRoutingRule, runtimeData *governance.GovernanceData) error {
	storeIDs := make(map[string]struct{}, len(storeRules))
	for i := range storeRules {
		rule := storeRules[i]
		if strings.TrimSpace(rule.ID) == "" {
			continue
		}
		storeIDs[rule.ID] = struct{}{}
		runtimeRule := runtimeData.RoutingRules[rule.ID]
		storeHash, err := configstore.GenerateRoutingRuleHash(rule)
		if err != nil {
			return err
		}
		runtimeHash, err := hashRuntimeGovernanceResource(runtimeRule, configstore.GenerateRoutingRuleHash)
		if err != nil {
			return err
		}
		if runtimeHash == storeHash {
			continue
		}
		if err := s.ReloadRoutingRule(ctx, rule.ID); err != nil {
			return fmt.Errorf("failed to reload routing rule %s: %w", rule.ID, err)
		}
	}
	for id := range runtimeData.RoutingRules {
		if _, ok := storeIDs[id]; ok {
			continue
		}
		if err := s.RemoveRoutingRule(ctx, id); err != nil {
			return fmt.Errorf("failed to remove stale routing rule %s: %w", id, err)
		}
	}
	return nil
}

func hashRuntimeGovernanceResource[T any](item *T, hashFn func(T) (string, error)) (string, error) {
	if item == nil {
		return "", nil
	}
	return hashFn(*item)
}

func hashNamedResourceByID(items []clusterNamedResource, id string) string {
	for _, item := range items {
		if item.ID == id {
			return item.Hash
		}
	}
	return ""
}

func findRuntimeVirtualKeyByID(data *governance.GovernanceData, id string) *configstoreTables.TableVirtualKey {
	if data == nil {
		return nil
	}
	for _, virtualKey := range data.VirtualKeys {
		if virtualKey != nil && virtualKey.ID == id {
			return virtualKey
		}
	}
	return nil
}

func findRuntimeModelConfigByID(data *governance.GovernanceData, id string) *configstoreTables.TableModelConfig {
	if data == nil {
		return nil
	}
	for _, modelConfig := range data.ModelConfigs {
		if modelConfig != nil && modelConfig.ID == id {
			return modelConfig
		}
	}
	return nil
}

func (s *BifrostHTTPServer) broadcastSelfHealStoreUpdates(domains []string) {
	if s == nil || s.WebSocketHandler == nil {
		return
	}

	var tags []string
	for _, domain := range supportedClusterConfigSelfHealDomains(domains) {
		switch domain {
		case "client", "auth", "framework", "proxy":
			tags = append(tags, clusterConfigChangeTags(&handlers.ClusterConfigChange{Scope: handlers.ClusterConfigScopeClient})...)
		case "providers":
			tags = append(tags, clusterConfigChangeTags(&handlers.ClusterConfigChange{Scope: handlers.ClusterConfigScopeProvider})...)
		case "governance":
			tags = append(tags, clusterConfigChangeTags(&handlers.ClusterConfigChange{Scope: handlers.ClusterConfigScopeCustomer})...)
		}
	}
	tags = dedupeStoreUpdateTags(tags)
	if len(tags) == 0 {
		return
	}
	s.WebSocketHandler.BroadcastUpdatesToClients(tags)
}
