package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// writeWait is the time allowed to write a message to the peer.
	writeWait = 10 * time.Second
)

// AgentRequestInput defines the structure for the 'request' input object in the GraphQL query.
type AgentRequestInput struct {
	ConversationContext ConversationContextInput `json:"conversationContext"`
	SystemContext       []SystemContextInput     `json:"systemContext"`
	UserContext         UserContextInput         `json:"userContext"`
	Messages            []MessageInput           `json:"messages"` // This MessageInput is specific to GraphQL
}

// ConversationContextInput defines the structure for the 'conversationContext' input object.
type ConversationContextInput struct {
	ConversationID string `json:"conversationId"`
}

// SystemContextInput defines the structure for a system context item.
type SystemContextInput struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// UserContextInput defines the structure for the 'userContext' input object.
type UserContextInput struct {
	UserID  string        `json:"userId"`
	Profile []interface{} `json:"profile"`
}

// MessageInput defines the structure for a message in the agent request for GraphQL.
// This is different from main.Message which is the domain model.
type MessageInput struct {
	Content string `json:"content"`
	Format  string `json:"format"`
	Role    string `json:"role"`
	TurnID  string `json:"turnId"` // Added TurnID to MessageInput for consistency if GraphQL expects it
}

// AgentResponseWrapper wraps the 'agent' field in the GraphQL response.
type AgentResponseWrapper struct {
	Agent AgentResponseData `json:"agent"`
}

// AgentResponseData defines the structure for the data returned by the 'agent' query.
type AgentResponseData struct {
	AnonymizationEntities []AnonymizationEntity `json:"anonymizationEntities"`
	Messages              []MessageOutput       `json:"messages"` // This MessageOutput is specific to GraphQL
}

// MessageOutput defines the structure for a message in the agent response from GraphQL.
// This is different from main.Message.
type MessageOutput struct {
	Content string `json:"content"`
	Format  string `json:"format"`
	Role    string `json:"role"`
	TurnID  string `json:"turnId"`
}

// AnonymizationEntity defines the structure for an anonymization entity in the GraphQL response.
type AnonymizationEntity struct {
	Replacement string `json:"replacement"`
	Type        string `json:"type"`
	Value       string `json:"value"`
}

// GQLError defines a structure for GraphQL errors (used in AgentResponse).
// Renamed from Error to GQLError to avoid conflict with built-in error type.
type GQLError struct {
	Message string `json:"message"`
}

// AgentResponse is the top-level structure for the GraphQL subscription data payload.
type AgentResponse struct {
	Data   AgentData  `json:"data"`
	Errors []GQLError `json:"errors,omitempty"`
}

// AgentData wraps the Agent details in the subscription payload.
type AgentData struct {
	Agent AgentDetails `json:"agent"` // Renamed from Agent to AgentDetails to avoid type conflict if Agent is a common name
}

// AgentDetails contains the actual response data like messages and entities from the agent.
// Renamed from Agent to AgentDetails.
type AgentDetails struct {
	AnonymizationEntities []AnonymizationEntity `json:"anonymizationEntities"`
	Messages              []MessageOutput       `json:"messages"` // Using MessageOutput here
}

// GraphQLMessage represents a WebSocket message for GraphQL subscriptions
type GraphQLMessage struct {
	ID      string      `json:"id,omitempty"`
	Type    string      `json:"type"`
	Payload interface{} `json:"payload,omitempty"`
}

// ResponseProcessor handles processing of multiple AgentResponse objects
type ResponseProcessor struct {
	processedResponses int
	totalMessages      int
	mu                 sync.RWMutex
	messageChan        chan<- string
	errorChan          chan<- error
	ctx                context.Context
}

// NewResponseProcessor creates a new response processor
func NewResponseProcessor(ctx context.Context, messageChan chan<- string, errorChan chan<- error) *ResponseProcessor {
	return &ResponseProcessor{
		processedResponses: 0,
		totalMessages:      0,
		messageChan:        messageChan,
		errorChan:          errorChan,
		ctx:                ctx,
	}
}

