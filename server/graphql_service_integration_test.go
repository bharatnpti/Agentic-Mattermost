package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

// mockGQLServer struct will hold components of our mock WebSocket server
// for GraphQL subscriptions.
type mockGQLServer struct {
	httpServer      *httptest.Server
	serverConn      *websocket.Conn // Server-side connection established with a client
	mu              sync.Mutex
	wg              sync.WaitGroup      // To wait for server goroutines if any
	clientConnected chan struct{}     // Signals when a client has connected and handshake is done
	receivedPingCount int             // Counter for received pings
	lastPingPayload string            // Store the payload of the last ping
	isClosed        bool              // Flag to indicate if the server has been closed
	t               *testing.T
	upgrader        websocket.Upgrader
}

// newMockWebsocketServer sets up and starts a new mock WebSocket server.
// It takes a handler function that defines the server's behavior after connection.
func newMockWebsocketServer(t *testing.T, customHandler func(w http.ResponseWriter, r *http.Request, conn *websocket.Conn, server *mockGQLServer)) *mockGQLServer {
	t.Helper()

	mockServer := &mockGQLServer{
		clientConnected: make(chan struct{}),
		t:               t,
		upgrader: websocket.Upgrader{
			Subprotocols: []string{"graphql-ws", "graphql-transport-ws"}, // Common subprotocols for GraphQL
			CheckOrigin: func(r *http.Request) bool { // Allow all origins for testing
				return true
			},
		},
	}

	// Define the HTTP handler that will upgrade to WebSocket
	handlerFunc := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := mockServer.upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("mockServer: failed to upgrade connection: %v", err)
			return
		}

		mockServer.mu.Lock()
		mockServer.serverConn = conn
		mockServer.isClosed = false
		mockServer.mu.Unlock()

		// Handle pings from the client
		conn.SetPingHandler(func(appData string) error {
			mockServer.mu.Lock()
			mockServer.receivedPingCount++
			mockServer.lastPingPayload = appData
			log.Printf("mockServer: received ping from client with data: %s", appData)
			mockServer.mu.Unlock()
			// gorilla/websocket handles sending pongs automatically by default
			return nil
		})

		// Perform GraphQL handshake
		var initMsg map[string]interface{}
		err = conn.ReadJSON(&initMsg)
		if err != nil {
			log.Printf("mockServer: error reading init message: %v", err)
			conn.Close()
			return
		}

		msgType, _ := initMsg["type"].(string)
		if msgType != "connection_init" {
			log.Printf("mockServer: expected 'connection_init', got '%s'", msgType)
			conn.Close()
			return
		}
		log.Printf("mockServer: received 'connection_init' from client")

		err = conn.WriteJSON(map[string]string{"type": "connection_ack"})
		if err != nil {
			log.Printf("mockServer: error sending 'connection_ack': %v", err)
			conn.Close()
			return
		}
		log.Printf("mockServer: sent 'connection_ack' to client")

		close(mockServer.clientConnected) // Signal that client is connected and handshake is done

		// If a custom handler is provided, call it.
		// This allows tests to define specific message exchange logic.
		if customHandler != nil {
			customHandler(w, r, conn, mockServer)
		} else {
			// Default behavior: keep connection open and read messages until close or error
			// This loop is important for the server to process control messages like pings.
			for {
				_, _, readErr := conn.ReadMessage()
				if readErr != nil {
					mockServer.mu.Lock()
					isAlreadyClosed := mockServer.isClosed
					mockServer.mu.Unlock()
					if !isAlreadyClosed { // Avoid logging errors if we closed it intentionally
						log.Printf("mockServer: error reading message: %v", readErr)
					}
					break
				}
			}
		}
	})

	mockServer.httpServer = httptest.NewServer(handlerFunc)

	// Replace http with ws/wss for the URL
	mockServer.httpServer.URL = strings.Replace(mockServer.httpServer.URL, "http", "ws", 1)
	log.Printf("mockServer: listening on %s", mockServer.httpServer.URL)

	return mockServer
}

