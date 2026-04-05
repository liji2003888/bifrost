package handlers

import (
	"context"
	"testing"

	bifrost "github.com/maximhq/bifrost/core"
	"github.com/maximhq/bifrost/framework/configstore"
	"github.com/maximhq/bifrost/framework/configstore/tables"
	"github.com/valyala/fasthttp"
)

type fakePromptClusterStore struct {
	configstore.ConfigStore
	folders       map[string]*tables.TableFolder
	prompts       map[string]*tables.TablePrompt
	versions      map[uint]*tables.TablePromptVersion
	sessions      map[uint]*tables.TablePromptSession
	nextVersionID uint
	nextSessionID uint
}

func newFakePromptClusterStore() *fakePromptClusterStore {
	return &fakePromptClusterStore{
		folders:       make(map[string]*tables.TableFolder),
		prompts:       make(map[string]*tables.TablePrompt),
		versions:      make(map[uint]*tables.TablePromptVersion),
		sessions:      make(map[uint]*tables.TablePromptSession),
		nextVersionID: 1,
		nextSessionID: 1,
	}
}

func (s *fakePromptClusterStore) CreateFolder(_ context.Context, folder *tables.TableFolder) error {
	s.folders[folder.ID] = clonePromptFolder(folder)
	return nil
}

func (s *fakePromptClusterStore) GetFolderByID(_ context.Context, id string) (*tables.TableFolder, error) {
	folder, ok := s.folders[id]
	if !ok {
		return nil, configstore.ErrNotFound
	}
	return clonePromptFolder(folder), nil
}

func (s *fakePromptClusterStore) UpdateFolder(_ context.Context, folder *tables.TableFolder) error {
	if _, ok := s.folders[folder.ID]; !ok {
		return configstore.ErrNotFound
	}
	s.folders[folder.ID] = clonePromptFolder(folder)
	return nil
}

func (s *fakePromptClusterStore) DeleteFolder(_ context.Context, id string) error {
	if _, ok := s.folders[id]; !ok {
		return configstore.ErrNotFound
	}
	delete(s.folders, id)
	return nil
}

func (s *fakePromptClusterStore) CreatePrompt(_ context.Context, prompt *tables.TablePrompt) error {
	s.prompts[prompt.ID] = clonePromptEntity(prompt)
	return nil
}

func (s *fakePromptClusterStore) GetPromptByID(_ context.Context, id string) (*tables.TablePrompt, error) {
	prompt, ok := s.prompts[id]
	if !ok {
		return nil, configstore.ErrNotFound
	}
	return clonePromptEntity(prompt), nil
}

func (s *fakePromptClusterStore) UpdatePrompt(_ context.Context, prompt *tables.TablePrompt) error {
	if _, ok := s.prompts[prompt.ID]; !ok {
		return configstore.ErrNotFound
	}
	s.prompts[prompt.ID] = clonePromptEntity(prompt)
	return nil
}

func (s *fakePromptClusterStore) DeletePrompt(_ context.Context, id string) error {
	if _, ok := s.prompts[id]; !ok {
		return configstore.ErrNotFound
	}
	delete(s.prompts, id)
	return nil
}

func (s *fakePromptClusterStore) CreatePromptVersion(_ context.Context, version *tables.TablePromptVersion) error {
	version.ID = s.nextVersionID
	s.nextVersionID++
	cloned := clonePromptVersionEntity(version)
	s.versions[version.ID] = cloned
	return nil
}

func (s *fakePromptClusterStore) GetPromptVersionByID(_ context.Context, id uint) (*tables.TablePromptVersion, error) {
	version, ok := s.versions[id]
	if !ok {
		return nil, configstore.ErrNotFound
	}
	return clonePromptVersionEntity(version), nil
}

func (s *fakePromptClusterStore) DeletePromptVersion(_ context.Context, id uint) error {
	if _, ok := s.versions[id]; !ok {
		return configstore.ErrNotFound
	}
	delete(s.versions, id)
	return nil
}