// ProcessAgentResponse processes a single AgentResponse and sends messages to channels
func (rp *ResponseProcessor) ProcessAgentResponse(response AgentResponse) {
	rp.mu.Lock()
	rp.processedResponses++
	responseNum := rp.processedResponses
	rp.mu.Unlock()

	log.Printf("[ResponseProcessor] Processing AgentResponse #%d", responseNum)

	// Handle GraphQL errors first
	if len(response.Errors) > 0 {
		for _, gqlErr := range response.Errors {
			log.Printf("[ResponseProcessor] GraphQL Error in response #%d: %s", responseNum, gqlErr.Message)
			select {
			case rp.errorChan <- fmt.Errorf("GraphQL error in response #%d: %s", responseNum, gqlErr.Message):
			case <-rp.ctx.Done():
				log.Printf("[ResponseProcessor] Context cancelled while sending error")
				return
			default:
				log.Printf("[ResponseProcessor] Error channel full, dropping error message")
			}
		}
		return // Don't process messages if there are errors
	}

	// Process anonymization entities
	if len(response.Data.Agent.AnonymizationEntities) > 0 {
		log.Printf("[ResponseProcessor] Response #%d contains %d anonymization entities",
			responseNum, len(response.Data.Agent.AnonymizationEntities))
		for i, entity := range response.Data.Agent.AnonymizationEntities {
			log.Printf("[ResponseProcessor] Entity %d - Type: %s, Value: %s, Replacement: %s",
				i+1, entity.Type, entity.Value, entity.Replacement)
		}
	}

	// Process messages
	if len(response.Data.Agent.Messages) > 0 {
		log.Printf("[ResponseProcessor] Response #%d contains %d messages",
			responseNum, len(response.Data.Agent.Messages))

		for _, msg := range response.Data.Agent.Messages {
			rp.mu.Lock()
			rp.totalMessages++
			msgNum := rp.totalMessages
			rp.mu.Unlock()

			log.Printf("[ResponseProcessor] Processing message #%d from response #%d - Role: %s, Format: %s, TurnID: %s",
				msgNum, responseNum, msg.Role, msg.Format, msg.TurnID)

			if msg.Content != "" {
				// Send message content to channel
				select {
				case rp.messageChan <- msg.Content:
					log.Printf("[ResponseProcessor] Successfully sent message #%d to channel: %.100s...",
						msgNum, msg.Content)
				case <-rp.ctx.Done():
					log.Printf("[ResponseProcessor] Context cancelled while sending message #%d", msgNum)
					return
				default:
					log.Printf("[ResponseProcessor] Message channel full, dropping message #%d", msgNum)
				}
			} else {
				log.Printf("[ResponseProcessor] Message #%d has empty content, skipping", msgNum)
			}
		}
	} else {
		log.Printf("[ResponseProcessor] Response #%d contains no messages", responseNum)
	}

	log.Printf("[ResponseProcessor] Completed processing response #%d (total messages processed: %d)",
		responseNum, rp.totalMessages)
}

// GetStats returns processing statistics
func (rp *ResponseProcessor) GetStats() (int, int) {
	rp.mu.RLock()
	defer rp.mu.RUnlock()
	return rp.processedResponses, rp.totalMessages
}

// escapeString handles any special characters in input strings for safety
func escapeString(input string) string {
	return strings.ReplaceAll(strings.ReplaceAll(input, `"`, `\"`), "\n", "\\n")
}

