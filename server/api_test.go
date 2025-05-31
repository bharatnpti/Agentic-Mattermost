package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestHelloWorld(t *testing.T) {
	apiMock := &plugintest.API{}
	// Test case where Write fails
	apiMock.On("LogError", "Failed to write response", "error", mock.AnythingOfType("*errors.errorString")).Maybe() // Allow LogError if write fails

	p := &Plugin{}
	p.SetAPI(apiMock) // Set the mock API for the plugin instance

	req, err := http.NewRequest("GET", "/api/v1/hello", nil)
	assert.NoError(t, err)

	rr := httptest.NewRecorder()
	p.HelloWorld(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "Hello, world!", rr.Body.String())

	// Assertions for LogError can be added here if a write failure is simulated,
	// but directly simulating a ResponseWriter.Write failure is non-trivial.
	// The current test mainly checks the success path.
}

func TestMattermostAuthorizationRequired_Authorized(t *testing.T) {
	apiMock := &plugintest.API{} // Not strictly needed for this middleware test unless it logs
	p := &Plugin{}
	p.SetAPI(apiMock)

	// Create a dummy handler to be wrapped by the middleware
	dummyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("Authorized!"))
	})

	// Create the middleware-wrapped handler
	authHandler := p.MattermostAuthorizationRequired(dummyHandler)

	req, err := http.NewRequest("GET", "/test", nil)
	assert.NoError(t, err)
	req.Header.Set("Mattermost-User-ID", "testuserid") // Set the required header

	rr := httptest.NewRecorder()
	authHandler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "Authorized!", rr.Body.String())
	apiMock.AssertNotCalled(t, "LogWarn") // Ensure no warning log for successful auth
}

func TestMattermostAuthorizationRequired_Unauthorized(t *testing.T) {
	apiMock := &plugintest.API{}
	// Middleware currently does not log on unauthorized, so no LogWarn expectation.
	// If logging was desired, it should be added to MattermostAuthorizationRequired.

	p := &Plugin{}
	p.SetAPI(apiMock)

	dummyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// This handler should not be called
		t.Error("Dummy handler called unexpectedly on unauthorized request")
	})

	authHandler := p.MattermostAuthorizationRequired(dummyHandler)

	req, err := http.NewRequest("GET", "/test", nil)
	assert.NoError(t, err)
	// Mattermost-User-ID header is NOT set

	rr := httptest.NewRecorder()
	authHandler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	assert.Equal(t, "Not authorized\n", rr.Body.String()) // http.Error adds a newline
	apiMock.AssertExpectations(t)                         // Verify no unexpected calls
}

// TestServeHTTP_Routing checks if the router setup in ServeHTTP (now initializeRouter) works.
func TestServeHTTP_RoutingToHelloWorld(t *testing.T) {
	apiMock := &plugintest.API{}
	apiMock.On("LogInfo", mock.Anything, mock.Anything, mock.Anything).Maybe()
	apiMock.On("LogError", mock.Anything, mock.Anything, mock.Anything).Maybe()

	p := &Plugin{}
	p.SetAPI(apiMock)
	// Manually initialize the router as OnActivate would do
	p.initializeRouter()
	// We need to ensure initializeRouter is called if ServeHTTP relies on p.router.
	// In the refactored code, initializeRouter is called in OnActivate.
	// For a direct test of ServeHTTP's behavior with a pre-initialized router:
	// p.router = mux.NewRouter() // or call p.initializeRouter()
	// p.initializeRouter() // This sets up p.router

	req, err := http.NewRequest("GET", "/api/v1/hello", nil)
	assert.NoError(t, err)
	req.Header.Set("Mattermost-User-ID", "testuserid") // To pass auth

	rr := httptest.NewRecorder()
	// ServeHTTP is the entry point for plugin HTTP requests
	p.ServeHTTP(&plugin.Context{}, rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "Hello, world!", rr.Body.String())
}

func TestServeHTTP_RoutingToUnknownPath(t *testing.T) {
	apiMock := &plugintest.API{}
	// No specific logging expected for a 404 from the router itself, unless we add custom NotFoundHandler
	p := &Plugin{}
	p.SetAPI(apiMock)
	p.initializeRouter()

	req, err := http.NewRequest("GET", "/api/v1/unknown", nil)
	assert.NoError(t, err)
	req.Header.Set("Mattermost-User-ID", "testuserid")

	rr := httptest.NewRecorder()
	p.ServeHTTP(&plugin.Context{}, rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code) // gorilla/mux default NotFound is 404
}

// Test for router initialization within ServeHTTP if OnActivate somehow wasn't called
// This tests the safety check `if p.router == nil` in ServeHTTP.
func TestServeHTTP_RouterNilSafetyCheck(t *testing.T) {
	apiMock := &plugintest.API{}
	// Expect the actual LogError call from ServeHTTP when router is nil
	apiMock.On("LogError", "HTTP router not initialized").Once()

	p := &Plugin{}
	p.SetAPI(apiMock)
	// p.router is deliberately nil here to trigger the safety check

	req, err := http.NewRequest("GET", "/api/v1/hello", nil) // Request path doesn't matter much here
	assert.NoError(t, err)
	// No Mattermost-User-ID needed as it should fail before routing/auth middleware

	rr := httptest.NewRecorder()
	p.ServeHTTP(&plugin.Context{}, rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Equal(t, "Plugin router not initialized\n", rr.Body.String()) // http.Error adds a newline
	apiMock.AssertExpectations(t)
}

// Test initializeRouter ensures that routes are correctly set up.
// This is implicitly tested by TestServeHTTP_RoutingToHelloWorld,
// but a more direct test can be useful.
func TestInitializeRouter(t *testing.T) {
	apiMock := &plugintest.API{}
	apiMock.On("LogInfo", mock.Anything, mock.Anything, mock.Anything).Maybe()
	apiMock.On("LogError", mock.Anything, mock.Anything, mock.Anything).Maybe()

	p := &Plugin{}
	p.SetAPI(apiMock)
	p.initializeRouter() // Call the function directly

	assert.NotNil(t, p.router)

	// Test if /api/v1/hello route exists and is handled by HelloWorld
	// This requires making a request to the router.
	req := httptest.NewRequest("GET", "/api/v1/hello", nil)
	req.Header.Set("Mattermost-User-ID", "userID") // For auth middleware
	rr := httptest.NewRecorder()
	p.router.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "Hello, world!", rr.Body.String())

	// Test a non-existent route on the API sub-router
	reqUnknown := httptest.NewRequest("GET", "/api/v1/foo", nil)
	reqUnknown.Header.Set("Mattermost-User-ID", "userID")
	rrUnknown := httptest.NewRecorder()
	p.router.ServeHTTP(rrUnknown, reqUnknown)
	assert.Equal(t, http.StatusNotFound, rrUnknown.Code)

	// Test a route not under /api/v1 (should also be 404 by this router setup)
	reqRootUnknown := httptest.NewRequest("GET", "/foo", nil)
	reqRootUnknown.Header.Set("Mattermost-User-ID", "userID") // Auth would apply if path matched anything
	rrRootUnknown := httptest.NewRecorder()
	p.router.ServeHTTP(rrRootUnknown, reqRootUnknown)
	assert.Equal(t, http.StatusNotFound, rrRootUnknown.Code)
}