// Close shuts down the mock server and associated resources.
func (ms *mockGQLServer) Close() {
	ms.mu.Lock()
	if ms.isClosed {
		ms.mu.Unlock()
		return
	}
	ms.isClosed = true
	ms.mu.Unlock()

	log.Printf("mockServer: closing server...")
	if ms.serverConn != nil {
		ms.serverConn.Close() // Close the server-side client connection
	}
	ms.httpServer.Close() // Close the HTTP test server
	log.Printf("mockServer: server closed.")
}

// TestGraphQLClient_FrameworkCheck is a basic test to ensure the mock server
// framework can be initialized and a client can perform the handshake.
func TestGraphQLClient_FrameworkCheck(t *testing.T) {
	// Use the default customHandler (nil) for this basic check,
	// which just keeps the connection open after handshake.
	mockServer := newMockWebsocketServer(t, nil)
	defer mockServer.Close()

	// Create a client (simplified for framework check)
	// In real tests, this would be our GraphQLSubscriptionClient
	dialer := websocket.Dialer{
		Subprotocols: []string{"graphql-ws"},
	}
	clientConn, _, err := dialer.Dial(mockServer.httpServer.URL, nil)
	require.NoError(t, err, "Client failed to connect to mock server")
	defer clientConn.Close()

	// Client sends connection_init
	err = clientConn.WriteJSON(map[string]interface{}{"type": "connection_init", "payload": map[string]string{}})
	require.NoError(t, err, "Client failed to send connection_init")

	// Client expects connection_ack
	var ackMsg map[string]string
	err = clientConn.ReadJSON(&ackMsg)
	require.NoError(t, err, "Client failed to read message from server")
	require.Equal(t, "connection_ack", ackMsg["type"], "Client did not receive connection_ack")

	log.Println("TestGraphQLClient_FrameworkCheck: Client handshake successful")

	// Check if clientConnected signal was received by the server
	select {
	case <-mockServer.clientConnected:
		log.Println("TestGraphQLClient_FrameworkCheck: Server signaled client connection")
	case <-time.After(2 * time.Second):
		t.Fatal("TestGraphQLClient_FrameworkCheck: Timeout waiting for server to signal client connection")
	}
}

// Note: The GraphQLSubscriptionClient (from graphql_service.go) itself is not
// directly tested here yet. This file sets up the server-side mock infrastructure.
// Subsequent tests will use this mock server to test the actual client's behavior,
// including pinging, subscription lifecycle, and message handling.

// The mockGQLServer's customHandler `func(w http.ResponseWriter, r *http.Request, conn *websocket.Conn, server *mockGQLServer)`
// will be used in other tests to simulate specific GraphQL server responses (e.g., sending 'data', 'error', 'complete' messages).
// For example, a test for client subscription would have the customHandler:
// 1. Wait for the client to send a 'subscribe' message.
// 2. Respond with 'next'/'data' messages.
// 3. Optionally send 'complete' or 'error'.
// The ping test will rely on the PingHandler set up in the main server logic.

// Placeholder for a basic ping test (to be detailed in a later step)
// func TestGraphQLClient_Ping(t *testing.T) {
// 	mockServer := newMockWebsocketServer(t, nil) // Default handler for now
// 	defer mockServer.Close()

// 	// TODO: Initialize your GraphQLSubscriptionClient here, pointing to mockServer.httpServer.URL
// 	// TODO: Connect the client, which should trigger pings.
// 	// TODO: Wait for a few seconds.
// 	// TODO: Assert mockServer.receivedPingCount > 0.
// }

