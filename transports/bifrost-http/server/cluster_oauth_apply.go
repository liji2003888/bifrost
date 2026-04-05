package server

import (
	"context"
	"fmt"
	"strings"

	"github.com/maximhq/bifrost/transports/bifrost-http/handlers"
)

func (s *BifrostHTTPServer) ApplyClusterOAuthConfig(ctx context.Context, id string, cfg *handlers.ClusterOAuthConfig) error {
	if s == nil || s.Config == nil || s.Config.ConfigStore == nil {
		return fmt.Errorf("config store not found")
	}

	id = strings.TrimSpace(id)
	if id == "" && cfg != nil {
		id = strings.TrimSpace(cfg.ID)
	}
	if id == "" {
		return fmt.Errorf("oauth config id is required")
	}
	if cfg == nil {
		return fmt.Errorf("oauth config is required")
	}

	record := cfg.ToTable()
	record.ID = id
	if existing, err := s.Config.ConfigStore.GetOauthConfigByID(ctx, id); err != nil {
		return fmt.Errorf("failed to get existing oauth config: %w", err)
	} else if existing == nil {
		if err := s.Config.ConfigStore.CreateOauthConfig(ctx, record); err != nil {
			return fmt.Errorf("failed to create oauth config: %w", err)
		}
	} else {
		if err := s.Config.ConfigStore.UpdateOauthConfig(ctx, record); err != nil {
			return fmt.Errorf("failed to update oauth config: %w", err)
		}
	}

	return nil
}

func (s *BifrostHTTPServer) ApplyClusterOAuthToken(ctx context.Context, id string, token *handlers.ClusterOAuthToken, deleteToken bool) error {
	if s == nil || s.Config == nil || s.Config.ConfigStore == nil {
		return fmt.Errorf("config store not found")
	}

	id = strings.TrimSpace(id)
	if id == "" && token != nil {
		id = strings.TrimSpace(token.ID)
	}
	if id == "" {
		return fmt.Errorf("oauth token id is required")
	}

	if deleteToken {
		if err := s.Config.ConfigStore.DeleteOauthToken(ctx, id); err != nil {
			return fmt.Errorf("failed to delete oauth token: %w", err)
		}
		return nil
	}
	if token == nil {
		return fmt.Errorf("oauth token is required")
	}

	record := token.ToTable()
	record.ID = id
	if existing, err := s.Config.ConfigStore.GetOauthTokenByID(ctx, id); err != nil {
		return fmt.Errorf("failed to get existing oauth token: %w", err)
	} else if existing == nil {
		if err := s.Config.ConfigStore.CreateOauthToken(ctx, record); err != nil {
			return fmt.Errorf("failed to create oauth token: %w", err)
		}
	} else {
		if err := s.Config.ConfigStore.UpdateOauthToken(ctx, record); err != nil {
			return fmt.Errorf("failed to update oauth token: %w", err)
		}
	}

	return nil
}
