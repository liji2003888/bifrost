package server

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/maximhq/bifrost/framework/configstore"
	configstoreTables "github.com/maximhq/bifrost/framework/configstore/tables"
	dynamicPlugins "github.com/maximhq/bifrost/framework/plugins"
	"github.com/maximhq/bifrost/transports/bifrost-http/handlers"
	"github.com/maximhq/bifrost/transports/bifrost-http/lib"
)

func (s *BifrostHTTPServer) ApplyClusterPluginConfig(ctx context.Context, name string, cfg *configstoreTables.TablePlugin, deletePlugin bool) error {
	if s == nil || s.Config == nil || s.Config.ConfigStore == nil {
		return fmt.Errorf("config store not found")
	}

	name = strings.TrimSpace(name)
	if name == "" && cfg != nil {
		name = strings.TrimSpace(cfg.Name)
	}
	if name == "" {
		return fmt.Errorf("plugin name is required")
	}
	if !lib.IsBuiltinPlugin(name) {
		return fmt.Errorf("cluster plugin sync only supports built-in plugins: %s", name)
	}

	if deletePlugin {
		if err := s.Config.ConfigStore.DeletePlugin(ctx, name); err != nil && !errors.Is(err, configstore.ErrNotFound) {
			return fmt.Errorf("failed to delete plugin config: %w", err)
		}
		if err := s.RemovePlugin(ctx, name); err != nil && !errors.Is(err, dynamicPlugins.ErrPluginNotFound) {
			return fmt.Errorf("failed to remove plugin from runtime: %w", err)
		}
		s.Config.DeletePluginOverallStatus(name)
		return nil
	}
	if cfg == nil {
		return fmt.Errorf("plugin config is required")
	}
	if cfg.Path != nil && strings.TrimSpace(*cfg.Path) != "" {
		return fmt.Errorf("cluster plugin sync does not support custom plugin paths for %s", name)
	}
	if cfg.IsCustom {
		return fmt.Errorf("cluster plugin sync does not support custom plugins: %s", name)
	}

	row := clusterBuiltinPluginRecord(name, cfg)
	if existing, err := s.Config.ConfigStore.GetPlugin(ctx, name); err != nil {
		if !errors.Is(err, configstore.ErrNotFound) {
			return fmt.Errorf("failed to get existing plugin config: %w", err)
		}
		if err := s.Config.ConfigStore.CreatePlugin(ctx, row); err != nil {
			return fmt.Errorf("failed to create plugin config: %w", err)
		}
	} else {
		row.ID = existing.ID
		if err := s.Config.ConfigStore.UpdatePlugin(ctx, row); err != nil {
			return fmt.Errorf("failed to update plugin config: %w", err)
		}
	}

	if row.Enabled {
		if err := s.ReloadPlugin(ctx, name, nil, row.Config, row.Placement, row.Order); err != nil {
			return err
		}
		return nil
	}

	disabledCtx := context.WithValue(ctx, handlers.PluginDisabledKey, true)
	if err := s.RemovePlugin(disabledCtx, name); err != nil && !errors.Is(err, dynamicPlugins.ErrPluginNotFound) {
		return fmt.Errorf("failed to disable plugin %s: %w", name, err)
	}
	if err := s.markPluginDisabled(name); err != nil {
		if logger != nil {
			logger.Warn("failed to mark plugin %s disabled after cluster sync: %v", name, err)
		}
	}
	return nil
}

func clusterBuiltinPluginRecord(name string, cfg *configstoreTables.TablePlugin) *configstoreTables.TablePlugin {
	record := &configstoreTables.TablePlugin{
		Name:       strings.TrimSpace(name),
		Enabled:    cfg.Enabled,
		Config:     cfg.Config,
		Path:       nil,
		IsCustom:   false,
		Placement:  cfg.Placement,
		Order:      cfg.Order,
		Version:    cfg.Version,
		ConfigHash: cfg.ConfigHash,
		CreatedAt:  cfg.CreatedAt,
		UpdatedAt:  cfg.UpdatedAt,
	}
	return record
}
