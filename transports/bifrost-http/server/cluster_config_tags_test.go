package server

import (
	"slices"
	"testing"

	"github.com/maximhq/bifrost/transports/bifrost-http/handlers"
)

func TestClusterConfigChangeTagsIncludesExpectedUIInvalidationTargets(t *testing.T) {
	tests := []struct {
		name      string
		scope     handlers.ClusterConfigScope
		expectAny []string
	}{
		{
			name:      "provider",
			scope:     handlers.ClusterConfigScopeProvider,
			expectAny: []string{"Providers", "DBKeys", "Models", "BaseModels", "ClusterNodes"},
		},
		{
			name:      "governance",
			scope:     handlers.ClusterConfigScopeVirtualKey,
			expectAny: []string{"VirtualKeys", "Teams", "Customers", "Budgets", "RateLimits", "ProviderGovernance", "RoutingRules", "ClusterNodes"},
		},
		{
			name:      "prompt-session",
			scope:     handlers.ClusterConfigScopePromptSession,
			expectAny: []string{"Sessions", "Prompts", "Versions", "ClusterNodes"},
		},
		{
			name:      "oauth",
			scope:     handlers.ClusterConfigScopeOAuthToken,
			expectAny: []string{"OAuth2Config", "MCPClients", "ClusterNodes"},
		},
		{
			name:      "session",
			scope:     handlers.ClusterConfigScopeSession,
			expectAny: []string{"SessionState", "ClusterNodes"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tags := clusterConfigChangeTags(&handlers.ClusterConfigChange{Scope: tt.scope})
			for _, expected := range tt.expectAny {
				if !slices.Contains(tags, expected) {
					t.Fatalf("expected tag %q in %+v", expected, tags)
				}
			}
		})
	}
}

func TestDedupeStoreUpdateTagsRemovesBlanksAndDuplicates(t *testing.T) {
	tags := dedupeStoreUpdateTags([]string{"Config", "", "Config", "Providers", "Providers", " ClusterNodes "})
	expected := []string{"Config", "Providers", "ClusterNodes"}
	if !slices.Equal(tags, expected) {
		t.Fatalf("expected %v, got %v", expected, tags)
	}
}

func TestClusterConfigChangeTagsCoverEverySupportedScope(t *testing.T) {
	scopes := []handlers.ClusterConfigScope{
		handlers.ClusterConfigScopeClient,
		handlers.ClusterConfigScopeAuth,
		handlers.ClusterConfigScopeCustomer,
		handlers.ClusterConfigScopeFolder,
		handlers.ClusterConfigScopeFramework,
		handlers.ClusterConfigScopeMCPClient,
		handlers.ClusterConfigScopeModelConfig,
		handlers.ClusterConfigScopeOAuthConfig,
		handlers.ClusterConfigScopeOAuthToken,
		handlers.ClusterConfigScopePlugin,
		handlers.ClusterConfigScopeProviderGovernance,
		handlers.ClusterConfigScopeProxy,
		handlers.ClusterConfigScopeProvider,
		handlers.ClusterConfigScopePrompt,
		handlers.ClusterConfigScopePromptSession,
		handlers.ClusterConfigScopePromptVersion,
		handlers.ClusterConfigScopeRoutingRule,
		handlers.ClusterConfigScopeSession,
		handlers.ClusterConfigScopeTeam,
		handlers.ClusterConfigScopeVirtualKey,
	}

	for _, scope := range scopes {
		t.Run(string(scope), func(t *testing.T) {
			tags := clusterConfigChangeTags(&handlers.ClusterConfigChange{Scope: scope})
			if len(tags) == 0 {
				t.Fatalf("expected non-empty tags for scope %q", scope)
			}
			if !slices.Contains(tags, "ClusterNodes") {
				t.Fatalf("expected ClusterNodes tag for scope %q, got %v", scope, tags)
			}
		})
	}
}
