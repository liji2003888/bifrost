package enterprise

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/bytedance/sonic"
	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework/configstore"
)

const vaultAutoDisabledDescriptionPrefix = "vault:auto-disabled:"

type vaultProviderRefresher interface {
	SnapshotProviders() map[schemas.ModelProvider]configstore.ProviderConfig
	UpdateProviderConfig(ctx context.Context, provider schemas.ModelProvider, config configstore.ProviderConfig) error
}

type vaultSecretBackend interface {
	Fetch(ctx context.Context) (map[string]string, error)
}

type VaultStatus struct {
	Enabled        bool       `json:"enabled"`
	Type           VaultType  `json:"type,omitempty"`
	LastSync       *time.Time `json:"last_sync,omitempty"`
	LastError      string     `json:"last_error,omitempty"`
	ManagedSecrets int        `json:"managed_secrets"`
}

type VaultService struct {
	cfg       *VaultConfig
	providers vaultProviderRefresher
	backend   vaultSecretBackend
	audit     *AuditService
	logger    schemas.Logger

	mu         sync.RWMutex
	lastSync   *time.Time
	lastError  string
	managedEnv map[string]string
	stopCh     chan struct{}
	stopOnce   sync.Once
	wg         sync.WaitGroup
}

type hashicorpVaultBackend struct {
	cfg    *VaultConfig
	client *http.Client
}

type kubernetesSecretBackend struct {
	cfg    *VaultConfig
	client *http.Client
	base   string
	token  string
}

type hashicorpVaultV2Response struct {
	Data struct {
		Data map[string]any `json:"data"`
	} `json:"data"`
}

type hashicorpVaultV1Response struct {
	Data map[string]any `json:"data"`
}

type kubernetesSecret struct {
	Metadata struct {
		Name string `json:"name"`
	} `json:"metadata"`
	Data map[string]string `json:"data"`
}

type kubernetesSecretList struct {
	Items []kubernetesSecret `json:"items"`
}

func NewVaultService(cfg *VaultConfig, providers vaultProviderRefresher, audit *AuditService, logger schemas.Logger) (*VaultService, error) {
	return newVaultService(cfg, providers, audit, logger, nil)
}

func newVaultService(cfg *VaultConfig, providers vaultProviderRefresher, audit *AuditService, logger schemas.Logger, backend vaultSecretBackend) (*VaultService, error) {
	if cfg == nil || !cfg.Enabled || providers == nil {
		return nil, nil
	}

	normalized := normalizeVaultConfig(cfg)
	if backend == nil {
		var err error
		backend, err = newVaultBackend(normalized)
		if err != nil {
			return nil, err
		}
	}

	return &VaultService{
		cfg:        normalized,
		providers:  providers,
		backend:    backend,
		audit:      audit,
		logger:     logger,
		managedEnv: make(map[string]string),
		stopCh:     make(chan struct{}),
	}, nil
}

func (s *VaultService) Start() {
	if s == nil {
		return
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		interval := vaultSyncInterval(s.cfg)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		s.syncOnce(context.Background())
		for {
			select {
			case <-ticker.C:
				s.syncOnce(context.Background())
			case <-s.stopCh:
				return
			}
		}
	}()
}

func (s *VaultService) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() {
		close(s.stopCh)
	})
	s.wg.Wait()
}

