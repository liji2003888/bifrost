package server

import (
	"context"

	configstoreTables "github.com/maximhq/bifrost/framework/configstore/tables"
	"github.com/maximhq/bifrost/transports/bifrost-http/handlers"
)

func (s *BifrostHTTPServer) OAuthConfigChanged(ctx context.Context, config *configstoreTables.TableOauthConfig) error {
	if s == nil || config == nil {
		return nil
	}
	return s.PropagateClusterConfigChange(ctx, &handlers.ClusterConfigChange{
		Scope:         handlers.ClusterConfigScopeOAuthConfig,
		OAuthConfigID: config.ID,
		OAuthConfig:   handlers.CloneClusterOAuthConfig(config),
	})
}

func (s *BifrostHTTPServer) OAuthTokenChanged(ctx context.Context, token *configstoreTables.TableOauthToken, deleted bool) error {
	if s == nil || token == nil {
		return nil
	}
	change := &handlers.ClusterConfigChange{
		Scope:        handlers.ClusterConfigScopeOAuthToken,
		OAuthTokenID: token.ID,
		Delete:       deleted,
	}
	if !deleted {
		change.OAuthToken = handlers.CloneClusterOAuthToken(token)
	}
	return s.PropagateClusterConfigChange(ctx, change)
}
