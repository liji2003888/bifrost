package server

import (
	"testing"

	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework/configstore"
)

func TestShouldSkipActiveModelDiscoveryWhenListModelsDisabled(t *testing.T) {
	config := modelDiscoveryConfigFromProviderConfig(configstore.ProviderConfig{
		CustomProviderConfig: &schemas.CustomProviderConfig{
			BaseProviderType: schemas.OpenAI,
			AllowedRequests: &schemas.AllowedRequests{
				ChatCompletion: true,
				ListModels:     false,
			},
		},
	})

	if !shouldSkipActiveModelDiscovery(config) {
		t.Fatal("expected custom provider with list_models disabled to skip active model discovery")
	}
}

func TestShouldFallbackToStaticModelCatalogForDashScope404(t *testing.T) {
	statusCode := 404
	config := modelDiscoveryConfigFromProviderConfig(configstore.ProviderConfig{
		CustomProviderConfig: &schemas.CustomProviderConfig{
			BaseProviderType: schemas.OpenAI,
		},
		NetworkConfig: &schemas.NetworkConfig{
			BaseURL: "https://dashscope.aliyuncs.com/compatible-mode",
		},
	})
	err := &schemas.BifrostError{
		StatusCode: &statusCode,
		ExtraFields: schemas.BifrostErrorExtraFields{
			RequestType: schemas.ListModelsRequest,
		},
	}

	if !shouldFallbackToStaticModelCatalog(config, err) {
		t.Fatal("expected DashScope-compatible custom provider to fall back to static model catalog on 404 list-models")
	}
}

func TestShouldFallbackToStaticModelCatalogForCustomProviderWithStaticModels(t *testing.T) {
	statusCode := 404
	config := modelDiscoveryConfigFromProviderConfig(configstore.ProviderConfig{
		CustomProviderConfig: &schemas.CustomProviderConfig{
			BaseProviderType: schemas.OpenAI,
		},
		Keys: []schemas.Key{
			{
				ID:     "key-1",
				Models: []string{"qwen-plus"},
			},
		},
	})
	err := &schemas.BifrostError{
		StatusCode: &statusCode,
		ExtraFields: schemas.BifrostErrorExtraFields{
			RequestType: schemas.ListModelsRequest,
		},
	}

	if !shouldFallbackToStaticModelCatalog(config, err) {
		t.Fatal("expected custom provider with statically configured models to fall back to static model catalog on 404 list-models")
	}
}

func TestShouldNotFallbackToStaticModelCatalogForNon404Errors(t *testing.T) {
	statusCode := 500
	config := modelDiscoveryConfigFromProviderConfig(configstore.ProviderConfig{
		CustomProviderConfig: &schemas.CustomProviderConfig{
			BaseProviderType: schemas.OpenAI,
		},
		NetworkConfig: &schemas.NetworkConfig{
			BaseURL: "https://dashscope.aliyuncs.com/compatible-mode",
		},
	})
	err := &schemas.BifrostError{
		StatusCode: &statusCode,
		ExtraFields: schemas.BifrostErrorExtraFields{
			RequestType: schemas.ListModelsRequest,
		},
	}

	if shouldFallbackToStaticModelCatalog(config, err) {
		t.Fatal("did not expect static model fallback for non-404 model discovery errors")
	}
}

func TestShouldNotFallbackToStaticModelCatalogForStandardProviders(t *testing.T) {
	statusCode := 404
	config := modelDiscoveryConfigFromProviderConfig(configstore.ProviderConfig{
		NetworkConfig: &schemas.NetworkConfig{
			BaseURL: "https://dashscope.aliyuncs.com/compatible-mode",
		},
	})
	err := &schemas.BifrostError{
		StatusCode: &statusCode,
		ExtraFields: schemas.BifrostErrorExtraFields{
			RequestType: schemas.ListModelsRequest,
		},
	}

	if shouldFallbackToStaticModelCatalog(config, err) {
		t.Fatal("did not expect static model fallback for standard providers")
	}
}