func (s *VaultService) Status() VaultStatus {
	if s == nil {
		return VaultStatus{}
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	var lastSync *time.Time
	if s.lastSync != nil {
		t := *s.lastSync
		lastSync = &t
	}
	return VaultStatus{
		Enabled:        true,
		Type:           s.cfg.Type,
		LastSync:       lastSync,
		LastError:      s.lastError,
		ManagedSecrets: len(s.managedEnv),
	}
}

func (s *VaultService) syncOnce(ctx context.Context) {
	if s == nil || s.backend == nil {
		return
	}

	secrets, err := s.backend.Fetch(ctx)
	now := time.Now().UTC()
	if err != nil {
		s.recordSyncFailure(now, err)
		if s.audit != nil {
			_ = s.audit.Append(&AuditEvent{
				Timestamp:    now,
				Category:     AuditCategorySecurityEvent,
				Action:       "vault_sync_failed",
				ResourceType: string(s.cfg.Type),
				Message:      err.Error(),
			})
		}
		return
	}

	changedNames, managedNames, unsetNames := s.applySecrets(secrets)
	reloadedProviders, reloadErrs := s.reloadImpactedProviders(ctx, changedNames, managedNames)

	s.mu.Lock()
	s.lastSync = &now
	s.lastError = joinErrors(reloadErrs)
	s.mu.Unlock()

	if s.audit != nil {
		metadata := map[string]any{
			"secret_count":       len(secrets),
			"changed_env_vars":   sortedKeysFromSet(changedNames),
			"unset_env_vars":     sortedKeysFromSet(unsetNames),
			"reloaded_providers": reloadedProviders,
		}
		if len(reloadErrs) > 0 {
			metadata["reload_errors"] = reloadErrs
		}
		_ = s.audit.Append(&AuditEvent{
			Timestamp:    now,
			Category:     AuditCategorySecurityEvent,
			Action:       "vault_sync",
			ResourceType: string(s.cfg.Type),
			Message:      fmt.Sprintf("vault sync applied %d secrets and reloaded %d providers", len(secrets), len(reloadedProviders)),
			Metadata:     metadata,
		})
	}
}

func (s *VaultService) recordSyncFailure(now time.Time, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastSync = &now
	s.lastError = err.Error()
}

func (s *VaultService) applySecrets(secrets map[string]string) (map[string]struct{}, map[string]struct{}, map[string]struct{}) {
	changed := make(map[string]struct{})
	managedNames := make(map[string]struct{})
	unset := make(map[string]struct{})

	s.mu.Lock()
	defer s.mu.Unlock()

	previousManaged := maps.Clone(s.managedEnv)
	for name, value := range secrets {
		name = normalizeVaultEnvName(name)
		if name == "" {
			continue
		}
		managedNames[name] = struct{}{}
		prevValue, existed := previousManaged[name]
		currentValue, currentExists := os.LookupEnv(name)
		if !existed || prevValue != value || !currentExists || currentValue != value {
			_ = os.Setenv(name, value)
			changed[name] = struct{}{}
		}
	}

	if s.cfg.AutoDeprecate {
		for name := range previousManaged {
			if _, ok := managedNames[name]; ok {
				continue
			}
			_ = os.Unsetenv(name)
			changed[name] = struct{}{}
			unset[name] = struct{}{}
		}
	}

	s.managedEnv = make(map[string]string, len(managedNames))
	for name := range managedNames {
		s.managedEnv[name] = secrets[name]
	}

	return changed, managedNames, unset
}

func (s *VaultService) reloadImpactedProviders(ctx context.Context, changedNames map[string]struct{}, managedNames map[string]struct{}) ([]string, []string) {
	if len(changedNames) == 0 {
		return nil, nil
	}

	snapshot := s.providers.SnapshotProviders()
	if len(snapshot) == 0 {
		return nil, nil
	}

	reloaded := make([]string, 0)
	errors := make([]string, 0)
	reloadCtx := context.WithValue(context.Background(), schemas.BifrostContextKeySkipDBUpdate, true)
	if ctx != nil {
		reloadCtx = context.WithValue(ctx, schemas.BifrostContextKeySkipDBUpdate, true)
	}

	for provider, providerConfig := range snapshot {
		if !providerUsesChangedEnv(providerConfig, changedNames) {
			continue
		}

		refreshed := refreshProviderConfigEnvVars(providerConfig)
		if s.cfg.AutoDeprecate {
			applyVaultDeprecation(&refreshed, managedNames)
		}

		if err := s.providers.UpdateProviderConfig(reloadCtx, provider, refreshed); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", provider, err))
			continue
		}
		reloaded = append(reloaded, string(provider))
	}

	sort.Strings(reloaded)
	sort.Strings(errors)
	return reloaded, errors
}

