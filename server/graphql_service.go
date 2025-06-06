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
	"runtime" // Added for runtime.Stack
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
var CallGraphQLAgentFunc = func(parentCtx context.Context, agentName string, conversationID string, userID string, tenantID string, channelIDSystemContext string, messages []Message, apiURL string, pingInterval time.Duration, messageChan chan<- string, errorChan chan<- error) {
	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()

	processor := NewResponseProcessor(ctx, messageChan, errorChan)
	client := NewGraphQLSubscriptionClient(apiURL, pingInterval)

	defer func() {
		log.Printf("[CallGraphQLAgentFunc] Cleaning up client connection...")
		if err := client.Close(); err != nil {
			log.Printf("[CallGraphQLAgentFunc] Error closing GraphQL client: %v", err)
		}
	}()

	if err := client.Connect(ctx); err != nil {
		log.Printf("[CallGraphQLAgentFunc] Connection failed: %v", err)
		select {
		case errorChan <- fmt.Errorf("graphql connection failed: %w", err):
		case <-parentCtx.Done():
			log.Printf("[CallGraphQLAgentFunc] Parent context done while sending connection error.")
		}
		return
	}

	log.Println("[CallGraphQLAgentFunc] Connected to GraphQL. Setting up subscription...")
	var messageBlockBuffer bytes.Buffer
	messageBlockBuffer.WriteString("[")
	for i, msg := range messages {
		messageInput := MessageInput{
			Content: msg.Content,
			Format:  msg.Format,
			Role:    msg.Role,
			TurnID:  msg.TurnID,
		}
		escapedContent := escapeString(messageInput.Content)
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
			case <-ctx.Done():
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
		return
	}

	log.Println("[CallGraphQLAgentFunc] Subscription started, now listening for AgentResponse objects until server closes or an error occurs...")
	listenErr := client.Listen(ctx)

	if listenErr != nil {
		if errors.Is(listenErr, context.Canceled) || errors.Is(listenErr, context.DeadlineExceeded) {
			log.Printf("[CallGraphQLAgentFunc] Listener stopped due to context cancellation/deadline: %v", listenErr)
		} else if strings.Contains(listenErr.Error(), "client explicitly closed") || strings.Contains(listenErr.Error(), "client connection field is nil") {
			log.Printf("[CallGraphQLAgentFunc] Listener stopped as client connection was intentionally closed: %v", listenErr)
		} else {
			log.Printf("[CallGraphQLAgentFunc] Listener returned an error: %v", listenErr)
			select {
			case errorChan <- fmt.Errorf("graphql listener error: %w", listenErr):
			case <-parentCtx.Done():
				log.Printf("[CallGraphQLAgentFunc] Parent context done while sending listener error.")
			default:
				log.Printf("[CallGraphQLAgentFunc] Error channel full or unavailable when sending listener error: %v", listenErr)
			}
		}
	} else {
		log.Println("[CallGraphQLAgentFunc] GraphQL subscription ended gracefully by server.")
	}

	processedCount, totalMsgCount := processor.GetStats()
	log.Printf("[CallGraphQLAgentFunc] GraphQL processing finished for this session. Final stats: %d responses processed, %d total messages sent",
		processedCount, totalMsgCount)
}

func handleAgentSubscriptionResponseMultiple(response AgentResponse) []*MessageOutput {
	var results []*MessageOutput
	if len(response.Errors) > 0 {
		for _, err := range response.Errors {
			log.Printf("GraphQL Error: %s\n", err.Message)
		}
		return results
	}
	log.Println("=== Agent Response ===")
	if len(response.Data.Agent.AnonymizationEntities) > 0 {
		log.Println("Anonymization Entities:")
		for _, entity := range response.Data.Agent.AnonymizationEntities {
			log.Printf("  Type: %s, Value: %s, Replacement: %s\n", entity.Type, entity.Value, entity.Replacement)
		}
	}
	if len(response.Data.Agent.Messages) > 0 {
		log.Printf("Processing %d messages:", len(response.Data.Agent.Messages))
		for i, msg := range response.Data.Agent.Messages {
			log.Printf("  Message %d - Role: %s, Format: %s, TurnID: %s\n", i+1, msg.Role, msg.Format, msg.TurnID)
			log.Printf("  Content: %s\n", msg.Content)
			log.Println("  ---")
			msgCopy := msg
			results = append(results, &msgCopy)
		}
	}
	log.Println("======================")
	return results
}

