package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestCallOpenAIAPIFunc tests the CallOpenAIAPIFunc function.
// It includes tests for successful API calls, API errors, and JSON parsing issues.
func TestCallOpenAIAPIFunc(t *testing.T) {
	// No need to store/restore OpenAIAPIURL as it's a const and tests will use the parameter
	// originalAPIURL := OpenAIAPIURL
	// defer func() { OpenAIAPIURL = originalAPIURL }()

	t.Run("Successful API Call", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check request method and content type
			assert.Equal(t, "POST", r.Method, "Expected POST request")
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"), "Expected application/json content type")

			// Check Authorization header (optional, but good for verifying API key usage)
			assert.NotEmpty(t, r.Header.Get("Authorization"), "Authorization header should not be empty")
			assert.True(t, strings.HasPrefix(r.Header.Get("Authorization"), "Bearer "), "Authorization header should start with Bearer")

			// Simulate a successful OpenAI API response
			response := OpenAIResponse{
				Choices: []Choice{
					{Message: OpenAIMessage{Role: "assistant", Content: "Test response"}},
				},
			}
			jsonResponse, _ := json.Marshal(response)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, err := w.Write(jsonResponse)
			assert.NoError(t, err, "Error writing response")
		}))
		defer server.Close()

		// OpenAIAPIURL = server.URL // No longer needed, pass server.URL as parameter

		response, err := CallOpenAIAPIFunc("test_api_key", "Test message", server.URL) // Pass server.URL also as apiURL param
		assert.NoError(t, err, "Expected no error for successful API call")
		assert.Equal(t, "Test response", response, "Expected different response message")
	})

	t.Run("API Error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError) // Simulate an API error
		}))
		defer server.Close()

		// OpenAIAPIURL = server.URL // No longer needed, pass server.URL as parameter

		_, err := CallOpenAIAPIFunc("test_api_key", "Test message", server.URL)
		assert.Error(t, err, "Expected an error for API failure")
		assert.Contains(t, err.Error(), "API request failed with status 500", "Error message should contain status code")
	})

	t.Run("Malformed JSON Response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte(`{"malformed_json": "test"`)) // Invalid JSON
			assert.NoError(t, err, "Error writing malformed response")
		}))
		defer server.Close()

		// OpenAIAPIURL = server.URL // No longer needed, pass server.URL as parameter

		_, err := CallOpenAIAPIFunc("test_api_key", "Test message", server.URL)
		assert.Error(t, err, "Expected an error for malformed JSON response")
		assert.Contains(t, err.Error(), "failed to unmarshal response body", "Error message should indicate JSON unmarshalling failure")
	})

	t.Run("Empty Choices in Response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := OpenAIResponse{
				Choices: []Choice{}, // Empty choices array
			}
			jsonResponse, _ := json.Marshal(response)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, err := w.Write(jsonResponse)
			assert.NoError(t, err, "Error writing empty choices response")
		}))
		defer server.Close()

		// OpenAIAPIURL = server.URL // No longer needed, pass server.URL as parameter

		_, err := CallOpenAIAPIFunc("test_api_key", "Test message", server.URL)
		assert.Error(t, err, "Expected an error for empty choices in response")
		assert.Contains(t, err.Error(), "no response text found or choices array is empty", "Error message should indicate no response text or empty choices")
	})

	t.Run("Network Error (Invalid URL)", func(t *testing.T) {
		// For this test, we don't need a mock server. We use an invalid URL.
		invalidURL := "http://localhost:12345/invalid" // A non-existent endpoint
		// OpenAIAPIURL = invalidURL // No longer needed

		_, err := CallOpenAIAPIFunc("test_api_key", "Test message", invalidURL) // Pass invalidURL as apiURL param
		assert.Error(t, err, "Expected a network error for invalid URL")
		// The error message might vary based on the OS and network stack,
		// so we check for a general part of it, e.g., "failed to send request"
		// or specific connection refused errors.
		assert.Contains(t, err.Error(), "failed to send request", "Error message should indicate failure to send request")
	})
}