func newVaultBackend(cfg *VaultConfig) (vaultSecretBackend, error) {
	switch cfg.Type {
	case VaultTypeHashicorp:
		client, err := newVaultHTTPClient(cfg.Hashicorp.TLSSkipVerify, cfg.Hashicorp.CACertPEM)
		if err != nil {
			return nil, err
		}
		return &hashicorpVaultBackend{cfg: cfg, client: client}, nil
	case VaultTypeKubernetesSecret:
		return newKubernetesSecretBackend(cfg)
	default:
		return nil, fmt.Errorf("vault type %s is not implemented yet", cfg.Type)
	}
}

func normalizeVaultConfig(cfg *VaultConfig) *VaultConfig {
	if cfg == nil {
		return nil
	}
	copyCfg := *cfg
	copyCfg.SyncPaths = dedupeStrings(cfg.SyncPaths)
	return &copyCfg
}

func vaultSyncInterval(cfg *VaultConfig) time.Duration {
	if cfg == nil || strings.TrimSpace(cfg.SyncInterval) == "" {
		return 5 * time.Minute
	}
	interval, err := time.ParseDuration(strings.TrimSpace(cfg.SyncInterval))
	if err != nil || interval <= 0 {
		return 5 * time.Minute
	}
	if interval < 15*time.Second {
		return 15 * time.Second
	}
	return interval
}

func newVaultHTTPClient(skipVerify bool, caCertPEM string) (*http.Client, error) {
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: skipVerify}
	if strings.TrimSpace(caCertPEM) != "" {
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM([]byte(caCertPEM)) {
			return nil, fmt.Errorf("failed to parse vault CA certificate")
		}
		tlsConfig.RootCAs = pool
	}
	transport := &http.Transport{
		TLSClientConfig: tlsConfig,
	}
	return &http.Client{
		Timeout:   15 * time.Second,
		Transport: transport,
	}, nil
}

func (b *hashicorpVaultBackend) Fetch(ctx context.Context) (map[string]string, error) {
	if b == nil || b.cfg == nil || b.cfg.Hashicorp == nil {
		return nil, fmt.Errorf("hashicorp vault backend is not configured")
	}

	address := strings.TrimRight(strings.TrimSpace(b.cfg.Hashicorp.Address), "/")
	mount := strings.Trim(strings.TrimSpace(b.cfg.Hashicorp.Mount), "/")
	token := ""
	if b.cfg.Hashicorp.Token != nil {
		token = strings.TrimSpace(b.cfg.Hashicorp.Token.GetValue())
	}
	if address == "" || mount == "" {
		return nil, fmt.Errorf("hashicorp vault address and mount are required")
	}
	if token == "" {
		return nil, fmt.Errorf("hashicorp vault token is required")
	}

	secrets := make(map[string]string)
	for _, path := range b.cfg.SyncPaths {
		path = strings.Trim(path, "/")
		if path == "" {
			continue
		}

		values, err := b.fetchHashicorpPath(ctx, address, mount, path, token)
		if err != nil {
			return nil, err
		}
		for key, value := range values {
			secrets[key] = value
		}
	}
	return secrets, nil
}

func (b *hashicorpVaultBackend) fetchHashicorpPath(ctx context.Context, address, mount, path, token string) (map[string]string, error) {
	var namespace string
	if b.cfg.Hashicorp != nil {
		namespace = strings.TrimSpace(b.cfg.Hashicorp.Namespace)
	}

	requests := []string{
		fmt.Sprintf("%s/v1/%s/data/%s", address, mount, path),
		fmt.Sprintf("%s/v1/%s/%s", address, mount, path),
	}
	var lastErr error
	for i, endpoint := range requests {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("X-Vault-Token", token)
		if namespace != "" {
			req.Header.Set("X-Vault-Namespace", namespace)
		}

		resp, err := b.client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusNotFound && i == 0 {
			lastErr = fmt.Errorf("vault path not found: %s", path)
			continue
		}
		if resp.StatusCode >= http.StatusBadRequest {
			return nil, fmt.Errorf("vault request failed for %s with status %d", path, resp.StatusCode)
		}

		payload, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read vault response for %s: %w", path, err)
		}
		var v2 hashicorpVaultV2Response
		if err := sonic.Unmarshal(payload, &v2); err == nil && len(v2.Data.Data) > 0 {
			return stringifySecretMap(v2.Data.Data), nil
		}

		req, err = http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("X-Vault-Token", token)
		if namespace != "" {
			req.Header.Set("X-Vault-Namespace", namespace)
		}
		resp, err = b.client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		payload, err = io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read vault response for %s: %w", path, err)
		}
		var v1 hashicorpVaultV1Response
		if err := sonic.Unmarshal(payload, &v1); err != nil {
			return nil, fmt.Errorf("failed to decode vault response for %s: %w", path, err)
		}
		return stringifySecretMap(v1.Data), nil
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("vault path %s could not be fetched", path)
	}
	return nil, lastErr
}