func (s *fakePromptClusterStore) CreatePromptSession(_ context.Context, session *tables.TablePromptSession) error {
	session.ID = s.nextSessionID
	s.nextSessionID++
	s.sessions[session.ID] = clonePromptSessionEntity(session)
	return nil
}

func (s *fakePromptClusterStore) GetPromptSessionByID(_ context.Context, id uint) (*tables.TablePromptSession, error) {
	session, ok := s.sessions[id]
	if !ok {
		return nil, configstore.ErrNotFound
	}
	return clonePromptSessionEntity(session), nil
}

func (s *fakePromptClusterStore) UpdatePromptSession(_ context.Context, session *tables.TablePromptSession) error {
	if _, ok := s.sessions[session.ID]; !ok {
		return configstore.ErrNotFound
	}
	s.sessions[session.ID] = clonePromptSessionEntity(session)
	return nil
}

func (s *fakePromptClusterStore) DeletePromptSession(_ context.Context, id uint) error {
	if _, ok := s.sessions[id]; !ok {
		return configstore.ErrNotFound
	}
	delete(s.sessions, id)
	return nil
}

func (s *fakePromptClusterStore) RenamePromptSession(_ context.Context, id uint, name string) error {
	session, ok := s.sessions[id]
	if !ok {
		return configstore.ErrNotFound
	}
	cloned := clonePromptSessionEntity(session)
	cloned.Name = name
	s.sessions[id] = cloned
	return nil
}

type fakePromptClusterPropagator struct {
	changes []*ClusterConfigChange
}

func (p *fakePromptClusterPropagator) PropagateClusterConfigChange(_ context.Context, change *ClusterConfigChange) error {
	if p == nil || change == nil {
		return nil
	}
	cloned := *change
	p.changes = append(p.changes, &cloned)
	return nil
}

func TestCreateFolderPropagatesClusterConfigChange(t *testing.T) {
	SetLogger(bifrost.NewNoOpLogger())

	store := newFakePromptClusterStore()
	propagator := &fakePromptClusterPropagator{}
	handler := NewPromptsHandler(store, propagator)

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fasthttp.MethodPost)
	ctx.Request.SetRequestURI("/api/prompt-repo/folders")
	ctx.Request.SetBodyString(`{"name":"Ops","description":"shared prompts"}`)

	handler.createFolder(ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusOK {
		t.Fatalf("expected 200, got %d: %s", ctx.Response.StatusCode(), string(ctx.Response.Body()))
	}
	if len(propagator.changes) != 1 {
		t.Fatalf("expected one propagated change, got %d", len(propagator.changes))
	}
	change := propagator.changes[0]
	if change.Scope != ClusterConfigScopeFolder || change.FolderConfig == nil || change.FolderConfig.Name != "Ops" {
		t.Fatalf("unexpected propagated folder change: %+v", change)
	}
}