// CallGraphQLAgentFunc connects to a GraphQL subscription endpoint and continuously processes multiple AgentResponse objects.
// Enhanced to better handle multiple responses from the GraphQL server.
var CallGraphQLAgentFunc = func(parentCtx context.Context, apiKey string, conversationID string, userID string, tenantID string, channelIDSystemContext string, messages []Message, apiURL string, pingInterval time.Duration, messageChan chan<- string, errorChan chan<- error) {
	// This context is for this specific agent call + subscription lifecycle.
	// It will be cancelled if parentCtx is cancelled or if cancel() is called explicitly.
	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel() // Ensures that if parentCtx isn't done, this specific operation's context is cleaned up on exit.

	processor := NewResponseProcessor(ctx, messageChan, errorChan)

	client := NewGraphQLSubscriptionClient(apiURL, pingInterval)
	// defer client.Close() is crucial. It's called when CallGraphQLAgentFunc exits.
	// This happens if:
	// 1. parentCtx is cancelled (and thus ctx is cancelled).
	// 2. Connect or Subscribe fails.
	// 3. client.Listen(ctx) returns (due to server close, fatal error, or its ctx being done).
	defer func() {
		log.Printf("[CallGraphQLAgentFunc] Cleaning up client connection...")
		if err := client.Close(); err != nil {
			log.Printf("[CallGraphQLAgentFunc] Error closing GraphQL client: %v", err)
		}
	}()

	if err := client.Connect(ctx); err != nil { // Use the cancellable ctx for connect
		log.Printf("[CallGraphQLAgentFunc] Connection failed: %v", err)
		select {
		case errorChan <- fmt.Errorf("graphql connection failed: %w", err):
		case <-parentCtx.Done(): // Check parentCtx if the errorChan send blocks
			log.Printf("[CallGraphQLAgentFunc] Parent context done while sending connection error.")
		}
		return
	}

	log.Println("[CallGraphQLAgentFunc] Connected to GraphQL. Setting up subscription...")
	// ... (your query setup code remains the same) ...
	var messageBlockBuffer bytes.Buffer
	messageBlockBuffer.WriteString("[")
	for i, msg := range messages {
		messageInput := MessageInput{
			Content: msg.Content,
			Format:  msg.Format,
			Role:    msg.Role,
			TurnID:  msg.TurnID,
		}
		escapedContent := escapeString(messageInput.Content) // Ensure escapeString is defined
		messageBlockBuffer.WriteString(fmt.Sprintf(`{
			content: "%s",
			format: "%s",
			role: "%s",
			turnId: "%s"
		}`, escapedContent, escapeString(messageInput.Format), escapeString(messageInput.Role), escapeString(messageInput.TurnID)))
		if i < len(messages)-1 {
			messageBlockBuffer.WriteString(",")
		}
	}
	messageBlockBuffer.WriteString("]")
	messageBlock := messageBlockBuffer.String()

	query := fmt.Sprintf(`subscription {
		agent(request: {
			conversationContext: {
				conversationId: "%s"
			},
			systemContext: [
				{ key: "channelId", value: "%s" },
				{ key: "tenantId", value: "%s" }
			],
			userContext: {
				userId: "%s",
				profile: []
			},
			messages: %s
		}) {
			anonymizationEntities { replacement, type, value },
			messages { content, format, role, turnId }
		}
	}`, escapeString(conversationID), escapeString(channelIDSystemContext), escapeString(tenantID), escapeString(userID), messageBlock)

	log.Printf("[CallGraphQLAgentFunc] GraphQL Query: %s", query)

	subscribeErr := client.Subscribe("agent-subscription", query, nil, func(payload interface{}) {
		log.Printf("[GraphQLCallback] Received new payload from GraphQL server")
		payloadBytes, marshalErr := json.Marshal(payload)
		if marshalErr != nil {
			log.Printf("[GraphQLCallback] Error marshalling payload: %v", marshalErr)
			select {
			case errorChan <- fmt.Errorf("payload marshal error: %w", marshalErr):
			case <-ctx.Done(): // Use the operation's context
			}
			return
		}
		var response AgentResponse
		if unmarshalErr := json.Unmarshal(payloadBytes, &response); unmarshalErr != nil {
			log.Printf("[GraphQLCallback] Error unmarshalling payload: %v", unmarshalErr)
			select {
			case errorChan <- fmt.Errorf("payload unmarshal error: %w", unmarshalErr):
			case <-ctx.Done():
			}
			return
		}
		processor.ProcessAgentResponse(response)
		processedCount, totalMsgCount := processor.GetStats()
		log.Printf("[GraphQLCallback] Processing stats: %d responses processed, %d total messages sent", processedCount, totalMsgCount)
	})

	if subscribeErr != nil {
		log.Printf("[CallGraphQLAgentFunc] Subscription failed: %v", subscribeErr)
		select {
		case errorChan <- fmt.Errorf("graphql subscription failed: %w", subscribeErr):
		case <-parentCtx.Done():
		}
		return // This will trigger defer client.Close()
	}

	log.Println("[CallGraphQLAgentFunc] Subscription started, now listening for AgentResponse objects until server closes or an error occurs...")
	listenErr := client.Listen(ctx) // Pass the cancellable ctx

	// After Listen returns:
	if listenErr != nil {
		if errors.Is(listenErr, context.Canceled) || errors.Is(listenErr, context.DeadlineExceeded) {
			// This means ctx (derived from parentCtx or cancelled by CallGraphQLAgentFunc's own logic) was done.
			log.Printf("[CallGraphQLAgentFunc] Listener stopped due to context cancellation/deadline: %v", listenErr)
		} else if strings.Contains(listenErr.Error(), "client explicitly closed") || strings.Contains(listenErr.Error(), "client connection field is nil") {
			log.Printf("[CallGraphQLAgentFunc] Listener stopped as client connection was intentionally closed: %v", listenErr)
		} else {
			// Other errors (network, unexpected server close, etc.)
			log.Printf("[CallGraphQLAgentFunc] Listener returned an error: %v", listenErr)
			select {
			case errorChan <- fmt.Errorf("graphql listener error: %w", listenErr):
			case <-parentCtx.Done():
				log.Printf("[CallGraphQLAgentFunc] Parent context done while sending listener error.")
			default: // Non-blocking send
				log.Printf("[CallGraphQLAgentFunc] Error channel full or unavailable when sending listener error: %v", listenErr)
			}
		}
	} else {
		// listenErr is nil - implies server closed connection gracefully (e.g., CloseNormalClosure).
		log.Println("[CallGraphQLAgentFunc] GraphQL subscription ended gracefully by server.")
	}

	processedCount, totalMsgCount := processor.GetStats()
	log.Printf("[CallGraphQLAgentFunc] GraphQL processing finished for this session. Final stats: %d responses processed, %d total messages sent",
		processedCount, totalMsgCount)
	// defer client.Close() will run.
}

