package main

import (
	"net/http"

	"github.com/gorilla/mux"
)

// initializeRouter sets up the HTTP router for the plugin.
func (p *Plugin) initializeRouter() {
	p.router = mux.NewRouter()

	// Middleware to require that the user is logged in
	p.router.Use(p.MattermostAuthorizationRequired)

	apiRouter := p.router.PathPrefix("/api/v1").Subrouter()

	apiRouter.HandleFunc("/hello", p.HelloWorld).Methods(http.MethodGet)

	// Note: The actual serving of HTTP requests is done by the ServeHTTP method in plugin.go
}

func (p *Plugin) MattermostAuthorizationRequired(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := r.Header.Get("Mattermost-User-ID")
		if userID == "" {
			http.Error(w, "Not authorized", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (p *Plugin) HelloWorld(w http.ResponseWriter, r *http.Request) {
	if _, err := w.Write([]byte("Hello, world!")); err != nil {
		p.API.LogError("Failed to write response", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