func newKubernetesSecretBackend(cfg *VaultConfig) (*kubernetesSecretBackend, error) {
	if cfg == nil || cfg.Kubernetes == nil {
		return nil, fmt.Errorf("kubernetes vault config is not configured")
	}

	host := strings.TrimSpace(os.Getenv("KUBERNETES_SERVICE_HOST"))
	port := strings.TrimSpace(os.Getenv("KUBERNETES_SERVICE_PORT"))
	if host == "" || port == "" {
		return nil, fmt.Errorf("kubernetes service host/port environment is not available")
	}
	tokenBytes, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/token")
	if err != nil {
		return nil, fmt.Errorf("failed to read kubernetes service account token: %w", err)
	}
	caPEMBytes, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/ca.crt")
	if err != nil {
		return nil, fmt.Errorf("failed to read kubernetes service account CA: %w", err)
	}

	client, err := newVaultHTTPClient(false, string(caPEMBytes))
	if err != nil {
		return nil, err
	}

	return &kubernetesSecretBackend{
		cfg:    cfg,
		client: client,
		base:   "https://" + netJoinHostPort(host, port),
		token:  strings.TrimSpace(string(tokenBytes)),
	}, nil
}

func (b *kubernetesSecretBackend) Fetch(ctx context.Context) (map[string]string, error) {
	if b == nil || b.cfg == nil || b.cfg.Kubernetes == nil {
		return nil, fmt.Errorf("kubernetes secret backend is not configured")
	}

	namespace := strings.TrimSpace(b.cfg.Kubernetes.Namespace)
	if namespace == "" {
		namespace = "default"
	}

	secrets := make(map[string]string)
	syncPaths := b.cfg.SyncPaths
	switch {
	case len(syncPaths) > 0:
		for _, name := range syncPaths {
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			secret, err := b.fetchNamedSecret(ctx, namespace, name)
			if err != nil {
				return nil, err
			}
			for key, value := range secret {
				secrets[key] = value
			}
		}
	case strings.TrimSpace(b.cfg.Kubernetes.LabelSelector) != "":
		values, err := b.fetchSecretsByLabel(ctx, namespace, b.cfg.Kubernetes.LabelSelector)
		if err != nil {
			return nil, err
		}
		for key, value := range values {
			secrets[key] = value
		}
	default:
		return nil, fmt.Errorf("kubernetes vault sync requires sync_paths or kubernetes.label_selector")
	}

	return secrets, nil
}

func (b *kubernetesSecretBackend) fetchNamedSecret(ctx context.Context, namespace, name string) (map[string]string, error) {
	endpoint := fmt.Sprintf("%s/api/v1/namespaces/%s/secrets/%s", b.base, url.PathEscape(namespace), url.PathEscape(name))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+b.token)

	resp, err := b.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("failed to fetch kubernetes secret %s/%s: status %d", namespace, name, resp.StatusCode)
	}

	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read kubernetes secret %s/%s: %w", namespace, name, err)
	}
	var secret kubernetesSecret
	if err := sonic.Unmarshal(payload, &secret); err != nil {
		return nil, fmt.Errorf("failed to decode kubernetes secret %s/%s: %w", namespace, name, err)
	}
	return decodeKubernetesSecretData(secret.Data)
}