// TestGraphQLClient_SuccessfulSubscriptionAndMultipleMessages tests the client's ability
// to subscribe, process multiple messages, and handle a graceful server shutdown.
func TestGraphQLClient_SuccessfulSubscriptionAndMultipleMessages(t *testing.T) {
	subscriptionTestHandler := func(w http.ResponseWriter, r *http.Request, conn *websocket.Conn, server *mockGQLServer) {
		// This handler is called after the initial connection_init/connection_ack handshake.
		// 1. Read subscription message from client
		var subMsg map[string]interface{}
		err := conn.ReadJSON(&subMsg)
		require.NoError(t, err, "mockServer: failed to read subscription message")

		subID, ok := subMsg["id"].(string)
		require.True(t, ok, "mockServer: subscription message missing id")
		require.NotEmpty(t, subID, "mockServer: subscription id is empty")

		msgType, _ := subMsg["type"].(string)
		// Protocol "graphql-ws" uses "start", "graphql-transport-ws" uses "subscribe"
		isSubscribe := msgType == "start" || msgType == "subscribe"
		require.True(t, isSubscribe, fmt.Sprintf("mockServer: expected 'start' or 'subscribe', got '%s'", msgType))
		log.Printf("mockServer: received subscription request with ID: %s, Type: %s", subID, msgType)

		// 2. Send multiple mock AgentResponse payloads
		payloads := []AgentResponse{
			{ // Payload 1
				Data: AgentData{Agent: AgentDetails{
					Messages: []MessageOutput{{Content: "Hello from mock server 1", Role: "assistant", Format: "text", TurnID: "t1"}},
				}},
			},
			{ // Payload 2
				Data: AgentData{Agent: AgentDetails{
					Messages: []MessageOutput{{Content: "Hello from mock server 2", Role: "assistant", Format: "text", TurnID: "t2"}},
				}},
			},
		}

		for i, p := range payloads {
			// Determine message type based on detected protocol for subscriptions
			serverMsgType := "next" // graphql-transport-ws
			if msgType == "start" { // graphql-ws
				serverMsgType = "data"
			}
			err = conn.WriteJSON(map[string]interface{}{"id": subID, "type": serverMsgType, "payload": p})
			require.NoError(t, err, fmt.Sprintf("mockServer: failed to send data message %d", i+1))
			log.Printf("mockServer: sent data message %d for subscription ID: %s", i+1, subID)
			time.Sleep(50 * time.Millisecond) // Small delay to allow client to process
		}

		// 3. Graceful server close (after a short delay to ensure client processes messages)
		time.Sleep(100 * time.Millisecond)
		log.Printf("mockServer: sending CloseNormalClosure to client for subID: %s", subID)
		err = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "Closing normally"))
		// Don't require.NoError here, as client might close connection first upon receiving all data + complete (if we sent one)
		if err != nil {
			log.Printf("mockServer: error sending close message: %v (this might be ok if client closed first)", err)
		}
	}

	mockServer := newMockWebsocketServer(t, subscriptionTestHandler)
	defer mockServer.Close()

	messageChan := make(chan string, 10)
	errorChan := make(chan error, 5)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		CallGraphQLAgentFunc(
			ctx,
			"test-api-key",
			"test-convo-id",
			"test-user-id",
			"test-tenant-id",
			"test-channel-id",
			[]Message{{Content: "Initial message", Format: "text", Role: "user", TurnID: "t0"}}, // Dummy initial messages
			mockServer.httpServer.URL, // Use the mock server's URL
			1*time.Second,             // Ping interval
			messageChan,
			errorChan,
		)
		log.Println("Test: CallGraphQLAgentFunc goroutine finished.")
	}()

	var receivedMessages []string
	var receivedErrors []error
	collectionTimeout := 10 * time.Second // Increased timeout
	collectingDone := time.NewTimer(collectionTimeout)

