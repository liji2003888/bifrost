package handlers

import (
	"encoding/json"

	"github.com/maximhq/bifrost/framework/configstore/tables"
	"github.com/valyala/fasthttp"
)

func (h *PromptsHandler) propagateClusterChange(ctx *fasthttp.RequestCtx, change *ClusterConfigChange) {
	if h == nil || h.propagator == nil || change == nil {
		return
	}
	if err := h.propagator.PropagateClusterConfigChange(ctx, change); err != nil && logger != nil {
		logger.Warn("failed to propagate prompt repository cluster change for %s: %v", change.Scope, err)
	}
}

func clonePromptFolder(folder *tables.TableFolder) *tables.TableFolder {
	if folder == nil {
		return nil
	}
	cloned := *folder
	if folder.Description != nil {
		description := *folder.Description
		cloned.Description = &description
	}
	cloned.PromptsCount = 0
	return &cloned
}

func clonePromptEntity(prompt *tables.TablePrompt) *tables.TablePrompt {
	if prompt == nil {
		return nil
	}
	cloned := *prompt
	if prompt.FolderID != nil {
		folderID := *prompt.FolderID
		cloned.FolderID = &folderID
	}
	cloned.Folder = nil
	cloned.Versions = nil
	cloned.Sessions = nil
	cloned.LatestVersion = nil
	return &cloned
}

func clonePromptVersionEntity(version *tables.TablePromptVersion) *tables.TablePromptVersion {
	if version == nil {
		return nil
	}
	cloned := *version
	cloned.Prompt = nil
	cloned.ModelParams = clonePromptModelParams(version.ModelParams)
	cloned.Messages = clonePromptVersionMessages(version.Messages)
	return &cloned
}

func clonePromptSessionEntity(session *tables.TablePromptSession) *tables.TablePromptSession {
	if session == nil {
		return nil
	}
	cloned := *session
	if session.VersionID != nil {
		versionID := *session.VersionID
		cloned.VersionID = &versionID
	}
	cloned.Prompt = nil
	cloned.Version = nil
	cloned.ModelParams = clonePromptModelParams(session.ModelParams)
	cloned.Messages = clonePromptSessionMessages(session.Messages)
	return &cloned
}

func clonePromptModelParams(params tables.ModelParams) tables.ModelParams {
	if params == nil {
		return nil
	}
	encoded, err := json.Marshal(params)
	if err != nil {
		cloned := make(tables.ModelParams, len(params))
		for key, value := range params {
			cloned[key] = value
		}
		return cloned
	}

	var cloned tables.ModelParams
	if err := json.Unmarshal(encoded, &cloned); err != nil {
		fallback := make(tables.ModelParams, len(params))
		for key, value := range params {
			fallback[key] = value
		}
		return fallback
	}
	return cloned
}

func clonePromptVersionMessages(messages []tables.TablePromptVersionMessage) []tables.TablePromptVersionMessage {
	if len(messages) == 0 {
		return nil
	}
	cloned := make([]tables.TablePromptVersionMessage, len(messages))
	for i := range messages {
		cloned[i] = messages[i]
		cloned[i].Version = nil
		cloned[i].Message = clonePromptMessage(messages[i].Message)
	}
	return cloned
}

func clonePromptSessionMessages(messages []tables.TablePromptSessionMessage) []tables.TablePromptSessionMessage {
	if len(messages) == 0 {
		return nil
	}
	cloned := make([]tables.TablePromptSessionMessage, len(messages))
	for i := range messages {
		cloned[i] = messages[i]
		cloned[i].Session = nil
		cloned[i].Message = clonePromptMessage(messages[i].Message)
	}
	return cloned
}

func clonePromptMessage(message tables.PromptMessage) tables.PromptMessage {
	if len(message) == 0 {
		return nil
	}
	cloned := make([]byte, len(message))
	copy(cloned, message)
	return cloned
}