func TestCreatePromptVersionPropagatesClusterConfigChange(t *testing.T) {
	SetLogger(bifrost.NewNoOpLogger())

	store := newFakePromptClusterStore()
	store.prompts["prompt-1"] = &tables.TablePrompt{ID: "prompt-1", Name: "Support Reply"}
	propagator := &fakePromptClusterPropagator{}
	handler := NewPromptsHandler(store, propagator)

	ctx := &fasthttp.RequestCtx{}
	ctx.SetUserValue("id", "prompt-1")
	ctx.Request.Header.SetMethod(fasthttp.MethodPost)
	ctx.Request.SetRequestURI("/api/prompt-repo/prompts/prompt-1/versions")
	ctx.Request.SetBodyString(`{
		"commit_message":"Initial",
		"messages":[{"role":"system","content":"Be concise"}],
		"model_params":{"temperature":0.2},
		"provider":"openai",
		"model":"gpt-4.1"
	}`)

	handler.createVersion(ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusOK {
		t.Fatalf("expected 200, got %d: %s", ctx.Response.StatusCode(), string(ctx.Response.Body()))
	}
	if len(propagator.changes) != 1 {
		t.Fatalf("expected one propagated change, got %d", len(propagator.changes))
	}
	change := propagator.changes[0]
	if change.Scope != ClusterConfigScopePromptVersion || change.PromptVersionID == 0 {
		t.Fatalf("unexpected propagated version change: %+v", change)
	}
	if change.PromptVersion == nil || change.PromptVersion.PromptID != "prompt-1" || len(change.PromptVersion.Messages) != 1 {
		t.Fatalf("unexpected propagated version payload: %+v", change.PromptVersion)
	}
}

func TestRenameSessionPropagatesClusterConfigChange(t *testing.T) {
	SetLogger(bifrost.NewNoOpLogger())

	store := newFakePromptClusterStore()
	store.sessions[7] = &tables.TablePromptSession{
		ID:       7,
		PromptID: "prompt-1",
		Name:     "Draft",
		Provider: "openai",
		Model:    "gpt-4.1",
		Messages: []tables.TablePromptSessionMessage{
			{ID: 11, PromptID: "prompt-1", SessionID: 7, OrderIndex: 0, Message: tables.PromptMessage(`{"role":"user","content":"hello"}`)},
		},
	}
	propagator := &fakePromptClusterPropagator{}
	handler := NewPromptsHandler(store, propagator)

	ctx := &fasthttp.RequestCtx{}
	ctx.SetUserValue("id", "7")
	ctx.Request.Header.SetMethod(fasthttp.MethodPut)
	ctx.Request.SetRequestURI("/api/prompt-repo/sessions/7/rename")
	ctx.Request.SetBodyString(`{"name":"Renamed draft"}`)

	handler.renameSession(ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusOK {
		t.Fatalf("expected 200, got %d: %s", ctx.Response.StatusCode(), string(ctx.Response.Body()))
	}
	if len(propagator.changes) != 1 {
		t.Fatalf("expected one propagated change, got %d", len(propagator.changes))
	}
	change := propagator.changes[0]
	if change.Scope != ClusterConfigScopePromptSession || change.PromptSessionID != 7 {
		t.Fatalf("unexpected propagated session change: %+v", change)
	}
	if change.PromptSession == nil || change.PromptSession.Name != "Renamed draft" {
		t.Fatalf("unexpected propagated session payload: %+v", change.PromptSession)
	}
}

func TestDeletePromptPropagatesClusterConfigChange(t *testing.T) {
	SetLogger(bifrost.NewNoOpLogger())

	store := newFakePromptClusterStore()
	store.prompts["prompt-1"] = &tables.TablePrompt{ID: "prompt-1", Name: "Support Reply"}
	propagator := &fakePromptClusterPropagator{}
	handler := NewPromptsHandler(store, propagator)

	ctx := &fasthttp.RequestCtx{}
	ctx.SetUserValue("id", "prompt-1")
	ctx.Request.Header.SetMethod(fasthttp.MethodDelete)
	ctx.Request.SetRequestURI("/api/prompt-repo/prompts/prompt-1")

	handler.deletePrompt(ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusOK {
		t.Fatalf("expected 200, got %d: %s", ctx.Response.StatusCode(), string(ctx.Response.Body()))
	}
	if len(propagator.changes) != 1 {
		t.Fatalf("expected one propagated change, got %d", len(propagator.changes))
	}
	change := propagator.changes[0]
	if change.Scope != ClusterConfigScopePrompt || change.PromptID != "prompt-1" || !change.Delete {
		t.Fatalf("unexpected propagated prompt delete: %+v", change)
	}
}

func TestPromptHandlersPreserveNotFoundSemantics(t *testing.T) {
	SetLogger(bifrost.NewNoOpLogger())

	store := newFakePromptClusterStore()
	handler := NewPromptsHandler(store, &fakePromptClusterPropagator{})

	ctx := &fasthttp.RequestCtx{}
	ctx.SetUserValue("id", "missing")
	ctx.Request.Header.SetMethod(fasthttp.MethodDelete)
	ctx.Request.SetRequestURI("/api/prompt-repo/prompts/missing")

	handler.deletePrompt(ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusNotFound {
		t.Fatalf("expected 404, got %d", ctx.Response.StatusCode())
	}
}
