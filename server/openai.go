package main

import (
	"context" // Required by machinebox/graphql
	"fmt"
	"log"
	"strings"
	"time"

	// "bytes" // May no longer be needed by CallGraphQLAgentFunc
	// "encoding/json" // May no longer be needed for top-level request/response marshalling by CallGraphQLAgentFunc
	// "net/http" // May no longer be needed by CallGraphQLAgentFunc
	// "strings" // No longer needed as GraphQL errors are handled by machinebox/graphql

	"github.com/machinebox/graphql"
	"io/ioutil" // Required by CallOpenAIAPIFunc

	// Keep these for CallOpenAIAPIFunc and its structs
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/gorilla/websocket"
)

// OpenAIRequest defines the structure for the OpenAI API request.
type OpenAIRequest struct {
	Model    string          `json:"model"`
	Messages []OpenAIMessage `json:"messages"`
}

// OpenAIMessage defines the structure for a message in the OpenAI API request.
type OpenAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// OpenAIResponse defines the structure for the OpenAI API response.
type OpenAIResponse struct {
	Choices []Choice `json:"choices"`
}

// Choice defines the structure for a choice in the OpenAI API response.
type Choice struct {
	Message OpenAIMessage `json:"message"`
}

// OpenAIAPIURL is the default URL for the OpenAI API.
const OpenAIAPIURL = "https://api.openai.com/v1/chat/completions"

// GraphQLAgentAPIURL is the URL for the GraphQL agent API.
// Changed to var for testability (allowing modification for network error tests).
var GraphQLAgentAPIURL = "https://ia-platform-contract-agent.dev.apps.oneai.yo-digital.com/graphiql?path=/graphql"

// CallOpenAIAPIFunc is a function variable that can be replaced for testing.
// It makes a request to the OpenAI API and returns the response.
// It takes an apiURL parameter to allow for testing with mock servers.
var CallOpenAIAPIFunc = func(apiKey string, message string, apiURL string) (string, error) {
	requestBody := OpenAIRequest{
		Model: "gpt-3.5-turbo",
		Messages: []OpenAIMessage{
			{
				Role:    "user",
				Content: message,
			},
		},
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request body: %w", err)
	}

	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := ioutil.ReadAll(resp.Body)
		return "", fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	responseBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	var openAIResponse OpenAIResponse
	err = json.Unmarshal(responseBody, &openAIResponse)
	if err != nil {
		return "", fmt.Errorf("failed to unmarshal response body: %w", err)
	}

	if len(openAIResponse.Choices) > 0 && openAIResponse.Choices[0].Message.Content != "" {
		return openAIResponse.Choices[0].Message.Content, nil
	}

	return "", fmt.Errorf("no response text found or choices array is empty")
}

