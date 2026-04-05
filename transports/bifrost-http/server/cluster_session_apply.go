package server

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/maximhq/bifrost/framework/configstore"
	configstoreTables "github.com/maximhq/bifrost/framework/configstore/tables"
)

func (s *BifrostHTTPServer) ApplyClusterSessionConfig(ctx context.Context, token string, session *configstoreTables.SessionsTable, deleteSession bool) error {
	if s == nil || s.Config == nil || s.Config.ConfigStore == nil {
		return fmt.Errorf("config store not found")
	}

	token = strings.TrimSpace(token)
	if token == "" && session != nil {
		token = strings.TrimSpace(session.Token)
	}
	if token == "" {
		return fmt.Errorf("session token is required")
	}

	if deleteSession {
		if err := s.Config.ConfigStore.DeleteSession(ctx, token); err != nil && !errors.Is(err, configstore.ErrNotFound) {
			return fmt.Errorf("failed to delete session: %w", err)
		}
		return nil
	}
	if session == nil {
		return fmt.Errorf("session config is required")
	}

	if existing, err := s.Config.ConfigStore.GetSession(ctx, token); err != nil && !errors.Is(err, configstore.ErrNotFound) {
		return fmt.Errorf("failed to get existing session: %w", err)
	} else if existing != nil {
		if err := s.Config.ConfigStore.DeleteSession(ctx, token); err != nil && !errors.Is(err, configstore.ErrNotFound) {
			return fmt.Errorf("failed to replace existing session: %w", err)
		}
	}

	record := &configstoreTables.SessionsTable{
		Token:     token,
		ExpiresAt: session.ExpiresAt,
		CreatedAt: session.CreatedAt,
		UpdatedAt: session.UpdatedAt,
	}
	if err := s.Config.ConfigStore.CreateSession(ctx, record); err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	return nil
}
