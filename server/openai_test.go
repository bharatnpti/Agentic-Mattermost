package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCallOpenAIAPI(t *testing.T) {
	t.Run("successful API response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "POST", r.Method)
			assert.Equal(t, "Bearer test-api-key", r.Header.Get("Authorization"))
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

			var reqBody OpenAIRequest
			err := json.NewDecoder(r.Body).Decode(&reqBody)
			require.NoError(t, err)
			assert.Equal(t, "gpt-3.5-turbo", reqBody.Model)
			require.Len(t, reqBody.Messages, 1)
			assert.Equal(t, "user", reqBody.Messages[0].Role)
			assert.Equal(t, "Hello", reqBody.Messages[0].Content)

			resp := OpenAIResponse{
				Choices: []Choice{
					{
						Message: OpenAIMessage{
							Role:    "assistant",
							Content: "Hi there!",
						},
					},
				},
			}
			jsonResp, _ := json.Marshal(resp)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(jsonResp)
		}))
		defer server.Close()

		response, err := CallOpenAIAPIFunc("test-api-key", "Hello", server.URL)
		require.NoError(t, err)
		assert.Equal(t, "Hi there!", response)
	})

	t.Run("API error response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error": {"message": "Invalid API key"}}`))
		}))
		defer server.Close()

		response, err := CallOpenAIAPIFunc("invalid-api-key", "Hello", server.URL)
		require.Error(t, err)
		assert.Empty(t, response)
		assert.Contains(t, err.Error(), "API request failed with status 401")
		assert.Contains(t, err.Error(), "Invalid API key")
	})

	t.Run("malformed JSON response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"choices": [{"message": {"content": "Test"}}`)) // Malformed JSON
		}))
		defer server.Close()

		response, err := CallOpenAIAPIFunc("test-api-key", "Hello", server.URL)
		require.Error(t, err)
		assert.Empty(t, response)
		assert.Contains(t, err.Error(), "failed to unmarshal response body")
	})

	t.Run("empty choices in response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := OpenAIResponse{
				Choices: []Choice{}, // Empty choices
			}
			jsonResp, _ := json.Marshal(resp)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(jsonResp)
		}))
		defer server.Close()

		response, err := CallOpenAIAPIFunc("test-api-key", "Hello", server.URL)
		require.Error(t, err)
		assert.Empty(t, response)
		assert.EqualError(t, err, "no response text found or choices array is empty")
	})

	t.Run("error reading response body", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Length", "1") // Set content length but send no body or less
		}))
		defer server.Close()

		response, err := CallOpenAIAPIFunc("test-api-key", "Hello", server.URL)
		require.Error(t, err)
		assert.Empty(t, response)
		if !strings.Contains(err.Error(), "failed to read response body") && !strings.Contains(err.Error(), "unexpected EOF") {
			t.Errorf("Expected error related to reading response body, but got: %v", err)
		}
	})
}