func (b *kubernetesSecretBackend) fetchSecretsByLabel(ctx context.Context, namespace, labelSelector string) (map[string]string, error) {
	values := url.Values{}
	values.Set("labelSelector", labelSelector)
	endpoint := fmt.Sprintf("%s/api/v1/namespaces/%s/secrets?%s", b.base, url.PathEscape(namespace), values.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+b.token)

	resp, err := b.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("failed to list kubernetes secrets for %s: status %d", namespace, resp.StatusCode)
	}

	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read kubernetes secret list: %w", err)
	}
	var secretList kubernetesSecretList
	if err := sonic.Unmarshal(payload, &secretList); err != nil {
		return nil, fmt.Errorf("failed to decode kubernetes secret list: %w", err)
	}

	secrets := make(map[string]string)
	for _, secret := range secretList.Items {
		decoded, err := decodeKubernetesSecretData(secret.Data)
		if err != nil {
			return nil, fmt.Errorf("failed to decode kubernetes secret %s: %w", secret.Metadata.Name, err)
		}
		for key, value := range decoded {
			secrets[key] = value
		}
	}
	return secrets, nil
}

func decodeKubernetesSecretData(data map[string]string) (map[string]string, error) {
	decoded := make(map[string]string, len(data))
	for key, value := range data {
		name := normalizeVaultEnvName(key)
		if name == "" {
			continue
		}
		raw, err := base64.StdEncoding.DecodeString(value)
		if err != nil {
			return nil, err
		}
		decoded[name] = string(raw)
	}
	return decoded, nil
}

func stringifySecretMap(values map[string]any) map[string]string {
	secrets := make(map[string]string, len(values))
	for key, value := range values {
		name := normalizeVaultEnvName(key)
		if name == "" {
			continue
		}
		secrets[name] = secretValueString(value)
	}
	return secrets
}

func secretValueString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case []byte:
		return string(typed)
	case nil:
		return ""
	default:
		payload, err := sonic.Marshal(typed)
		if err != nil {
			return fmt.Sprintf("%v", typed)
		}
		return string(payload)
	}
}

func providerUsesChangedEnv(config configstore.ProviderConfig, changed map[string]struct{}) bool {
	if len(changed) == 0 {
		return false
	}
	refs := providerEnvRefs(config)
	for ref := range refs {
		if _, ok := changed[ref]; ok {
			return true
		}
	}
	return false
}

func providerEnvRefs(config configstore.ProviderConfig) map[string]struct{} {
	refs := make(map[string]struct{})
	for _, key := range config.Keys {
		collectEnvVarRef(refs, &key.Value)
		if key.AzureKeyConfig != nil {
			collectEnvVarRef(refs, &key.AzureKeyConfig.Endpoint)
			collectEnvVarRef(refs, key.AzureKeyConfig.APIVersion)
			collectEnvVarRef(refs, key.AzureKeyConfig.ClientID)
			collectEnvVarRef(refs, key.AzureKeyConfig.ClientSecret)
			collectEnvVarRef(refs, key.AzureKeyConfig.TenantID)
		}
		if key.VertexKeyConfig != nil {
			collectEnvVarRef(refs, &key.VertexKeyConfig.ProjectID)
			collectEnvVarRef(refs, &key.VertexKeyConfig.ProjectNumber)
			collectEnvVarRef(refs, &key.VertexKeyConfig.Region)
			collectEnvVarRef(refs, &key.VertexKeyConfig.AuthCredentials)
		}
		if key.BedrockKeyConfig != nil {
			collectEnvVarRef(refs, &key.BedrockKeyConfig.AccessKey)
			collectEnvVarRef(refs, &key.BedrockKeyConfig.SecretKey)
			collectEnvVarRef(refs, key.BedrockKeyConfig.SessionToken)
			collectEnvVarRef(refs, key.BedrockKeyConfig.Region)
			collectEnvVarRef(refs, key.BedrockKeyConfig.ARN)
			collectEnvVarRef(refs, key.BedrockKeyConfig.RoleARN)
			collectEnvVarRef(refs, key.BedrockKeyConfig.ExternalID)
			collectEnvVarRef(refs, key.BedrockKeyConfig.RoleSessionName)
		}
		if key.VLLMKeyConfig != nil {
			collectEnvVarRef(refs, &key.VLLMKeyConfig.URL)
		}
	}
	return refs
}

