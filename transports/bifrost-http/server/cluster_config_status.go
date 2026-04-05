package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework"
	"github.com/maximhq/bifrost/framework/configstore"
	configstoreTables "github.com/maximhq/bifrost/framework/configstore/tables"
	"github.com/maximhq/bifrost/plugins/governance"
	enterprisecfg "github.com/maximhq/bifrost/transports/bifrost-http/enterprise"
	"gorm.io/gorm"
)

const clusterConfigSyncCacheTTL = 5 * time.Second

type clusterConfigSyncReporter struct {
	server *BifrostHTTPServer
	ttl    time.Duration

	mu        sync.Mutex
	cached    enterprisecfg.ClusterConfigSyncStatus
	expiresAt time.Time
}

type clusterConfigFingerprint struct {
	Client     string `json:"client,omitempty"`
	Auth       string `json:"auth,omitempty"`
	Framework  string `json:"framework,omitempty"`
	Proxy      string `json:"proxy,omitempty"`
	Providers  string `json:"providers,omitempty"`
	Governance string `json:"governance,omitempty"`
	MCP        string `json:"mcp,omitempty"`
	Plugins    string `json:"plugins,omitempty"`
}

type clusterConfigResourceCounts struct {
	CustomerCount   int
	ProviderCount   int
	TeamCount       int
	VirtualKeyCount int
	MCPClientCount  int
	PluginCount     int
}

type clusterProviderFingerprint struct {
	Provider   string                 `json:"provider"`
	ConfigHash string                 `json:"config_hash,omitempty"`
	KeyHashes  []clusterNamedResource `json:"key_hashes,omitempty"`
}

type clusterNamedResource struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
	Hash string `json:"hash,omitempty"`
}

type clusterGovernanceFingerprint struct {
	VirtualKeys  []clusterNamedResource `json:"virtual_keys,omitempty"`
	Teams        []clusterNamedResource `json:"teams,omitempty"`
	Customers    []clusterNamedResource `json:"customers,omitempty"`
	Budgets      []clusterNamedResource `json:"budgets,omitempty"`
	RateLimits   []clusterNamedResource `json:"rate_limits,omitempty"`
	RoutingRules []clusterNamedResource `json:"routing_rules,omitempty"`
	ModelConfigs []clusterNamedResource `json:"model_configs,omitempty"`
	Providers    []clusterNamedResource `json:"providers,omitempty"`
}

type clusterMCPFingerprint struct {
	ToolSyncIntervalSeconds int64                  `json:"tool_sync_interval_seconds,omitempty"`
	ToolExecutionTimeout    int64                  `json:"tool_execution_timeout_seconds,omitempty"`
	MaxAgentDepth           int                    `json:"max_agent_depth,omitempty"`
	CodeModeBindingLevel    string                 `json:"code_mode_binding_level,omitempty"`
	Clients                 []clusterNamedResource `json:"clients,omitempty"`
}

type clusterFrameworkFingerprint struct {
	PricingURL             string `json:"pricing_url,omitempty"`
	PricingSyncIntervalSec int64  `json:"pricing_sync_interval_sec,omitempty"`
}

type clusterPluginFingerprint struct {
	Name      string                   `json:"name"`
	Enabled   bool                     `json:"enabled"`
	Path      *string                  `json:"path,omitempty"`
	Version   *int16                   `json:"version,omitempty"`
	Config    any                      `json:"config,omitempty"`
	Placement *schemas.PluginPlacement `json:"placement,omitempty"`
	Order     *int                     `json:"order,omitempty"`
}

type clusterMCPClientFingerprint struct {
	ID                 string                    `json:"id"`
	Name               string                    `json:"name"`
	IsCodeModeClient   bool                      `json:"is_code_mode_client"`
	ConnectionType     string                    `json:"connection_type"`
	ConnectionString   *schemas.EnvVar           `json:"connection_string,omitempty"`
	StdioConfig        *schemas.MCPStdioConfig   `json:"stdio_config,omitempty"`
	AuthType           string                    `json:"auth_type"`
	OauthConfigID      *string                   `json:"oauth_config_id,omitempty"`
	Headers            map[string]schemas.EnvVar `json:"headers,omitempty"`
	ToolsToExecute     []string                  `json:"tools_to_execute,omitempty"`
	ToolsToAutoExecute []string                  `json:"tools_to_auto_execute,omitempty"`
}

