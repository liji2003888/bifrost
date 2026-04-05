package server

import (
	"context"
	"testing"
	"time"

	bifrost "github.com/maximhq/bifrost/core"
	configstoreTables "github.com/maximhq/bifrost/framework/configstore/tables"
	"github.com/maximhq/bifrost/transports/bifrost-http/lib"
)

func TestApplyClusterSessionConfig(t *testing.T) {
	SetLogger(bifrost.NewNoOpLogger())

	store := newClusterPluginApplyStore(t)
	server := &BifrostHTTPServer{
		Config: &lib.Config{ConfigStore: store},
	}

	session := &configstoreTables.SessionsTable{
		Token:     "cluster-session-token",
		ExpiresAt: time.Unix(1700004000, 0).UTC(),
		CreatedAt: time.Unix(1700000400, 0).UTC(),
		UpdatedAt: time.Unix(1700000500, 0).UTC(),
	}
	if err := server.ApplyClusterSessionConfig(context.Background(), session.Token, session, false); err != nil {
		t.Fatalf("ApplyClusterSessionConfig(create) error = %v", err)
	}

	stored, err := store.GetSession(context.Background(), session.Token)
	if err != nil {
		t.Fatalf("GetSession() error = %v", err)
	}
	if stored == nil || !stored.ExpiresAt.Equal(session.ExpiresAt) {
		t.Fatalf("unexpected stored session: %+v", stored)
	}

	updated := &configstoreTables.SessionsTable{
		Token:     session.Token,
		ExpiresAt: time.Unix(1700010000, 0).UTC(),
		CreatedAt: session.CreatedAt,
		UpdatedAt: time.Unix(1700000600, 0).UTC(),
	}
	if err := server.ApplyClusterSessionConfig(context.Background(), session.Token, updated, false); err != nil {
		t.Fatalf("ApplyClusterSessionConfig(update) error = %v", err)
	}
	stored, err = store.GetSession(context.Background(), session.Token)
	if err != nil {
		t.Fatalf("GetSession(after update) error = %v", err)
	}
	if !stored.ExpiresAt.Equal(updated.ExpiresAt) {
		t.Fatalf("expected updated expiry, got %+v", stored)
	}

	if err := server.ApplyClusterSessionConfig(context.Background(), session.Token, nil, true); err != nil {
		t.Fatalf("ApplyClusterSessionConfig(delete) error = %v", err)
	}
	if stored, err := store.GetSession(context.Background(), session.Token); err != nil {
		t.Fatalf("GetSession(after delete) error = %v", err)
	} else if stored != nil {
		t.Fatalf("expected session to be deleted, got %+v", stored)
	}
}
