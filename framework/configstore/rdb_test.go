package configstore

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework/configstore/tables"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// setupRDBTestStore creates an in-memory SQLite database and returns an RDBConfigStore for testing
func setupRDBTestStore(t *testing.T) *RDBConfigStore {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err, "Failed to create test database")

	// Run migrations for all tables
	err = db.AutoMigrate(
		&tables.TableProvider{},
		&tables.TableKey{},
		&tables.TableModelPricing{},
		&tables.TableModelParameters{},
		&tables.TableBudget{},
		&tables.TableRateLimit{},
		&tables.TableVirtualKey{},
		&tables.TableVirtualKeyProviderConfig{},
		&tables.TableVirtualKeyProviderConfigKey{},
		&tables.TableCustomer{},
		&tables.TableTeam{},
		&tables.TableClientConfig{},
		&tables.TablePlugin{},
		&tables.TableMCPClient{},
		&tables.TableMCPHostedTool{},
		&tables.TableVirtualKeyMCPConfig{},
		&tables.TableFolder{},
		&tables.TablePrompt{},
		&tables.TablePromptVersion{},
		&tables.TablePromptVersionMessage{},
		&tables.TablePromptSession{},
		&tables.TablePromptSessionMessage{},
	)
	require.NoError(t, err, "Failed to migrate test database")

	// Setup join table
	err = db.SetupJoinTable(&tables.TableVirtualKeyProviderConfig{}, "Keys", &tables.TableVirtualKeyProviderConfigKey{})
	require.NoError(t, err, "Failed to setup join table")

	return &RDBConfigStore{
		db:     db,
		logger: nil,
	}
}

func ptr[T any](v T) *T {
	return &v
}

// =============================================================================
// Provider and Key Tests
// =============================================================================

func TestUpdateProvidersConfig_CreateNew(t *testing.T) {
	store := setupRDBTestStore(t)
	ctx := context.Background()

	providers := map[schemas.ModelProvider]ProviderConfig{
		"openai": {
			Keys: []schemas.Key{
				{
					ID:     "key-uuid-1",
					Name:   "openai-primary",
					Value:  *schemas.NewEnvVar("sk-test-key"),
					Weight: 1.0,
				},
			},
		},
	}

	err := store.UpdateProvidersConfig(ctx, providers)
	require.NoError(t, err)

	// Verify provider was created
	result, err := store.GetProvidersConfig(ctx)
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Contains(t, result, schemas.ModelProvider("openai"))
	assert.Len(t, result["openai"].Keys, 1)
	assert.Equal(t, "openai-primary", result["openai"].Keys[0].Name)
}

func TestUpdateProvidersConfig_UpdateExistingByKeyID(t *testing.T) {
	store := setupRDBTestStore(t)
	ctx := context.Background()

	// Create initial provider with key
	providers := map[schemas.ModelProvider]ProviderConfig{
		"openai": {
			Keys: []schemas.Key{
				{
					ID:     "key-uuid-1",
					Name:   "openai-primary",
					Value:  *schemas.NewEnvVar("sk-test-key-v1"),
					Weight: 1.0,
				},
			},
		},
	}
	err := store.UpdateProvidersConfig(ctx, providers)
	require.NoError(t, err)

	// Update with same KeyID but different value
	providers["openai"] = ProviderConfig{
		Keys: []schemas.Key{
			{
				ID:     "key-uuid-1", // Same KeyID
				Name:   "openai-primary",
				Value:  *schemas.NewEnvVar("sk-test-key-v2"), // Updated value
				Weight: 2.0,
			},
		},
	}
	err = store.UpdateProvidersConfig(ctx, providers)
	require.NoError(t, err)

	// Verify key was updated, not duplicated
	result, err := store.GetProvidersConfig(ctx)
	require.NoError(t, err)
	assert.Len(t, result["openai"].Keys, 1)
	assert.Equal(t, "sk-test-key-v2", result["openai"].Keys[0].Value.Val)
}

func TestUpdateProvidersConfig_UpdateExistingByName_FallbackFix(t *testing.T) {
	// This test verifies the fix for the unique constraint violation issue
	// when a new UUID is generated for a key that already exists by name
	store := setupRDBTestStore(t)
	ctx := context.Background()

	// Create initial provider with key
	providers := map[schemas.ModelProvider]ProviderConfig{
		"openai": {
			Keys: []schemas.Key{
				{
					ID:     "original-uuid",
					Name:   "openai-primary",
					Value:  *schemas.NewEnvVar("sk-test-key-v1"),
					Weight: 1.0,
				},
			},
		},
	}
	err := store.UpdateProvidersConfig(ctx, providers)
	require.NoError(t, err)

	// Simulate config reload with NEW UUID (as happens when loading from config file)
	providers["openai"] = ProviderConfig{
		Keys: []schemas.Key{
			{
				ID:     "new-uuid-from-config-reload", // Different UUID!
				Name:   "openai-primary",              // Same name
				Value:  *schemas.NewEnvVar("sk-test-key-v2"),
				Weight: 1.5,
			},
		},
	}
	err = store.UpdateProvidersConfig(ctx, providers)
	require.NoError(t, err, "Should not fail with unique constraint violation")

	// Verify key was updated (not duplicated) and original KeyID preserved
	result, err := store.GetProvidersConfig(ctx)
	require.NoError(t, err)
	assert.Len(t, result["openai"].Keys, 1, "Should have exactly one key, not duplicated")
	assert.Equal(t, "sk-test-key-v2", result["openai"].Keys[0].Value.Val, "Value should be updated")
	assert.Equal(t, "original-uuid", result["openai"].Keys[0].ID, "Original KeyID should be preserved")
}

func TestUpdateProvidersConfig_MultipleKeys(t *testing.T) {
	store := setupRDBTestStore(t)
	ctx := context.Background()

	providers := map[schemas.ModelProvider]ProviderConfig{
		"openai": {
			Keys: []schemas.Key{
				{ID: "key-1", Name: "openai-primary", Value: *schemas.NewEnvVar("sk-key-1"), Weight: 1.0},
				{ID: "key-2", Name: "openai-secondary", Value: *schemas.NewEnvVar("sk-key-2"), Weight: 0.5},
			},
		},
		"anthropic": {
			Keys: []schemas.Key{
				{ID: "key-3", Name: "anthropic-main", Value: *schemas.NewEnvVar("sk-key-3"), Weight: 1.0},
			},
		},
	}

	err := store.UpdateProvidersConfig(ctx, providers)
	require.NoError(t, err)

	result, err := store.GetProvidersConfig(ctx)
	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Len(t, result["openai"].Keys, 2)
	assert.Len(t, result["anthropic"].Keys, 1)
}

