package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEscapeString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty string", "", ""},
		{"no special characters", "hello", "hello"},
		{"with quotes", `hello "world"`, `hello \"world\"`},
		{"with newline", "hello\nworld", `hello\nworld`},
		{"with quotes and newline", `hello "world"\nbye`, `hello \"world\"\nbye`},
		{"all special characters", `"\n"`, `\"\n\"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, escapeString(tt.input))
		})
	}
}

// Mock WebSocket Upgrader
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true }, // Allow all origins
}

// Helper to simulate a WebSocket server for testing CallGraphQLAgentFunc
// This is a simplified mock server.
type mockGraphQLWsServer struct {
	server        *httptest.Server
	conn          *websocket.Conn
	mu            sync.Mutex
	messageHandler func(t *testing.T, msg GraphQLMessage) // Optional: handler for messages from client
	customResponse func(t *testing.T, clientMsg GraphQLMessage) GraphQLMessage // Function to determine server response
	simulatedError error // Error to return on connect or subscribe
	protocolToUse string // "graphql-ws" or "graphql-transport-ws"
}

func newMockGraphQLWsServer(t *testing.T, handlerFunc func(w http.ResponseWriter, r *http.Request)) *mockGraphQLWsServer {
	s := &mockGraphQLWsServer{}
	s.server = httptest.NewServer(http.HandlerFunc(handlerFunc))
	s.protocolToUse = "graphql-ws" // Default
	return s
}

func (s *mockGraphQLWsServer) Close() {
	s.mu.Lock()
	if s.conn != nil {
		s.conn.Close()
	}
	s.mu.Unlock()
	s.server.Close()
}

func (s *mockGraphQLWsServer) URL() string {
	return strings.Replace(s.server.URL, "http", "ws", 1)
}

// defaultHandler for mock server - handles connection init and subscription
func (s *mockGraphQLWsServer) defaultHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, http.Header{"Sec-WebSocket-Protocol": []string{s.protocolToUse}})
	if err != nil {
		http.Error(w, fmt.Sprintf("could not upgrade: %v", err), http.StatusInternalServerError)
		return
	}

	s.mu.Lock()
	s.conn = conn
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		if s.conn != nil {
			s.conn.Close()
			s.conn = nil
		}
		s.mu.Unlock()
	}()

	// Handle connection_init
	var initMsg GraphQLMessage
	if err := conn.ReadJSON(&initMsg); err != nil {
		return
	}
	if initMsg.Type != "connection_init" {
		return
	}
	ackMsg := GraphQLMessage{Type: "connection_ack"}
	if err := conn.WriteJSON(ackMsg); err != nil {
		return
	}

	// Handle subscription start/subscribe
	var subMsg GraphQLMessage
	if err := conn.ReadJSON(&subMsg); err != nil {
		return
	}

	if s.messageHandler != nil {
		s.messageHandler(nil, subMsg) // t can be nil if not running in a test context for handler
	}

	if s.customResponse != nil {
		responseMsg := s.customResponse(nil, subMsg)
		conn.WriteJSON(responseMsg)
	}
}


func TestCallGraphQLAgentFunc_Success(t *testing.T) {
	mockServer := newMockGraphQLWsServer(t, nil)
	mockServer.protocolToUse = "graphql-ws" // Ensure client uses this
	testSpecificHandler := func(w http.ResponseWriter, r *http.Request) {
		// This is the server-side handler for the WebSocket connection
		conn, err := upgrader.Upgrade(w, r, http.Header{"Sec-WebSocket-Protocol": []string{mockServer.protocolToUse}})
		require.NoError(t, err)
		defer conn.Close()

		// 1. Expect 'connection_init'
		var msg GraphQLMessage
		err = conn.ReadJSON(&msg)
		require.NoError(t, err)
		require.Equal(t, "connection_init", msg.Type)

		// 2. Send 'connection_ack'
		err = conn.WriteJSON(GraphQLMessage{Type: "connection_ack"})
		require.NoError(t, err)

		// 3. Expect 'start' (for graphql-ws) or 'subscribe'
		err = conn.ReadJSON(&msg)
		require.NoError(t, err)
		expectedSubMsgType := "start"
		if mockServer.protocolToUse == "graphql-transport-ws" {
			expectedSubMsgType = "subscribe"
		}
		require.Equal(t, expectedSubMsgType, msg.Type)
		require.NotNil(t, msg.Payload)
		payloadData, _ := msg.Payload.(map[string]interface{})
		assert.Contains(t, payloadData["query"], "subscription")

		// 4. Send 'data'/'next' message (the actual response)
		respPayload := AgentResponse{
			Data: AgentData{
				Agent: AgentDetails{
					Messages: []MessageOutput{{Content: "Test successful", Role: "assistant", Format: "text", TurnID: "turn1"}},
				},
			},
		}
		dataMsgType := "data" // for graphql-ws
		if mockServer.protocolToUse == "graphql-transport-ws" {
			dataMsgType = "next"
		}
		err = conn.WriteJSON(GraphQLMessage{ID: msg.ID, Type: dataMsgType, Payload: respPayload})
		require.NoError(t, err)

		// 5. Optionally, send 'complete'
		// completeMsgType := "complete" // for both
		// err = conn.WriteJSON(GraphQLMessage{ID: msg.ID, Type: completeMsgType})
		// require.NoError(t, err)
	}
	mockServer.server.Config.Handler = http.HandlerFunc(testSpecificHandler)


	messages := []Message{{Role: "user", Content: "Hello", Format: "text", TurnID: "t0"}}
	response, err := CallGraphQLAgentFunc("apiKey", "conv1", "user1", "tenant1", "channel1", messages, mockServer.URL())

	assert.NoError(t, err)
	assert.Equal(t, "Test successful", response)
	mockServer.Close()
}


func TestCallGraphQLAgentFunc_ConnectionFailure(t *testing.T) {
	// Create a server that will be closed immediately to simulate connection failure.
	mockServer := newMockGraphQLWsServer(t, func(w http.ResponseWriter, r *http.Request) {
		// This handler will likely not be called if the server is down.
		t.Log("Server handler called, which is unexpected for connection failure test")
		http.Error(w, "server should be down", http.StatusServiceUnavailable)
	})
	serverURL := mockServer.URL()
	mockServer.Close() // Close the server immediately

	messages := []Message{{Role: "user", Content: "Hello", Format: "text", TurnID: "t0"}}
	_, err := CallGraphQLAgentFunc("apiKey", "conv1", "user1", "tenant1", "channel1", messages, serverURL)

	assert.Error(t, err)
	// Error message depends on OS, but should indicate connection failure
	t.Logf("Connection failure error: %v", err)
	assert.True(t, strings.Contains(err.Error(), "connection failed") || strings.Contains(err.Error(), "dial"), "Error message should indicate a connection/dialing problem")
}

func TestCallGraphQLAgentFunc_SubscriptionFailure_AckNotReceived(t *testing.T) {
	mockServer := newMockGraphQLWsServer(t, nil)
	mockServer.protocolToUse = "graphql-ws"
	testSpecificHandler_AckNotReceived := func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, http.Header{"Sec-WebSocket-Protocol": []string{mockServer.protocolToUse}})
		require.NoError(t, err)
		defer conn.Close()

		// 1. Expect 'connection_init'
		var msg GraphQLMessage
		err = conn.ReadJSON(&msg)
		require.NoError(t, err)
		require.Equal(t, "connection_init", msg.Type)

		// 2. Send something other than 'connection_ack' or close connection
		// conn.WriteJSON(GraphQLMessage{Type: "error", Payload: "No ack for you"})
		// Or simply close:
		// server closes connection instead of sending ack
	}
	mockServer.server.Config.Handler = http.HandlerFunc(testSpecificHandler_AckNotReceived)


	messages := []Message{{Role: "user", Content: "Hello", Format: "text", TurnID: "t0"}}
	_, err := CallGraphQLAgentFunc("apiKey", "conv1", "user1", "tenant1", "channel1", messages, mockServer.URL())

	assert.Error(t, err)
	t.Logf("Ack not received error: %v", err)
	assert.Contains(t, err.Error(), "ack read error", "Expected error due to no ACK or wrong ACK")
	mockServer.Close()
}

func TestCallGraphQLAgentFunc_TimeoutWaitingForMessage(t *testing.T) {
	mockServer := newMockGraphQLWsServer(t, nil)
	mockServer.protocolToUse = "graphql-ws"
	testSpecificHandler_Timeout := func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, http.Header{"Sec-WebSocket-Protocol": []string{mockServer.protocolToUse}})
		require.NoError(t, err)
		defer conn.Close()

		// 1. Expect 'connection_init' & send 'connection_ack'
		var msg GraphQLMessage
		err = conn.ReadJSON(&msg)
		require.NoError(t, err)
		err = conn.WriteJSON(GraphQLMessage{Type: "connection_ack"})
		require.NoError(t, err)

		// 2. Expect 'start'
		err = conn.ReadJSON(&msg)
		require.NoError(t, err)

		// 3. Server never sends a 'data'/'next' message or 'complete'
		// Client should time out based on its context.
		// Keep the connection open but silent.
		time.Sleep(2 * time.Second) // Keep connection alive beyond typical test timeout if client has short timeout
	}
	mockServer.server.Config.Handler = http.HandlerFunc(testSpecificHandler_Timeout)

	// CallGraphQLAgentFunc has an internal 30-second timeout.
	// We need to make this test timeout shorter if we want to test it explicitly.
	// For now, we assume the internal timeout of CallGraphQLAgentFunc will be hit.
	// To make this test faster, we could pass a context with a shorter deadline to CallGraphQLAgentFunc,
	// but the current signature doesn't allow that. So this test will run for up to 30s.
	// For CI, this is too long. Let's modify CallGraphQLAgentFunc to accept a context for testing,
	// or use a shorter, hardcoded timeout for tests.
	// The original function uses a hardcoded 30s timeout. This test will be slow.
	// Alternative: If the client's context is cancelled by the test itself after a short period,
	// this can also simulate a timeout from the client's perspective.
	// The `CallGraphQLAgentFunc` has `ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)`

	messages := []Message{{Role: "user", Content: "Hello", Format: "text", TurnID: "t0"}}

	// To make this test faster, we can't easily change the 30s timeout in CallGraphQLAgentFunc
	// without refactoring it. We'll assume the timeout works as intended.
	// If this test is run in a context where 30s is too long, it would need adjustment
	// of the source code or a more complex test setup.
	// For now, let's just test the error message for timeout.
	t.Log("Testing timeout, this might take up to 30 seconds due to internal timeout in CallGraphQLAgentFunc...")

	startTime := time.Now()
	_, err := CallGraphQLAgentFunc("apiKey", "conv1", "user1", "tenant1", "channel1", messages, mockServer.URL())
	duration := time.Since(startTime)
	t.Logf("CallGraphQLAgentFunc returned after %v", duration)


	assert.Error(t, err)
	assert.Contains(t, err.Error(), "timeout waiting for first message", "Expected timeout error")

	// Check if it actually timed out near the expected time (e.g. < 35s, > 25s if it was 30s)
	// This is hard to assert precisely.
	// For this test, we just check the error message.

	mockServer.Close()
}


func TestCallGraphQLAgentFunc_GraphQLReturnsError(t *testing.T) {
	mockServer := newMockGraphQLWsServer(t, nil)
	mockServer.protocolToUse = "graphql-ws"
	testSpecificHandler_GraphQLError := func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, http.Header{"Sec-WebSocket-Protocol": []string{mockServer.protocolToUse}})
		require.NoError(t, err)
		defer conn.Close()

		var msg GraphQLMessage
		err = conn.ReadJSON(&msg) // init
		require.NoError(t, err)
		err = conn.WriteJSON(GraphQLMessage{Type: "connection_ack"}) // ack
		require.NoError(t, err)
		err = conn.ReadJSON(&msg) // subscribe/start
		require.NoError(t, err)

		// Send GraphQL error message
		errorPayload := AgentResponse{ // This is the structure the client expects for data/next messages
			Errors: []GQLError{{Message: "Intentional GraphQL error"}},
		}
		dataMsgType := "data"
		if mockServer.protocolToUse == "graphql-transport-ws" {
			dataMsgType = "next"
		}
		err = conn.WriteJSON(GraphQLMessage{ID: msg.ID, Type: dataMsgType, Payload: errorPayload})
		require.NoError(t, err)
	}
	mockServer.server.Config.Handler = http.HandlerFunc(testSpecificHandler_GraphQLError)

	messages := []Message{{Role: "user", Content: "Hello", Format: "text", TurnID: "t0"}}
	_, err := CallGraphQLAgentFunc("apiKey", "conv1", "user1", "tenant1", "channel1", messages, mockServer.URL())

	assert.Error(t, err)
	t.Logf("GraphQL error test: %v", err)
	assert.Contains(t, err.Error(), "GraphQL error: Intentional GraphQL error")
	mockServer.Close()
}

// Testing the GraphQLSubscriptionClient itself can be done by more granular tests
// for Connect, Subscribe, Listen, Close methods, but the above tests for
// CallGraphQLAgentFunc cover the integrated behavior.

// Note: The complexity of accurately mocking a WebSocket server and various protocol states
// means these tests might need refinement or may hit edge cases depending on environment/timing.
// The current implementation attempts a basic mock server.
// End of file