type gormDBProvider interface {
	DB() *gorm.DB
}

func newClusterConfigSyncReporter(server *BifrostHTTPServer) *clusterConfigSyncReporter {
	return &clusterConfigSyncReporter{
		server: server,
		ttl:    clusterConfigSyncCacheTTL,
	}
}

func (r *clusterConfigSyncReporter) Status() enterprisecfg.ClusterConfigSyncStatus {
	if r == nil {
		return enterprisecfg.ClusterConfigSyncStatus{}
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	if now.Before(r.expiresAt) {
		return cloneClusterConfigSyncStatus(r.cached)
	}

	status := r.compute()
	r.cached = cloneClusterConfigSyncStatus(status)
	r.expiresAt = now.Add(r.ttl)
	return cloneClusterConfigSyncStatus(status)
}

func (r *clusterConfigSyncReporter) compute() enterprisecfg.ClusterConfigSyncStatus {
	status := enterprisecfg.ClusterConfigSyncStatus{}
	if r == nil || r.server == nil || r.server.Config == nil {
		status.LastError = "config is not initialized"
		return status
	}

	runtimeFingerprint, runtimeCounts, err := buildRuntimeClusterConfigFingerprint(r.server)
	if err != nil {
		status.LastError = fmt.Sprintf("failed to build runtime config fingerprint: %v", err)
		return status
	}

	status.RuntimeHash = runtimeFingerprint.Hash()
	status.CustomerCount = runtimeCounts.CustomerCount
	status.ProviderCount = runtimeCounts.ProviderCount
	status.TeamCount = runtimeCounts.TeamCount
	status.VirtualKeyCount = runtimeCounts.VirtualKeyCount
	status.MCPClientCount = runtimeCounts.MCPClientCount
	status.PluginCount = runtimeCounts.PluginCount

	store := r.server.Config.ConfigStore
	if store == nil {
		return status
	}

	status.StoreConnected = true
	status.StoreKind = detectConfigStoreKind(store)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	storeFingerprint, err := buildStoreClusterConfigFingerprint(ctx, store)
	if err != nil {
		status.LastError = fmt.Sprintf("failed to build config store fingerprint: %v", err)
		return status
	}

	status.StoreHash = storeFingerprint.Hash()
	inSync := status.RuntimeHash == status.StoreHash
	status.InSync = &inSync
	status.DriftDomains = runtimeFingerprint.DriftDomains(storeFingerprint)
	return status
}

func buildRuntimeClusterConfigFingerprint(server *BifrostHTTPServer) (clusterConfigFingerprint, clusterConfigResourceCounts, error) {
	if server == nil || server.Config == nil {
		return clusterConfigFingerprint{}, clusterConfigResourceCounts{}, nil
	}

	clientHash, err := hashClientConfig(server.Config.ClientConfig)
	if err != nil {
		return clusterConfigFingerprint{}, clusterConfigResourceCounts{}, err
	}
	authHash, err := hashAuthConfig(currentRuntimeAuthConfig(server))
	if err != nil {
		return clusterConfigFingerprint{}, clusterConfigResourceCounts{}, err
	}
	frameworkHash, err := hashFrameworkConfigFromRuntime(server.Config.FrameworkConfig)
	if err != nil {
		return clusterConfigFingerprint{}, clusterConfigResourceCounts{}, err
	}
	proxyHash, err := hashSortedValue(server.Config.ProxyConfig)
	if err != nil {
		return clusterConfigFingerprint{}, clusterConfigResourceCounts{}, err
	}
	providersHash, providerCount, err := hashProvidersConfig(server.Config.SnapshotProviders())
	if err != nil {
		return clusterConfigFingerprint{}, clusterConfigResourceCounts{}, err
	}
	governanceHash, governanceCounts, err := hashRuntimeGovernanceData(server.GetGovernanceData())
	if err != nil {
		return clusterConfigFingerprint{}, clusterConfigResourceCounts{}, err
	}
	mcpHash, mcpCount, err := hashMCPConfig(server.Config.MCPConfig)
	if err != nil {
		return clusterConfigFingerprint{}, clusterConfigResourceCounts{}, err
	}
	pluginsHash, pluginCount, err := hashRuntimePlugins(server.Config.PluginConfigs)
	if err != nil {
		return clusterConfigFingerprint{}, clusterConfigResourceCounts{}, err
	}

	return clusterConfigFingerprint{
			Client:     clientHash,
			Auth:       authHash,
			Framework:  frameworkHash,
			Proxy:      proxyHash,
			Providers:  providersHash,
			Governance: governanceHash,
			MCP:        mcpHash,
			Plugins:    pluginsHash,
		},
		clusterConfigResourceCounts{
			CustomerCount:   governanceCounts.CustomerCount,
			ProviderCount:   providerCount,
			TeamCount:       governanceCounts.TeamCount,
			VirtualKeyCount: governanceCounts.VirtualKeyCount,
			MCPClientCount:  mcpCount,
			PluginCount:     pluginCount,
		},
		nil
}

func buildStoreClusterConfigFingerprint(ctx context.Context, store configstore.ConfigStore) (clusterConfigFingerprint, error) {
	if store == nil {
		return clusterConfigFingerprint{}, nil
	}

	clientConfig, err := store.GetClientConfig(ctx)
	if err != nil && !errors.Is(err, configstore.ErrNotFound) {
		return clusterConfigFingerprint{}, err
	}
	authConfig, err := store.GetAuthConfig(ctx)
	if err != nil && !errors.Is(err, configstore.ErrNotFound) {
		return clusterConfigFingerprint{}, err
	}
	frameworkConfig, err := store.GetFrameworkConfig(ctx)
	if err != nil && !errors.Is(err, configstore.ErrNotFound) {
		return clusterConfigFingerprint{}, err
	}
	providersConfig, err := store.GetProvidersConfig(ctx)
	if err != nil && !errors.Is(err, configstore.ErrNotFound) {
		return clusterConfigFingerprint{}, err
	}
	governanceConfig, err := store.GetGovernanceConfig(ctx)
	if err != nil && !errors.Is(err, configstore.ErrNotFound) {
		return clusterConfigFingerprint{}, err
	}
	mcpConfig, err := store.GetMCPConfig(ctx)
	if err != nil && !errors.Is(err, configstore.ErrNotFound) {
		return clusterConfigFingerprint{}, err
	}
	proxyConfig, err := store.GetProxyConfig(ctx)
	if err != nil && !errors.Is(err, configstore.ErrNotFound) {
		return clusterConfigFingerprint{}, err
	}
	plugins, err := store.GetPlugins(ctx)
	if err != nil && !errors.Is(err, configstore.ErrNotFound) {
		return clusterConfigFingerprint{}, err
	}

	clientHash, err := hashClientConfig(clientConfig)
	if err != nil {
		return clusterConfigFingerprint{}, err
	}
	authHash, err := hashAuthConfig(authConfig)
	if err != nil {
		return clusterConfigFingerprint{}, err
	}
	frameworkHash, err := hashFrameworkConfigFromStore(frameworkConfig)
	if err != nil {
		return clusterConfigFingerprint{}, err
	}
	proxyHash, err := hashSortedValue(proxyConfig)
	if err != nil {
		return clusterConfigFingerprint{}, err
	}
	providersHash, _, err := hashProvidersConfig(providersConfig)
	if err != nil {
		return clusterConfigFingerprint{}, err
	}
	governanceHash, err := hashStoreGovernanceConfig(governanceConfig)
	if err != nil {
		return clusterConfigFingerprint{}, err
	}
	mcpHash, _, err := hashMCPConfig(mcpConfig)
	if err != nil {
		return clusterConfigFingerprint{}, err
	}
	pluginsHash, err := hashStorePlugins(plugins)
	if err != nil {
		return clusterConfigFingerprint{}, err
	}

	return clusterConfigFingerprint{
		Client:     clientHash,
		Auth:       authHash,
		Framework:  frameworkHash,
		Proxy:      proxyHash,
		Providers:  providersHash,
		Governance: governanceHash,
		MCP:        mcpHash,
		Plugins:    pluginsHash,
	}, nil
}

func (f clusterConfigFingerprint) Hash() string {
	digest, err := hashSortedValue(f)
	if err != nil {
		return ""
	}
	return digest
}

func (f clusterConfigFingerprint) DriftDomains(other clusterConfigFingerprint) []string {
	drift := make([]string, 0, 8)
	if f.Client != other.Client {
		drift = append(drift, "client")
	}
	if f.Auth != other.Auth {
		drift = append(drift, "auth")
	}
	if f.Framework != other.Framework {
		drift = append(drift, "framework")
	}
	if f.Proxy != other.Proxy {
		drift = append(drift, "proxy")
	}
	if f.Providers != other.Providers {
		drift = append(drift, "providers")
	}
	if f.Governance != other.Governance {
		drift = append(drift, "governance")
	}
	if f.MCP != other.MCP {
		drift = append(drift, "mcp")
	}
	if f.Plugins != other.Plugins {
		drift = append(drift, "plugins")
	}
	return drift
}

func currentRuntimeAuthConfig(server *BifrostHTTPServer) *configstore.AuthConfig {
	if server == nil {
		return nil
	}
	if server.AuthMiddleware != nil {
		if authConfig := server.AuthMiddleware.CurrentAuthConfig(); authConfig != nil {
			return authConfig
		}
	}
	if server.Config != nil && server.Config.GovernanceConfig != nil {
		return server.Config.GovernanceConfig.AuthConfig
	}
	return nil
}

func hashClientConfig(cfg *configstore.ClientConfig) (string, error) {
	if cfg == nil {
		return "", nil
	}
	cloned := *cfg
	cloned.ConfigHash = ""
	return cloned.GenerateClientConfigHash()
}

func hashAuthConfig(cfg *configstore.AuthConfig) (string, error) {
	if cfg == nil {
		return "", nil
	}

	payload := struct {
		AdminUserName          *schemas.EnvVar `json:"admin_username,omitempty"`
		AdminPassword          *schemas.EnvVar `json:"admin_password,omitempty"`
		IsEnabled              bool            `json:"is_enabled"`
		DisableAuthOnInference bool            `json:"disable_auth_on_inference"`
	}{
		AdminUserName:          cfg.AdminUserName,
		AdminPassword:          cfg.AdminPassword,
		IsEnabled:              cfg.IsEnabled,
		DisableAuthOnInference: cfg.DisableAuthOnInference,
	}
	return hashSortedValue(payload)
}

func hashFrameworkConfigFromRuntime(cfg *framework.FrameworkConfig) (string, error) {
	if cfg == nil || cfg.Pricing == nil {
		return "", nil
	}
	payload := clusterFrameworkFingerprint{}
	if cfg.Pricing.PricingURL != nil {
		payload.PricingURL = strings.TrimSpace(*cfg.Pricing.PricingURL)
	}
	if cfg.Pricing.PricingSyncInterval != nil {
		payload.PricingSyncIntervalSec = int64(cfg.Pricing.PricingSyncInterval.Seconds())
	}
	return hashSortedValue(payload)
}

func hashFrameworkConfigFromStore(cfg *configstoreTables.TableFrameworkConfig) (string, error) {
	if cfg == nil {
		return "", nil
	}
	payload := clusterFrameworkFingerprint{}
	if cfg.PricingURL != nil {
		payload.PricingURL = strings.TrimSpace(*cfg.PricingURL)
	}
	if cfg.PricingSyncInterval != nil {
		payload.PricingSyncIntervalSec = *cfg.PricingSyncInterval
	}
	return hashSortedValue(payload)
}

func hashProvidersConfig(providers map[schemas.ModelProvider]configstore.ProviderConfig) (string, int, error) {
	if len(providers) == 0 {
		return "", 0, nil
	}

	fingerprints := make([]clusterProviderFingerprint, 0, len(providers))
	for provider, providerConfig := range providers {
		providerHash, err := providerConfig.GenerateConfigHash(string(provider))
		if err != nil {
			return "", 0, err
		}

		keyHashes := make([]clusterNamedResource, 0, len(providerConfig.Keys))
		for _, key := range providerConfig.Keys {
			keyHash, err := configstore.GenerateKeyHash(key)
			if err != nil {
				return "", 0, err
			}
			keyHashes = append(keyHashes, clusterNamedResource{
				Name: strings.TrimSpace(key.Name),
				Hash: keyHash,
			})
		}
		sortNamedResources(keyHashes)

		fingerprints = append(fingerprints, clusterProviderFingerprint{
			Provider:   string(provider),
			ConfigHash: providerHash,
			KeyHashes:  keyHashes,
		})
	}

	sort.Slice(fingerprints, func(i, j int) bool {
		return fingerprints[i].Provider < fingerprints[j].Provider
	})

	hash, err := hashSortedValue(fingerprints)
	if err != nil {
		return "", 0, err
	}
	return hash, len(fingerprints), nil
}

type clusterGovernanceCounts struct {
	CustomerCount   int
	TeamCount       int
	VirtualKeyCount int
}

func hashRuntimeGovernanceData(data *governance.GovernanceData) (string, clusterGovernanceCounts, error) {
	if data == nil {
		return "", clusterGovernanceCounts{}, nil
	}

	fingerprint := clusterGovernanceFingerprint{
		VirtualKeys: hashRuntimeResourceMap(data.VirtualKeys, func(item *configstoreTables.TableVirtualKey) (string, error) {
			return configstore.GenerateVirtualKeyHash(*item)
		}),
		Teams: hashRuntimeResourceMap(data.Teams, func(item *configstoreTables.TableTeam) (string, error) { return configstore.GenerateTeamHash(*item) }),
		Customers: hashRuntimeResourceMap(data.Customers, func(item *configstoreTables.TableCustomer) (string, error) {
			return configstore.GenerateCustomerHash(*item)
		}),
		Budgets: hashRuntimeResourceMap(data.Budgets, func(item *configstoreTables.TableBudget) (string, error) {
			return configstore.GenerateBudgetHash(*item)
		}),
		RateLimits: hashRuntimeResourceMap(data.RateLimits, func(item *configstoreTables.TableRateLimit) (string, error) {
			return configstore.GenerateRateLimitHash(*item)
		}),
		RoutingRules: hashRuntimeResourceMap(data.RoutingRules, func(item *configstoreTables.TableRoutingRule) (string, error) {
			return configstore.GenerateRoutingRuleHash(*item)
		}),
		ModelConfigs: hashRuntimeModelConfigs(data.ModelConfigs),
		Providers:    hashRuntimeGovernanceProviders(data.Providers),
	}
	if err := validateNamedResources(fingerprint.VirtualKeys, fingerprint.Teams, fingerprint.Customers, fingerprint.Budgets, fingerprint.RateLimits, fingerprint.RoutingRules, fingerprint.ModelConfigs, fingerprint.Providers); err != nil {
		return "", clusterGovernanceCounts{}, err
	}
	hash, err := hashSortedValue(fingerprint)
	if err != nil {
		return "", clusterGovernanceCounts{}, err
	}
	return hash, clusterGovernanceCounts{
		CustomerCount:   len(fingerprint.Customers),
		TeamCount:       len(fingerprint.Teams),
		VirtualKeyCount: len(fingerprint.VirtualKeys),
	}, nil
}

func hashStoreGovernanceConfig(cfg *configstore.GovernanceConfig) (string, error) {
	if cfg == nil {
		return "", nil
	}

	fingerprint := clusterGovernanceFingerprint{
		VirtualKeys:  hashStoreVirtualKeys(cfg.VirtualKeys),
		Teams:        hashStoreTeams(cfg.Teams),
		Customers:    hashStoreCustomers(cfg.Customers),
		Budgets:      hashStoreBudgets(cfg.Budgets),
		RateLimits:   hashStoreRateLimits(cfg.RateLimits),
		RoutingRules: hashStoreRoutingRules(cfg.RoutingRules),
		ModelConfigs: hashStoreModelConfigs(cfg.ModelConfigs),
		Providers:    hashStoreGovernanceProviders(cfg.Providers),
	}
	if err := validateNamedResources(fingerprint.VirtualKeys, fingerprint.Teams, fingerprint.Customers, fingerprint.Budgets, fingerprint.RateLimits, fingerprint.RoutingRules, fingerprint.ModelConfigs, fingerprint.Providers); err != nil {
		return "", err
	}
	return hashSortedValue(fingerprint)
}

func hashMCPConfig(cfg *schemas.MCPConfig) (string, int, error) {
	if cfg == nil {
		return "", 0, nil
	}

	fingerprint := clusterMCPFingerprint{}
	if cfg.ToolManagerConfig != nil {
		fingerprint.ToolExecutionTimeout = int64(cfg.ToolManagerConfig.ToolExecutionTimeout.Seconds())
		fingerprint.MaxAgentDepth = cfg.ToolManagerConfig.MaxAgentDepth
		fingerprint.CodeModeBindingLevel = string(cfg.ToolManagerConfig.CodeModeBindingLevel)
	}
	fingerprint.ToolSyncIntervalSeconds = int64(cfg.ToolSyncInterval.Seconds())
	fingerprint.Clients = make([]clusterNamedResource, 0, len(cfg.ClientConfigs))
	for _, clientConfig := range cfg.ClientConfigs {
		if clientConfig == nil {
			continue
		}
		hash, err := hashMCPClientConfig(clientConfig)
		if err != nil {
			return "", 0, err
		}
		fingerprint.Clients = append(fingerprint.Clients, clusterNamedResource{
			ID:   strings.TrimSpace(clientConfig.ID),
			Name: strings.TrimSpace(clientConfig.Name),
			Hash: hash,
		})
	}
	sortNamedResources(fingerprint.Clients)
	hash, err := hashSortedValue(fingerprint)
	if err != nil {
		return "", 0, err
	}
	return hash, len(fingerprint.Clients), nil
}

func hashRuntimePlugins(plugins []*schemas.PluginConfig) (string, int, error) {
	if len(plugins) == 0 {
		return "", 0, nil
	}
	digests := make([]clusterNamedResource, 0, len(plugins))
	for _, plugin := range plugins {
		if plugin == nil {
			continue
		}
		hash, err := hashPluginConfig(plugin)
		if err != nil {
			return "", 0, err
		}
		digests = append(digests, clusterNamedResource{
			Name: strings.TrimSpace(plugin.Name),
			Hash: hash,
		})
	}
	sortNamedResources(digests)
	hash, err := hashSortedValue(digests)
	if err != nil {
		return "", 0, err
	}
	return hash, len(digests), nil
}

func hashStorePlugins(plugins []*configstoreTables.TablePlugin) (string, error) {
	if len(plugins) == 0 {
		return "", nil
	}
	digests := make([]clusterNamedResource, 0, len(plugins))
	for _, plugin := range plugins {
		if plugin == nil {
			continue
		}
		hash, err := hashPluginTable(plugin)
		if err != nil {
			return "", err
		}
		digests = append(digests, clusterNamedResource{
			Name: strings.TrimSpace(plugin.Name),
			Hash: hash,
		})
	}
	sortNamedResources(digests)
	return hashSortedValue(digests)
}

func hashMCPClientConfig(cfg *schemas.MCPClientConfig) (string, error) {
	if cfg == nil {
		return "", nil
	}
	payload := clusterMCPClientFingerprint{
		ID:                 strings.TrimSpace(cfg.ID),
		Name:               strings.TrimSpace(cfg.Name),
		IsCodeModeClient:   cfg.IsCodeModeClient,
		ConnectionType:     string(cfg.ConnectionType),
		ConnectionString:   cfg.ConnectionString,
		StdioConfig:        cfg.StdioConfig,
		AuthType:           string(cfg.AuthType),
		OauthConfigID:      cfg.OauthConfigID,
		Headers:            cfg.Headers,
		ToolsToExecute:     append([]string(nil), cfg.ToolsToExecute...),
		ToolsToAutoExecute: append([]string(nil), cfg.ToolsToAutoExecute...),
	}
	sort.Strings(payload.ToolsToExecute)
	sort.Strings(payload.ToolsToAutoExecute)
	return hashSortedValue(payload)
}

func hashPluginConfig(cfg *schemas.PluginConfig) (string, error) {
	if cfg == nil {
		return "", nil
	}
	payload := clusterPluginFingerprint{
		Name:      strings.TrimSpace(cfg.Name),
		Enabled:   cfg.Enabled,
		Path:      cfg.Path,
		Version:   cfg.Version,
		Config:    cfg.Config,
		Placement: cfg.Placement,
		Order:     cfg.Order,
	}
	return hashSortedValue(payload)
}

func hashPluginTable(plugin *configstoreTables.TablePlugin) (string, error) {
	if plugin == nil {
		return "", nil
	}
	version := plugin.Version
	payload := clusterPluginFingerprint{
		Name:      strings.TrimSpace(plugin.Name),
		Enabled:   plugin.Enabled,
		Path:      plugin.Path,
		Version:   &version,
		Config:    plugin.Config,
		Placement: plugin.Placement,
		Order:     plugin.Order,
	}
	return hashSortedValue(payload)
}

func hashRuntimeResourceMap[T any](items map[string]*T, hasher func(item *T) (string, error)) []clusterNamedResource {
	if len(items) == 0 {
		return nil
	}
	digests := make([]clusterNamedResource, 0, len(items))
	for id, item := range items {
		if item == nil {
			continue
		}
		hash, err := hasher(item)
		if err != nil {
			digests = append(digests, clusterNamedResource{
				ID:   strings.TrimSpace(id),
				Hash: "error:" + err.Error(),
			})
			continue
		}
		digests = append(digests, clusterNamedResource{
			ID:   strings.TrimSpace(id),
			Hash: hash,
		})
	}
	sortNamedResources(digests)
	return digests
}

func hashStoreVirtualKeys(items []configstoreTables.TableVirtualKey) []clusterNamedResource {
	return hashSlice(items, func(item configstoreTables.TableVirtualKey) (clusterNamedResource, error) {
		hash, err := configstore.GenerateVirtualKeyHash(item)
		return clusterNamedResource{ID: strings.TrimSpace(item.ID), Hash: hash}, err
	})
}

func hashStoreTeams(items []configstoreTables.TableTeam) []clusterNamedResource {
	return hashSlice(items, func(item configstoreTables.TableTeam) (clusterNamedResource, error) {
		hash, err := configstore.GenerateTeamHash(item)
		return clusterNamedResource{ID: strings.TrimSpace(item.ID), Hash: hash}, err
	})
}

func hashStoreCustomers(items []configstoreTables.TableCustomer) []clusterNamedResource {
	return hashSlice(items, func(item configstoreTables.TableCustomer) (clusterNamedResource, error) {
		hash, err := configstore.GenerateCustomerHash(item)
		return clusterNamedResource{ID: strings.TrimSpace(item.ID), Hash: hash}, err
	})
}

func hashStoreBudgets(items []configstoreTables.TableBudget) []clusterNamedResource {
	return hashSlice(items, func(item configstoreTables.TableBudget) (clusterNamedResource, error) {
		hash, err := configstore.GenerateBudgetHash(item)
		return clusterNamedResource{ID: strings.TrimSpace(item.ID), Hash: hash}, err
	})
}

func hashStoreRateLimits(items []configstoreTables.TableRateLimit) []clusterNamedResource {
	return hashSlice(items, func(item configstoreTables.TableRateLimit) (clusterNamedResource, error) {
		hash, err := configstore.GenerateRateLimitHash(item)
		return clusterNamedResource{ID: strings.TrimSpace(item.ID), Hash: hash}, err
	})
}

func hashStoreRoutingRules(items []configstoreTables.TableRoutingRule) []clusterNamedResource {
	return hashSlice(items, func(item configstoreTables.TableRoutingRule) (clusterNamedResource, error) {
		hash, err := configstore.GenerateRoutingRuleHash(item)
		return clusterNamedResource{ID: strings.TrimSpace(item.ID), Hash: hash}, err
	})
}

func hashRuntimeModelConfigs(items []*configstoreTables.TableModelConfig) []clusterNamedResource {
	if len(items) == 0 {
		return nil
	}
	digests := make([]clusterNamedResource, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		hash, err := hashModelConfig(item)
		if err != nil {
			hash = "error:" + err.Error()
		}
		digests = append(digests, clusterNamedResource{
			ID:   strings.TrimSpace(item.ID),
			Hash: hash,
		})
	}
	sortNamedResources(digests)
	return digests
}

func hashStoreModelConfigs(items []configstoreTables.TableModelConfig) []clusterNamedResource {
	return hashSlice(items, func(item configstoreTables.TableModelConfig) (clusterNamedResource, error) {
		hash, err := hashModelConfig(&item)
		return clusterNamedResource{ID: strings.TrimSpace(item.ID), Hash: hash}, err
	})
}

func hashRuntimeGovernanceProviders(items []*configstoreTables.TableProvider) []clusterNamedResource {
	if len(items) == 0 {
		return nil
	}
	digests := make([]clusterNamedResource, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		hash, err := hashGovernanceProvider(item)
		if err != nil {
			hash = "error:" + err.Error()
		}
		digests = append(digests, clusterNamedResource{
			Name: strings.TrimSpace(item.Name),
			Hash: hash,
		})
	}
	sortNamedResources(digests)
	return digests
}

func hashStoreGovernanceProviders(items []configstoreTables.TableProvider) []clusterNamedResource {
	return hashSlice(items, func(item configstoreTables.TableProvider) (clusterNamedResource, error) {
		hash, err := hashGovernanceProvider(&item)
		return clusterNamedResource{Name: strings.TrimSpace(item.Name), Hash: hash}, err
	})
}

func hashModelConfig(item *configstoreTables.TableModelConfig) (string, error) {
	if item == nil {
		return "", nil
	}
	payload := struct {
		ID          string  `json:"id"`
		ModelName   string  `json:"model_name"`
		Provider    *string `json:"provider,omitempty"`
		BudgetID    *string `json:"budget_id,omitempty"`
		RateLimitID *string `json:"rate_limit_id,omitempty"`
		ConfigHash  string  `json:"config_hash,omitempty"`
	}{
		ID:          strings.TrimSpace(item.ID),
		ModelName:   strings.TrimSpace(item.ModelName),
		Provider:    item.Provider,
		BudgetID:    item.BudgetID,
		RateLimitID: item.RateLimitID,
		ConfigHash:  strings.TrimSpace(item.ConfigHash),
	}
	return hashSortedValue(payload)
}

func hashGovernanceProvider(item *configstoreTables.TableProvider) (string, error) {
	if item == nil {
		return "", nil
	}
	payload := struct {
		Name                    string  `json:"name"`
		BudgetID                *string `json:"budget_id,omitempty"`
		RateLimitID             *string `json:"rate_limit_id,omitempty"`
		SendBackRawRequest      bool    `json:"send_back_raw_request"`
		SendBackRawResponse     bool    `json:"send_back_raw_response"`
		StoreRawRequestResponse bool    `json:"store_raw_request_response"`
		Status                  string  `json:"status,omitempty"`
		Description             string  `json:"description,omitempty"`
		ConfigHash              string  `json:"config_hash,omitempty"`
	}{
		Name:                    strings.TrimSpace(item.Name),
		BudgetID:                item.BudgetID,
		RateLimitID:             item.RateLimitID,
		SendBackRawRequest:      item.SendBackRawRequest,
		SendBackRawResponse:     item.SendBackRawResponse,
		StoreRawRequestResponse: item.StoreRawRequestResponse,
		Status:                  strings.TrimSpace(item.Status),
		Description:             strings.TrimSpace(item.Description),
		ConfigHash:              strings.TrimSpace(item.ConfigHash),
	}
	return hashSortedValue(payload)
}

func hashSlice[T any](items []T, hasher func(item T) (clusterNamedResource, error)) []clusterNamedResource {
	if len(items) == 0 {
		return nil
	}
	digests := make([]clusterNamedResource, 0, len(items))
	for _, item := range items {
		digest, err := hasher(item)
		if err != nil {
			digest.Hash = "error:" + err.Error()
		}
		digests = append(digests, digest)
	}
	sortNamedResources(digests)
	return digests
}

func validateNamedResources(groups ...[]clusterNamedResource) error {
	for _, group := range groups {
		for _, item := range group {
			if strings.HasPrefix(item.Hash, "error:") {
				return errors.New(strings.TrimPrefix(item.Hash, "error:"))
			}
		}
	}
	return nil
}

func sortNamedResources(items []clusterNamedResource) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].ID != items[j].ID {
			return items[i].ID < items[j].ID
		}
		if items[i].Name != items[j].Name {
			return items[i].Name < items[j].Name
		}
		return items[i].Hash < items[j].Hash
	})
}

func hashSortedValue(value any) (string, error) {
	if value == nil {
		return "", nil
	}
	payload, err := schemas.MarshalSorted(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}

func detectConfigStoreKind(store configstore.ConfigStore) string {
	if store == nil {
		return ""
	}
	if dbProvider, ok := store.(gormDBProvider); ok && dbProvider.DB() != nil && dbProvider.DB().Dialector != nil {
		if name := strings.TrimSpace(dbProvider.DB().Dialector.Name()); name != "" {
			return name
		}
	}
	typeName := fmt.Sprintf("%T", store)
	typeName = strings.TrimPrefix(typeName, "*")
	return strings.TrimSpace(typeName)
}

func cloneClusterConfigSyncStatus(status enterprisecfg.ClusterConfigSyncStatus) enterprisecfg.ClusterConfigSyncStatus {
	cloned := status
	if len(status.DriftDomains) > 0 {
		cloned.DriftDomains = append([]string(nil), status.DriftDomains...)
	}
	if status.InSync != nil {
		value := *status.InSync
		cloned.InSync = &value
	}
	return cloned
}