func TestMCPClientDiscoveredToolsPersistAcrossCreateAndUpdate(t *testing.T) {
	store := setupRDBTestStore(t)
	ctx := context.Background()

	description := "Search enterprise docs"
	initialTools := map[string]schemas.ChatTool{
		"docs_client-search": {
			Type: schemas.ChatToolTypeFunction,
			Function: &schemas.ChatToolFunction{
				Name:        "docs_client-search",
				Description: &description,
			},
		},
	}
	initialMapping := map[string]string{"docs_client_search": "docs-client-search"}

	err := store.CreateMCPClientConfig(ctx, &schemas.MCPClientConfig{
		ID:                        "mcp-client-1",
		Name:                      "docs_client",
		ConnectionType:            schemas.MCPConnectionTypeHTTP,
		ConnectionString:          schemas.NewEnvVar("http://mcp.internal"),
		AuthType:                  schemas.MCPAuthTypeNone,
		ToolsToExecute:            []string{"*"},
		ToolsToAutoExecute:        []string{},
		IsPingAvailable:           true,
		ToolSyncInterval:          5 * time.Minute,
		DiscoveredTools:           initialTools,
		DiscoveredToolNameMapping: initialMapping,
	})
	require.NoError(t, err)

	cfg, err := store.GetMCPConfig(ctx)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.Len(t, cfg.ClientConfigs, 1)
	require.Len(t, cfg.ClientConfigs[0].DiscoveredTools, 1)
	assert.Equal(t, "docs_client-search", cfg.ClientConfigs[0].DiscoveredTools["docs_client-search"].Function.Name)
	assert.Equal(t, "docs-client-search", cfg.ClientConfigs[0].DiscoveredToolNameMapping["docs_client_search"])

	row, err := store.GetMCPClientByID(ctx, "mcp-client-1")
	require.NoError(t, err)
	require.NotNil(t, row)

	updatedDescription := "List enterprise docs"
	row.DiscoveredTools = map[string]schemas.ChatTool{
		"docs_client-list": {
			Type: schemas.ChatToolTypeFunction,
			Function: &schemas.ChatToolFunction{
				Name:        "docs_client-list",
				Description: &updatedDescription,
			},
		},
	}
	row.ToolNameMapping = map[string]string{"docs_client_list": "docs-client-list"}

	err = store.UpdateMCPClientConfig(ctx, "mcp-client-1", row)
	require.NoError(t, err)

	cfg, err = store.GetMCPConfig(ctx)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.Len(t, cfg.ClientConfigs, 1)
	require.Len(t, cfg.ClientConfigs[0].DiscoveredTools, 1)
	assert.Equal(t, "docs_client-list", cfg.ClientConfigs[0].DiscoveredTools["docs_client-list"].Function.Name)
	assert.Equal(t, "docs-client-list", cfg.ClientConfigs[0].DiscoveredToolNameMapping["docs_client_list"])
}

func TestUpsertModelPrices_UpdatesExistingRowAtomically(t *testing.T) {
	store := setupRDBTestStore(t)
	ctx := context.Background()

	initial := &tables.TableModelPricing{
		Model:              "gpt-4.1",
		Provider:           "openai",
		Mode:               "chat",
		InputCostPerToken:  0.1,
		OutputCostPerToken: 0.2,
		ContextLength:      ptr(128000),
		MaxInputTokens:     ptr(128000),
		MaxOutputTokens:    ptr(4096),
	}
	require.NoError(t, store.UpsertModelPrices(ctx, initial))

	updated := &tables.TableModelPricing{
		Model:              "gpt-4.1",
		Provider:           "openai",
		Mode:               "chat",
		InputCostPerToken:  0.3,
		OutputCostPerToken: 0.4,
		ContextLength:      ptr(272000),
		MaxInputTokens:     ptr(272000),
		MaxOutputTokens:    ptr(8192),
	}
	require.NoError(t, store.UpsertModelPrices(ctx, updated))

	rows, err := store.GetModelPrices(ctx)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, 0.3, rows[0].InputCostPerToken)
	assert.Equal(t, 0.4, rows[0].OutputCostPerToken)
	require.NotNil(t, rows[0].ContextLength)
	assert.Equal(t, 272000, *rows[0].ContextLength)
}

func TestUpsertModelParameters_UpdatesExistingRowAtomically(t *testing.T) {
	store := setupRDBTestStore(t)
	ctx := context.Background()

	initial := &tables.TableModelParameters{
		Model: "gpt-4.1",
		Data:  `{"supports_tools":true}`,
	}
	require.NoError(t, store.UpsertModelParameters(ctx, initial))

	updated := &tables.TableModelParameters{
		Model: "gpt-4.1",
		Data:  `{"supports_tools":true,"supports_responses":true}`,
	}
	require.NoError(t, store.UpsertModelParameters(ctx, updated))

	params, err := store.GetModelParameters(ctx, "gpt-4.1")
	require.NoError(t, err)
	require.NotNil(t, params)
	assert.JSONEq(t, `{"supports_tools":true,"supports_responses":true}`, params.Data)
}

// =============================================================================
// Budget Tests
// =============================================================================

func TestCreateBudget(t *testing.T) {
	store := setupRDBTestStore(t)
	ctx := context.Background()

	budget := &tables.TableBudget{
		ID:            "budget-test",
		MaxLimit:      100.0,
		ResetDuration: "1M",
	}

	err := store.CreateBudget(ctx, budget)
	require.NoError(t, err)

	// Verify budget was created
	result, err := store.GetBudget(ctx, "budget-test")
	require.NoError(t, err)
	assert.Equal(t, "budget-test", result.ID)
	assert.Equal(t, 100.0, result.MaxLimit)
	assert.Equal(t, "1M", result.ResetDuration)
}

func TestCreateBudget_InvalidDuration(t *testing.T) {
	store := setupRDBTestStore(t)
	ctx := context.Background()

	budget := &tables.TableBudget{
		ID:            "budget-invalid",
		MaxLimit:      100.0,
		ResetDuration: "invalid",
	}

	err := store.CreateBudget(ctx, budget)
	assert.Error(t, err, "Should fail with invalid duration")
}

func TestCreateBudget_NegativeLimit(t *testing.T) {
	store := setupRDBTestStore(t)
	ctx := context.Background()

	budget := &tables.TableBudget{
		ID:            "budget-negative",
		MaxLimit:      -50.0,
		ResetDuration: "1h",
	}

	err := store.CreateBudget(ctx, budget)
	assert.Error(t, err, "Should fail with negative max limit")
}

