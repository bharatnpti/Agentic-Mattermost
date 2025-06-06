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
		MaestroURL: "ws://url1",
		CustomEndpoints: []CustomEndpoint{
			{Name: "ep1", Endpoint: "http://localhost:8001"},
		},
	}

	clone := original.Clone()

	assert.NotSame(t, original, clone, "Clone should return a new pointer.")
	assert.Equal(t, original, clone, "Clone should have the same values.")

	// Modify the clone
	clone.MaestroURL = "ws://url2"
	clone.CustomEndpoints = []CustomEndpoint{
		{Name: "ep2", Endpoint: "http://localhost:8002"},
	}

	assert.NotEqual(t, original.MaestroURL, clone.MaestroURL, "Original should not be affected by clone modification.")
	assert.NotEqual(t, original.CustomEndpoints, clone.CustomEndpoints, "Original should not be affected by clone modification.")
	assert.Equal(t, "ws://url1", original.MaestroURL, "Original URL should remain unchanged.")
	assert.Equal(t, "ep1", original.CustomEndpoints[0].Name, "Original CustomEndpoints should remain unchanged.")
}

func TestGetSetConfiguration(t *testing.T) {
	p := &Plugin{
		configurationLock: sync.RWMutex{},
	}

	t.Run("get initial nil configuration", func(t *testing.T) {
		cfg := p.getConfiguration()
		require.NotNil(t, cfg, "getConfiguration should return a non-nil configuration even if not set.")
		assert.Empty(t, cfg.MaestroURL, "Initial config should be empty.")
		assert.Empty(t, cfg.CustomEndpoints, "Initial config should be empty.")
	})

	t.Run("set and get configuration", func(t *testing.T) {
		newCfg := &configuration{
			MaestroURL: "ws://newurl",
			CustomEndpoints: []CustomEndpoint{
				{Name: "ep1", Endpoint: "http://localhost:8001"},
			},
		}
		p.setConfiguration(newCfg)

		retrievedCfg := p.getConfiguration()
		assert.Equal(t, newCfg, retrievedCfg, "Retrieved configuration should match the set one.")
		assert.Equal(t, "ws://newurl", retrievedCfg.MaestroURL)
		require.Len(t, retrievedCfg.CustomEndpoints, 1)
		assert.Equal(t, "ep1", retrievedCfg.CustomEndpoints[0].Name)
		// getConfiguration returns the direct pointer, not a clone.
		assert.Same(t, newCfg, retrievedCfg, "getConfiguration should return the same pointer that was set.")

		// Test that p.configuration points to newCfg
		p.configurationLock.RLock()
		assert.Same(t, newCfg, p.configuration, "Internal plugin configuration should point to the set config")
		p.configurationLock.RUnlock()
	})

	t.Run("setConfiguration with existing configuration panics", func(t *testing.T) {
		cfgToSet := &configuration{MaestroURL: "panicURL"}
		p.setConfiguration(cfgToSet) // Set it once

		// Calling with the exact same pointer should panic
		assert.PanicsWithValue(t, "setConfiguration called with the existing configuration", func() {
			p.setConfiguration(cfgToSet)
		}, "Calling setConfiguration with the same non-empty config pointer should panic.")
	})

	t.Run("setConfiguration with empty struct does not panic", func(t *testing.T) {
		emptyCfg := &configuration{} // This struct has fields, so it's not truly "empty" in terms of NumField.
		p.setConfiguration(emptyCfg) // Set it once

		// Second call with the same empty struct pointer *will* panic because NumField > 0.
		// The check `reflect.ValueOf(*configuration).NumField() == 0` is for a struct with no fields at all.
		assert.PanicsWithValue(t, "setConfiguration called with the existing configuration", func() {
			p.setConfiguration(emptyCfg)
		}, "Calling setConfiguration with the same non-empty field struct pointer should panic.")
	})

	t.Run("set nil configuration", func(t *testing.T) {
		p.setConfiguration(nil)
		cfg := p.getConfiguration()
		require.NotNil(t, cfg, "getConfiguration should return a non-nil configuration.")
		assert.Empty(t, cfg.MaestroURL)
		assert.Empty(t, cfg.CustomEndpoints)
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

		loadedMaestroURL := "ws://loadedURL"
		loadedCustomEndpoints := []CustomEndpoint{
			{Name: "service1", Endpoint: "http://service1.example.com"},
			{Name: "service2", Endpoint: "http://service2.example.com"},
		}

		// Mock LoadPluginConfiguration
		// This simulates Mattermost successfully loading the configuration, including CustomEndpoints
		api.On("LoadPluginConfiguration", mock.AnythingOfType("*main.configuration")).Run(func(args mock.Arguments) {
			cfgPtr := args.Get(0).(*configuration)
			cfgPtr.MaestroURL = loadedMaestroURL
			cfgPtr.CustomEndpoints = loadedCustomEndpoints
			// GraphQLPingIntervalSeconds is deliberately not set here to test default handling.
		}).Return(nil).Once()

		// Expect LogInfo to be called because GraphQLPingIntervalSeconds is missing
		api.On("LogInfo", "GraphQLPingIntervalSeconds not configured or invalid, defaulting to 30 seconds.").Return().Once()

		err := p.OnConfigurationChange()
		assert.NoError(t, err)

		retrievedCfg := p.getConfiguration()
		assert.Equal(t, loadedMaestroURL, retrievedCfg.MaestroURL)
		assert.Equal(t, loadedCustomEndpoints, retrievedCfg.CustomEndpoints)

		// Assert that the default value for GraphQLPingIntervalSeconds was applied
		expectedDefaultPingInterval := 30
		assert.NotNil(t, retrievedCfg.GraphQLPingIntervalSeconds, "GraphQLPingIntervalSeconds should have been defaulted")
		if retrievedCfg.GraphQLPingIntervalSeconds != nil {
			assert.Equal(t, expectedDefaultPingInterval, *retrievedCfg.GraphQLPingIntervalSeconds, "GraphQLPingIntervalSeconds should be set to the default value")
		}

		api.AssertExpectations(t)
	})

	t.Run("successful configuration change with empty CustomEndpoints", func(t *testing.T) {
		api := &plugintest.API{}
		p.API = api

		loadedMaestroURL := "ws://loadedURLForEmptyCE"
		// Simulate CustomEndpoints being explicitly empty in the config, or default: [] from plugin.json
		loadedCustomEndpoints := []CustomEndpoint{}

		api.On("LoadPluginConfiguration", mock.AnythingOfType("*main.configuration")).Run(func(args mock.Arguments) {
			cfgPtr := args.Get(0).(*configuration)
			cfgPtr.MaestroURL = loadedMaestroURL
			cfgPtr.CustomEndpoints = loadedCustomEndpoints
			// GraphQLPingIntervalSeconds is set to a valid value this time
			validPingInterval := 15
			cfgPtr.GraphQLPingIntervalSeconds = &validPingInterval
		}).Return(nil).Once()

		// No LogInfo expected for GraphQLPingIntervalSeconds as it's provided
		err := p.OnConfigurationChange()
		assert.NoError(t, err)

		retrievedCfg := p.getConfiguration()
		assert.Equal(t, loadedMaestroURL, retrievedCfg.MaestroURL)
		assert.NotNil(t, retrievedCfg.CustomEndpoints, "CustomEndpoints should be an empty slice, not nil")
		assert.Len(t, retrievedCfg.CustomEndpoints, 0, "CustomEndpoints should be empty")

		require.NotNil(t, retrievedCfg.GraphQLPingIntervalSeconds)
		assert.Equal(t, 15, *retrievedCfg.GraphQLPingIntervalSeconds)


		api.AssertExpectations(t)
	})

	t.Run("failed configuration load", func(t *testing.T) {
		api := &plugintest.API{}
		p.API = api // Ensure plugin uses the mock API

		// Keep the old configuration to check it doesn't change on load failure
		oldCfg := &configuration{
			MaestroURL: "ws://oldURL",
			CustomEndpoints: []CustomEndpoint{{Name: "old", Endpoint: "http://old.co"}},
		}
		p.setConfiguration(oldCfg)

		expectedError := errors.New("load configuration error") // Using errors.New from "github.com/pkg/errors"
		api.On("LoadPluginConfiguration", mock.AnythingOfType("*main.configuration")).Return(expectedError).Once()

		err := p.OnConfigurationChange()
		assert.Error(t, err)
		// Check if the cause of the error is expectedError
		assert.Equal(t, expectedError, errors.Cause(err), "The cause of the error should be the one from LoadPluginConfiguration.")

		retrievedCfg := p.getConfiguration()
		assert.Equal(t, oldCfg, retrievedCfg, "Configuration should not have changed on load error.")
		api.AssertExpectations(t)
	})
}
