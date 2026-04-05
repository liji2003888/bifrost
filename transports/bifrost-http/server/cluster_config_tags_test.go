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