// CallGraphQLAgentFunc makes a request to the GraphQL agent API and returns the messages.
// The apiKey parameter is kept for now for structural consistency but might not be used
// if the GraphQL endpoint does not require bearer token authentication in this manner.
// Added apiURL parameter for testability.
var CallGraphQLAgentFunc_backup = func(apiKey string, conversationID string, userID string, tenantID string, channelIDSystemContext string, userMessage string, apiURL string) ([]MessageOutput, error) {
	// Create a new GraphQL client
	client := graphql.NewClient("ws://localhost:8080/subscriptions") // Use the passed apiURL

	// The original query is a subscription.
	// Note: machinebox/graphql primarily sends queries/mutations over HTTP POST.
	// If the server strictly requires WebSockets for subscriptions, this might not work as a true subscription.
	// However, some servers accept 'subscription' keyword over POST and treat it as a query.
	graphQLQueryString := `
           subscription Agent($request: AgentRequestInput!) {
               agent(request: $request) {
                   anonymizationEntities {
                       replacement
                       type
                       value
                   }
                   messages {
                       content
                       format
                       role
                       turnId
                   }
               }
           }`

	// Note: The GraphQL query provided is a 'subscription'.
	// Subscriptions are typically handled over WebSockets, not plain HTTP POST.
	// However, the issue asks to make a "graphql call" and provides an HTTP endpoint.
	// Many GraphQL servers support subscriptions over HTTP for simple cases or as a fallback,
	// often treating them like queries if the transport doesn't support WebSockets.
	// If this call fails due to the subscription type, the query might need to be changed
	// to a 'mutation' or 'query' if the backend supports it for this operation,
	// or the transport mechanism would need to change to WebSockets.
	// For this step, we will proceed with HTTP POST as implied by current structure.

	// Create a request
	gqlReq := graphql.NewRequest(graphQLQueryString)

	// Define the request payload (variables)
	requestPayload := AgentRequestInput{
		ConversationContext: ConversationContextInput{
			ConversationID: conversationID,
		},
		SystemContext: []SystemContextInput{
			{Key: "channelId", Value: channelIDSystemContext}, // Parameter name updated
			{Key: "tenantId", Value: tenantID},
		},
		UserContext: UserContextInput{
			UserID:  userID,
			Profile: []interface{}{}, // Empty profile as per example
		},
		Messages: []MessageInput{
			{
				Content: userMessage,
				Format:  "text",
				Role:    "user",
			},
		},
	}
	gqlReq.Var("request", requestPayload)

	// Set headers if needed (e.g., for authentication)
	// The apiKey is passed but not used here unless a specific header is required.
	// if apiKey != "" {
	//  gqlReq.Header.Set("Authorization", "Bearer " + apiKey)
	// }
	// gqlReq.Header.Set("Content-Type", "application/json") // Library usually sets this.

	// Define a context
	ctx := context.Background()

	// Define the structure for the response.
	// machinebox/graphql unmarshals the content of the "data" field.
	var respData AgentResponseWrapper // This struct has `Agent AgentResponseData `json:"agent"`

	// Run the request
	if err := client.Run(ctx, gqlReq, &respData); err != nil {
		// This error could be a network error, a non-200 status, or a GraphQL error returned in the error object by the library.
		return nil, fmt.Errorf("graphql request failed: %w", err)
	}

	// If client.Run returns no error, it implies GraphQL errors were not in the main error path.
	// machinebox/graphql typically returns an error from Run if the response JSON contains an "errors" field.

	if respData.Agent.Messages == nil {
		// This case handles when 'agent' is present but 'messages' is null, or if 'agent' itself is null.
		// It could also happen if the unmarshalling into respData didn't work as expected.
		return nil, fmt.Errorf("no messages found in GraphQL response, or response structure mismatch")
	}

	return respData.Agent.Messages, nil
}

// AgentRequestInput defines the structure for the 'request' input object in the GraphQL query.
type AgentRequestInput struct {
	ConversationContext ConversationContextInput `json:"conversationContext"`
	SystemContext       []SystemContextInput     `json:"systemContext"`
	UserContext         UserContextInput         `json:"userContext"`
	Messages            []MessageInput           `json:"messages"`
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
	Profile []interface{} `json:"profile"` // Assuming profile can be an array of various simple types or empty.
}

// MessageInput defines the structure for a message in the agent request.
type MessageInput struct {
	Content string `json:"content"`
	Format  string `json:"format"`
	Role    string `json:"role"`
}

// GraphQLRequest struct is no longer needed by CallGraphQLAgentFunc.
// type GraphQLRequest struct {
// 	Query     string                 `json:"query"`
// 	Variables map[string]interface{} `json:"variables"`
// }

// GraphQLResponse struct may also be obsolete for CallGraphQLAgentFunc if machinebox/graphql handles errors well.
// type GraphQLResponse struct {
// 	Data   AgentResponseWrapper `json:"data"`
// 	Errors []GraphQLError       `json:"errors,omitempty"`
// }