// handleAgentSubscriptionResponseMultiple processes the response from the GraphQL subscription.
// It returns all MessageOutput messages instead of just the first one.
// Note: This function is kept for backward compatibility but the new ResponseProcessor is preferred.
func handleAgentSubscriptionResponseMultiple(response AgentResponse) []*MessageOutput {
	var results []*MessageOutput

	if len(response.Errors) > 0 {
		for _, err := range response.Errors {
			log.Printf("GraphQL Error: %s\n", err.Message)
		}
		return results // Return empty slice if there are GraphQL errors
	}

	log.Println("=== Agent Response ===")

	if len(response.Data.Agent.AnonymizationEntities) > 0 {
		log.Println("Anonymization Entities:")
		for _, entity := range response.Data.Agent.AnonymizationEntities {
			log.Printf("  Type: %s, Value: %s, Replacement: %s\n",
				entity.Type, entity.Value, entity.Replacement)
		}
	}

	if len(response.Data.Agent.Messages) > 0 {
		log.Printf("Processing %d messages:", len(response.Data.Agent.Messages))
		for i, msg := range response.Data.Agent.Messages {
			log.Printf("  Message %d - Role: %s, Format: %s, TurnID: %s\n",
				i+1, msg.Role, msg.Format, msg.TurnID)
			log.Printf("  Content: %s\n", msg.Content)
			log.Println("  ---")

			// Add each message to results
			msgCopy := msg
			results = append(results, &msgCopy)
		}
	}

	log.Println("======================")
	return results
}

// GraphQLSubscriptionClient manages a GraphQL subscription over WebSocket.
type GraphQLSubscriptionClient struct {
	conn           *websocket.Conn
	url            string
	callbacks      map[string]func(interface{})
	protocol       string
	internalCtx    context.Context
	internalCancel context.CancelFunc
	listenDone     chan struct{}
	mu             sync.RWMutex // Added mutex for thread safety
	pingInterval   time.Duration
}

// NewGraphQLSubscriptionClient creates a new client for GraphQL subscriptions.
func NewGraphQLSubscriptionClient(wsURL string, pingInterval time.Duration) *GraphQLSubscriptionClient {
	iCtx, iCancel := context.WithCancel(context.Background())
	return &GraphQLSubscriptionClient{
		url:            wsURL,
		callbacks:      make(map[string]func(interface{})),
		internalCtx:    iCtx,
		internalCancel: iCancel,
		listenDone:     make(chan struct{}),
		pingInterval:   pingInterval,
	}
}

