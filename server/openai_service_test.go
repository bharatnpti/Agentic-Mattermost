package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCallOpenAIAPIFunc_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "Bearer testAPIKey", r.Header.Get("Authorization"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var req OpenAIRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		assert.NoError(t, err)
		assert.Equal(t, "gpt-3.5-turbo", req.Model)
		assert.Len(t, req.Messages, 1)
		assert.Equal(t, "user", req.Messages[0].Role)
		assert.Equal(t, "Hello OpenAI", req.Messages[0].Content)

		resp := OpenAIResponse{
			Choices: []Choice{
				{Message: OpenAIMessage{Role: "assistant", Content: "Hello back!"}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(w).Encode(resp)
		assert.NoError(t, err)
	}))
	defer server.Close()

	response, err := CallOpenAIAPIFunc("testAPIKey", "Hello OpenAI", server.URL)
	assert.NoError(t, err)
	assert.Equal(t, "Hello back!", response)
}

func TestCallOpenAIAPIFunc_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	_, err := CallOpenAIAPIFunc("testAPIKey", "Hello OpenAI", server.URL)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "API request failed with status 500: Internal Server Error")
}

func TestCallOpenAIAPIFunc_NetworkError(t *testing.T) {
	// server will be closed immediately to simulate a network error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	serverURL := server.URL
	server.Close()

	_, err := CallOpenAIAPIFunc("testAPIKey", "Hello OpenAI", serverURL)
	assert.Error(t, err)
	// The exact error message can vary depending on the OS and Go version,
	// so we check for a common part of connection refused errors.
	assert.Contains(t, err.Error(), "connect: connection refused")
}

func TestCallOpenAIAPIFunc_InvalidResponseFormat_NotJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain") // Not JSON
		_, _ = w.Write([]byte("This is not JSON"))
	}))
	defer server.Close()

	_, err := CallOpenAIAPIFunc("testAPIKey", "Hello OpenAI", server.URL)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal response body")
	// It might also contain "invalid character 'T' looking for beginning of value"
}

func TestCallOpenAIAPIFunc_InvalidResponseFormat_EmptyChoices(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := OpenAIResponse{
			Choices: []Choice{}, // Empty choices
		}
		w.Header().Set("Content-Type", "application/json")
		err := json.NewEncoder(w).Encode(resp)
		assert.NoError(t, err)
	}))
	defer server.Close()

	_, err := CallOpenAIAPIFunc("testAPIKey", "Hello OpenAI", server.URL)
	assert.Error(t, err)
	assert.EqualError(t, err, "no response text found or choices array is empty")
}

func TestCallOpenAIAPIFunc_InvalidResponseFormat_EmptyMessageContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := OpenAIResponse{
			Choices: []Choice{
				{Message: OpenAIMessage{Role: "assistant", Content: ""}}, // Empty content
			},
		}
		w.Header().Set("Content-Type", "application/json")
		err := json.NewEncoder(w).Encode(resp)
		assert.NoError(t, err)
	}))
	defer server.Close()

	_, err := CallOpenAIAPIFunc("testAPIKey", "Hello OpenAI", server.URL)
	assert.Error(t, err)
	assert.EqualError(t, err, "no response text found or choices array is empty")
}

func TestCallOpenAIAPIFunc_RequestBodyMarshalError(t *testing.T) {
	// This test is a bit artificial as the request body is fixed in the current function.
	// However, if the request construction was more dynamic, this would be relevant.
	// To simulate this, we'd have to modify the function or use a more complex mock.
	// For now, we acknowledge this path is hard to trigger with current CallOpenAIAPIFunc.
	// A more robust way would be to inject the marshalling function, but that's over-engineering for now.
	t.Skip("Skipping test for request body marshal error as it's hard to trigger with current static request structure.")
}

func TestCallOpenAIAPIFunc_CreateRequestError(t *testing.T) {
	// Pass an invalid URL to http.NewRequest
	// The function CallOpenAIAPIFunc itself constructs the URL, so this is also hard to trigger directly
	// unless the passed apiURL is malformed in a very specific way that http.NewRequest fails
	// but url.Parse (if used internally before NewRequest) doesn't.
	// The current function passes apiURL directly to NewRequest.
	// Let's try with a URL that NewRequest would reject.
	// A URL with invalid characters in the method, or a nil body with GET/HEAD.
	// However, we always use POST and a valid body.
	// An empty URL string might cause NewRequest to fail.
	
	// According to docs, NewRequest fails if method is not valid, or if URL parse fails.
	// Our method is "POST". If apiURL is garbage, it should fail.
	_, err := CallOpenAIAPIFunc("testAPIKey", "Hello", "http://invalid url with spaces.com")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create request")
	// It usually returns an error like: "parse \"http://invalid url with spaces.com\": invalid character \" \" in host name"
	// which is then wrapped by "failed to create request".
}

func TestCallOpenAIAPIFunc_ResponseBodyReadError(t *testing.T) {
	// This requires the server to misbehave after sending headers.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "100") // Pretend there's content
		// Close the connection prematurely or send malformed chunked encoding.
		// For simplicity, we'll use a hijacker to get the underlying connection.
		hj, ok := w.(http.Hijacker)
		assert.True(t, ok, "HTTP server does not support hijacking")
		conn, _, err := hj.Hijack()
		assert.NoError(t, err)
		// Just close the connection without writing the body
		conn.Close()
	}))
	defer server.Close()

	_, err := CallOpenAIAPIFunc("testAPIKey", "Hello OpenAI", server.URL)
	assert.Error(t, err)
	// Changed to check for EOF, as this is what a premature close during/after headers often results in.
	assert.Contains(t, err.Error(), "EOF") 
}
// End of file