func TestUpdateBudget(t *testing.T) {
	store := setupRDBTestStore(t)
	ctx := context.Background()

	// Create budget
	budget := &tables.TableBudget{
		ID:            "budget-update",
		MaxLimit:      100.0,
		ResetDuration: "1h",
	}
	err := store.CreateBudget(ctx, budget)
	require.NoError(t, err)

	// Update budget
	budget.MaxLimit = 200.0
	err = store.UpdateBudget(ctx, budget)
	require.NoError(t, err)

	// Verify update
	result, err := store.GetBudget(ctx, "budget-update")
	require.NoError(t, err)
	assert.Equal(t, 200.0, result.MaxLimit)
}

func TestGetBudgets(t *testing.T) {
	store := setupRDBTestStore(t)
	ctx := context.Background()

	// Create multiple budgets
	budgets := []*tables.TableBudget{
		{ID: "budget-1", MaxLimit: 100.0, ResetDuration: "1h"},
		{ID: "budget-2", MaxLimit: 200.0, ResetDuration: "1d"},
		{ID: "budget-3", MaxLimit: 300.0, ResetDuration: "1M"},
	}

	for _, b := range budgets {
		err := store.CreateBudget(ctx, b)
		require.NoError(t, err)
	}

	result, err := store.GetBudgets(ctx)
	require.NoError(t, err)
	assert.Len(t, result, 3)
}

// =============================================================================
// Rate Limit Tests
// =============================================================================

func TestCreateRateLimit(t *testing.T) {
	store := setupRDBTestStore(t)
	ctx := context.Background()

	tokenMax := int64(100000)
	requestMax := int64(1000)
	tokenDuration := "1h"
	requestDuration := "1h"

	rateLimit := &tables.TableRateLimit{
		ID:                   "rate-limit-test",
		TokenMaxLimit:        &tokenMax,
		TokenResetDuration:   &tokenDuration,
		RequestMaxLimit:      &requestMax,
		RequestResetDuration: &requestDuration,
	}

	err := store.CreateRateLimit(ctx, rateLimit)
	require.NoError(t, err)

	result, err := store.GetRateLimit(ctx, "rate-limit-test")
	require.NoError(t, err)
	assert.Equal(t, "rate-limit-test", result.ID)
	assert.Equal(t, int64(100000), *result.TokenMaxLimit)
	assert.Equal(t, int64(1000), *result.RequestMaxLimit)
}

func TestCreateRateLimit_InvalidDuration(t *testing.T) {
	store := setupRDBTestStore(t)
	ctx := context.Background()

	tokenMax := int64(100000)
	invalidDuration := "invalid"

	rateLimit := &tables.TableRateLimit{
		ID:                 "rate-limit-invalid",
		TokenMaxLimit:      &tokenMax,
		TokenResetDuration: &invalidDuration,
	}

	err := store.CreateRateLimit(ctx, rateLimit)
	assert.Error(t, err, "Should fail with invalid duration")
}

func TestCreateRateLimit_MissingDuration(t *testing.T) {
	store := setupRDBTestStore(t)
	ctx := context.Background()

	tokenMax := int64(100000)

	rateLimit := &tables.TableRateLimit{
		ID:            "rate-limit-missing",
		TokenMaxLimit: &tokenMax,
		// Missing TokenResetDuration
	}

	err := store.CreateRateLimit(ctx, rateLimit)
	assert.Error(t, err, "Should fail when max limit set without duration")
}

func TestUpdateRateLimit(t *testing.T) {
	store := setupRDBTestStore(t)
	ctx := context.Background()

	tokenMax := int64(100000)
	tokenDuration := "1h"

	rateLimit := &tables.TableRateLimit{
		ID:                 "rate-limit-update",
		TokenMaxLimit:      &tokenMax,
		TokenResetDuration: &tokenDuration,
	}
	err := store.CreateRateLimit(ctx, rateLimit)
	require.NoError(t, err)

	// Update
	newMax := int64(200000)
	rateLimit.TokenMaxLimit = &newMax
	err = store.UpdateRateLimit(ctx, rateLimit)
	require.NoError(t, err)

	result, err := store.GetRateLimit(ctx, "rate-limit-update")
	require.NoError(t, err)
	assert.Equal(t, int64(200000), *result.TokenMaxLimit)
}

// =============================================================================
// Virtual Key Tests
// =============================================================================

func TestCreateVirtualKey(t *testing.T) {
	store := setupRDBTestStore(t)
	ctx := context.Background()

	vk := &tables.TableVirtualKey{
		ID:       "vk-test",
		Name:     "Test Virtual Key",
		Value:    "vk-test-value-123",
		IsActive: true,
	}

	err := store.CreateVirtualKey(ctx, vk)
	require.NoError(t, err)

	result, err := store.GetVirtualKey(ctx, "vk-test")
	require.NoError(t, err)
	assert.Equal(t, "vk-test", result.ID)
	assert.Equal(t, "Test Virtual Key", result.Name)
	assert.Equal(t, "vk-test-value-123", result.Value)
	assert.True(t, result.IsActive)
}

func TestCreateVirtualKey_WithBudgetAndRateLimit(t *testing.T) {
	store := setupRDBTestStore(t)
	ctx := context.Background()

	// Create budget first
	budget := &tables.TableBudget{
		ID:            "budget-for-vk",
		MaxLimit:      100.0,
		ResetDuration: "1M",
	}
	err := store.CreateBudget(ctx, budget)
	require.NoError(t, err)

	// Create rate limit
	tokenMax := int64(100000)
	tokenDuration := "1h"
	rateLimit := &tables.TableRateLimit{
		ID:                 "rate-limit-for-vk",
		TokenMaxLimit:      &tokenMax,
		TokenResetDuration: &tokenDuration,
	}
	err = store.CreateRateLimit(ctx, rateLimit)
	require.NoError(t, err)

	// Create virtual key with references
	budgetID := "budget-for-vk"
	rateLimitID := "rate-limit-for-vk"
	vk := &tables.TableVirtualKey{
		ID:          "vk-with-refs",
		Name:        "VK With References",
		Value:       "vk-refs-value",
		IsActive:    true,
		BudgetID:    &budgetID,
		RateLimitID: &rateLimitID,
	}

	err = store.CreateVirtualKey(ctx, vk)
	require.NoError(t, err)

	result, err := store.GetVirtualKey(ctx, "vk-with-refs")
	require.NoError(t, err)
	assert.NotNil(t, result.BudgetID)
	assert.Equal(t, "budget-for-vk", *result.BudgetID)
	assert.NotNil(t, result.RateLimitID)
	assert.Equal(t, "rate-limit-for-vk", *result.RateLimitID)
}