// Connect establishes a WebSocket connection and initializes the GraphQL protocol.
func (c *GraphQLSubscriptionClient) Connect(ctx context.Context) error {
	u, err := url.Parse(c.url)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	dialer := websocket.Dialer{
		Subprotocols: []string{"graphql-transport-ws", "graphql-ws"},
	}

	conn, resp, err := dialer.DialContext(ctx, u.String(), nil)
	if err != nil {
		return fmt.Errorf("dial error: %w", err)
	}

	c.mu.Lock()
	c.conn = conn
	c.mu.Unlock()

	if resp != nil && len(resp.Header.Get("Sec-WebSocket-Protocol")) > 0 {
		c.protocol = resp.Header.Get("Sec-WebSocket-Protocol")
	} else {
		c.protocol = "graphql-ws"
	}
	log.Printf("[GraphQLClient] Using protocol: %s\n", c.protocol)

	initMsgType := "connection_init"
	initPayload := make(map[string]interface{})

	if err := c.sendMessage(GraphQLMessage{Type: initMsgType, Payload: initPayload}); err != nil {
		c.conn.Close()
		return fmt.Errorf("init error: %w", err)
	}

	// Wait for acknowledgment
	var ackMsg GraphQLMessage
	c.conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	if err := c.conn.ReadJSON(&ackMsg); err != nil {
		c.conn.Close()
		return fmt.Errorf("ack read error: %w", err)
	}
	c.conn.SetReadDeadline(time.Time{})

	expectedAckType := "connection_ack"

	if ackMsg.Type != expectedAckType {
		c.conn.Close()
		return fmt.Errorf("expected ack type '%s', got: '%s'", expectedAckType, ackMsg.Type)
	}

	log.Printf("[GraphQLClient] Connection established and acknowledged")

	// Set custom pong handler
	c.conn.SetPongHandler(func(appData string) error {
		log.Printf("[GraphQLClient] Received pong from server. AppData: %s", appData)
		// The gorilla/websocket library automatically updates the read deadline on pong.
		// No need to manually call c.conn.SetReadDeadline here unless custom logic is required.
		return nil
	})

	return nil
}

// Close signals the listener to stop and closes the WebSocket connection.
func (c *GraphQLSubscriptionClient) Close() error {
	log.Printf("[GraphQLClient] Initiating close sequence")
	c.internalCancel()

	c.mu.Lock()
	conn := c.conn
	c.conn = nil
	c.mu.Unlock()

	if conn != nil {
		log.Printf("[GraphQLClient] Closing WebSocket connection")
		err := conn.Close()
		return err
	}
	log.Printf("[GraphQLClient] Connection was already closed")
	return nil
}

func (c *GraphQLSubscriptionClient) sendMessage(msg GraphQLMessage) error {
	c.mu.RLock()
	conn := c.conn
	c.mu.RUnlock()

	if conn == nil {
		return fmt.Errorf("cannot send message: connection is not established or already closed")
	}

	log.Printf("[GraphQLClient] Sending WebSocket message: Type=%s, ID=%s\n", msg.Type, msg.ID)
	conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	err := conn.WriteJSON(msg)
	conn.SetWriteDeadline(time.Time{})
	return err
}

// Subscribe sends a subscription request over the WebSocket.
func (c *GraphQLSubscriptionClient) Subscribe(subscriptionID, query string, variables map[string]interface{}, callback func(interface{})) error {
	c.mu.Lock()
	c.callbacks[subscriptionID] = callback
	c.mu.Unlock()

	var msgType string
	if c.protocol == "graphql-transport-ws" {
		msgType = "subscribe"
	} else {
		msgType = "start"
	}

	payload := map[string]interface{}{
		"query": query,
	}
	if variables != nil {
		payload["variables"] = variables
	}

	msg := GraphQLMessage{
		ID:      subscriptionID,
		Type:    msgType,
		Payload: payload,
	}

	log.Printf("[GraphQLClient] Subscribing with ID: %s", subscriptionID)
	return c.sendMessage(msg)
}

