package main

import (
	"sync"
	"testing"

	// "github.com/mattermost/mattermost/server/public/model" // Not directly used
	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock" // For mock.AnythingOfType
	"github.com/stretchr/testify/require"
	"github.com/pkg/errors" // Using this for New, Cause, Wrap
)

func TestConfigurationClone(t *testing.T) {
	original := &configuration{
		OpenAIAPIKey:             "key1",
		GraphQLAgentWebSocketURL: "ws://url1",
	}

	clone := original.Clone()

	assert.NotSame(t, original, clone, "Clone should return a new pointer.")
	assert.Equal(t, original, clone, "Clone should have the same values.")

	// Modify the clone
	clone.OpenAIAPIKey = "key2"
	clone.GraphQLAgentWebSocketURL = "ws://url2"

	assert.NotEqual(t, original.OpenAIAPIKey, clone.OpenAIAPIKey, "Original should not be affected by clone modification.")
	assert.NotEqual(t, original.GraphQLAgentWebSocketURL, clone.GraphQLAgentWebSocketURL, "Original should not be affected by clone modification.")
	assert.Equal(t, "key1", original.OpenAIAPIKey, "Original key should remain unchanged.")
	assert.Equal(t, "ws://url1", original.GraphQLAgentWebSocketURL, "Original URL should remain unchanged.")
}

func TestGetSetConfiguration(t *testing.T) {
	p := &Plugin{
		configurationLock: sync.RWMutex{},
	}

	t.Run("get initial nil configuration", func(t *testing.T) {
		cfg := p.getConfiguration()
		require.NotNil(t, cfg, "getConfiguration should return a non-nil configuration even if not set.")
		assert.Empty(t, cfg.OpenAIAPIKey, "Initial config should be empty.")
		assert.Empty(t, cfg.GraphQLAgentWebSocketURL, "Initial config should be empty.")
	})

	t.Run("set and get configuration", func(t *testing.T) {
		newCfg := &configuration{
			OpenAIAPIKey:             "newKey",
			GraphQLAgentWebSocketURL: "ws://newurl",
		}
		p.setConfiguration(newCfg)

		retrievedCfg := p.getConfiguration()
		assert.Equal(t, newCfg, retrievedCfg, "Retrieved configuration should match the set one.")
		assert.Equal(t, "newKey", retrievedCfg.OpenAIAPIKey)
		// getConfiguration returns the direct pointer, not a clone.
		assert.Same(t, newCfg, retrievedCfg, "getConfiguration should return the same pointer that was set.")

		// Test that p.configuration points to newCfg
		p.configurationLock.RLock()
		assert.Same(t, newCfg, p.configuration, "Internal plugin configuration should point to the set config")
		p.configurationLock.RUnlock()
	})

	t.Run("setConfiguration with existing configuration panics", func(t *testing.T) {
		cfgToSet := &configuration{OpenAIAPIKey: "panicKey"}
		p.setConfiguration(cfgToSet) // Set it once

		// Calling with the exact same pointer should panic
		assert.PanicsWithValue(t, "setConfiguration called with the existing configuration", func() {
			p.setConfiguration(cfgToSet)
		}, "Calling setConfiguration with the same non-empty config pointer should panic.")
	})

	t.Run("setConfiguration with empty struct does not panic", func(t *testing.T) {
		emptyCfg := &configuration{}
		p.setConfiguration(emptyCfg) // Set it once

		// Second call with the same empty struct pointer *will* panic because NumField > 0
		// The comment in setConfiguration about NumField == 0 is misleading for this struct.
		assert.PanicsWithValue(t, "setConfiguration called with the existing configuration", func() {
			p.setConfiguration(emptyCfg)
		}, "Calling setConfiguration with the same non-empty field struct pointer should panic.")
	})

	t.Run("set nil configuration", func(t *testing.T) {
		p.setConfiguration(nil)
		cfg := p.getConfiguration()
		require.NotNil(t, cfg, "getConfiguration should return a non-nil configuration.")
		assert.Empty(t, cfg.OpenAIAPIKey)
		assert.Empty(t, cfg.GraphQLAgentWebSocketURL)
	})
}