// GraphQLError and Location structs might also be obsolete if machinebox/graphql handles these.
// type GraphQLError struct {
// 	Message    string        `json:"message"`
// 	Locations  []Location    `json:"locations,omitempty"`
// 	Path       []interface{} `json:"path,omitempty"`
// 	Extensions interface{}   `json:"extensions,omitempty"`
// }
// type Location struct {
// 	Line   int `json:"line"`
// 	Column int `json:"column"`
// }

// AgentResponseWrapper wraps the 'agent' field in the GraphQL response.
// This is now the primary response struct for client.Run.
type AgentResponseWrapper struct {
	Agent AgentResponseData `json:"agent"`
}

// AgentResponseData defines the structure for the data returned by the 'agent' query.
type AgentResponseData struct {
	AnonymizationEntities []AnonymizationEntity `json:"anonymizationEntities"`
	Messages              []MessageOutput       `json:"messages"`
}

// MessageOutput defines the structure for a message in the agent response.
type MessageOutput struct {
	Content string `json:"content"`
	Format  string `json:"format"`
	Role    string `json:"role"`
	TurnID  string `json:"turnId"`
}

//var CallGraphQLAgentFunc2 = func(apiKey string, conversationID string, userID string, tenantID string, channelIDSystemContext string, userMessage string, apiURL string) ([]MessageOutput, error) {
//
//	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
//	defer cancel()
//
//	// Create client
//	client := NewGraphQLSubscriptionClient("ws://localhost:8080/subscriptions")
//	defer client.Close()
//
//	// Connect
//	if err := client.Connect(ctx); err != nil {
//		log.Fatal("Connection failed:", err)
//	}
//
//	fmt.Println("Connected to GraphQL subscription endpoint")
//
//	contentChan := make(chan string, 1)
//
//	err := client.Subscribe("agent-subscription", query, nil, func(payload interface{}) {
//		payloadBytes, _ := json.Marshal(payload)
//		var response AgentResponse
//		if err := json.Unmarshal(payloadBytes, &response); err != nil {
//			log.Println("unmarshal error:", err)
//			return
//		}
//
//		message := handleAgentResponse(response)
//		if message.Content != "" {
//			select {
//			case contentChan <- message.Content:
//			default:
//			}
//		}
//	})
//
//	if err != nil {
//		log.Fatal("Subscription failed:", err)
//	}
//
//	fmt.Println("Subscription started, listening for messages...")
//
//	go func() {
//		if err := client.Listen(ctx); err != nil && err != context.DeadlineExceeded {
//			log.Printf("Listen error: %v", err)
//		}
//	}()
//
//	select {
//	case content := <-contentChan:
//		return content, nil
//	case <-ctx.Done():
//		return "", fmt.Errorf("timeout waiting for first message")
//	}
//
//}

// escapeString handles any special characters in input strings for safety
func escapeString(str string) string {
	return strings.ReplaceAll(str, `"`, `\"`)
}