func TestCreateVirtualKey_DuplicateName(t *testing.T) {
	store := setupRDBTestStore(t)
	ctx := context.Background()

	vk1 := &tables.TableVirtualKey{
		ID:       "vk-1",
		Name:     "Same Name",
		Value:    "vk-value-1",
		IsActive: true,
	}
	err := store.CreateVirtualKey(ctx, vk1)
	require.NoError(t, err)

	vk2 := &tables.TableVirtualKey{
		ID:       "vk-2",
		Name:     "Same Name", // Duplicate name
		Value:    "vk-value-2",
		IsActive: true,
	}
	err = store.CreateVirtualKey(ctx, vk2)
	assert.Error(t, err, "Should fail with duplicate name")
}

func TestGetVirtualKeyByValue(t *testing.T) {
	store := setupRDBTestStore(t)
	ctx := context.Background()

	vk := &tables.TableVirtualKey{
		ID:       "vk-lookup",
		Name:     "Lookup Key",
		Value:    "vk-unique-value-xyz",
		IsActive: true,
	}
	err := store.CreateVirtualKey(ctx, vk)
	require.NoError(t, err)

	result, err := store.GetVirtualKeyByValue(ctx, "vk-unique-value-xyz")
	require.NoError(t, err)
	assert.Equal(t, "vk-lookup", result.ID)
}

func TestUpdateVirtualKey(t *testing.T) {
	store := setupRDBTestStore(t)
	ctx := context.Background()

	vk := &tables.TableVirtualKey{
		ID:       "vk-update",
		Name:     "Original Name",
		Value:    "vk-update-value",
		IsActive: true,
	}
	err := store.CreateVirtualKey(ctx, vk)
	require.NoError(t, err)

	// Update
	vk.Name = "Updated Name"
	vk.IsActive = false
	err = store.UpdateVirtualKey(ctx, vk)
	require.NoError(t, err)

	result, err := store.GetVirtualKey(ctx, "vk-update")
	require.NoError(t, err)
	assert.Equal(t, "Updated Name", result.Name)
	assert.False(t, result.IsActive)
}

func TestDeleteVirtualKey(t *testing.T) {
	store := setupRDBTestStore(t)
	ctx := context.Background()

	vk := &tables.TableVirtualKey{
		ID:       "vk-delete",
		Name:     "Delete Me",
		Value:    "vk-delete-value",
		IsActive: true,
	}
	err := store.CreateVirtualKey(ctx, vk)
	require.NoError(t, err)

	err = store.DeleteVirtualKey(ctx, "vk-delete")
	require.NoError(t, err)

	_, err = store.GetVirtualKey(ctx, "vk-delete")
	assert.Error(t, err, "Should not find deleted virtual key")
}

// =============================================================================
// Virtual Key Provider Config Tests
// =============================================================================

func TestCreateVirtualKeyProviderConfig(t *testing.T) {
	store := setupRDBTestStore(t)
	ctx := context.Background()

	// Create virtual key first
	vk := &tables.TableVirtualKey{
		ID:       "vk-for-pc",
		Name:     "VK For Provider Config",
		Value:    "vk-pc-value",
		IsActive: true,
	}
	err := store.CreateVirtualKey(ctx, vk)
	require.NoError(t, err)

	// Create provider config
	weight := 1.0
	pc := &tables.TableVirtualKeyProviderConfig{
		VirtualKeyID: "vk-for-pc",
		Provider:     "openai",
		Weight:       &weight,
	}

	err = store.CreateVirtualKeyProviderConfig(ctx, pc)
	require.NoError(t, err)

	// Verify
	configs, err := store.GetVirtualKeyProviderConfigs(ctx, "vk-for-pc")
	require.NoError(t, err)
	assert.Len(t, configs, 1)
	assert.Equal(t, "openai", configs[0].Provider)
}

func TestCreateVirtualKeyProviderConfig_WithKeys(t *testing.T) {
	store := setupRDBTestStore(t)
	ctx := context.Background()

	// Create provider with keys first
	providers := map[schemas.ModelProvider]ProviderConfig{
		"openai": {
			Keys: []schemas.Key{
				{ID: "key-for-pc", Name: "openai-pc-key", Value: *schemas.NewEnvVar("sk-test"), Weight: 1.0},
			},
		},
	}
	err := store.UpdateProvidersConfig(ctx, providers)
	require.NoError(t, err)

	// Create virtual key
	vk := &tables.TableVirtualKey{
		ID:       "vk-with-keys",
		Name:     "VK With Keys",
		Value:    "vk-keys-value",
		IsActive: true,
	}
	err = store.CreateVirtualKey(ctx, vk)
	require.NoError(t, err)

	// Create provider config with key reference
	weight := 1.0
	pc := &tables.TableVirtualKeyProviderConfig{
		VirtualKeyID: "vk-with-keys",
		Provider:     "openai",
		Weight:       &weight,
		Keys: []tables.TableKey{
			{Name: "openai-pc-key"}, // Reference by name
		},
	}

	err = store.CreateVirtualKeyProviderConfig(ctx, pc)
	require.NoError(t, err)

	// Verify keys are associated
	configs, err := store.GetVirtualKeyProviderConfigs(ctx, "vk-with-keys")
	require.NoError(t, err)
	assert.Len(t, configs, 1)

	// Load with keys
	var configWithKeys tables.TableVirtualKeyProviderConfig
	err = store.db.Preload("Keys").First(&configWithKeys, "id = ?", configs[0].ID).Error
	require.NoError(t, err)
	assert.Len(t, configWithKeys.Keys, 1)
}

func TestCreateVirtualKeyProviderConfig_UnresolvedKeys(t *testing.T) {
	store := setupRDBTestStore(t)
	ctx := context.Background()

	// Create virtual key
	vk := &tables.TableVirtualKey{
		ID:       "vk-unresolved",
		Name:     "VK Unresolved",
		Value:    "vk-unresolved-value",
		IsActive: true,
	}
	err := store.CreateVirtualKey(ctx, vk)
	require.NoError(t, err)

	// Try to create provider config with non-existent key
	weight := 1.0
	pc := &tables.TableVirtualKeyProviderConfig{
		VirtualKeyID: "vk-unresolved",
		Provider:     "openai",
		Weight:       &weight,
		Keys: []tables.TableKey{
			{Name: "non-existent-key"},
		},
	}

	err = store.CreateVirtualKeyProviderConfig(ctx, pc)
	assert.Error(t, err, "Should fail with unresolved keys")

	var unresolvedErr *ErrUnresolvedKeys
	assert.ErrorAs(t, err, &unresolvedErr, "Should be ErrUnresolvedKeys")
}

