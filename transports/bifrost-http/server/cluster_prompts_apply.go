package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/maximhq/bifrost/framework/configstore"
	configstoreTables "github.com/maximhq/bifrost/framework/configstore/tables"
	"github.com/maximhq/bifrost/transports/bifrost-http/lib"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (s *BifrostHTTPServer) ApplyClusterFolderConfig(ctx context.Context, id string, cfg *configstoreTables.TableFolder, deleteFolder bool) error {
	if s == nil || s.Config == nil || s.Config.ConfigStore == nil {
		return fmt.Errorf("config store not found")
	}

	id = strings.TrimSpace(id)
	if id == "" && cfg != nil {
		id = strings.TrimSpace(cfg.ID)
	}
	if id == "" {
		return fmt.Errorf("folder id is required")
	}

	if deleteFolder {
		if err := s.Config.ConfigStore.DeleteFolder(ctx, id); err != nil && !errors.Is(err, configstore.ErrNotFound) {
			return fmt.Errorf("failed to delete folder: %w", err)
		}
		return nil
	}
	if cfg == nil {
		return fmt.Errorf("folder config is required")
	}

	db, err := clusterPromptRepoDB(s.Config.ConfigStore)
	if err != nil {
		return err
	}

	record := clusterPromptFolderRecord(cfg)
	record.ID = id
	return db.WithContext(ctx).Clauses(clause.OnConflict{UpdateAll: true}).Create(record).Error
}

func (s *BifrostHTTPServer) ApplyClusterPromptConfig(ctx context.Context, id string, cfg *configstoreTables.TablePrompt, deletePrompt bool) error {
	if s == nil || s.Config == nil || s.Config.ConfigStore == nil {
		return fmt.Errorf("config store not found")
	}

	id = strings.TrimSpace(id)
	if id == "" && cfg != nil {
		id = strings.TrimSpace(cfg.ID)
	}
	if id == "" {
		return fmt.Errorf("prompt id is required")
	}

	if deletePrompt {
		if err := s.Config.ConfigStore.DeletePrompt(ctx, id); err != nil && !errors.Is(err, configstore.ErrNotFound) {
			return fmt.Errorf("failed to delete prompt: %w", err)
		}
		return nil
	}
	if cfg == nil {
		return fmt.Errorf("prompt config is required")
	}
	if err := s.waitForClusterFolder(ctx, cfg.FolderID); err != nil {
		return err
	}

	db, err := clusterPromptRepoDB(s.Config.ConfigStore)
	if err != nil {
		return err
	}

	record := clusterPromptRecord(cfg)
	record.ID = id
	return db.WithContext(ctx).Clauses(clause.OnConflict{UpdateAll: true}).Create(record).Error
}