// Listen starts listening for messages from the WebSocket connection.
func (c *GraphQLSubscriptionClient) Listen(ctx context.Context) error {
	defer close(c.listenDone) // Signal that Listen has finished its execution.
	log.Printf("[GraphQLClient] Starting to listen for messages. Will continue until context is done, client is closed, or a fatal websocket error occurs.")

	ticker := time.NewTicker(c.pingInterval)
	defer ticker.Stop()

	// readWait should be longer than pingInterval.
	// It's the maximum time to wait for a message (including pongs which are handled by the library)
	// before the connection is considered stale.
	// Adding a buffer (e.g., 45 seconds) to the ping interval.
	localReadWait := c.pingInterval + 45*time.Second

	messageCount := 0

	for {
		var msg GraphQLMessage
		var err error

		// Prioritize checking for stop signals (from parent context or internal client.Close())
		select {
		case <-ctx.Done():
			log.Printf("[GraphQLClient] Main context done (from CallGraphQLAgentFunc), stopping listener (processed %d messages). Error: %v", messageCount, ctx.Err())
			return ctx.Err()
		case <-c.internalCtx.Done(): // c.internalCtx is cancelled by c.Close()
			log.Printf("[GraphQLClient] Internal context done (client.Close() called), stopping listener (processed %d messages). Error: %v", messageCount, c.internalCtx.Err())
			return fmt.Errorf("client explicitly closed: %w", c.internalCtx.Err())
		case <-ticker.C:
			c.mu.RLock()
			conn := c.conn
			c.mu.RUnlock()
			if conn == nil {
				log.Printf("[GraphQLClient] Ping: Connection is nil, skipping ping.")
				// If conn is nil, the main read loop will likely handle shutdown soon.
				continue
			}
			if err := conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(writeWait)); err != nil {
				log.Printf("[GraphQLClient] Error sending ping: %v", err)
				// Depending on policy, we might not want to return error immediately.
				// The read loop will likely detect a dead connection.
			} else {
				log.Printf("[GraphQLClient] Ping sent successfully.")
			}
		default:
			// No immediate stop signal, proceed to attempt a read.
		}

		c.mu.RLock()
		currentConn := c.conn // Use 'currentConn' for clarity within this iteration
		c.mu.RUnlock()

		if currentConn == nil {
			log.Printf("[GraphQLClient] Connection is nil (already closed by client instance). Stopping listener (processed %d messages).", messageCount)
			// This typically means c.Close() was called and completed.
			return errors.New("websocket connection unavailable (client connection field is nil)")
		}

		// Set a read deadline. This is important to prevent ReadJSON from blocking indefinitely,
		// allowing the loop to regularly check the contexts (ctx.Done, c.internalCtx.Done).
		// If the server sends keep-alives, they will be read. If not, this timeout ensures liveness checks.
		// The localReadWait duration should be longer than the ping interval.
		deadline := time.Now().Add(localReadWait)
		if err := currentConn.SetReadDeadline(deadline); err != nil {
			log.Printf("[GraphQLClient] Error setting read deadline: %v. Stopping listener.", err)
			return fmt.Errorf("error setting read deadline: %w", err)
		}

		err = currentConn.ReadJSON(&msg)
		// It's good practice to clear the deadline after the read operation.
		// Ignoring error on clearing deadline as the primary error 'err' from ReadJSON is more important.
		_ = currentConn.SetReadDeadline(time.Time{})

		if err != nil {
			// 1. Handle timeout: Check contexts, then continue if no shutdown signal.
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				log.Printf("[GraphQLClient] Read timeout (waited up to %s). Checking for shutdown signals...", localReadWait.String())
				// Explicitly check contexts again, as the select at the loop start might not have run if ReadJSON blocked for its full duration.
				select {
				case <-ctx.Done():
					log.Printf("[GraphQLClient] Read timeout, but main context is done. Stopping. Error: %v", ctx.Err())
					return ctx.Err()
				case <-c.internalCtx.Done():
					log.Printf("[GraphQLClient] Read timeout, but internal context (client.Close) is done. Stopping. Error: %v", c.internalCtx.Err())
					return fmt.Errorf("client explicitly closed during timeout: %w", c.internalCtx.Err())
				default:
					// Genuine timeout on read. Server might be legitimately quiet.
					// Connection is not necessarily broken. Continue listening.
					log.Printf("[GraphQLClient] Read timed out, but no shutdown signal. Will attempt to read again.")
					continue // Continue to the next iteration of the for loop.
				}
			}

			// 2. Handle WebSocket close errors (server or client initiated proper WebSocket closure)
			if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
				log.Printf("[GraphQLClient] WebSocket closed normally by peer (CloseNormalClosure): %v (processed %d messages)", err, messageCount)
				return nil // Indicates graceful server-side closure. CallGraphQLAgentFunc will then clean up.
			}
			// Check for any other type of WebSocket close error.
			var wsCloseErr *websocket.CloseError
			if errors.As(err, &wsCloseErr) {
				log.Printf("[GraphQLClient] WebSocket closed with specific code %d: %v (processed %d messages)", wsCloseErr.Code, err, messageCount)
				return fmt.Errorf("websocket closed with code %d: %w", wsCloseErr.Code, err) // Indicate an error closure.
			}
			// IsUnexpectedCloseError for abrupt network issues where no proper WS close frame was received.
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure, websocket.CloseNoStatusReceived) {
				log.Printf("[GraphQLClient] WebSocket unexpected close error (e.g. network issue, server crash): %v (processed %d messages)", err, messageCount)
				return fmt.Errorf("websocket unexpected close: %w", err)
			}

			// 3. Re-check c.conn field state, as c.Close() could have been called concurrently
			// and nulled c.conn while this read operation was failing for a different reason.
			c.mu.RLock()
			connStillAssignedInClient := c.conn != nil
			c.mu.RUnlock()
			if !connStillAssignedInClient {
				log.Printf("[GraphQLClient] Connection field became nil during read error processing. Client likely closed. (processed %d messages). Original error: %v", messageCount, err)
				return errors.New("graphql client connection field is nil (client closed during read error)")
			}

			// 4. Any other error is likely a persistent issue with the connection.
			log.Printf("[GraphQLClient] Unhandled persistent read error: %v (processed %d messages)", err, messageCount)
			return fmt.Errorf("persistent websocket read error: %w", err)
		}

		// If err is nil, process the message
		messageCount++
		log.Printf("[GraphQLClient] Received message #%d: Type=%s, ID=%s", messageCount, msg.Type, msg.ID)

		switch msg.Type {
		case "next", "data":
			c.mu.RLock()
			callback, exists := c.callbacks[msg.ID]
			c.mu.RUnlock()
			if exists && callback != nil {
				log.Printf("[GraphQLClient] Calling callback for subscription ID: %s (message #%d)", msg.ID, messageCount)
				callback(msg.Payload)
			} else {
				log.Printf("[GraphQLClient] No callback found for subscription ID: %s", msg.ID)
			}
		case "error": // GraphQL specific error over WebSocket, not a transport error
			log.Printf("[GraphQLClient] GraphQL subscription error message received: ID=%s, Payload=%+v", msg.ID, msg.Payload)
			c.mu.RLock()
			callback, exists := c.callbacks[msg.ID]
			c.mu.RUnlock()
			if exists && callback != nil {
				callback(map[string]interface{}{ // Adapt to how AgentResponse handles errors
					"errors": []map[string]interface{}{
						{"message": fmt.Sprintf("GraphQL subscription error via WebSocket: ID=%s, Payload=%+v", msg.ID, msg.Payload)},
					},
				})
			}
		case "complete":
			log.Printf("[GraphQLClient] GraphQL subscription completed for ID: %s", msg.ID)
			c.mu.Lock()
			delete(c.callbacks, msg.ID) // This specific subscription is done.
			c.mu.Unlock()
			// The connection itself stays open for other active/new subscriptions.
		case "connection_keep_alive", "ka", "ping": // Common keep-alive types
			log.Printf("[GraphQLClient] Received keep-alive/ping message type: %s", msg.Type)
			if msg.Type == "ping" && c.protocol == "graphql-transport-ws" { // graphql-transport-ws expects pong
				if err := c.sendMessage(GraphQLMessage{Type: "pong", Payload: msg.Payload}); err != nil {
					log.Printf("[GraphQLClient] Error sending pong: %v", err)
					// This might be a critical error for the connection if pongs are mandatory.
					// However, an error sending pong might mean the connection is already bad.
				} else {
					log.Printf("[GraphQLClient] Sent pong in response to ping.")
				}
			}
		case "pong": // Received a pong, perhaps in response to a client-sent ping (not implemented here)
			log.Printf("[GraphQLClient] Received pong message type: %s", msg.Type)
		default:
			log.Printf("[GraphQLClient] Unhandled message type: %s. Payload: %+v", msg.Type, msg.Payload)
		}
	}
}

