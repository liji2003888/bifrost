package server

import (
	"context"
	"errors"
	"testing"
	"time"

	bifrost "github.com/maximhq/bifrost/core"
	"github.com/maximhq/bifrost/framework/configstore"
	configstoreTables "github.com/maximhq/bifrost/framework/configstore/tables"
	"github.com/maximhq/bifrost/transports/bifrost-http/lib"
)

func TestApplyClusterPromptRepositoryConfig(t *testing.T) {
	SetLogger(bifrost.NewNoOpLogger())

	store := newClusterPluginApplyStore(t)
	server := &BifrostHTTPServer{
		Config: &lib.Config{ConfigStore: store},
	}

	ctx := context.Background()

	description := "Shared prompt library"
	if err := server.ApplyClusterFolderConfig(ctx, "folder-1", &configstoreTables.TableFolder{
		ID:          "folder-1",
		Name:        "Shared",
		Description: &description,
		CreatedAt:   time.Unix(1700000000, 0).UTC(),
		UpdatedAt:   time.Unix(1700000000, 0).UTC(),
	}, false); err != nil {
		t.Fatalf("ApplyClusterFolderConfig() error = %v", err)
	}

	if err := server.ApplyClusterPromptConfig(ctx, "prompt-1", &configstoreTables.TablePrompt{
		ID:        "prompt-1",
		Name:      "Support Reply",
		FolderID:  bifrost.Ptr("folder-1"),
		CreatedAt: time.Unix(1700000001, 0).UTC(),
		UpdatedAt: time.Unix(1700000001, 0).UTC(),
	}, false); err != nil {
		t.Fatalf("ApplyClusterPromptConfig() error = %v", err)
	}

	versionCreatedAt := time.Unix(1700000002, 0).UTC()
	if err := server.ApplyClusterPromptVersionConfig(ctx, 41, &configstoreTables.TablePromptVersion{
		ID:            41,
		PromptID:      "prompt-1",
		VersionNumber: 3,
		CommitMessage: "Imported",
		ModelParams:   configstoreTables.ModelParams{"temperature": 0.15},
		Provider:      "openai",
		Model:         "gpt-4.1",
		IsLatest:      true,
		CreatedAt:     versionCreatedAt,
		Messages: []configstoreTables.TablePromptVersionMessage{
			{ID: 501, PromptID: "prompt-1", VersionID: 41, OrderIndex: 0, Message: configstoreTables.PromptMessage(`{"role":"system","content":"You are concise."}`)},
			{ID: 502, PromptID: "prompt-1", VersionID: 41, OrderIndex: 1, Message: configstoreTables.PromptMessage(`{"role":"user","content":"Hi"}`)},
		},
	}, false); err != nil {
		t.Fatalf("ApplyClusterPromptVersionConfig() error = %v", err)
	}

	versionID := uint(41)
	if err := server.ApplyClusterPromptSessionConfig(ctx, 71, &configstoreTables.TablePromptSession{
		ID:          71,
		PromptID:    "prompt-1",
		VersionID:   &versionID,
		Name:        "Draft 1",
		ModelParams: configstoreTables.ModelParams{"temperature": 0.2},
		Provider:    "openai",
		Model:       "gpt-4.1",
		CreatedAt:   time.Unix(1700000003, 0).UTC(),
		UpdatedAt:   time.Unix(1700000004, 0).UTC(),
		Messages: []configstoreTables.TablePromptSessionMessage{
			{ID: 801, PromptID: "prompt-1", SessionID: 71, OrderIndex: 0, Message: configstoreTables.PromptMessage(`{"role":"assistant","content":"Draft reply"}`)},
		},
	}, false); err != nil {
		t.Fatalf("ApplyClusterPromptSessionConfig(create) error = %v", err)
	}

	storedFolder, err := store.GetFolderByID(ctx, "folder-1")
	if err != nil {
		t.Fatalf("GetFolderByID() error = %v", err)
	}
	if storedFolder.Name != "Shared" || storedFolder.Description == nil || *storedFolder.Description != description {
		t.Fatalf("unexpected stored folder: %+v", storedFolder)
	}

	storedPrompt, err := store.GetPromptByID(ctx, "prompt-1")
	if err != nil {
		t.Fatalf("GetPromptByID() error = %v", err)
	}
	if storedPrompt.FolderID == nil || *storedPrompt.FolderID != "folder-1" {
		t.Fatalf("unexpected stored prompt folder: %+v", storedPrompt)
	}

	storedVersion, err := store.GetPromptVersionByID(ctx, 41)
	if err != nil {
		t.Fatalf("GetPromptVersionByID() error = %v", err)
	}
	if storedVersion.VersionNumber != 3 || !storedVersion.IsLatest || len(storedVersion.Messages) != 2 {
		t.Fatalf("unexpected stored version: %+v", storedVersion)
	}

	storedSession, err := store.GetPromptSessionByID(ctx, 71)
	if err != nil {
		t.Fatalf("GetPromptSessionByID() error = %v", err)
	}
	if storedSession.VersionID == nil || *storedSession.VersionID != 41 || storedSession.Name != "Draft 1" || len(storedSession.Messages) != 1 {
		t.Fatalf("unexpected stored session: %+v", storedSession)
	}

	if err := server.ApplyClusterPromptSessionConfig(ctx, 71, &configstoreTables.TablePromptSession{
		ID:          71,
		PromptID:    "prompt-1",
		VersionID:   &versionID,
		Name:        "Draft 2",
		ModelParams: configstoreTables.ModelParams{"temperature": 0.25},
		Provider:    "openai",
		Model:       "gpt-4.1",
		CreatedAt:   time.Unix(1700000003, 0).UTC(),
		UpdatedAt:   time.Unix(1700000005, 0).UTC(),
		Messages: []configstoreTables.TablePromptSessionMessage{
			{ID: 802, PromptID: "prompt-1", SessionID: 71, OrderIndex: 0, Message: configstoreTables.PromptMessage(`{"role":"assistant","content":"Updated draft"}`)},
		},
	}, false); err != nil {
		t.Fatalf("ApplyClusterPromptSessionConfig(update) error = %v", err)
	}

	storedSession, err = store.GetPromptSessionByID(ctx, 71)
	if err != nil {
		t.Fatalf("GetPromptSessionByID(after update) error = %v", err)
	}
	if storedSession.Name != "Draft 2" || len(storedSession.Messages) != 1 || string(storedSession.Messages[0].Message) != `{"role":"assistant","content":"Updated draft"}` {
		t.Fatalf("unexpected updated session: %+v", storedSession)
	}

	if err := server.ApplyClusterPromptSessionConfig(ctx, 71, nil, true); err != nil {
		t.Fatalf("ApplyClusterPromptSessionConfig(delete) error = %v", err)
	}
	if _, err := store.GetPromptSessionByID(ctx, 71); !errors.Is(err, configstore.ErrNotFound) {
		t.Fatalf("expected session to be deleted, got err=%v", err)
	}
}
