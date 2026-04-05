package lib

import (
	"encoding/json"
	"testing"
)

func TestConfigDataUnmarshalEnterpriseFields(t *testing.T) {
	raw := []byte(`{
		"client": {
			"drop_excess_requests": false,
			"initial_pool_size": 100,
			"prometheus_labels": [],
			"enable_logging": true,
			"disable_content_logging": false,
			"disable_db_pings_in_health": false,
			"log_retention_days": 7,
			"enforce_auth_on_inference": false,
			"allow_direct_keys": false,
			"max_request_body_size_mb": 100,
			"enable_litellm_fallbacks": false,
			"mcp_agent_depth": 10,
			"mcp_tool_execution_timeout": 30,
			"mcp_code_mode_binding_level": "server",
			"mcp_tool_sync_interval": 0,
			"async_job_result_ttl": 3600,
			"hide_deleted_virtual_keys_in_filters": false
		},
		"providers": {},
		"cluster_config": {
			"enabled": true,
			"region": "us-east-1",
			"auth_token": "env.CLUSTER_AUTH_TOKEN"
		},
		"load_balancer_config": {
			"enabled": true,
			"tracker_config": {
				"minimum_samples": 5
			}
		},
		"audit_logs": {
			"disabled": false,
			"hmac_key": "env.AUDIT_HMAC_KEY",
			"retention_days": 90
		},
		"alerts": {
			"enabled": true,
			"evaluation_interval_seconds": 30
		},
		"log_exports": {
			"enabled": true,
			"format": "jsonl",
			"compression": "gzip"
		},
		"vault": {
			"enabled": true,
			"type": "hashicorp",
			"sync_interval": "300s"
		}
	}`)

	var cfg ConfigData
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if cfg.ClusterConfig == nil || !cfg.ClusterConfig.Enabled {
		t.Fatal("expected cluster config to be parsed")
	}
	if cfg.ClusterConfig.AuthToken == nil {
		t.Fatal("expected cluster auth token to be parsed")
	}
	if cfg.LoadBalancerConfig == nil || !cfg.LoadBalancerConfig.Enabled {
		t.Fatal("expected load balancer config to be parsed")
	}
	if cfg.AuditLogsConfig == nil || cfg.AuditLogsConfig.RetentionDays != 90 {
		t.Fatal("expected audit logs config to be parsed")
	}
	if cfg.AlertsConfig == nil || !cfg.AlertsConfig.Enabled {
		t.Fatal("expected alerts config to be parsed")
	}
	if cfg.LogExportsConfig == nil || cfg.LogExportsConfig.Format != "jsonl" {
		t.Fatal("expected log exports config to be parsed")
	}
	if cfg.VaultConfig == nil || cfg.VaultConfig.SyncInterval != "300s" {
		t.Fatal("expected vault config to be parsed")
	}
}