var CallGraphQLAgentFunc = func(apiKey string, conversationID string, userID string, tenantID string, channelIDSystemContext string, userMessage string, apiURL string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := NewGraphQLSubscriptionClient(apiURL) // Use the apiURL parameter
	defer client.Close()

	if err := client.Connect(ctx); err != nil {
		log.Fatal("Connection failed:", err)
	}

	fmt.Println("Connected to GraphQL subscription endpoint")

	// Channel to receive first message content
	contentChan := make(chan string, 1)

	messages := []Message{
		{Content: "Can I cancel my contract?", Format: "text", Role: "user"},
		{Content: "Please let me know the process.", Format: "text", Role: "user"},
	}

	var messageEntries []string
	for _, msg := range messages {
		messageEntry := fmt.Sprintf(`{
			content: "%s",
			format: "%s",
			role: "%s"
		}`, escapeString(msg.Content), escapeString(msg.Format), escapeString(msg.Role))
		messageEntries = append(messageEntries, messageEntry)
	}
	messageBlock := strings.Join(messageEntries, ",\n")

	query := fmt.Sprintf(`subscription {
	agent(request: {
		conversationContext: {
			conversationId: "%s"
		},
		systemContext: [
			{
				key: "channelId",
				value: "%s"
			},
			{
				key: "tenantId",
				value: "%s"
			}
		],
		userContext: {
			userId: "%s",
			profile: []
		},
		messages: [
			%s
		]
	}) {
		anonymizationEntities {
			replacement,
			type,
			value
		},
		messages {
			content,
			format,
			role,
			turnId
		}
	}
}`, escapeString(conversationID), escapeString(channelIDSystemContext), escapeString(tenantID), escapeString(userID), messageBlock)

	fmt.Println(query)

	err := client.Subscribe("agent-subscription", query, nil, func(payload interface{}) {
		payloadBytes, _ := json.Marshal(payload)
		var response AgentResponse
		if err := json.Unmarshal(payloadBytes, &response); err != nil {
			log.Println("unmarshal error:", err)
			return
		}

		message := handleAgentResponse(response)
		if message.Content != "" {
			select {
			case contentChan <- message.Content:
			default:
			}
		}
	})

	if err != nil {
		log.Fatal("Subscription failed:", err)
	}

	fmt.Println("Subscription started, listening for messages...")

	go func() {
		if err := client.Listen(ctx); err != nil && err != context.DeadlineExceeded {
			log.Printf("Listen error: %v", err)
		}
	}()

	// Wait for first message content or timeout
	select {
	case content := <-contentChan:
		return content, nil
	case <-ctx.Done():
		return "", fmt.Errorf("timeout waiting for first message")
	}
}

func handleAgentResponse(response AgentResponse) *Message {
	if len(response.Errors) > 0 {
		for _, err := range response.Errors {
			log.Printf("GraphQL Error: %s\n", err.Message)
		}
		return nil
	}

	fmt.Println("=== Agent Response ===")

	// Print anonymization entities
	if len(response.Data.Agent.AnonymizationEntities) > 0 {
		fmt.Println("Anonymization Entities:")
		for _, entity := range response.Data.Agent.AnonymizationEntities {
			fmt.Printf("  Type: %s, Value: %s, Replacement: %s\n",
				entity.Type, entity.Value, entity.Replacement)
		}
	}

	// Print messages
	if len(response.Data.Agent.Messages) > 0 {
		firstMsg := &response.Data.Agent.Messages[0]
		fmt.Println("Messages:")
		for _, msg := range response.Data.Agent.Messages {
			fmt.Printf("  Role: %s, Format: %s, TurnID: %s\n",
				msg.Role, msg.Format, msg.TurnID)
			fmt.Printf("  Content: %s\n", msg.Content)
			fmt.Println("  ---")
		}
		return firstMsg
	}

	fmt.Println("======================")
	return nil
}

func NewGraphQLSubscriptionClient(wsURL string) *GraphQLSubscriptionClient {
	return &GraphQLSubscriptionClient{
		url:       wsURL,
		callbacks: make(map[string]func(interface{})),
	}
}

func (c *GraphQLSubscriptionClient) Connect(ctx context.Context) error {
	u, err := url.Parse(c.url)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	// Set up WebSocket headers with GraphQL subprotocols
	dialer := websocket.Dialer{
		Subprotocols: []string{"graphql-transport-ws", "graphql-ws"},
	}

	conn, resp, err := dialer.DialContext(ctx, u.String(), nil)
	if err != nil {
		return fmt.Errorf("dial error: %w", err)
	}

	c.conn = conn

	// Determine which protocol was selected
	if resp != nil && len(resp.Header.Get("Sec-WebSocket-Protocol")) > 0 {
		c.protocol = resp.Header.Get("Sec-WebSocket-Protocol")
	} else {
		c.protocol = "graphql-ws" // Default fallback
	}

	fmt.Printf("Using protocol: %s\n", c.protocol)

	// Initialize connection based on protocol
	var initMsg GraphQLMessage
	if c.protocol == "graphql-transport-ws" {
		initMsg = GraphQLMessage{
			Type: "connection_init",
			Payload: map[string]interface{}{
				"connectionParams": map[string]interface{}{},
			},
		}
	} else {
		// graphql-ws protocol
		initMsg = GraphQLMessage{
			Type:    "connection_init",
			Payload: map[string]interface{}{},
		}
	}

	if err := c.sendMessage(initMsg); err != nil {
		return fmt.Errorf("init error: %w", err)
	}

	// Wait for acknowledgment
	var ackMsg GraphQLMessage
	if err := c.conn.ReadJSON(&ackMsg); err != nil {
		return fmt.Errorf("ack read error: %w", err)
	}

	expectedAckType := "connection_ack"
	if c.protocol == "graphql-transport-ws" {
		expectedAckType = "connection_ack"
	}

	if ackMsg.Type != expectedAckType {
		return fmt.Errorf("expected %s, got: %s", expectedAckType, ackMsg.Type)
	}

	return nil
}

