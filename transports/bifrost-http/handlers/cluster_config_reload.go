package handlers

import (
	"context"

	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework/configstore"
	configstoreTables "github.com/maximhq/bifrost/framework/configstore/tables"
)

const ClusterConfigReloadEndpoint = "/_cluster/config/reload"

type ClusterConfigScope string

const (
	ClusterConfigScopeClient    ClusterConfigScope = "client"
	ClusterConfigScopeAuth      ClusterConfigScope = "auth"
	ClusterConfigScopeFramework ClusterConfigScope = "framework"
	ClusterConfigScopeProxy     ClusterConfigScope = "proxy"
	ClusterConfigScopeProvider  ClusterConfigScope = "provider"
)

type ClusterConfigChange struct {
	Scope           ClusterConfigScope                      `json:"scope"`
	Provider        schemas.ModelProvider                   `json:"provider,omitempty"`
	Delete          bool                                    `json:"delete,omitempty"`
	FlushSessions   bool                                    `json:"flush_sessions,omitempty"`
	ClientConfig    *configstore.ClientConfig               `json:"client_config,omitempty"`
	AuthConfig      *configstore.AuthConfig                 `json:"auth_config,omitempty"`
	FrameworkConfig *configstoreTables.TableFrameworkConfig `json:"framework_config,omitempty"`
	ProxyConfig     *configstoreTables.GlobalProxyConfig    `json:"proxy_config,omitempty"`
	ProviderConfig  *configstore.ProviderConfig             `json:"provider_config,omitempty"`
}

type ClusterConfigPropagator interface {
	PropagateClusterConfigChange(ctx context.Context, change *ClusterConfigChange) error
}

type ClusterConfigApplier interface {
	ApplyClusterConfigChange(ctx context.Context, change *ClusterConfigChange) error
}
