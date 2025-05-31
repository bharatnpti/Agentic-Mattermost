package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHelloWorldAPIIntegration(t *testing.T) {
	p := &Plugin{}

	// Setup mock API and Driver for the plugin
	api := &plugintest.API{}
	driver := &plugintest.Driver{} // plugintest.Driver implements plugin.Driver

	p.SetAPI(api) // Plugin needs an API to initialize (e.g., for LogError)
	p.SetDriver(driver) // Plugin might need a driver

	// Initialize router
	p.initializeRouter()
	require.NotNil(t, p.router, "Router should be initialized")

	t.Run("Unauthorized request to /hello", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, "/api/v1/hello", nil)
		require.NoError(t, err)

		recorder := httptest.NewRecorder()
		p.ServeHTTP(nil, recorder, req) // Added nil for plugin.Context

		assert.Equal(t, http.StatusUnauthorized, recorder.Code, "Should return 401 Unauthorized")
		assert.Contains(t, recorder.Body.String(), "Not authorized", "Response body should indicate not authorized")
	})

	t.Run("Authorized request to /hello", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, "/api/v1/hello", nil)
		require.NoError(t, err)

		// Add Mattermost-User-ID header for authorization
		req.Header.Set("Mattermost-User-ID", "testuserid")

		recorder := httptest.NewRecorder()
		p.ServeHTTP(nil, recorder, req) // Added nil for plugin.Context

		assert.Equal(t, http.StatusOK, recorder.Code, "Should return 200 OK")
		assert.Equal(t, "Hello, world!", recorder.Body.String(), "Response body should be 'Hello, world!'")
	})

	t.Run("Not found request", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, "/api/v1/nonexistent", nil)
		require.NoError(t, err)

		req.Header.Set("Mattermost-User-ID", "testuserid") // Authorized

		recorder := httptest.NewRecorder()
		p.ServeHTTP(nil, recorder, req) // Added nil for plugin.Context

		assert.Equal(t, http.StatusNotFound, recorder.Code, "Should return 404 Not Found for non-existent route")
	})
}
