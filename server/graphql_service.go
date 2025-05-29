package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors" // Added for errors.New
	"fmt"
	"log"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	// Message struct is now in model.go, ensure it's imported if needed, or this service uses its own types.
	// For CallGraphQLAgentFunc, it takes []main.Message, so that type must be known.
	// It's defined in model.go in the same package, so it should be fine.
)

// GraphQLAgentAPIURL is the URL for the GraphQL agent API.
// Changed to var for testability (allowing modification for network error tests).
var GraphQLAgentAPIURL = "https://ia-platform-contract-agent.dev.apps.oneai.yo-digital.com/graphiql?path=/graphql"

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

// escapeString handles any special characters in input strings for safety
func escapeString(input string) string {
	return strings.ReplaceAll(strings.ReplaceAll(input, `"`, `\"`), "\n", "\\n")
}

// CallGraphQLAgentFunc connects to a GraphQL subscription endpoint and returns the first message content.
// It takes []main.Message (from model.go) as input.
var CallGraphQLAgentFunc = func(apiKey string, conversationID string, userID string, tenantID string, channelIDSystemContext string, messages []Message, apiURL string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := NewGraphQLSubscriptionClient(apiURL)
	defer client.Close()

	if err := client.Connect(ctx); err != nil {
		// Log or return a more specific error instead of log.Fatal
		return "", fmt.Errorf("connection failed: %w", err)
	}

	fmt.Println("Connected to GraphQL subscription endpoint")

	contentChan := make(chan string, 1)
	errChan := make(chan error, 1) // Channel for errors from callback

	var messageBlockBuffer bytes.Buffer
	messageBlockBuffer.WriteString("[")
	for i, msg := range messages {
		// Here, msg is of type main.Message (from model.go)
		// It should have Role, Content, Format, TurnID
		messageInput := MessageInput{
			Content: msg.Content,
			Format:  msg.Format, // Ensure main.Message has Format
			Role:    msg.Role,
			TurnID:  msg.TurnID, // Ensure main.Message has TurnID
		}
		escapedContent := escapeString(messageInput.Content)
		// Format and Role are typically enums or fixed strings, so direct escaping might not be needed
		// unless they can contain special JSON characters. TurnID should also be safe.
		// For simplicity, assuming Format and Role are safe.
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

	fmt.Println(query) // Logging the query can be verbose, consider reducing for production

	err := client.Subscribe("agent-subscription", query, nil, func(payload interface{}) {
		payloadBytes, marshalErr := json.Marshal(payload)
		if marshalErr != nil {
			log.Printf("Error marshalling payload in callback: %v", marshalErr)
			errChan <- fmt.Errorf("payload marshal error: %w", marshalErr)
			return
		}

		var response AgentResponse // Uses the new AgentResponse struct for parsing
		if unmarshalErr := json.Unmarshal(payloadBytes, &response); unmarshalErr != nil {
			log.Printf("Error unmarshalling payload in callback: %v", unmarshalErr)
			errChan <- fmt.Errorf("payload unmarshal error: %w", unmarshalErr)
			return
		}

		// Handle response (including errors from GraphQL itself)
		processedMessage := handleAgentSubscriptionResponse(response) // Renamed for clarity
		if processedMessage != nil && processedMessage.Content != "" {
			select {
			case contentChan <- processedMessage.Content:
			default: // Avoid blocking if channel is full (should not happen with buffer 1)
			}
		} else if len(response.Errors) > 0 {
			// If there's no content but there are GraphQL errors, signal this
			errChan <- fmt.Errorf("GraphQL error: %s", response.Errors[0].Message)
		}
		// If no content and no errors, it might be an empty message list, handled by timeout later
	})

	if err != nil {
		// log.Fatal("Subscription failed:", err) // Avoid fatal
		return "", fmt.Errorf("subscription failed: %w", err)
	}

	fmt.Println("Subscription started, listening for messages...")

	go func() {
		// Listen can return context.DeadlineExceeded if the parent context times out,
		// or another error if the connection drops.
		if listenErr := client.Listen(ctx); listenErr != nil && listenErr != context.Canceled && listenErr != context.DeadlineExceeded {
			log.Printf("Listen error: %v", listenErr)
			// Propagate listen error only if it's not a context cancellation/timeout
			// as those are handled by the select statement.
			// Ensure errChan can receive this or handle appropriately.
			select {
			case errChan <- fmt.Errorf("listener error: %w", listenErr):
			default: // Avoid blocking
			}
		}
	}()

	select {
	case content := <-contentChan:
		return content, nil
	case err := <-errChan: // Check for errors from callback or listener
		return "", err
	case <-ctx.Done():
		return "", fmt.Errorf("timeout waiting for first message: %w", ctx.Err())
	}
}

// handleAgentSubscriptionResponse processes the response from the GraphQL subscription.
// It returns the first MessageOutput if available, or nil.
func handleAgentSubscriptionResponse(response AgentResponse) *MessageOutput { // Returns *MessageOutput
	if len(response.Errors) > 0 {
		for _, err := range response.Errors {
			log.Printf("GraphQL Error: %s\n", err.Message) // Log all errors
		}
		// Depending on requirements, you might return an error here or just log
		return nil // No message content if there are GraphQL errors
	}

	fmt.Println("=== Agent Response ===") // Debug logging

	if len(response.Data.Agent.AnonymizationEntities) > 0 {
		fmt.Println("Anonymization Entities:")
		for _, entity := range response.Data.Agent.AnonymizationEntities {
			fmt.Printf("  Type: %s, Value: %s, Replacement: %s\n",
				entity.Type, entity.Value, entity.Replacement)
		}
	}

	if len(response.Data.Agent.Messages) > 0 {
		firstMsg := &response.Data.Agent.Messages[0] // This is of type MessageOutput
		fmt.Println("Messages:")
		for _, msg := range response.Data.Agent.Messages {
			fmt.Printf("  Role: %s, Format: %s, TurnID: %s\n",
				msg.Role, msg.Format, msg.TurnID)
			fmt.Printf("  Content: %s\n", msg.Content)
			fmt.Println("  ---")
		}
		return firstMsg // Return the first message of type MessageOutput
	}

	fmt.Println("======================") // Debug logging
	return nil                            // No messages in the response
}

// GraphQLSubscriptionClient manages a GraphQL subscription over WebSocket.
type GraphQLSubscriptionClient struct {
	conn           *websocket.Conn
	url            string
	callbacks      map[string]func(interface{})
	protocol       string
	internalCtx    context.Context
	internalCancel context.CancelFunc
	listenDone     chan struct{} // Add this channel
}

// NewGraphQLSubscriptionClient creates a new client for GraphQL subscriptions.
func NewGraphQLSubscriptionClient(wsURL string) *GraphQLSubscriptionClient {
	iCtx, iCancel := context.WithCancel(context.Background())
	return &GraphQLSubscriptionClient{
		url:            wsURL,
		callbacks:      make(map[string]func(interface{})),
		internalCtx:    iCtx,
		internalCancel: iCancel,
		listenDone:     make(chan struct{}), // Initialize here
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
	c.conn = conn

	if resp != nil && len(resp.Header.Get("Sec-WebSocket-Protocol")) > 0 {
		c.protocol = resp.Header.Get("Sec-WebSocket-Protocol")
	} else {
		c.protocol = "graphql-ws" // Default fallback
	}
	fmt.Printf("Using protocol: %s\n", c.protocol)

	initMsgType := "connection_init"
	// Payload for graphql-transport-ws can be just {}, for graphql-ws it's also often {} or not needed.
	initPayload := make(map[string]interface{})

	if err := c.sendMessage(GraphQLMessage{Type: initMsgType, Payload: initPayload}); err != nil {
		c.conn.Close() // Close connection on error
		return fmt.Errorf("init error: %w", err)
	}

	// Wait for acknowledgment
	var ackMsg GraphQLMessage
	// Set a read deadline for the ack
	c.conn.SetReadDeadline(time.Now().Add(10 * time.Second)) // Timeout for ack
	if err := c.conn.ReadJSON(&ackMsg); err != nil {
		c.conn.Close()
		return fmt.Errorf("ack read error: %w", err)
	}
	c.conn.SetReadDeadline(time.Time{}) // Clear read deadline

	expectedAckType := "connection_ack"
	// graphql-transport-ws uses 'connection_ack', graphql-ws also uses 'connection_ack' or 'GQL_CONNECTION_ACK'
	// The library or server might normalize this; check specific server docs if issues arise.

	if ackMsg.Type != expectedAckType {
		c.conn.Close()
		return fmt.Errorf("expected ack type '%s', got: '%s'", expectedAckType, ackMsg.Type)
	}

	return nil
}

// Close signals the listener to stop, waits for it, then closes the WebSocket connection.
func (c *GraphQLSubscriptionClient) Close() error {
	c.internalCancel() // Signal Listen to stop

	// Wait for Listen goroutine to finish
	if c.listenDone != nil { 
		select {
		case <-c.listenDone:
			// Listen goroutine confirmed shutdown
		case <-time.After(5 * time.Second): // Safety timeout
			log.Println("Timeout waiting for Listen goroutine to stop during Close")
		}
	}

	if c.conn != nil {
		err := c.conn.Close()
		c.conn = nil // Ensure conn is nil after closing
		return err
	}
	return nil
}

func (c *GraphQLSubscriptionClient) sendMessage(msg GraphQLMessage) error {
	if c.conn == nil {
		return fmt.Errorf("cannot send message: connection is not established or already closed")
	}
	fmt.Printf("Sending WebSocket message: Type=%s\n", msg.Type) // Debug log
	// Set a write deadline
	c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	err := c.conn.WriteJSON(msg)
	c.conn.SetWriteDeadline(time.Time{}) // Clear deadline
	return err
}

// Subscribe sends a subscription request over the WebSocket.
func (c *GraphQLSubscriptionClient) Subscribe(subscriptionID, query string, variables map[string]interface{}, callback func(interface{})) error {
	c.callbacks[subscriptionID] = callback

	var msgType string
	if c.protocol == "graphql-transport-ws" {
		msgType = "subscribe"
	} else { // graphql-ws
		msgType = "start" // or "GQL_START"
	}

	msg := GraphQLMessage{
		ID:   subscriptionID,
		Type: msgType,
		Payload: SubscriptionPayload{
			Query:     query,
			Variables: variables,
		},
	}
	return c.sendMessage(msg)
}

// Listen continuously reads messages from the WebSocket and dispatches them.
// It should be run in a separate goroutine.
// It exits when the context is cancelled or an unrecoverable error occurs.
func (c *GraphQLSubscriptionClient) Listen(ctx context.Context) error {
	defer close(c.listenDone) // Ensure listenDone is closed on any exit path

	if c.conn == nil {
		return fmt.Errorf("cannot listen: connection is not established")
	}

	for {
		// **CRITICAL:** Proactively check for cancellation before any read attempt.
		select {
		case <-c.internalCtx.Done():
			log.Println("Listen: internalCtx done, exiting.")
			return errors.New("connection closed by client (internal)")
		case <-ctx.Done():
			log.Println("Listen: main ctx done, exiting.")
			return ctx.Err()
		default:
			// Proceed only if not cancelled.
		}

		// Set a short read deadline to ensure ReadJSON doesn't block indefinitely
		// if the connection is in a weird state but not yet fully closed
		// in a way that ReadJSON would immediately return an error.
		if err := c.conn.SetReadDeadline(time.Now().Add(1 * time.Second)); err != nil {
			log.Printf("Listen: Error setting read deadline: %v. Assuming connection is dead.", err)
			// If SetReadDeadline fails, it's likely the connection is already dead.
			// We should check contexts to return the most accurate error.
			if c.internalCtx.Err() != nil {
				return errors.New("connection closed by client (internal, on set deadline error)")
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("set read deadline: %w", err)
		}

		var msg GraphQLMessage
		err := c.conn.ReadJSON(&msg) // This is the critical call

		// After ReadJSON, check if the connection became nil due to a concurrent Close.
		// This is a safeguard, though the proactive checks should ideally prevent this state.
		if c.conn == nil {
			 log.Println("Listen: Connection became nil during ReadJSON or before clearing deadline, exiting.")
			 // Prefer context errors if available, as they indicate the shutdown trigger.
			 if c.internalCtx.Err() != nil {
				return errors.New("connection closed by client (internal, conn nil post-read)")
			 }
			 if ctx.Err() != nil {
				return ctx.Err()
			 }
			 return errors.New("connection became nil")
		}
		
		// Only clear the read deadline if the error was a timeout.
		// If it was a more serious error, the connection might be truly dead,
		// and trying to operate on it (even to clear a deadline) could be risky.
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			 if errClear := c.conn.SetReadDeadline(time.Time{}); errClear != nil {
				log.Printf("Listen: Error clearing read deadline after timeout: %v. Assuming connection is dead.", errClear)
				// As above, prefer context errors if available.
				if c.internalCtx.Err() != nil {
					return errors.New("connection closed by client (internal, on clear deadline error)")
				}
				if ctx.Err() != nil {
					return ctx.Err()
				}
				return fmt.Errorf("clear read deadline: %w", errClear)
			 }
		}


		if err != nil {
			// Block until a context is done, or proceed if one is already done.
			// This ensures that if a context was cancelled during ReadJSON, we see it.
			// Add a short timeout to this select itself to prevent indefinite block if contexts are truly fine
			// and ReadJSON returned an error for other reasons.
			select {
			case <-c.internalCtx.Done():
				log.Printf("Listen: internalCtx done after ReadJSON error (%v), exiting.", err)
				return errors.New("connection closed by client (internal, after read error)")
			case <-ctx.Done():
				log.Printf("Listen: main ctx done after ReadJSON error (%v), exiting.", err)
				return ctx.Err()
			case <-time.After(50 * time.Millisecond): // Short timeout
				// Contexts didn't report done quickly. Now analyze 'err'.
				log.Printf("Listen: Contexts not immediately done after ReadJSON error (%v). Analyzing error.", err)
			}

			// Now, proceed with error analysis if contexts weren't found done above quickly.
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				// Before continuing, one final check of contexts. If a context is now reporting done,
				// then this timeout was likely part of its shutdown sequence.
				if c.internalCtx.Err() != nil {
					 log.Printf("Listen: internalCtx is now done after net.Error timeout for err=%v. Exiting.", err)
					 return errors.New("connection closed by client (internal, during net.Error timeout handling)")
				}
				if ctx.Err() != nil {
					 log.Printf("Listen: ctx is now done after net.Error timeout for err=%v. Exiting.", err)
					 return ctx.Err()
				}
				log.Println("Listen: ReadJSON timeout, contexts still appear active, continuing loop.")
				continue
			}

			// Handle specific WebSocket close errors or "use of closed network connection".
			// These indicate the connection is definitively closed.
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) ||
				(err != nil && strings.Contains(err.Error(), "use of closed network connection")) {
				log.Printf("Listen: WebSocket connection closed gracefully or expectedly: %v", err)
				return nil // Normal or expected closure, signal graceful exit from Listen.
			}

			// Any other error is unexpected and likely fatal for this listener.
			log.Printf("Listen: Unexpected error reading JSON message: %v", err)
			return fmt.Errorf("unexpected read error: %w", err)
		}
		
		// If read was successful, process the message.
		// Ensure conn is not nil before processing.
		if c.conn == nil { // Should be redundant due to earlier check, but as a safeguard.
			 log.Println("Listen: Connection became nil before handling message, exiting.")
			 return errors.New("connection became nil before handling message")
		}
		c.handleMessage(msg)
	}
}