func (c *GraphQLSubscriptionClient) Close() error {
	if c.conn != nil {
		c.sendMessage(GraphQLMessage{Type: "connection_terminate"})
		return c.conn.Close()
	}
	return nil
}

func (c *GraphQLSubscriptionClient) sendMessage(msg GraphQLMessage) error {
	fmt.Printf("Sending: %s\n", msg.Type)
	return c.conn.WriteJSON(msg)
}

func (c *GraphQLSubscriptionClient) Subscribe(subscriptionID, query string, variables map[string]interface{}, callback func(interface{})) error {
	c.callbacks[subscriptionID] = callback

	var msgType string
	if c.protocol == "graphql-transport-ws" {
		msgType = "subscribe"
	} else {
		msgType = "start"
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

type SubscriptionPayload struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables,omitempty"`
}

type GraphQLMessage struct {
	ID      string      `json:"id,omitempty"`
	Type    string      `json:"type"`
	Payload interface{} `json:"payload,omitempty"`
}

func (c *GraphQLSubscriptionClient) Listen(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			var msg GraphQLMessage
			if err := c.conn.ReadJSON(&msg); err != nil {
				return fmt.Errorf("read error: %w", err)
			}

			c.handleMessage(msg)
		}
	}
}

func (c *GraphQLSubscriptionClient) handleMessage(msg GraphQLMessage) {
	switch msg.Type {
	case "next", "data": // Handle both protocols
		if callback, exists := c.callbacks[msg.ID]; exists {
			callback(msg.Payload)
		}
	case "error":
		log.Printf("Subscription error for ID %s: %v", msg.ID, msg.Payload)
	case "complete":
		log.Printf("Subscription %s completed", msg.ID)
		delete(c.callbacks, msg.ID)
	case "ping":
		// Send pong for graphql-transport-ws
		c.sendMessage(GraphQLMessage{Type: "pong"})
	case "pong":
		// Pong received, ignore
	case "ka":
		// Keep-alive message for graphql-ws, ignore
	default:
		log.Printf("Unknown message type: %s", msg.Type)
	}
}

type GraphQLSubscriptionClient struct {
	conn      *websocket.Conn
	url       string
	callbacks map[string]func(interface{})
	protocol  string // Track which protocol is being used
}

type AgentResponse struct {
	Data   AgentData `json:"data"`
	Errors []Error   `json:"errors,omitempty"`
}

type AgentData struct {
	Agent Agent `json:"agent"`
}

type Agent struct {
	AnonymizationEntities []AnonymizationEntity `json:"anonymizationEntities"`
	Messages              []Message             `json:"messages"`
}

type AnonymizationEntity struct {
	Replacement string `json:"replacement"`
	Type        string `json:"type"`
	Value       string `json:"value"`
}

type Message struct {
	Content string `json:"content"`
	Format  string `json:"format"`
	Role    string `json:"role"`
	TurnID  string `json:"turnId"`
}

type Error struct {
	Message string `json:"message"`
}
