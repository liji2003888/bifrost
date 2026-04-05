package handlers

import (
	"context"
	"testing"
	"time"

	bifrost "github.com/maximhq/bifrost/core"
	"github.com/maximhq/bifrost/framework/configstore"
	configstoreTables "github.com/maximhq/bifrost/framework/configstore/tables"
	"github.com/maximhq/bifrost/transports/bifrost-http/lib"
	"github.com/valyala/fasthttp"
)

type oauthClusterTestStore struct {
	configstore.ConfigStore
	config *configstoreTables.TableOauthConfig
}

func (s *oauthClusterTestStore) GetOauthConfigByState(_ context.Context, state string) (*configstoreTables.TableOauthConfig, error) {
	if s.config == nil || s.config.State != state {
		return nil, nil
	}
	cloned := *s.config
	return &cloned, nil
}

func (s *oauthClusterTestStore) UpdateOauthConfig(_ context.Context, config *configstoreTables.TableOauthConfig) error {
	if config == nil {
		s.config = nil
		return nil
	}
	cloned := *config
	s.config = &cloned
	return nil
}

func TestHandleCallbackErrorPropagatesOAuthConfigClusterChange(t *testing.T) {
	SetLogger(bifrost.NewNoOpLogger())

	store := &oauthClusterTestStore{
		config: &configstoreTables.TableOauthConfig{
			ID:          "oauth-config-1",
			State:       "oauth-state",
			Status:      "pending",
			ExpiresAt:   time.Now().Add(10 * time.Minute),
			RedirectURI: "https://gateway.example.com/api/oauth/callback",
		},
	}
	propagator := &fakeClusterConfigPropagator{}
	handler := NewOAuthHandler(nil, nil, &lib.Config{ConfigStore: store}, propagator)

	ctx := &fasthttp.RequestCtx{}
	handler.handleCallbackError(ctx, "oauth-state", "access_denied", "user denied access")

	if store.config == nil || store.config.Status != "failed" {
		t.Fatalf("expected oauth config status to be updated to failed, got %+v", store.config)
	}
	if len(propagator.changes) != 1 {
		t.Fatalf("expected one propagated oauth config change, got %+v", propagator.changes)
	}
	change := propagator.changes[0]
	if change.Scope != ClusterConfigScopeOAuthConfig || change.OAuthConfigID != "oauth-config-1" {
		t.Fatalf("unexpected propagated oauth config change: %+v", change)
	}
	if change.OAuthConfig == nil || change.OAuthConfig.Status != "failed" {
		t.Fatalf("expected propagated oauth config to be failed, got %+v", change.OAuthConfig)
	}
}