collectLoop:
	for {
		select {
		case msg, ok := <-messageChan:
			if !ok {
				log.Println("Test: Message channel closed.")
				messageChan = nil // Avoid busy-looping on closed channel
			} else {
				log.Printf("Test: Received message: %s", msg)
				receivedMessages = append(receivedMessages, msg)
			}
		case err, ok := <-errorChan:
			if !ok {
				log.Println("Test: Error channel closed.")
				errorChan = nil // Avoid busy-looping
			} else {
				log.Printf("Test: Received error: %v", err)
				receivedErrors = append(receivedErrors, err)
			}
		case <-collectingDone.C:
			log.Println("Test: Collection timeout reached.")
			break collectLoop
		}
		if messageChan == nil && errorChan == nil {
			log.Println("Test: Both channels closed, exiting collection loop.")
			break collectLoop
		}
	}

	// Wait for CallGraphQLAgentFunc goroutine to complete
	// This ensures that client.Close() and other cleanup in CallGraphQLAgentFunc have a chance to run.
	log.Println("Test: Waiting for CallGraphQLAgentFunc to finish...")
	wg.Wait()
	log.Println("Test: CallGraphQLAgentFunc finished.")

	// Cancel context to ensure any remaining client operations are signalled to stop
	cancel()
	time.Sleep(100 * time.Millisecond) // Give a moment for cleanup due to cancel

	// Close channels explicitly if they weren't closed by the producer
	// This is more of a cleanup for the test itself.
	// Note: CallGraphQLAgentFunc should ideally ensure its downstream channels (messageChan, errorChan)
	// are not written to after it exits or its context is cancelled.
	// The ResponseProcessor is context-aware, so it should stop sending.
	// If CallGraphQLAgentFunc is well-behaved, these channels would be closed or become non-writable.

	require.Len(t, receivedMessages, 2, "Expected 2 messages from the server")
	if len(receivedMessages) == 2 {
		require.Equal(t, "Hello from mock server 1", receivedMessages[0])
		require.Equal(t, "Hello from mock server 2", receivedMessages[1])
	}

	// Assertions on errors:
	// For a graceful CloseNormalClosure from the server, the client's Listen method
	// should ideally return nil or a specific, non-alarming error.
	// websocket.CloseNormalClosure (1000) is a common one.
	// errors.Is(err, context.Canceled) is also possible if client shuts down first.
	if len(receivedErrors) > 0 {
		isExpectedError := false
		for _, err := range receivedErrors {
			if err == nil { // Should not happen if channel is for errors
				continue
			}
			wsErr := &websocket.CloseError{}
			if errors.As(err, &wsErr) {
				if wsErr.Code == websocket.CloseNormalClosure {
					isExpectedError = true
					log.Printf("Test: Received expected CloseNormalClosure error: %v", err)
					break
				}
			}
			// The client might also return an error indicating it was explicitly closed if its context is cancelled
			// before the listener fully processes the server's close.
			if strings.Contains(err.Error(), "client explicitly closed") || errors.Is(err, context.Canceled) {
				isExpectedError = true
				log.Printf("Test: Received expected client closed/context canceled error: %v", err)
				break
			}
			// EOF can sometimes occur with httptest server during shutdown interactions
			if strings.Contains(err.Error(), "EOF") && strings.Contains(err.Error(),"unexpected EOF") == false {
                 isExpectedError = true
                 log.Printf("Test: Received potentially acceptable EOF error: %v", err)
                 break
            }

		}
		require.True(t, isExpectedError, fmt.Sprintf("Received unexpected errors: %+v", receivedErrors))
	} else {
		log.Println("Test: No errors received on errorChan, which is good.")
	}
}

// Example of how a custom handler could be used for a specific test:
// ... (previous tests and helper code remains the same) ...