func (s *BifrostHTTPServer) ApplyClusterPromptVersionConfig(ctx context.Context, id uint, cfg *configstoreTables.TablePromptVersion, deleteVersion bool) error {
	if s == nil || s.Config == nil || s.Config.ConfigStore == nil {
		return fmt.Errorf("config store not found")
	}

	if id == 0 && cfg != nil {
		id = cfg.ID
	}
	if id == 0 {
		return fmt.Errorf("prompt version id is required")
	}

	if deleteVersion {
		if err := s.Config.ConfigStore.DeletePromptVersion(ctx, id); err != nil && !errors.Is(err, configstore.ErrNotFound) {
			return fmt.Errorf("failed to delete prompt version: %w", err)
		}
		return nil
	}
	if cfg == nil {
		return fmt.Errorf("prompt version config is required")
	}
	if err := s.waitForClusterPrompt(ctx, cfg.PromptID); err != nil {
		return err
	}

	db, err := clusterPromptRepoDB(s.Config.ConfigStore)
	if err != nil {
		return err
	}

	record := clusterPromptVersionRecord(cfg)
	record.ID = id
	messages := clusterPromptVersionMessages(cfg.Messages, record.ID, record.PromptID)

	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if record.IsLatest {
			if err := tx.Model(&configstoreTables.TablePromptVersion{}).
				Where("prompt_id = ? AND id <> ?", record.PromptID, record.ID).
				Update("is_latest", false).Error; err != nil {
				return err
			}
		}

		if err := tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(record).Error; err != nil {
			return err
		}
		if err := tx.Where("version_id = ?", record.ID).Delete(&configstoreTables.TablePromptVersionMessage{}).Error; err != nil {
			return err
		}
		for i := range messages {
			if err := tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(&messages[i]).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *BifrostHTTPServer) ApplyClusterPromptSessionConfig(ctx context.Context, id uint, cfg *configstoreTables.TablePromptSession, deleteSession bool) error {
	if s == nil || s.Config == nil || s.Config.ConfigStore == nil {
		return fmt.Errorf("config store not found")
	}

	if id == 0 && cfg != nil {
		id = cfg.ID
	}
	if id == 0 {
		return fmt.Errorf("prompt session id is required")
	}

	if deleteSession {
		if err := s.Config.ConfigStore.DeletePromptSession(ctx, id); err != nil && !errors.Is(err, configstore.ErrNotFound) {
			return fmt.Errorf("failed to delete prompt session: %w", err)
		}
		return nil
	}
	if cfg == nil {
		return fmt.Errorf("prompt session config is required")
	}
	if err := s.waitForClusterPrompt(ctx, cfg.PromptID); err != nil {
		return err
	}
	if err := s.waitForClusterPromptVersion(ctx, cfg.VersionID); err != nil {
		return err
	}

	db, err := clusterPromptRepoDB(s.Config.ConfigStore)
	if err != nil {
		return err
	}

	record := clusterPromptSessionRecord(cfg)
	record.ID = id
	messages := clusterPromptSessionMessages(cfg.Messages, record.ID, record.PromptID)

	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(record).Error; err != nil {
			return err
		}
		if err := tx.Where("session_id = ?", record.ID).Delete(&configstoreTables.TablePromptSessionMessage{}).Error; err != nil {
			return err
		}
		for i := range messages {
			if err := tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(&messages[i]).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func clusterPromptRepoDB(store configstore.ConfigStore) (*gorm.DB, error) {
	if store == nil {
		return nil, fmt.Errorf("config store not found")
	}
	dbProvider, ok := store.(gormDBProvider)
	if !ok || dbProvider.DB() == nil {
		return nil, fmt.Errorf("prompt repository cluster sync requires an RDB config store")
	}
	return dbProvider.DB(), nil
}

func (s *BifrostHTTPServer) waitForClusterFolder(ctx context.Context, folderID *string) error {
	id := strings.TrimSpace(derefString(folderID))
	if id == "" {
		return nil
	}
	_, err := s.Config.ConfigStore.RetryOnNotFound(ctx, func(ctx context.Context) (any, error) {
		return s.Config.ConfigStore.GetFolderByID(ctx, id)
	}, lib.DBLookupMaxRetries, lib.DBLookupDelay)
	if err != nil {
		return fmt.Errorf("folder dependency %s is not available on this node: %w", id, err)
	}
	return nil
}

func (s *BifrostHTTPServer) waitForClusterPrompt(ctx context.Context, promptID string) error {
	id := strings.TrimSpace(promptID)
	if id == "" {
		return fmt.Errorf("prompt id is required")
	}
	_, err := s.Config.ConfigStore.RetryOnNotFound(ctx, func(ctx context.Context) (any, error) {
		return s.Config.ConfigStore.GetPromptByID(ctx, id)
	}, lib.DBLookupMaxRetries, lib.DBLookupDelay)
	if err != nil {
		return fmt.Errorf("prompt dependency %s is not available on this node: %w", id, err)
	}
	return nil
}

func (s *BifrostHTTPServer) waitForClusterPromptVersion(ctx context.Context, versionID *uint) error {
	if versionID == nil || *versionID == 0 {
		return nil
	}
	_, err := s.Config.ConfigStore.RetryOnNotFound(ctx, func(ctx context.Context) (any, error) {
		return s.Config.ConfigStore.GetPromptVersionByID(ctx, *versionID)
	}, lib.DBLookupMaxRetries, lib.DBLookupDelay)
	if err != nil {
		return fmt.Errorf("prompt version dependency %d is not available on this node: %w", *versionID, err)
	}
	return nil
}

func clusterPromptFolderRecord(cfg *configstoreTables.TableFolder) *configstoreTables.TableFolder {
	record := *cfg
	if cfg.Description != nil {
		description := *cfg.Description
		record.Description = &description
	}
	record.PromptsCount = 0
	return &record
}

func clusterPromptRecord(cfg *configstoreTables.TablePrompt) *configstoreTables.TablePrompt {
	record := *cfg
	if cfg.FolderID != nil {
		folderID := *cfg.FolderID
		record.FolderID = &folderID
	}
	record.Folder = nil
	record.Versions = nil
	record.Sessions = nil
	record.LatestVersion = nil
	return &record
}

func clusterPromptVersionRecord(cfg *configstoreTables.TablePromptVersion) *configstoreTables.TablePromptVersion {
	record := *cfg
	record.Prompt = nil
	record.ModelParams = clusterPromptModelParams(cfg.ModelParams)
	record.Messages = nil
	return &record
}

func clusterPromptSessionRecord(cfg *configstoreTables.TablePromptSession) *configstoreTables.TablePromptSession {
	record := *cfg
	if cfg.VersionID != nil {
		versionID := *cfg.VersionID
		record.VersionID = &versionID
	}
	record.Prompt = nil
	record.Version = nil
	record.ModelParams = clusterPromptModelParams(cfg.ModelParams)
	record.Messages = nil
	return &record
}

func clusterPromptVersionMessages(messages []configstoreTables.TablePromptVersionMessage, versionID uint, promptID string) []configstoreTables.TablePromptVersionMessage {
	if len(messages) == 0 {
		return nil
	}
	cloned := make([]configstoreTables.TablePromptVersionMessage, len(messages))
	for i := range messages {
		cloned[i] = messages[i]
		cloned[i].Version = nil
		cloned[i].VersionID = versionID
		cloned[i].PromptID = promptID
		cloned[i].OrderIndex = i
		cloned[i].Message = clusterPromptMessage(messages[i].Message)
	}
	return cloned
}

func clusterPromptSessionMessages(messages []configstoreTables.TablePromptSessionMessage, sessionID uint, promptID string) []configstoreTables.TablePromptSessionMessage {
	if len(messages) == 0 {
		return nil
	}
	cloned := make([]configstoreTables.TablePromptSessionMessage, len(messages))
	for i := range messages {
		cloned[i] = messages[i]
		cloned[i].Session = nil
		cloned[i].SessionID = sessionID
		cloned[i].PromptID = promptID
		cloned[i].OrderIndex = i
		cloned[i].Message = clusterPromptMessage(messages[i].Message)
	}
	return cloned
}

func clusterPromptModelParams(params configstoreTables.ModelParams) configstoreTables.ModelParams {
	if params == nil {
		return nil
	}
	encoded, err := json.Marshal(params)
	if err != nil {
		cloned := make(configstoreTables.ModelParams, len(params))
		for key, value := range params {
			cloned[key] = value
		}
		return cloned
	}

	var cloned configstoreTables.ModelParams
	if err := json.Unmarshal(encoded, &cloned); err != nil {
		fallback := make(configstoreTables.ModelParams, len(params))
		for key, value := range params {
			fallback[key] = value
		}
		return fallback
	}
	return cloned
}

func clusterPromptMessage(message configstoreTables.PromptMessage) configstoreTables.PromptMessage {
	if len(message) == 0 {
		return nil
	}
	cloned := make([]byte, len(message))
	copy(cloned, message)
	return cloned
}