func collectEnvVarRef(refs map[string]struct{}, value *schemas.EnvVar) {
	if value == nil {
		return
	}
	ref := envRefName(value)
	if ref == "" {
		return
	}
	refs[ref] = struct{}{}
}

func envRefName(value *schemas.EnvVar) string {
	if value == nil || !value.FromEnv {
		return ""
	}
	if envRef := strings.TrimSpace(value.EnvVar); strings.HasPrefix(envRef, "env.") {
		return strings.TrimPrefix(envRef, "env.")
	}
	return ""
}

func refreshProviderConfigEnvVars(config configstore.ProviderConfig) configstore.ProviderConfig {
	refreshed := config
	refreshed.Keys = make([]schemas.Key, len(config.Keys))
	for i, key := range config.Keys {
		refreshed.Keys[i] = refreshKeyEnvVars(key)
	}
	return refreshed
}

func refreshKeyEnvVars(key schemas.Key) schemas.Key {
	refreshed := key
	refreshed.Value = refreshEnvVarValue(&key.Value)
	if key.AzureKeyConfig != nil {
		azure := *key.AzureKeyConfig
		azure.Endpoint = refreshEnvVarValue(&key.AzureKeyConfig.Endpoint)
		if key.AzureKeyConfig.APIVersion != nil {
			value := refreshEnvVarValue(key.AzureKeyConfig.APIVersion)
			azure.APIVersion = &value
		}
		if key.AzureKeyConfig.ClientID != nil {
			value := refreshEnvVarValue(key.AzureKeyConfig.ClientID)
			azure.ClientID = &value
		}
		if key.AzureKeyConfig.ClientSecret != nil {
			value := refreshEnvVarValue(key.AzureKeyConfig.ClientSecret)
			azure.ClientSecret = &value
		}
		if key.AzureKeyConfig.TenantID != nil {
			value := refreshEnvVarValue(key.AzureKeyConfig.TenantID)
			azure.TenantID = &value
		}
		refreshed.AzureKeyConfig = &azure
	}
	if key.VertexKeyConfig != nil {
		vertex := *key.VertexKeyConfig
		vertex.ProjectID = refreshEnvVarValue(&key.VertexKeyConfig.ProjectID)
		vertex.ProjectNumber = refreshEnvVarValue(&key.VertexKeyConfig.ProjectNumber)
		vertex.Region = refreshEnvVarValue(&key.VertexKeyConfig.Region)
		vertex.AuthCredentials = refreshEnvVarValue(&key.VertexKeyConfig.AuthCredentials)
		refreshed.VertexKeyConfig = &vertex
	}
	if key.BedrockKeyConfig != nil {
		bedrock := *key.BedrockKeyConfig
		bedrock.AccessKey = refreshEnvVarValue(&key.BedrockKeyConfig.AccessKey)
		bedrock.SecretKey = refreshEnvVarValue(&key.BedrockKeyConfig.SecretKey)
		if key.BedrockKeyConfig.SessionToken != nil {
			value := refreshEnvVarValue(key.BedrockKeyConfig.SessionToken)
			bedrock.SessionToken = &value
		}
		if key.BedrockKeyConfig.Region != nil {
			value := refreshEnvVarValue(key.BedrockKeyConfig.Region)
			bedrock.Region = &value
		}
		if key.BedrockKeyConfig.ARN != nil {
			value := refreshEnvVarValue(key.BedrockKeyConfig.ARN)
			bedrock.ARN = &value
		}
		if key.BedrockKeyConfig.RoleARN != nil {
			value := refreshEnvVarValue(key.BedrockKeyConfig.RoleARN)
			bedrock.RoleARN = &value
		}
		if key.BedrockKeyConfig.ExternalID != nil {
			value := refreshEnvVarValue(key.BedrockKeyConfig.ExternalID)
			bedrock.ExternalID = &value
		}
		if key.BedrockKeyConfig.RoleSessionName != nil {
			value := refreshEnvVarValue(key.BedrockKeyConfig.RoleSessionName)
			bedrock.RoleSessionName = &value
		}
		refreshed.BedrockKeyConfig = &bedrock
	}
	if key.VLLMKeyConfig != nil {
		vllm := *key.VLLMKeyConfig
		vllm.URL = refreshEnvVarValue(&key.VLLMKeyConfig.URL)
		refreshed.VLLMKeyConfig = &vllm
	}
	return refreshed
}