type GraphQLSubscriptionClient struct {
	conn           *websocket.Conn
	url            string
	callbacks      map[string]func(interface{})
	protocol       string
	internalCtx    context.Context
	internalCancel context.CancelFunc
	listenDone     chan struct{}
	mu             sync.RWMutex
	pingInterval   time.Duration
}

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

func (c *GraphQLSubscriptionClient) Connect(ctx context.Context) error {
	u, err := url.Parse(c.url)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	dialer := websocket.Dialer{Subprotocols: []string{"graphql-transport-ws", "graphql-ws"}}
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
		c.protocol = "graphql-ws" // Default if not specified by server
	}
	log.Printf("[GraphQLClient] Using protocol: %s\n", c.protocol)
	initMsgType := "connection_init"
	initPayload := make(map[string]interface{}) // Empty payload for graphql-ws
	if err := c.sendMessage(GraphQLMessage{Type: initMsgType, Payload: initPayload}); err != nil {
		conn.Close() // Ensure connection is closed on error
		return fmt.Errorf("init error: %w", err)
	}
	var ackMsg GraphQLMessage
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	if err := conn.ReadJSON(&ackMsg); err != nil {
		conn.Close()
		return fmt.Errorf("ack read error: %w", err)
	}
	conn.SetReadDeadline(time.Time{}) // Clear deadline
	expectedAckType := "connection_ack"
	if ackMsg.Type != expectedAckType {
		conn.Close()
		return fmt.Errorf("expected ack type '%s', got: '%s'", expectedAckType, ackMsg.Type)
	}
	log.Printf("[GraphQLClient] Connection established and acknowledged")
	conn.SetPongHandler(func(appData string) error {
		log.Printf("[GraphQLClient] Received pong from server. AppData: %s", appData)
		return nil
	})
	return nil
}

func (c *GraphQLSubscriptionClient) Close() error {
	log.Printf("[GraphQLClient] Initiating close sequence")
	c.internalCancel() // Signal Listen loop and other operations to stop

	c.mu.Lock()
	conn := c.conn
	c.conn = nil // Make original conn unavailable for new operations
	c.mu.Unlock()

	if conn != nil {
		log.Printf("[GraphQLClient] Closing WebSocket connection")
		return conn.Close()
	}
	log.Printf("[GraphQLClient] Connection was already closed or nil")
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
	conn.SetWriteDeadline(time.Now().Add(writeWait))
	err := conn.WriteJSON(msg)
	conn.SetWriteDeadline(time.Time{}) // Clear deadline immediately after write
	return err
}

func (c *GraphQLSubscriptionClient) Subscribe(subscriptionID, query string, variables map[string]interface{}, callback func(interface{})) error {
	c.mu.Lock()
	c.callbacks[subscriptionID] = callback
	c.mu.Unlock()
	msgType := "start" // Default for graphql-ws
	if c.protocol == "graphql-transport-ws" {
		msgType = "subscribe"
	}
	payload := map[string]interface{}{"query": query}
	if variables != nil {
		payload["variables"] = variables
	}
	msg := GraphQLMessage{ID: subscriptionID, Type: msgType, Payload: payload}
	log.Printf("[GraphQLClient] Subscribing with ID: %s", subscriptionID)
	return c.sendMessage(msg)
}