//func main() {

//var GraphQLAgentAPIURL = "ws://localhost:8080/subscriptions"
//	messageChan := make(chan string, 500) // Buffer size as appropriate
//	errorChan := make(chan error, 50)     // Buffer size as appropriate
//
//	// Create a context that can be cancelled on application shutdown (e.g., by OS signal)
//	appCtx, appShutdownSignal := context.WithCancel(context.Background())
//	// Ensure appShutdownSignal() is called to clean up all long-running goroutines when main exits.
//	// One way is to defer it, another is to call it explicitly before main returns.
//	// defer appShutdownSignal() // Call this if main might exit for reasons other than signals
//
//	log.Println("[Main] Starting GraphQL agent function in a goroutine...")
//
//	// Use a WaitGroup if you want main to wait for CallGraphQLAgentFunc to finish before exiting completely
//	var wg sync.WaitGroup
//	wg.Add(1)
//	go func() {
//		defer wg.Done()
//		// Pass appCtx to CallGraphQLAgentFunc for its operational lifetime
//		CallGraphQLAgentFunc(
//			appCtx,
//			"your-api-key",
//			"conversation-123",
//			"user-456",
//			"tenant-789",
//			"channel-001",
//			[]Message{ // Sample messages
//				{Content: "Hello, how can I help you?", Format: "text", Role: "user", TurnID: "turn-1"},
//				{Content: "Please provide information about your services.", Format: "text", Role: "user", TurnID: "turn-2"},
//			},
//			GraphQLAgentAPIURL,
//			messageChan,
//			errorChan,
//		)
//		log.Println("[Main] CallGraphQLAgentFunc goroutine has finished.")
//		// Since CallGraphQLAgentFunc might be the producer for messageChan and errorChan indirectly
//		// via ResponseProcessor, and ResponseProcessor stops on context cancellation,
//		// these channels will stop receiving new items. Consider if they need explicit closing.
//		// For this example, we'll rely on them draining or GC.
//	}()
//
//	log.Println("[Main] Indefinitely processing messages and errors. Press Ctrl+C to shutdown.")
//	messageCount := 0
//	errorCount := 0
//	keepRunning := true
//
//	// Setup signal handling for graceful shutdown
//	osSignalChan := make(chan os.Signal, 1)
//	signal.Notify(osSignalChan, syscall.SIGINT, syscall.SIGTERM)
//
//	for keepRunning {
//		select {
//		case msg, ok := <-messageChan:
//			if !ok {
//				log.Printf("[Main] Message channel closed. Total messages received: %d", messageCount)
//				// If messageChan closes, it might mean the producer (CallGraphQLAgentFunc) has exited.
//				// You might want to trigger a shutdown or just stop this part of the loop.
//				messageChan = nil // Disable this case
//				if errorChan == nil {
//					keepRunning = false
//				} // Exit if both channels are done
//				break
//			}
//			messageCount++
//			log.Printf("[Main] Received message #%d: %.100s...", messageCount, msg)
//			// Process the message here
//
//		case err, ok := <-errorChan:
//			if !ok {
//				log.Printf("[Main] Error channel closed. Total errors received: %d", errorCount)
//				errorChan = nil // Disable this case
//				if messageChan == nil {
//					keepRunning = false
//				} // Exit if both channels are done
//				break
//			}
//			errorCount++
//			log.Printf("[Main] Received error #%d: %v", errorCount, err)
//			// Handle the error. Depending on the error, you might decide to shut down the application.
//			// For example, if it's a critical, unrecoverable error from CallGraphQLAgentFunc.
//			// if isFatal(err) { appShutdownSignal() }
//
//		case s := <-osSignalChan:
//			log.Printf("[Main] Received OS signal %v. Initiating graceful shutdown...", s)
//			appShutdownSignal() // Signal all components using appCtx to stop
//			// keepRunning will eventually become false as appCtx.Done() is processed by CallGraphQLAgentFunc
//			// and channels might close. Or set keepRunning = false directly.
//			// For a more deterministic shutdown, you can have a timeout here.
//
//		case <-appCtx.Done(): // If appCtx is cancelled (e.g., by the signal handler above)
//			log.Println("[Main] Application context is done. Exiting message processing loop.")
//			keepRunning = false // This will break the loop.
//		}
//	}
//
//	log.Println("[Main] Waiting for CallGraphQLAgentFunc to complete shutdown...")
//	wg.Wait() // Wait for the agent goroutine to finish its cleanup
//
//	log.Printf("[Main] Finished all processing. Final stats - Messages: %d, Errors: %d", messageCount, errorCount)
//	log.Println("[Main] Application exiting.")
//}
