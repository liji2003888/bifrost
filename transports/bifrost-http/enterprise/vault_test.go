package enterprise

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	bifrost "github.com/maximhq/bifrost/core"
	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework/configstore"
)

type fakeVaultBackend struct {
	responses []map[string]string
	err       error
	calls     int
}

func (f *fakeVaultBackend) Fetch(_ context.Context) (map[string]string, error) {
	if f.err != nil {
		return nil, f.err
	}
	if len(f.responses) == 0 {
		return map[string]string{}, nil
	}
	index := f.calls
	if index >= len(f.responses) {
		index = len(f.responses) - 1
	}
	f.calls++

	result := make(map[string]string, len(f.responses[index]))
	for key, value := range f.responses[index] {
		result[key] = value
	}
	return result, nil
}

type fakeVaultProviderRefresher struct {
	providers map[schemas.ModelProvider]configstore.ProviderConfig
	updated   map[schemas.ModelProvider]configstore.ProviderConfig
}

func (f *fakeVaultProviderRefresher) SnapshotProviders() map[schemas.ModelProvider]configstore.ProviderConfig {
	result := make(map[schemas.ModelProvider]configstore.ProviderConfig, len(f.providers))
	for provider, providerConfig := range f.providers {
		result[provider] = providerConfig
	}
	return result
}

func (f *fakeVaultProviderRefresher) UpdateProviderConfig(_ context.Context, provider schemas.ModelProvider, config configstore.ProviderConfig) error {
	if f.updated == nil {
		f.updated = make(map[schemas.ModelProvider]configstore.ProviderConfig)
	}
	f.updated[provider] = config
	f.providers[provider] = config
	return nil
}

func TestVaultServiceSyncsSecretsAndReloadsProviders(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "sk-old")

	refresher := &fakeVaultProviderRefresher{
		providers: map[schemas.ModelProvider]configstore.ProviderConfig{
			schemas.OpenAI: {
				Keys: []schemas.Key{
					{
						ID:    "key-1",
						Name:  "primary",
						Value: *schemas.NewEnvVar("env.OPENAI_API_KEY"),
					},
				},
			},
		},
	}
	service, err := newVaultService(&VaultConfig{
		Enabled: true,
		Type:    VaultTypeHashicorp,
	}, refresher, nil, bifrost.NewNoOpLogger(), &fakeVaultBackend{
		responses: []map[string]string{
			{"OPENAI_API_KEY": "sk-vault"},
		},
	})
	if err != nil {
		t.Fatalf("newVaultService() error = %v", err)
	}

	service.syncOnce(context.Background())

	if got := os.Getenv("OPENAI_API_KEY"); got != "sk-vault" {
		t.Fatalf("expected synced env var to be updated, got %q", got)
	}

	updated, ok := refresher.updated[schemas.OpenAI]
	if !ok {
		t.Fatal("expected provider to be reloaded after secret sync")
	}
	if updated.Keys[0].Value.GetValue() != "sk-vault" {
		t.Fatalf("expected provider key to pick up synced env value, got %q", updated.Keys[0].Value.GetValue())
	}
}

func TestVaultServiceAutoDeprecatesMissingManagedSecrets(t *testing.T) {
	refresher := &fakeVaultProviderRefresher{
		providers: map[schemas.ModelProvider]configstore.ProviderConfig{
			schemas.OpenAI: {
				Keys: []schemas.Key{
					{
						ID:    "key-1",
						Name:  "primary",
						Value: *schemas.NewEnvVar("env.OPENAI_API_KEY"),
					},
				},
			},
		},
	}
	service, err := newVaultService(&VaultConfig{
		Enabled:       true,
		Type:          VaultTypeHashicorp,
		AutoDeprecate: true,
	}, refresher, nil, bifrost.NewNoOpLogger(), &fakeVaultBackend{
		responses: []map[string]string{
			{"OPENAI_API_KEY": "sk-vault"},
			{},
		},
	})
	if err != nil {
		t.Fatalf("newVaultService() error = %v", err)
	}

	service.syncOnce(context.Background())
	service.syncOnce(context.Background())

	if _, ok := os.LookupEnv("OPENAI_API_KEY"); ok {
		t.Fatal("expected managed env var to be unset when secret disappears")
	}

	updated := refresher.providers[schemas.OpenAI]
	if updated.Keys[0].Enabled == nil || *updated.Keys[0].Enabled {
		t.Fatalf("expected key to be auto-disabled, got %+v", updated.Keys[0].Enabled)
	}
	if updated.Keys[0].Description != vaultAutoDisabledDescriptionPrefix+"OPENAI_API_KEY" {
		t.Fatalf("unexpected auto-disabled description: %q", updated.Keys[0].Description)
	}
}

func TestHashicorpVaultBackendFetchesKVv2Secrets(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Vault-Token") != "vault-token" {
			t.Fatalf("expected vault token header to be forwarded")
		}
		if r.URL.Path != "/v1/kv/data/bifrost/providers" {
			t.Fatalf("unexpected vault path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"data":{"data":{"OPENAI_API_KEY":"sk-test","JSON_SECRET":{"enabled":true}}}}`))
	}))
	defer server.Close()

	backend := &hashicorpVaultBackend{
		cfg: &VaultConfig{
			Type:      VaultTypeHashicorp,
			SyncPaths: []string{"bifrost/providers"},
			Hashicorp: &HashicorpVaultConfig{
				Address: server.URL,
				Mount:   "kv",
				Token:   schemas.NewEnvVar("vault-token"),
			},
		},
		client: server.Client(),
	}

	secrets, err := backend.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if secrets["OPENAI_API_KEY"] != "sk-test" {
		t.Fatalf("expected OPENAI_API_KEY to be fetched, got %+v", secrets)
	}
	if secrets["JSON_SECRET"] != `{"enabled":true}` {
		t.Fatalf("expected JSON secret payload to be stringified, got %q", secrets["JSON_SECRET"])
	}
}

func TestKubernetesSecretBackendFetchesNamedSecrets(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer cluster-token" {
			t.Fatalf("expected bearer token header to be forwarded")
		}
		if r.URL.Path != "/api/v1/namespaces/bifrost/secrets/provider-secrets" {
			t.Fatalf("unexpected kubernetes secret path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"data":{"OPENAI_API_KEY":"` + base64.StdEncoding.EncodeToString([]byte("sk-k8s")) + `"}}`))
	}))
	defer server.Close()

	backend := &kubernetesSecretBackend{
		cfg: &VaultConfig{
			Type:      VaultTypeKubernetesSecret,
			SyncPaths: []string{"provider-secrets"},
			Kubernetes: &KubernetesSecretsConfig{
				Namespace: "bifrost",
			},
		},
		client: server.Client(),
		base:   server.URL,
		token:  "cluster-token",
	}

	secrets, err := backend.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if secrets["OPENAI_API_KEY"] != "sk-k8s" {
		t.Fatalf("expected kubernetes secret to be decoded, got %+v", secrets)
	}
}