func refreshEnvVarValue(value *schemas.EnvVar) schemas.EnvVar {
	if value == nil {
		return schemas.EnvVar{}
	}
	if value.FromEnv && strings.TrimSpace(value.EnvVar) != "" {
		return *schemas.NewEnvVar(value.EnvVar)
	}
	return *schemas.NewEnvVar(value.Val)
}

func applyVaultDeprecation(config *configstore.ProviderConfig, managedNames map[string]struct{}) {
	if config == nil {
		return
	}
	for i := range config.Keys {
		missing := keyMissingManagedEnvRefs(config.Keys[i], managedNames)
		if len(missing) > 0 {
			enabled := false
			config.Keys[i].Enabled = &enabled
			config.Keys[i].Description = vaultAutoDisabledDescriptionPrefix + strings.Join(missing, ",")
			continue
		}
		if strings.HasPrefix(config.Keys[i].Description, vaultAutoDisabledDescriptionPrefix) {
			enabled := true
			config.Keys[i].Enabled = &enabled
			config.Keys[i].Description = ""
		}
	}
}

func keyMissingManagedEnvRefs(key schemas.Key, managedNames map[string]struct{}) []string {
	missing := make([]string, 0)
	addIfMissing := func(value *schemas.EnvVar) {
		ref := envRefName(value)
		if ref == "" {
			return
		}
		if len(managedNames) > 0 {
			if _, ok := managedNames[ref]; !ok {
				return
			}
		}
		if current, ok := os.LookupEnv(ref); !ok || current == "" {
			missing = append(missing, ref)
		}
	}

	addIfMissing(&key.Value)
	if key.AzureKeyConfig != nil {
		addIfMissing(&key.AzureKeyConfig.Endpoint)
		addIfMissing(key.AzureKeyConfig.ClientID)
		addIfMissing(key.AzureKeyConfig.ClientSecret)
		addIfMissing(key.AzureKeyConfig.TenantID)
	}
	if key.VertexKeyConfig != nil {
		addIfMissing(&key.VertexKeyConfig.ProjectID)
		addIfMissing(&key.VertexKeyConfig.ProjectNumber)
		addIfMissing(&key.VertexKeyConfig.Region)
		addIfMissing(&key.VertexKeyConfig.AuthCredentials)
	}
	if key.BedrockKeyConfig != nil {
		addIfMissing(&key.BedrockKeyConfig.AccessKey)
		addIfMissing(&key.BedrockKeyConfig.SecretKey)
		addIfMissing(key.BedrockKeyConfig.SessionToken)
		addIfMissing(key.BedrockKeyConfig.Region)
		addIfMissing(key.BedrockKeyConfig.ARN)
		addIfMissing(key.BedrockKeyConfig.RoleARN)
		addIfMissing(key.BedrockKeyConfig.ExternalID)
		addIfMissing(key.BedrockKeyConfig.RoleSessionName)
	}
	if key.VLLMKeyConfig != nil {
		addIfMissing(&key.VLLMKeyConfig.URL)
	}

	sort.Strings(missing)
	return missing
}

func normalizeVaultEnvName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	return strings.ToUpper(strings.ReplaceAll(name, "-", "_"))
}

func joinErrors(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return strings.Join(values, "; ")
}

func sortedKeysFromSet(values map[string]struct{}) []string {
	if len(values) == 0 {
		return nil
	}
	result := make([]string, 0, len(values))
	for key := range values {
		result = append(result, key)
	}
	sort.Strings(result)
	return result
}

func netJoinHostPort(host, port string) string {
	if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
		host = "[" + host + "]"
	}
	return host + ":" + port
}