// Mock for p.API.LoadPluginConfiguration
// This needs to be a function type that matches the signature of LoadPluginConfiguration
type mockLoadPluginConfiguration func(dest interface{}) error

func (m mockLoadPluginConfiguration) LoadPluginConfiguration(dest interface{}) error {
	return m(dest)
}

func TestOnConfigurationChange(t *testing.T) {
	p := &Plugin{
		configurationLock: sync.RWMutex{},
	}

	t.Run("successful configuration change", func(t *testing.T) {
		api := &plugintest.API{}
		// p.API = api // Direct assignment if SetAPI isn't available or needed for simple cases
		// For robust testing, ensure the plugin instance `p` uses the mocked API.
		// If p.API is not exported, this test needs to be structured to allow API injection,
		// or be part of the main package to access unexported fields.
		// For now, assuming p.API can be set or is already using a mockable global/interface.
		// Let's assume the plugin has p.API field that can be set for testing.
		p.API = api


		expectedCfg := &configuration{
			OpenAIAPIKey:             "loadedKey",
			GraphQLAgentWebSocketURL: "ws://loadedURL",
		}

		// Mock LoadPluginConfiguration
		api.On("LoadPluginConfiguration", mock.AnythingOfType("*main.configuration")).Run(func(args mock.Arguments) {
			cfgPtr := args.Get(0).(*configuration)
			*cfgPtr = *expectedCfg
			// Note: GraphQLPingIntervalSeconds is deliberately NOT set in expectedCfg
			// to simulate it being missing from the loaded configuration, thereby triggering the default.
		}).Return(nil).Once()

		// Expect LogInfo to be called because GraphQLPingIntervalSeconds is missing
		api.On("LogInfo", "GraphQLPingIntervalSeconds not configured or invalid, defaulting to 30 seconds.").Return().Once()

		err := p.OnConfigurationChange()
		assert.NoError(t, err)

		retrievedCfg := p.getConfiguration()
		assert.Equal(t, expectedCfg.OpenAIAPIKey, retrievedCfg.OpenAIAPIKey)
		assert.Equal(t, expectedCfg.GraphQLAgentWebSocketURL, retrievedCfg.GraphQLAgentWebSocketURL)

		// Assert that the default value for GraphQLPingIntervalSeconds was applied
		expectedDefaultPingInterval := 30
		assert.NotNil(t, retrievedCfg.GraphQLPingIntervalSeconds, "GraphQLPingIntervalSeconds should have been defaulted")
		if retrievedCfg.GraphQLPingIntervalSeconds != nil {
			assert.Equal(t, expectedDefaultPingInterval, *retrievedCfg.GraphQLPingIntervalSeconds, "GraphQLPingIntervalSeconds should be set to the default value")
		}

		api.AssertExpectations(t)
	})

	t.Run("failed configuration load", func(t *testing.T) {
		api := &plugintest.API{}
		p.API = api // Ensure plugin uses the mock API

		// Keep the old configuration to check it doesn't change on load failure
		oldCfg := &configuration{OpenAIAPIKey: "oldKey", GraphQLAgentWebSocketURL: "ws://oldURL"}
		p.setConfiguration(oldCfg)

		expectedError := errors.New("load configuration error") // Using errors.New from "github.com/pkg/errors"
		api.On("LoadPluginConfiguration", mock.AnythingOfType("*main.configuration")).Return(expectedError).Once()

		err := p.OnConfigurationChange()
		assert.Error(t, err)
		// Check if the cause of the error is expectedError
		assert.Equal(t, expectedError, errors.Cause(err), "The cause of the error should be the one from LoadPluginConfiguration.")

		// Configuration should remain the old one or be empty if setConfiguration(new(configuration)) was called internally before error
		// Based on current OnConfigurationChange, p.setConfiguration is called only on success.
		// However, the `configuration` variable in OnConfigurationChange is local.
		// If LoadPluginConfiguration fails, p.setConfiguration is NOT called with the new (empty) config.
		// So, the existing p.configuration should remain.
		retrievedCfg := p.getConfiguration()
		assert.Equal(t, oldCfg, retrievedCfg, "Configuration should not have changed on load error.")
		api.AssertExpectations(t)
	})
}