// TestGraphQLClient_ServerGracefulCloseAfterIdle verifies client behavior when the server
// sends initial messages, remains idle (client sends pings), and then gracefully closes.
func TestGraphQLClient_ServerGracefulCloseAfterIdle(t *testing.T) {
	clientPingInterval := 500 * time.Millisecond
	serverIdlePeriod := clientPingInterval * 4 // Server stays idle for ~4 client ping intervals

	idleCloseTestHandler := func(w http.ResponseWriter, r *http.Request, conn *websocket.Conn, server *mockGQLServer) {
		// After init/ack handshake
		var subMsg map[string]interface{}
		err := conn.ReadJSON(&subMsg)
		require.NoError(t, err, "mockServer: failed to read subscription message")
		subID, _ := subMsg["id"].(string)
		msgType, _ := subMsg["type"].(string)
		log.Printf("mockServer: received subscription request ID: %s, Type: %s", subID, msgType)

		// 1. Send initial message(s)
		serverMsgType := "next" // graphql-transport-ws
		if msgType == "start" { // graphql-ws
			serverMsgType = "data"
		}
		initialPayload1 := AgentResponse{Data: AgentData{Agent: AgentDetails{
			Messages: []MessageOutput{{Content: "Initial data batch 1", Role: "assistant", Format: "text", TurnID: "ti1"}},
		}}}
		err = conn.WriteJSON(map[string]interface{}{"id": subID, "type": serverMsgType, "payload": initialPayload1})
		require.NoError(t, err, "mockServer: failed to send initial data message 1")
		log.Printf("mockServer: sent initial data message 1 for subID: %s", subID)

		initialPayload2 := AgentResponse{Data: AgentData{Agent: AgentDetails{
			Messages: []MessageOutput{{Content: "Initial data batch 2", Role: "assistant", Format: "text", TurnID: "ti2"}},
		}}}
		err = conn.WriteJSON(map[string]interface{}{"id": subID, "type": serverMsgType, "payload": initialPayload2})
		require.NoError(t, err, "mockServer: failed to send initial data message 2")
		log.Printf("mockServer: sent initial data message 2 for subID: %s", subID)

		// 2. Server remains idle while client sends pings
		log.Printf("mockServer: entering idle period for %s...", serverIdlePeriod)
		time.Sleep(serverIdlePeriod)
		log.Printf("mockServer: idle period finished. Pings received so far: %d", server.receivedPingCount)

		// 3. Graceful server close
		log.Printf("mockServer: sending CloseNormalClosure to client for subID: %s", subID)
		err = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "Closing after idle period"))
		if err != nil {
			log.Printf("mockServer: error sending close message: %v", err)
		}
	}

	mockServer := newMockWebsocketServer(t, idleCloseTestHandler)
	defer mockServer.Close()

	messageChan := make(chan string, 10)
	errorChan := make(chan error, 5)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		CallGraphQLAgentFunc(
			ctx,
			"test-api-key-idle",
			"test-convo-id-idle",
			"test-user-id-idle",
			"test-tenant-id-idle",
			"test-channel-id-idle",
			[]Message{{Content: "Idle test initial message", Format: "text", Role: "user", TurnID: "tp-idle0"}},
			mockServer.httpServer.URL,
			clientPingInterval,
			messageChan,
			errorChan,
		)
		log.Println("TestIdleClose: CallGraphQLAgentFunc goroutine finished.")
	}()

	var receivedMessages []string
	var receivedErrors []error
	collectionTimeout := serverIdlePeriod + 2*time.Second
	collectingDone := time.NewTimer(collectionTimeout)

collectLoopIdle:
	for {
		select {
		case msg, ok := <-messageChan:
			if !ok {
				log.Println("TestIdleClose: Message channel closed.")
				messageChan = nil
			} else {
				log.Printf("TestIdleClose: Received message: %s", msg)
				receivedMessages = append(receivedMessages, msg)
			}
		case err, ok := <-errorChan:
			if !ok {
				log.Println("TestIdleClose: Error channel closed.")
				errorChan = nil
			} else {
				log.Printf("TestIdleClose: Received error: %v", err)
				receivedErrors = append(receivedErrors, err)
			}
		case <-collectingDone.C:
			log.Println("TestIdleClose: Collection timeout reached.")
			break collectLoopIdle
		}
		if messageChan == nil && errorChan == nil {
			log.Println("TestIdleClose: Both channels closed, exiting collection loop.")
			break collectLoopIdle
		}
	}

	log.Println("TestIdleClose: Waiting for CallGraphQLAgentFunc to finish...")
	wg.Wait()
	log.Println("TestIdleClose: CallGraphQLAgentFunc finished.")
	cancel()
	time.Sleep(100 * time.Millisecond)

	require.Len(t, receivedMessages, 2, "Expected 2 initial data messages from the server")
	if len(receivedMessages) == 2 {
		require.Equal(t, "Initial data batch 1", receivedMessages[0])
		require.Equal(t, "Initial data batch 2", receivedMessages[1])
	}

	mockServer.mu.Lock()
	pingCount := mockServer.receivedPingCount
	mockServer.mu.Unlock()
	// Expect at least 2 pings for a 2s idle period with 0.5s interval
	require.GreaterOrEqual(t, pingCount, 2, "Server should have received at least 2 pings from the client during idle period")
	log.Printf("TestIdleClose: Server received %d pings from client.", pingCount)

	if len(receivedErrors) > 0 {
		isExpectedError := false
		for _, err := range receivedErrors {
			if err == nil { continue }
			wsErr := &websocket.CloseError{}
			if errors.As(err, &wsErr) && wsErr.Code == websocket.CloseNormalClosure {
				isExpectedError = true; break
			}
			if strings.Contains(err.Error(), "client explicitly closed") || errors.Is(err, context.Canceled) {
				isExpectedError = true; break
			}
            if strings.Contains(err.Error(), "EOF") && !strings.Contains(err.Error(),"unexpected EOF") {
                 isExpectedError = true; break
            }
		}
		require.True(t, isExpectedError, fmt.Sprintf("Received unexpected errors during idle close test: %+v", receivedErrors))
	} else {
		log.Println("TestIdleClose: No errors received on errorChan.")
	}
}