// =============================================================================
// Client Config Tests
// =============================================================================

func TestUpdateClientConfig(t *testing.T) {
	store := setupRDBTestStore(t)
	ctx := context.Background()

	config := &ClientConfig{
		EnableLogging:        new(true),
		AllowDirectKeys:      true,
		InitialPoolSize:      100,
		LogRetentionDays:     30,
		MaxRequestBodySizeMB: 50,
	}

	err := store.UpdateClientConfig(ctx, config)
	require.NoError(t, err)

	result, err := store.GetClientConfig(ctx)
	require.NoError(t, err)
	assert.True(t, result.EnableLogging != nil && *result.EnableLogging)
	assert.Equal(t, 100, result.InitialPoolSize)
}

// =============================================================================
// Transaction Tests
// =============================================================================

func TestExecuteTransaction_Success(t *testing.T) {
	store := setupRDBTestStore(t)
	ctx := context.Background()

	err := store.ExecuteTransaction(ctx, func(tx *gorm.DB) error {
		// Create budget in transaction
		budget := &tables.TableBudget{
			ID:            "tx-budget",
			MaxLimit:      100.0,
			ResetDuration: "1h",
		}
		return tx.Create(budget).Error
	})
	require.NoError(t, err)

	// Verify budget was created
	result, err := store.GetBudget(ctx, "tx-budget")
	require.NoError(t, err)
	assert.Equal(t, "tx-budget", result.ID)
}

func TestExecuteTransaction_Rollback(t *testing.T) {
	store := setupRDBTestStore(t)
	ctx := context.Background()

	err := store.ExecuteTransaction(ctx, func(tx *gorm.DB) error {
		// Create budget
		budget := &tables.TableBudget{
			ID:            "tx-rollback-budget",
			MaxLimit:      100.0,
			ResetDuration: "1h",
		}
		if err := tx.Create(budget).Error; err != nil {
			return err
		}

		// Force error to trigger rollback
		return assert.AnError
	})
	assert.Error(t, err)

	// Verify budget was NOT created (rolled back)
	_, err = store.GetBudget(ctx, "tx-rollback-budget")
	assert.Error(t, err, "Budget should not exist after rollback")
}

// =============================================================================
// Customer and Team Tests
// =============================================================================

func TestCreateCustomer(t *testing.T) {
	store := setupRDBTestStore(t)
	ctx := context.Background()

	customer := &tables.TableCustomer{
		ID:   "customer-test",
		Name: "Test Customer",
	}

	err := store.CreateCustomer(ctx, customer)
	require.NoError(t, err)

	result, err := store.GetCustomer(ctx, "customer-test")
	require.NoError(t, err)
	assert.Equal(t, "Test Customer", result.Name)
}

func TestCreateTeam(t *testing.T) {
	store := setupRDBTestStore(t)
	ctx := context.Background()

	// Create customer first
	customer := &tables.TableCustomer{
		ID:   "customer-for-team",
		Name: "Customer For Team",
	}
	err := store.CreateCustomer(ctx, customer)
	require.NoError(t, err)

	// Create team
	customerID := "customer-for-team"
	team := &tables.TableTeam{
		ID:         "team-test",
		Name:       "Test Team",
		CustomerID: &customerID,
	}

	err = store.CreateTeam(ctx, team)
	require.NoError(t, err)

	result, err := store.GetTeam(ctx, "team-test")
	require.NoError(t, err)
	assert.Equal(t, "Test Team", result.Name)
	assert.Equal(t, "customer-for-team", *result.CustomerID)
}

// =============================================================================
// Ping and Health Tests
// =============================================================================

func TestPing(t *testing.T) {
	store := setupRDBTestStore(t)
	ctx := context.Background()

	err := store.Ping(ctx)
	assert.NoError(t, err)
}

// =============================================================================
// Error Handling Tests
// =============================================================================