// Listen starts listening for messages from the WebSocket connection.
func (c *GraphQLSubscriptionClient) Listen(ctx context.Context) error {
	defer close(c.listenDone)
	log.Printf("[GraphQLClient] Starting to listen for messages...")

	ticker := time.NewTicker(c.pingInterval)
	defer ticker.Stop()

	// Use shorter read wait to make ping attempts more frequent
	localReadWait := c.pingInterval + 100*time.Millisecond
	if localReadWait < 500*time.Millisecond { // Ensure a minimum practical read wait
		localReadWait = 500 * time.Millisecond
	}
	log.Printf("[GraphQLClient] Effective read wait for connection: %s (ping interval: %s)", localReadWait, c.pingInterval)

	messageCount := 0

	for {
		var msg GraphQLMessage
		var errLoop error // Error for this loop iteration, distinct from errReadJSON

		// Prioritize context checks.
		select {
		case <-ctx.Done():
			log.Printf("[GraphQLClient] Main context done, stopping listener. Processed %d messages. Error: %v", messageCount, ctx.Err())
			return ctx.Err()
		case <-c.internalCtx.Done():
			log.Printf("[GraphQLClient] Internal context done (client.Close likely called), stopping listener. Processed %d messages. Error: %v", messageCount, c.internalCtx.Err())
			c.mu.RLock()
			connToInterrupt := c.conn
			c.mu.RUnlock()
			if connToInterrupt != nil {
				log.Printf("[GraphQLClient] Setting immediate read deadline on connection to interrupt ReadJSON due to internal close.")
				_ = connToInterrupt.SetReadDeadline(time.Now().Add(1 * time.Millisecond))
			}
			return fmt.Errorf("client explicitly closed: %w", c.internalCtx.Err())
		case <-ticker.C:
			c.mu.RLock()
			conn := c.conn
			c.mu.RUnlock()
			if conn == nil {
				log.Printf("[GraphQLClient] Ping: Connection is nil. Listener terminating.")
				return errors.New("ping attempt on nil connection")
			}
			log.Printf("[GraphQLClient] Attempting to send ping...")
			if err := conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(writeWait)); err != nil {
				log.Printf("[GraphQLClient] Error sending ping: %v. Terminating listener.", err)
				return fmt.Errorf("failed to send ping: %w", err)
			}
			log.Printf("[GraphQLClient] Ping sent successfully.")
			continue // Restart select to prioritize contexts and avoid immediate read.
		default:
			// No immediate context cancellation or ping, proceed to attempt a read.
		}

		// Pre-read check for internal context, if it was cancelled while in default path of above select.
		select {
		case <-c.internalCtx.Done():
			log.Printf("[GraphQLClient] Internal context done (pre-read check), stopping. Processed %d messages. Error: %v", messageCount, c.internalCtx.Err())
			return fmt.Errorf("client explicitly closed (pre-read): %w", c.internalCtx.Err())
		default:
			// Continue to read
		}

		c.mu.RLock()
		currentConn := c.conn
		c.mu.RUnlock()

		if currentConn == nil {
			log.Printf("[GraphQLClient] Read attempt: Connection is nil. Stopping. (processed %d messages)", messageCount)
			return errors.New("read attempt on nil connection")
		}

		deadline := time.Now().Add(localReadWait)
		log.Printf("[GraphQLClient] Setting read deadline to: %v (current time: %v, wait: %s)", deadline, time.Now(), localReadWait)
		if errSettingDeadline := currentConn.SetReadDeadline(deadline); errSettingDeadline != nil {
			log.Printf("[GraphQLClient] Error setting read deadline: %v. Checking internal context.", errSettingDeadline)
			select {
			case <-c.internalCtx.Done():
				return fmt.Errorf("client explicitly closed during SetReadDeadline: %w (original error: %v)", c.internalCtx.Err(), errSettingDeadline)
			default:
				return fmt.Errorf("error setting read deadline: %w", errSettingDeadline)
			}
		}

		var readPanicErr error
		func() {
			defer func() {
				if r := recover(); r != nil {
					stackBuf := make([]byte, 4096)                // Buffer for stack trace
					stackLength := runtime.Stack(stackBuf, false) // Get stack trace
					log.Printf("[GraphQLClient] PANIC recovered during ReadJSON: %v\nStack: %s", r, stackBuf[:stackLength])
					readPanicErr = fmt.Errorf("panic recovered from ReadJSON: %v", r)
				}
			}()
			// This is the line (approx 653 based on previous logs) that might panic
			errLoop = currentConn.ReadJSON(&msg)
		}()

		if readPanicErr != nil {
			errLoop = readPanicErr // Prioritize panic error
		}

		if errLoop == nil { // Successful read
			_ = currentConn.SetReadDeadline(time.Time{}) // Clear deadline
		}

		if errLoop != nil {
			log.Printf("[GraphQLClient] ReadJSON returned error: %T %v", errLoop, errLoop)
			select {
			case <-c.internalCtx.Done():
				log.Printf("[GraphQLClient] Read error '%v', and internal context is also done. Client explicitly closed. (processed %d messages)", errLoop, messageCount)
				return fmt.Errorf("client explicitly closed, ReadJSON unblocked with error: %w (original error: %v)", c.internalCtx.Err(), errLoop)
			default:
			}

			if netErr, ok := errLoop.(net.Error); ok && netErr.Timeout() {
				log.Printf("[GraphQLClient] Read timeout (waited up to %s). Checking for shutdown signals and connection health...", localReadWait.String())
				select {
				case <-c.internalCtx.Done():
					log.Printf("[GraphQLClient] Read timeout, but internal context is done (client.Close likely called). Processed %d messages.", messageCount)
					return fmt.Errorf("client explicitly closed, inducing read timeout: %w (original error: %v)", c.internalCtx.Err(), errLoop)
				case <-ctx.Done():
					log.Printf("[GraphQLClient] Read timeout, but main context is done. Processed %d messages. Error: %v", messageCount, ctx.Err())
					return ctx.Err()
				default:
					// Contexts are not done. Before continuing, try a quick ping to see if the connection is still responsive.
					c.mu.RLock()
					pingCheckConn := c.conn
					c.mu.RUnlock()

					if pingCheckConn != nil {
						pingErr := pingCheckConn.WriteControl(websocket.PingMessage, []byte("healthcheck"), time.Now().Add(1*time.Second))
						if pingErr == nil {
							log.Printf("[GraphQLClient] Read timed out, but connection responded to health check ping. Will attempt to read again.")
							continue // Healthy enough to try reading again
						}
						log.Printf("[GraphQLClient] Read timed out, AND health check ping failed (%v). Connection assumed dead.", pingErr)
						return fmt.Errorf("read timeout, health check ping failed: %w (original timeout: %v)", pingErr, errLoop)
					}
					// Connection is nil, cannot perform health check.
					log.Printf("[GraphQLClient] Read timed out, but connection became nil before health check. Terminating.")
					return fmt.Errorf("connection nil before health check after read timeout: %w", errLoop)
				}
			}

			if websocket.IsCloseError(errLoop, websocket.CloseNormalClosure) {
				log.Printf("[GraphQLClient] WebSocket closed normally by peer: %v (processed %d messages)", errLoop, messageCount)
				return nil
			}
			var wsCloseErr *websocket.CloseError
			if errors.As(errLoop, &wsCloseErr) {
				log.Printf("[GraphQLClient] WebSocket closed with specific code %d: %v (processed %d messages)", wsCloseErr.Code, errLoop, messageCount)
				return fmt.Errorf("websocket closed with code %d: %w", wsCloseErr.Code, errLoop)
			}
			if websocket.IsUnexpectedCloseError(errLoop, websocket.CloseGoingAway, websocket.CloseAbnormalClosure, websocket.CloseNoStatusReceived) {
				log.Printf("[GraphQLClient] WebSocket unexpected close error: %v (processed %d messages)", errLoop, messageCount) // Corrected to errLoop
				return fmt.Errorf("websocket unexpected close: %w", errLoop)
			}

			log.Printf("[GraphQLClient] Unhandled persistent read error: %v (processed %d messages)", errLoop, messageCount)
			return fmt.Errorf("persistent websocket read error: %w", errLoop)
		}

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
		case "error":
			log.Printf("[GraphQLClient] GraphQL subscription error message received: ID=%s, Payload=%+v", msg.ID, msg.Payload)
			c.mu.RLock()
			callback, exists := c.callbacks[msg.ID]
			c.mu.RUnlock()
			if exists && callback != nil {
				callback(map[string]interface{}{"errors": []map[string]interface{}{{"message": fmt.Sprintf("GraphQL subscription error via WebSocket: ID=%s, Payload=%+v", msg.ID, msg.Payload)}}})
			}
		case "complete":
			log.Printf("[GraphQLClient] GraphQL subscription completed for ID: %s", msg.ID)
			c.mu.Lock()
			delete(c.callbacks, msg.ID)
			c.mu.Unlock()
		case "connection_keep_alive", "ka", "ping":
			log.Printf("[GraphQLClient] Received keep-alive/ping message type: %s", msg.Type)
			if msg.Type == "ping" && c.protocol == "graphql-transport-ws" {
				if err := c.sendMessage(GraphQLMessage{Type: "pong", Payload: msg.Payload}); err != nil { // Use loop-scoped 'err' here or new var
					log.Printf("[GraphQLClient] Error sending pong: %v", err)
				} else {
					log.Printf("[GraphQLClient] Sent pong in response to ping.")
				}
			}
		case "pong":
			log.Printf("[GraphQLClient] Received pong message type: %s", msg.Type)
		default:
			log.Printf("[GraphQLClient] Unhandled message type: %s. Payload: %+v", msg.Type, msg.Payload)
		}
	}
}