// handleMessage dispatches incoming WebSocket messages to appropriate callbacks or handles control messages.
func (c *GraphQLSubscriptionClient) handleMessage(msg GraphQLMessage) {
	fmt.Printf("Received WebSocket message: Type=%s, ID=%s\n", msg.Type, msg.ID) // Debug log
	switch msg.Type {
	case "next", "data": // 'data' for graphql-ws, 'next' for graphql-transport-ws
		if callback, exists := c.callbacks[msg.ID]; exists {
			if msg.Payload != nil {
				callback(msg.Payload)
			} else {
				log.Printf("Received %s message with nil payload for ID %s", msg.Type, msg.ID)
			}
		} else {
			log.Printf("No callback registered for subscription ID %s", msg.ID)
		}
	case "error": // Used by both protocols for subscription-specific errors
		log.Printf("Subscription error for ID %s: %v", msg.ID, msg.Payload)
		// Optionally, notify callback or remove subscription
		delete(c.callbacks, msg.ID) // Stop processing further messages for this sub
	case "complete": // Used by both protocols
		log.Printf("Subscription %s completed by server.", msg.ID)
		delete(c.callbacks, msg.ID) // Clean up callback
	case "ping": // graphql-transport-ws
		// Respond with pong
		if err := c.sendMessage(GraphQLMessage{Type: "pong"}); err != nil {
			log.Printf("Error sending pong: %v", err)
		}
	case "pong": // graphql-transport-ws, response to our ping (if we were to send one)
		// Usually, client does not send pings, but handles server pings.
		log.Println("Received pong from server.")
	case "ka": // graphql-ws (Keep Alive)
		log.Println("Received keep-alive from server.")
	default:
		log.Printf("Unknown WebSocket message type received: %s", msg.Type)
	}
}

// GraphQLMessage defines the generic structure for messages exchanged over WebSocket.
type GraphQLMessage struct {
	ID      string      `json:"id,omitempty"`
	Type    string      `json:"type"`
	Payload interface{} `json:"payload,omitempty"`
}

// SubscriptionPayload defines the payload for a subscription request.
type SubscriptionPayload struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables,omitempty"`
	// OperationName string      `json:"operationName,omitempty"` // Optional
}

// Note: The main.Message struct (used in CallGraphQLAgentFunc parameter) is defined in model.go:
// type Message struct {
// Role    string `json:"role"`
// Content string `json:"content"`
// Format  string `json:"format"`
// TurnID  string `json:"turnId"`
// }
// This is compatible with what CallGraphQLAgentFunc expects for its `messages` parameter.
// The internal MessageInput and MessageOutput in this file are for GraphQL-specific structures.