func TestGetBudget_NotFound(t *testing.T) {
	store := setupRDBTestStore(t)
	ctx := context.Background()

	_, err := store.GetBudget(ctx, "non-existent-budget")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestGetVirtualKey_NotFound(t *testing.T) {
	store := setupRDBTestStore(t)
	ctx := context.Background()

	_, err := store.GetVirtualKey(ctx, "non-existent-vk")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestGetRateLimit_NotFound(t *testing.T) {
	store := setupRDBTestStore(t)
	ctx := context.Background()

	_, err := store.GetRateLimit(ctx, "non-existent-rate-limit")
	assert.ErrorIs(t, err, ErrNotFound)
}

// =============================================================================
// Plugin Tests
// =============================================================================

func TestCreateAndGetPlugin(t *testing.T) {
	store := setupRDBTestStore(t)
	ctx := context.Background()

	plugin := &tables.TablePlugin{
		Name:    "test-plugin",
		Enabled: true,
		Version: 1,
	}

	err := store.CreatePlugin(ctx, plugin)
	require.NoError(t, err)

	result, err := store.GetPlugin(ctx, "test-plugin")
	require.NoError(t, err)
	assert.Equal(t, "test-plugin", result.Name)
	assert.True(t, result.Enabled)
}

func TestUpsertPlugin(t *testing.T) {
	store := setupRDBTestStore(t)
	ctx := context.Background()

	// Create plugin
	plugin := &tables.TablePlugin{
		Name:    "upsert-plugin",
		Enabled: true,
		Version: 1,
	}
	err := store.UpsertPlugin(ctx, plugin)
	require.NoError(t, err)

	// Upsert with update
	plugin.Version = 2
	err = store.UpsertPlugin(ctx, plugin)
	require.NoError(t, err)

	result, err := store.GetPlugin(ctx, "upsert-plugin")
	require.NoError(t, err)
	assert.Equal(t, int16(2), result.Version)
}

// =============================================================================
// Integration Test: Full Virtual Key with Provider Config Flow
// =============================================================================

func TestFullVirtualKeyFlow(t *testing.T) {
	store := setupRDBTestStore(t)
	ctx := context.Background()

	// Step 1: Create provider with keys
	providers := map[schemas.ModelProvider]ProviderConfig{
		"openai": {
			Keys: []schemas.Key{
				{ID: "key-1", Name: "openai-main", Value: *schemas.NewEnvVar("sk-main"), Weight: 1.0},
				{ID: "key-2", Name: "openai-backup", Value: *schemas.NewEnvVar("sk-backup"), Weight: 0.5},
			},
		},
	}
	err := store.UpdateProvidersConfig(ctx, providers)
	require.NoError(t, err)

	// Step 2: Create budget
	budget := &tables.TableBudget{
		ID:            "integration-budget",
		MaxLimit:      500.0,
		ResetDuration: "1M",
	}
	err = store.CreateBudget(ctx, budget)
	require.NoError(t, err)

	// Step 3: Create rate limit
	tokenMax := int64(1000000)
	tokenDuration := "1d"
	rateLimit := &tables.TableRateLimit{
		ID:                 "integration-rate-limit",
		TokenMaxLimit:      &tokenMax,
		TokenResetDuration: &tokenDuration,
	}
	err = store.CreateRateLimit(ctx, rateLimit)
	require.NoError(t, err)

	// Step 4: Create virtual key
	budgetID := "integration-budget"
	rateLimitID := "integration-rate-limit"
	vk := &tables.TableVirtualKey{
		ID:          "integration-vk",
		Name:        "Integration Virtual Key",
		Value:       "vk-integration-xyz",
		IsActive:    true,
		BudgetID:    &budgetID,
		RateLimitID: &rateLimitID,
	}
	err = store.CreateVirtualKey(ctx, vk)
	require.NoError(t, err)

	// Step 5: Create provider config with key reference
	weight := 1.0
	pc := &tables.TableVirtualKeyProviderConfig{
		VirtualKeyID: "integration-vk",
		Provider:     "openai",
		Weight:       &weight,
		Keys: []tables.TableKey{
			{Name: "openai-main"},
		},
	}
	err = store.CreateVirtualKeyProviderConfig(ctx, pc)
	require.NoError(t, err)

	// Step 6: Verify complete setup
	result, err := store.GetVirtualKey(ctx, "integration-vk")
	require.NoError(t, err)
	assert.Equal(t, "Integration Virtual Key", result.Name)
	assert.NotNil(t, result.BudgetID)
	assert.NotNil(t, result.RateLimitID)

	configs, err := store.GetVirtualKeyProviderConfigs(ctx, "integration-vk")
	require.NoError(t, err)
	assert.Len(t, configs, 1)
	assert.Equal(t, "openai", configs[0].Provider)
}

// =============================================================================
// Helper function tests
// =============================================================================

func TestGetWeight(t *testing.T) {
	// Test nil weight returns default
	assert.Equal(t, 1.0, getWeight(nil))

	// Test explicit weight
	w := 2.5
	assert.Equal(t, 2.5, getWeight(&w))

	// Test zero weight
	zero := 0.0
	assert.Equal(t, 0.0, getWeight(&zero))
}

// =============================================================================
// Concurrent Access Tests
// =============================================================================

func TestMultipleBudgetUpdates(t *testing.T) {
	store := setupRDBTestStore(t)
	ctx := context.Background()

	// Create initial budget
	budget := &tables.TableBudget{
		ID:            "multi-update-budget",
		MaxLimit:      100.0,
		ResetDuration: "1h",
		CurrentUsage:  0,
	}
	err := store.CreateBudget(ctx, budget)
	require.NoError(t, err)

	// Simulate multiple sequential updates
	for i := 0; i < 10; i++ {
		b := &tables.TableBudget{
			ID:            "multi-update-budget",
			MaxLimit:      100.0 + float64(i),
			ResetDuration: "1h",
		}
		err := store.UpdateBudget(ctx, b)
		require.NoError(t, err)
	}

	// Verify budget exists and has the last value
	result, err := store.GetBudget(ctx, "multi-update-budget")
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 109.0, result.MaxLimit) // 100 + 9
}

// =============================================================================
// Duration Validation Tests (for budgets and rate limits)
// =============================================================================

func TestBudgetDurationFormats(t *testing.T) {
	store := setupRDBTestStore(t)
	ctx := context.Background()

	validDurations := []string{"30s", "5m", "1h", "1d", "1w", "1M", "1Y"}

	for i, duration := range validDurations {
		budget := &tables.TableBudget{
			ID:            "budget-duration-" + string(rune('a'+i)),
			MaxLimit:      100.0,
			ResetDuration: duration,
		}
		err := store.CreateBudget(ctx, budget)
		assert.NoError(t, err, "Duration %s should be valid", duration)
	}
}

func TestRateLimitDurationFormats(t *testing.T) {
	store := setupRDBTestStore(t)
	ctx := context.Background()

	validDurations := []string{"30s", "5m", "1h", "1d", "1w", "1M", "1Y"}

	for i, duration := range validDurations {
		tokenMax := int64(1000)
		rateLimit := &tables.TableRateLimit{
			ID:                 "rate-limit-duration-" + string(rune('a'+i)),
			TokenMaxLimit:      &tokenMax,
			TokenResetDuration: &duration,
		}
		err := store.CreateRateLimit(ctx, rateLimit)
		assert.NoError(t, err, "Duration %s should be valid", duration)
	}
}

// =============================================================================
// Prompt Deletion Tests
// =============================================================================

// testPromptTree holds IDs of entities created by createTestPromptTree for verification
type testPromptTree struct {
	FolderID   string
	PromptIDs  []string
	VersionIDs []uint
	SessionIDs []uint
}

// createTestPromptTree creates a folder with 2 prompts, each having 2 versions (with messages) and 1 session (with messages).
func createTestPromptTree(t *testing.T, store *RDBConfigStore, ctx context.Context) testPromptTree {
	t.Helper()

	tree := testPromptTree{}

	// Create folder
	folder := &tables.TableFolder{ID: "folder-1", Name: "Test Folder"}
	require.NoError(t, store.CreateFolder(ctx, folder))
	tree.FolderID = folder.ID

	for i, promptID := range []string{"prompt-1", "prompt-2"} {
		_ = i
		prompt := &tables.TablePrompt{ID: promptID, Name: "Prompt " + promptID, FolderID: &tree.FolderID}
		require.NoError(t, store.CreatePrompt(ctx, prompt))
		tree.PromptIDs = append(tree.PromptIDs, promptID)

		// Create 2 versions with messages
		for v := 0; v < 2; v++ {
			version := &tables.TablePromptVersion{
				PromptID:      promptID,
				CommitMessage: "version commit",
				Messages: []tables.TablePromptVersionMessage{
					{PromptID: promptID, Message: json.RawMessage(`{"role":"user","content":"hello"}`)},
				},
			}
			require.NoError(t, store.CreatePromptVersion(ctx, version))
			tree.VersionIDs = append(tree.VersionIDs, version.ID)
		}

		// Create 1 session with messages
		session := &tables.TablePromptSession{
			PromptID: promptID,
			Name:     "Session " + promptID,
			Messages: []tables.TablePromptSessionMessage{
				{PromptID: promptID, Message: json.RawMessage(`{"role":"user","content":"hi"}`)},
			},
		}
		require.NoError(t, store.CreatePromptSession(ctx, session))
		tree.SessionIDs = append(tree.SessionIDs, session.ID)
	}

	return tree
}

// countRows returns the number of rows in a table
func countRows(t *testing.T, store *RDBConfigStore, model interface{}) int64 {
	t.Helper()
	var count int64
	require.NoError(t, store.db.Model(model).Count(&count).Error)
	return count
}

func TestDeleteFolder(t *testing.T) {
	t.Run("NotFound", func(t *testing.T) {
		store := setupRDBTestStore(t)
		ctx := context.Background()
		err := store.DeleteFolder(ctx, "nonexistent")
		assert.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("Empty", func(t *testing.T) {
		store := setupRDBTestStore(t)
		ctx := context.Background()
		folder := &tables.TableFolder{ID: "folder-empty", Name: "Empty"}
		require.NoError(t, store.CreateFolder(ctx, folder))

		require.NoError(t, store.DeleteFolder(ctx, "folder-empty"))

		_, err := store.GetFolderByID(ctx, "folder-empty")
		assert.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("CascadesAll", func(t *testing.T) {
		store := setupRDBTestStore(t)
		ctx := context.Background()
		tree := createTestPromptTree(t, store, ctx)

		// Verify entities exist before deletion
		assert.Greater(t, countRows(t, store, &tables.TablePrompt{}), int64(0))
		assert.Greater(t, countRows(t, store, &tables.TablePromptVersion{}), int64(0))
		assert.Greater(t, countRows(t, store, &tables.TablePromptVersionMessage{}), int64(0))
		assert.Greater(t, countRows(t, store, &tables.TablePromptSession{}), int64(0))
		assert.Greater(t, countRows(t, store, &tables.TablePromptSessionMessage{}), int64(0))

		require.NoError(t, store.DeleteFolder(ctx, tree.FolderID))

		// All child entities should be deleted
		assert.Equal(t, int64(0), countRows(t, store, &tables.TableFolder{}))
		assert.Equal(t, int64(0), countRows(t, store, &tables.TablePrompt{}))
		assert.Equal(t, int64(0), countRows(t, store, &tables.TablePromptVersion{}))
		assert.Equal(t, int64(0), countRows(t, store, &tables.TablePromptVersionMessage{}))
		assert.Equal(t, int64(0), countRows(t, store, &tables.TablePromptSession{}))
		assert.Equal(t, int64(0), countRows(t, store, &tables.TablePromptSessionMessage{}))
	})
}

func TestDeletePrompt(t *testing.T) {
	t.Run("NotFound", func(t *testing.T) {
		store := setupRDBTestStore(t)
		ctx := context.Background()
		err := store.DeletePrompt(ctx, "nonexistent")
		assert.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("CascadesAll", func(t *testing.T) {
		store := setupRDBTestStore(t)
		ctx := context.Background()
		tree := createTestPromptTree(t, store, ctx)

		require.NoError(t, store.DeletePrompt(ctx, tree.PromptIDs[0]))

		// First prompt and its children should be gone
		_, err := store.GetPromptByID(ctx, tree.PromptIDs[0])
		assert.ErrorIs(t, err, ErrNotFound)

		// Second prompt should still exist
		p2, err := store.GetPromptByID(ctx, tree.PromptIDs[1])
		require.NoError(t, err)
		assert.Equal(t, tree.PromptIDs[1], p2.ID)
	})

	t.Run("LeavesFolder", func(t *testing.T) {
		store := setupRDBTestStore(t)
		ctx := context.Background()
		tree := createTestPromptTree(t, store, ctx)

		require.NoError(t, store.DeletePrompt(ctx, tree.PromptIDs[0]))

		// Folder should still exist
		folder, err := store.GetFolderByID(ctx, tree.FolderID)
		require.NoError(t, err)
		assert.Equal(t, tree.FolderID, folder.ID)
	})

	t.Run("LeavesSiblings", func(t *testing.T) {
		store := setupRDBTestStore(t)
		ctx := context.Background()
		tree := createTestPromptTree(t, store, ctx)

		require.NoError(t, store.DeletePrompt(ctx, tree.PromptIDs[0]))

		// Sibling prompt's versions and sessions should be unaffected
		versions, err := store.GetPromptVersions(ctx, tree.PromptIDs[1])
		require.NoError(t, err)
		assert.Len(t, versions, 2)

		sessions, err := store.GetPromptSessions(ctx, tree.PromptIDs[1])
		require.NoError(t, err)
		assert.Len(t, sessions, 1)
	})
}

func TestDeletePromptVersion(t *testing.T) {
	t.Run("NotFound", func(t *testing.T) {
		store := setupRDBTestStore(t)
		ctx := context.Background()
		err := store.DeletePromptVersion(ctx, 99999)
		assert.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("NonLatest", func(t *testing.T) {
		store := setupRDBTestStore(t)
		ctx := context.Background()
		tree := createTestPromptTree(t, store, ctx)

		// Version at index 0 is v1 (non-latest), index 1 is v2 (latest) for prompt-1
		nonLatestID := tree.VersionIDs[0]
		latestID := tree.VersionIDs[1]

		require.NoError(t, store.DeletePromptVersion(ctx, nonLatestID))

		// Non-latest version should be gone
		_, err := store.GetPromptVersionByID(ctx, nonLatestID)
		assert.ErrorIs(t, err, ErrNotFound)

		// Latest version should still be latest
		latest, err := store.GetPromptVersionByID(ctx, latestID)
		require.NoError(t, err)
		assert.True(t, latest.IsLatest)
	})

	t.Run("LatestPromotesPrevious", func(t *testing.T) {
		store := setupRDBTestStore(t)
		ctx := context.Background()
		tree := createTestPromptTree(t, store, ctx)

		// Delete the latest version (index 1 = v2 for prompt-1)
		latestID := tree.VersionIDs[1]
		prevID := tree.VersionIDs[0]

		require.NoError(t, store.DeletePromptVersion(ctx, latestID))

		// Previous version should now be latest
		prev, err := store.GetPromptVersionByID(ctx, prevID)
		require.NoError(t, err)
		assert.True(t, prev.IsLatest)
	})

	t.Run("LeavesPrompt", func(t *testing.T) {
		store := setupRDBTestStore(t)
		ctx := context.Background()
		tree := createTestPromptTree(t, store, ctx)

		require.NoError(t, store.DeletePromptVersion(ctx, tree.VersionIDs[0]))

		// Prompt should still exist
		prompt, err := store.GetPromptByID(ctx, tree.PromptIDs[0])
		require.NoError(t, err)
		assert.Equal(t, tree.PromptIDs[0], prompt.ID)
	})
}

func TestDeletePromptSession(t *testing.T) {
	t.Run("NotFound", func(t *testing.T) {
		store := setupRDBTestStore(t)
		ctx := context.Background()
		err := store.DeletePromptSession(ctx, 99999)
		assert.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("CascadesMessages", func(t *testing.T) {
		store := setupRDBTestStore(t)
		ctx := context.Background()
		tree := createTestPromptTree(t, store, ctx)

		sessionID := tree.SessionIDs[0]
		require.NoError(t, store.DeletePromptSession(ctx, sessionID))

		// Session should be gone
		_, err := store.GetPromptSessionByID(ctx, sessionID)
		assert.ErrorIs(t, err, ErrNotFound)

		// Session messages for that session should be gone
		var msgCount int64
		require.NoError(t, store.db.Model(&tables.TablePromptSessionMessage{}).Where("session_id = ?", sessionID).Count(&msgCount).Error)
		assert.Equal(t, int64(0), msgCount)
	})

	t.Run("LeavesPrompt", func(t *testing.T) {
		store := setupRDBTestStore(t)
		ctx := context.Background()
		tree := createTestPromptTree(t, store, ctx)

		require.NoError(t, store.DeletePromptSession(ctx, tree.SessionIDs[0]))

		// Prompt and versions should still exist
		prompt, err := store.GetPromptByID(ctx, tree.PromptIDs[0])
		require.NoError(t, err)
		assert.Equal(t, tree.PromptIDs[0], prompt.ID)

		versions, err := store.GetPromptVersions(ctx, tree.PromptIDs[0])
		require.NoError(t, err)
		assert.Len(t, versions, 2)
	})
}

func TestMCPHostedToolPersistsQueryParamsAndResponseFormatting(t *testing.T) {
	store := setupRDBTestStore(t)
	ctx := context.Background()

	description := "Search internal catalog"
	bodyTemplate := `{"query":"{{args.query}}"}`
	responseJSONPath := "data.summary"
	responseTemplate := "Summary: {{response.data.summary}}"
	responseExamples := []any{
		map[string]any{"summary": "First example"},
		map[string]any{"summary": "Second example"},
	}
	timeoutSeconds := 12
	maxResponseBodyBytes := 4096
	tool := &tables.TableMCPHostedTool{
		ToolID:      "tool-1",
		Name:        "search_catalog",
		Description: &description,
		Method:      "POST",
		URL:         "https://api.internal/search",
		Headers: map[string]string{
			"Authorization": "{{req.header.authorization}}",
		},
		QueryParams: map[string]string{
			"tenant_id": "{{args.tenant_id}}",
		},
		AuthProfile: &tables.MCPHostedToolAuthProfile{
			Mode: tables.MCPHostedToolAuthModeHeaderPassthrough,
			HeaderMappings: map[string]string{
				"X-Tenant-ID": "x-tenant-id",
			},
		},
		ExecutionProfile: &tables.MCPHostedToolExecutionProfile{
			TimeoutSeconds:       &timeoutSeconds,
			MaxResponseBodyBytes: &maxResponseBodyBytes,
		},
		BodyTemplate:     &bodyTemplate,
		ResponseExamples: responseExamples,
		ResponseJSONPath: &responseJSONPath,
		ResponseTemplate: &responseTemplate,
		ToolSchema: schemas.ChatTool{
			Type: schemas.ChatToolTypeFunction,
			Function: &schemas.ChatToolFunction{
				Name:        "search_catalog",
				Description: &description,
				Parameters: &schemas.ToolFunctionParameters{
					Type:     "object",
					Required: []string{"query", "tenant_id"},
				},
			},
		},
	}

	require.NoError(t, store.CreateMCPHostedTool(ctx, tool))

	persisted, err := store.GetMCPHostedToolByID(ctx, "tool-1")
	require.NoError(t, err)
	require.NotNil(t, persisted)
	assert.Equal(t, "{{args.tenant_id}}", persisted.QueryParams["tenant_id"])
	assert.Equal(t, responseJSONPath, *persisted.ResponseJSONPath)
	assert.Equal(t, responseTemplate, *persisted.ResponseTemplate)
	require.Len(t, persisted.ResponseExamples, 2)
	firstExample, ok := persisted.ResponseExamples[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "First example", firstExample["summary"])
	require.NotNil(t, persisted.AuthProfile)
	assert.Equal(t, tables.MCPHostedToolAuthModeHeaderPassthrough, persisted.AuthProfile.Mode)
	assert.Equal(t, "x-tenant-id", persisted.AuthProfile.HeaderMappings["X-Tenant-ID"])
	require.NotNil(t, persisted.ExecutionProfile)
	require.NotNil(t, persisted.ExecutionProfile.TimeoutSeconds)
	assert.Equal(t, timeoutSeconds, *persisted.ExecutionProfile.TimeoutSeconds)
	require.NotNil(t, persisted.ExecutionProfile.MaxResponseBodyBytes)
	assert.Equal(t, maxResponseBodyBytes, *persisted.ExecutionProfile.MaxResponseBodyBytes)

	updatedTemplate := "Result: {{response.data.summary}}"
	persisted.QueryParams["tenant_id"] = "{{args.customer_id}}"
	persisted.ResponseTemplate = &updatedTemplate
	persisted.ResponseExamples = []any{map[string]any{"summary": "Updated example"}}
	persisted.AuthProfile = &tables.MCPHostedToolAuthProfile{Mode: tables.MCPHostedToolAuthModeBearerPassthrough}
	require.NoError(t, store.UpdateMCPHostedTool(ctx, persisted))

	updated, err := store.GetMCPHostedToolByID(ctx, "tool-1")
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, "{{args.customer_id}}", updated.QueryParams["tenant_id"])
	assert.Equal(t, updatedTemplate, *updated.ResponseTemplate)
	require.Len(t, updated.ResponseExamples, 1)
	updatedExample, ok := updated.ResponseExamples[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "Updated example", updatedExample["summary"])
	require.NotNil(t, updated.AuthProfile)
	assert.Equal(t, tables.MCPHostedToolAuthModeBearerPassthrough, updated.AuthProfile.Mode)
}
