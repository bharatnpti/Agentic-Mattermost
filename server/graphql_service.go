package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors" // Added for errors.New
	"fmt"
	"log"
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
		log.Printf("[GraphQLCallback DEBUG] Entered callback with payload: %+v", payload)

		payloadBytes, marshalErr := json.Marshal(payload)
		if marshalErr != nil {
			log.Printf("[GraphQLCallback DEBUG] Error marshalling payload: %v", marshalErr)
			select {
			case errChan <- fmt.Errorf("payload marshal error: %w", marshalErr):
			default:
				log.Printf("[GraphQLCallback DEBUG] errChan full or unavailable when sending marshalErr")
			}
			return
		}
		log.Printf("[GraphQLCallback DEBUG] Payload marshalled: %s", string(payloadBytes))

		var response AgentResponse
		if unmarshalErr := json.Unmarshal(payloadBytes, &response); unmarshalErr != nil {
			log.Printf("[GraphQLCallback DEBUG] Error unmarshalling payloadBytes: %v", unmarshalErr)
			select {
			case errChan <- fmt.Errorf("payload unmarshal error: %w", unmarshalErr):
			default:
				log.Printf("[GraphQLCallback DEBUG] errChan full or unavailable when sending unmarshalErr")
			}
			return
		}
		log.Printf("[GraphQLCallback DEBUG] Payload unmarshalled into AgentResponse: %+v", response)

		processedMessage := handleAgentSubscriptionResponse(response)
		log.Printf("[GraphQLCallback DEBUG] Processed message from handleAgentSubscriptionResponse: %+v", processedMessage)

		if processedMessage != nil && processedMessage.Content != "" {
			log.Printf("[GraphQLCallback DEBUG] Attempting to send content to contentChan: %s", processedMessage.Content)
			select {
			case contentChan <- processedMessage.Content:
				log.Printf("[GraphQLCallback DEBUG] Successfully sent content to contentChan.")
			default:
				log.Printf("[GraphQLCallback DEBUG] contentChan was full or unavailable; message dropped.")
			}
		} else if len(response.Errors) > 0 {
			log.Printf("[GraphQLCallback DEBUG] GraphQL response contained errors: %+v. Sending to errChan.", response.Errors)
			select {
			case errChan <- fmt.Errorf("GraphQL error: %s", response.Errors[0].Message):
				log.Printf("[GraphQLCallback DEBUG] Successfully sent GraphQL error to errChan.")
			default:
				log.Printf("[GraphQLCallback DEBUG] errChan full or unavailable when sending GraphQL error.")
			}
		} else {
			log.Printf("[GraphQLCallback DEBUG] No content in processedMessage and no GraphQL errors. Nothing sent to contentChan or errChan.")
		}
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
	log.Printf("[GraphQLClient DEBUG] Close: Aggressive Version - Calling internalCancel().")
	c.internalCancel() // Signal Listen to stop

	// No waiting for listenDone in this version.

	if c.conn != nil {
		log.Printf("[GraphQLClient DEBUG] Close: Aggressive Version - Calling c.conn.Close() immediately.")
		err := c.conn.Close()
		c.conn = nil // Ensure conn is nil after closing
		return err
	}
	log.Printf("[GraphQLClient DEBUG] Close: Aggressive Version - Connection was already nil.")
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
// Listen continuously reads messages from the WebSocket and dispatches them.
// It should be run in a separate goroutine.
// It exits when the context is cancelled or an unrecoverable error occurs.
func (c *GraphQLSubscriptionClient) Listen(ctx context.Context) error {
	defer close(c.listenDone) // Ensure listenDone is closed on any exit path

	if c.conn == nil {
		// It's possible internalCancel was called before Listen even started if Connect failed
		// and Close was called immediately.
		if c.internalCtx.Err() != nil {
			log.Printf("[GraphQLClient DEBUG] Listen: Connection is nil and internalCtx is done, exiting early.")
			return errors.New("connection closed by client (internal, connection nil at start)")
		}
		if ctx.Err() != nil {
			log.Printf("[GraphQLClient DEBUG] Listen: Connection is nil and main ctx is done, exiting early.")
			return ctx.Err()
		}
		log.Printf("[GraphQLClient DEBUG] Listen: Connection is nil but contexts are active. This is unexpected if Connect succeeded.")
		return fmt.Errorf("cannot listen: connection is not established")
	}

	// Create a channel to receive messages
	msgChan := make(chan GraphQLMessage, 1)
	errChan := make(chan error, 1)

	// Start a goroutine to read messages
	go func() {
		defer close(msgChan)
		defer close(errChan)

		for {
			var msg GraphQLMessage
			err := c.conn.ReadJSON(&msg)

			if err != nil {
				// Check if it's a normal close
				if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) ||
					strings.Contains(err.Error(), "use of closed network connection") {
					log.Printf("[GraphQLClient DEBUG] Listen: WebSocket connection closed: %v", err)
					return // Exit gracefully
				}

				// Send error to main loop
				select {
				case errChan <- err:
				case <-c.internalCtx.Done():
					return
				case <-ctx.Done():
					return
				}
				return
			}

			// Send message to main loop
			select {
			case msgChan <- msg:
			case <-c.internalCtx.Done():
				return
			case <-ctx.Done():
				return
			}
		}
	}()

	// Main event loop
	for {
		select {
		case <-c.internalCtx.Done():
			log.Printf("[GraphQLClient DEBUG] Listen: internalCtx done, exiting.")
			return errors.New("connection closed by client (internal)")

		case <-ctx.Done():
			log.Printf("[GraphQLClient DEBUG] Listen: main ctx done, exiting.")
			return ctx.Err()

		case msg, ok := <-msgChan:
			if !ok {
				// Channel closed, reader goroutine exited
				log.Printf("[GraphQLClient DEBUG] Listen: Message channel closed, reader exited.")
				return nil
			}
			log.Printf("[GraphQLClient DEBUG] Listen: Successfully received message: Type=%s, ID=%s", msg.Type, msg.ID)
			c.handleMessage(msg)

		case err, ok := <-errChan:
			if !ok {
				// Channel closed, should not happen before msgChan
				log.Printf("[GraphQLClient DEBUG] Listen: Error channel closed unexpectedly.")
				return nil
			}
			log.Printf("[GraphQLClient DEBUG] Listen: Received error from reader: %v", err)
			return fmt.Errorf("read error: %w", err)
		}
	}
}

// handleMessage dispatches incoming WebSocket messages to appropriate callbacks or handles control messages.
func (c *GraphQLSubscriptionClient) handleMessage(msg GraphQLMessage) {
	log.Printf("[GraphQLClient DEBUG] handleMessage: Processing message: Type=%s, ID=%s", msg.Type, msg.ID)
	// fmt.Printf("Received WebSocket message: Type=%s, ID=%s\n", msg.Type, msg.ID) // Original Debug log
	switch msg.Type {
	case "next", "data": // 'data' for graphql-ws, 'next' for graphql-transport-ws
		if callback, exists := c.callbacks[msg.ID]; exists {
			if msg.Payload != nil {
				log.Printf("[GraphQLClient DEBUG] handleMessage: Invoking callback for ID %s", msg.ID)
				callback(msg.Payload)
			} else {
				log.Printf("[GraphQLClient DEBUG] handleMessage: Received %s message with nil payload for ID %s", msg.Type, msg.ID)
			}
		} else {
			log.Printf("[GraphQLClient DEBUG] handleMessage: No callback registered for subscription ID %s", msg.ID)
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
