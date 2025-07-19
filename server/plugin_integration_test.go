package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestHelloWorldAPIIntegration(t *testing.T) {
	p := &Plugin{}

	// Setup mock API and Driver for the plugin
	api := &plugintest.API{}
	driver := &plugintest.Driver{} // plugintest.Driver implements plugin.Driver

	p.SetAPI(api)       // Plugin needs an API to initialize (e.g., for LogError)
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

func TestCallWorkflowMessageAPI(t *testing.T) {
	p := &Plugin{}
	api := &plugintest.API{}
	// Mock LogError and LogDebug to avoid panics on error cases
	api.On("LogError", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	api.On("LogError", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	api.On("LogError", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	api.On("LogError", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	api.On("LogError", mock.Anything, mock.Anything).Return(nil)
	api.On("LogDebug", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	api.On("LogDebug", mock.Anything, mock.Anything).Return(nil)
	p.SetAPI(api)

	t.Run("2xx success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method)
			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"status":"ok"}`))
		}))
		defer server.Close()

		resp, err := p.CallWorkflowMessageAPI("chan", "msg", "user", "thread", server.URL, server.Client())
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, "ok", resp.Status)
	})

	t.Run("4xx client error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("bad request"))
		}))
		defer server.Close()

		resp, err := p.CallWorkflowMessageAPI("chan", "msg", "user", "thread", server.URL, server.Client())
		assert.Error(t, err)
		assert.Nil(t, resp)
		assert.Contains(t, err.Error(), "status code 400")
	})

	t.Run("5xx server error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("internal error"))
		}))
		defer server.Close()

		resp, err := p.CallWorkflowMessageAPI("chan", "msg", "user", "thread", server.URL, server.Client())
		assert.Error(t, err)
		assert.Nil(t, resp)
		assert.Contains(t, err.Error(), "status code 500")
	})

	t.Run("timeout error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(200 * time.Millisecond)
		}))
		defer server.Close()

		client := &http.Client{Timeout: 50 * time.Millisecond}
		resp, err := p.CallWorkflowMessageAPI("chan", "msg", "user", "thread", server.URL, client)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, "timeout", resp.Status)
	})
}