// Example of how a custom handler could be used for a specific test:
// ... (previous test and helper code remains the same) ...

// TestGraphQLClient_PingPongKeepAliveAndGracefulClose verifies the client's ping mechanism,
// ensures the connection is kept alive, and then handles a graceful server shutdown.
func TestGraphQLClient_PingPongKeepAliveAndGracefulClose(t *testing.T) {
	clientPingInterval := 500 * time.Millisecond // Client's configured ping interval
	serverWaitToObservePings := clientPingInterval * 4 // Wait for ~4 pings

	pingTestHandler := func(w http.ResponseWriter, r *http.Request, conn *websocket.Conn, server *mockGQLServer) {
		// After init/ack handshake by newMockWebsocketServer
		var subMsg map[string]interface{}
		err := conn.ReadJSON(&subMsg)
		require.NoError(t, err, "mockServer: failed to read subscription message")
		subID, _ := subMsg["id"].(string)
		msgType, _ := subMsg["type"].(string)
		log.Printf("mockServer: received subscription request ID: %s, Type: %s", subID, msgType)

		// 1. Send an initial message
		serverMsgType := "next" // graphql-transport-ws
		if msgType == "start" { // graphql-ws
			serverMsgType = "data"
		}
		initialPayload := AgentResponse{Data: AgentData{Agent: AgentDetails{
			Messages: []MessageOutput{{Content: "Initial data after subscribe", Role: "assistant", Format: "text", TurnID: "t-init"}},
		}}}
		err = conn.WriteJSON(map[string]interface{}{"id": subID, "type": serverMsgType, "payload": initialPayload})
		require.NoError(t, err, "mockServer: failed to send initial data message")
		log.Printf("mockServer: sent initial data message for subID: %s", subID)

		// 2. Wait for client to send pings
		log.Printf("mockServer: waiting for %s to observe client pings...", serverWaitToObservePings)
		time.Sleep(serverWaitToObservePings)

		// 3. Send another message to confirm connection is still alive
		finalPayload := AgentResponse{Data: AgentData{Agent: AgentDetails{
			Messages: []MessageOutput{{Content: "Data after pings", Role: "assistant", Format: "text", TurnID: "t-final"}},
		}}}
		err = conn.WriteJSON(map[string]interface{}{"id": subID, "type": serverMsgType, "payload": finalPayload})
		require.NoError(t, err, "mockServer: failed to send final data message")
		log.Printf("mockServer: sent final data message for subID: %s", subID)

		// 4. Graceful server close
		time.Sleep(100 * time.Millisecond) // Brief pause
		log.Printf("mockServer: sending CloseNormalClosure to client for subID: %s", subID)
		err = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "Closing normally after ping test"))
		if err != nil {
			log.Printf("mockServer: error sending close message: %v", err)
		}
	}

	mockServer := newMockWebsocketServer(t, pingTestHandler)
	defer mockServer.Close()

	messageChan := make(chan string, 10)
	errorChan := make(chan error, 5)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		CallGraphQLAgentFunc(
			ctx,
			"test-api-key-ping",
			"test-convo-id-ping",
			"test-user-id-ping",
			"test-tenant-id-ping",
			"test-channel-id-ping",
			[]Message{{Content: "Ping test initial message", Format: "text", Role: "user", TurnID: "tp0"}},
			mockServer.httpServer.URL,
			clientPingInterval, // Short ping interval for the client
			messageChan,
			errorChan,
		)
		log.Println("TestPing: CallGraphQLAgentFunc goroutine finished.")
	}()

	var receivedMessages []string
	var receivedErrors []error
	// Timeout should be longer than serverWaitToObservePings + some buffer
	collectionTimeout := serverWaitToObservePings + 2*time.Second
	collectingDone := time.NewTimer(collectionTimeout)