func TestCallGraphQLAgentFunc(t *testing.T) {
	// originalGraphQLAgentAPIURL := GraphQLAgentAPIURL // No longer needed
	// defer func() { GraphQLAgentAPIURL = originalGraphQLAgentAPIURL }()

	defaultAPIKey := "fakeAPIKey"
	defaultConvID := "conv1"
	defaultUserID := "user1"
	defaultTenantID := "tenantDE"
	defaultChannelID := "channelWEB"
	defaultUserMessage := "Hello server"

	t.Run("Success - Single Message", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "POST", r.Method)
			bodyBytes, err := ioutil.ReadAll(r.Body)
			assert.NoError(t, err)

			var reqPayload GraphQLRequest
			err = json.Unmarshal(bodyBytes, &reqPayload)
			assert.NoError(t, err)
			assert.Contains(t, reqPayload.Query, "subscription Agent")
			assert.NotNil(t, reqPayload.Variables["request"])

			response := GraphQLResponse{
				Data: AgentResponseWrapper{
					Agent: AgentResponseData{
						Messages: []MessageOutput{
							{Content: "Hello from agent", Format: "text", Role: "agent", TurnID: "1"},
						},
					},
				},
			}
			jsonResponse, _ := json.Marshal(response)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, err = w.Write(jsonResponse)
			assert.NoError(t, err)
		}))
		defer server.Close()
		// GraphQLAgentAPIURL = server.URL // No longer needed, pass server.URL as parameter

		messages, err := CallGraphQLAgentFunc(defaultAPIKey, defaultConvID, defaultUserID, defaultTenantID, defaultChannelID, defaultUserMessage, server.URL)

		assert.NoError(t, err)
		assert.Len(t, messages, 1)
		if len(messages) == 1 {
			assert.Equal(t, "Hello from agent", messages[0].Content)
			assert.Equal(t, "text", messages[0].Format)
			assert.Equal(t, "agent", messages[0].Role)
			assert.Equal(t, "1", messages[0].TurnID)
		}
	})

	t.Run("Success - Multiple Messages", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := GraphQLResponse{
				Data: AgentResponseWrapper{
					Agent: AgentResponseData{
						Messages: []MessageOutput{
							{Content: "First message", Format: "text", Role: "agent", TurnID: "1"},
							{Content: "Second message", Format: "markdown", Role: "agent", TurnID: "2"},
						},
					},
				},
			}
			jsonResponse, _ := json.Marshal(response)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, err := w.Write(jsonResponse)
			assert.NoError(t, err)
		}))
		defer server.Close()
		// GraphQLAgentAPIURL = server.URL // No longer needed

		messages, err := CallGraphQLAgentFunc(defaultAPIKey, defaultConvID, defaultUserID, defaultTenantID, defaultChannelID, "Multiple messages test", server.URL)

		assert.NoError(t, err)
		assert.Len(t, messages, 2)
		if len(messages) == 2 {
			assert.Equal(t, "First message", messages[0].Content)
			assert.Equal(t, "Second message", messages[1].Content)
		}
	})

	t.Run("Success - With Anonymization Entities", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := GraphQLResponse{
				Data: AgentResponseWrapper{
					Agent: AgentResponseData{
						AnonymizationEntities: []AnonymizationEntity{
							{Replacement: "[NAME]", Type: "PERSON_NAME", Value: "John Doe"},
						},
						Messages: []MessageOutput{
							{Content: "Hello [NAME]", Format: "text", Role: "agent", TurnID: "1"},
						},
					},
				},
			}
			jsonResponse, _ := json.Marshal(response)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, err := w.Write(jsonResponse)
			assert.NoError(t, err)
		}))
		defer server.Close()
		// GraphQLAgentAPIURL = server.URL // No longer needed

		messages, err := CallGraphQLAgentFunc(defaultAPIKey, defaultConvID, defaultUserID, defaultTenantID, defaultChannelID, "Anonymization test", server.URL)

		assert.NoError(t, err)
		assert.Len(t, messages, 1)
		if len(messages) == 1 {
			assert.Equal(t, "Hello [NAME]", messages[0].Content)
		}
	})

	t.Run("GraphQL Error in Response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := GraphQLResponse{
				Errors: []GraphQLError{
					{Message: "Invalid query"},
					{Message: "Another GraphQL issue"},
				},
			}
			jsonResponse, _ := json.Marshal(response)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK) // GraphQL errors often come with 200 OK
			_, err := w.Write(jsonResponse)
			assert.NoError(t, err)
		}))
		defer server.Close()
		// GraphQLAgentAPIURL = server.URL // No longer needed

		messages, err := CallGraphQLAgentFunc(defaultAPIKey, defaultConvID, defaultUserID, defaultTenantID, defaultChannelID, "Test GraphQL error", server.URL)

		assert.Error(t, err)
		assert.Nil(t, messages)
		assert.Contains(t, err.Error(), "GraphQL query returned errors: Invalid query; Another GraphQL issue")
	})

	t.Run("HTTP Error Status Code", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, err := w.Write([]byte("Internal server error details"))
			assert.NoError(t, err)
		}))
		defer server.Close()
		// GraphQLAgentAPIURL = server.URL // No longer needed

		messages, err := CallGraphQLAgentFunc(defaultAPIKey, defaultConvID, defaultUserID, defaultTenantID, defaultChannelID, "Test HTTP error", server.URL)

		assert.Error(t, err)
		assert.Nil(t, messages)
		assert.Contains(t, err.Error(), "GraphQL API request failed with status 500: Internal server error details")
	})

	t.Run("Malformed JSON Response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte(`{"data": {"agent": {"messages": [{"content": "Test"}]}}`)) // Missing closing brackets
			assert.NoError(t, err)
		}))
		defer server.Close()
		// GraphQLAgentAPIURL = server.URL // No longer needed

		messages, err := CallGraphQLAgentFunc(defaultAPIKey, defaultConvID, defaultUserID, defaultTenantID, defaultChannelID, "Test malformed JSON", server.URL)

		assert.Error(t, err)
		assert.Nil(t, messages)
		assert.Contains(t, err.Error(), "failed to unmarshal GraphQL response body")
		assert.Contains(t, err.Error(), "unexpected end of JSON input") // Specific error from json.Unmarshal
	})

	t.Run("Network Error (Invalid URL)", func(t *testing.T) {
		// This URL should not resolve or connect
		// Using a deliberately non-standard port and domain to increase chance of immediate connection error
		invalidURL := "http://nonexistentdomain.local:12345/graphql"
		// GraphQLAgentAPIURL = invalidURL // No longer needed

		messages, err := CallGraphQLAgentFunc(defaultAPIKey, defaultConvID, defaultUserID, defaultTenantID, defaultChannelID, "Test network error", invalidURL)

		assert.Error(t, err)
		assert.Nil(t, messages)
		// The exact error message can vary based on OS and network conditions (e.g., "connection refused", "no such host")
		// Checking for "failed to send GraphQL request" is a general check.
		assert.Contains(t, err.Error(), "failed to send GraphQL request")
	})

	t.Run("No Messages in Response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := GraphQLResponse{
				Data: AgentResponseWrapper{
					Agent: AgentResponseData{
						Messages: nil, // Explicitly nil messages
					},
				},
			}
			jsonResponse, _ := json.Marshal(response)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, err := w.Write(jsonResponse)
			assert.NoError(t, err)
		}))
		defer server.Close()
		// GraphQLAgentAPIURL = server.URL // No longer needed

		messages, err := CallGraphQLAgentFunc(defaultAPIKey, defaultConvID, defaultUserID, defaultTenantID, defaultChannelID, "Test no messages", server.URL)

		// The current implementation returns an error if Messages is nil.
		assert.Error(t, err)
		assert.Nil(t, messages)
		assert.Contains(t, err.Error(), "no messages found in GraphQL response")
	})

	t.Run("Empty Messages Array in Response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := GraphQLResponse{
				Data: AgentResponseWrapper{
					Agent: AgentResponseData{
						Messages: []MessageOutput{}, // Empty array
					},
				},
			}
			jsonResponse, _ := json.Marshal(response)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, err := w.Write(jsonResponse)
			assert.NoError(t, err)
		}))
		defer server.Close()
		// GraphQLAgentAPIURL = server.URL // No longer needed

		messages, err := CallGraphQLAgentFunc(defaultAPIKey, defaultConvID, defaultUserID, defaultTenantID, defaultChannelID, "Test empty messages array", server.URL)

		// The current implementation returns the empty slice and no error.
		assert.NoError(t, err)
		assert.NotNil(t, messages)
		assert.Len(t, messages, 0)
	})
}