collectLoopPing:
	for {
		select {
		case msg, ok := <-messageChan:
			if !ok {
				log.Println("TestPing: Message channel closed.")
				messageChan = nil
			} else {
				log.Printf("TestPing: Received message: %s", msg)
				receivedMessages = append(receivedMessages, msg)
			}
		case err, ok := <-errorChan:
			if !ok {
				log.Println("TestPing: Error channel closed.")
				errorChan = nil
			} else {
				log.Printf("TestPing: Received error: %v", err)
				receivedErrors = append(receivedErrors, err)
			}
		case <-collectingDone.C:
			log.Println("TestPing: Collection timeout reached.")
			break collectLoopPing
		}
		if messageChan == nil && errorChan == nil {
			log.Println("TestPing: Both channels closed, exiting collection loop.")
			break collectLoopPing
		}
	}

	log.Println("TestPing: Waiting for CallGraphQLAgentFunc to finish...")
	wg.Wait()
	log.Println("TestPing: CallGraphQLAgentFunc finished.")
	cancel() // Ensure client context is cancelled
	time.Sleep(100 * time.Millisecond)


	require.Len(t, receivedMessages, 2, "Expected 2 data messages from the server")
	if len(receivedMessages) == 2 {
		require.Equal(t, "Initial data after subscribe", receivedMessages[0])
		require.Equal(t, "Data after pings", receivedMessages[1])
	}

	// Crucially, assert that pings were received by the server
	mockServer.mu.Lock()
	pingCount := mockServer.receivedPingCount
	mockServer.mu.Unlock()
	// Expect at least 2 pings ( (2s wait / 0.5s interval) - some buffer for timing)
	// For 2s wait and 0.5s interval, ideally 3-4 pings. Let's be conservative.
	require.GreaterOrEqual(t, pingCount, 2, "Server should have received at least 2 pings from the client")
	log.Printf("TestPing: Server received %d pings from client.", pingCount)


	if len(receivedErrors) > 0 {
		isExpectedError := false
		for _, err := range receivedErrors {
			if err == nil { continue }
			wsErr := &websocket.CloseError{}
			if errors.As(err, &wsErr) && wsErr.Code == websocket.CloseNormalClosure {
				isExpectedError = true; break
			}
			if strings.Contains(err.Error(), "client explicitly closed") || errors.Is(err, context.Canceled) {
				isExpectedError = true; break
			}
            if strings.Contains(err.Error(), "EOF") && !strings.Contains(err.Error(),"unexpected EOF") {
                 isExpectedError = true; break
            }
		}
		require.True(t, isExpectedError, fmt.Sprintf("Received unexpected errors: %+v", receivedErrors))
	} else {
		log.Println("TestPing: No errors received on errorChan.")
	}
}


// Example of how a custom handler could be used for a specific test:
// func myCustomServerBehavior(w http.ResponseWriter, r *http.Request, conn *websocket.Conn, server *mockGQLServer) {
//   // Loop to read messages from client and send specific responses
//   for {
//     var clientMsg map[string]interface{}
//     err := conn.ReadJSON(&clientMsg)
//     if err != nil {
//       // Handle error (client disconnected, etc.)
//       return
//     }
//
//     msgType, _ := clientMsg["type"].(string)
//     if msgType == "subscribe" {
//       // Client wants to subscribe, send some data
//       conn.WriteJSON(map[string]interface{}{"id": clientMsg["id"], "type": "next", "payload": {"data": "some data"}})
//       conn.WriteJSON(map[string]interface{}{"id": clientMsg["id"], "type": "complete"})
//     }
//   }
// }
//
// func TestClientSubscription(t *testing.T) {
//  mockServer := newMockWebsocketServer(t, myCustomServerBehavior)
//  defer mockServer.Close()
//
//  // ... setup and run GraphQLSubscriptionClient ...
// }
