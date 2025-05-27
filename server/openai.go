package main

import (
	"context" // Required by machinebox/graphql
	"fmt"
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
)

// OpenAIRequest defines the structure for the OpenAI API request.
type OpenAIRequest struct {
	Model    string        `json:"model"`
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
var CallGraphQLAgentFunc = func(apiKey string, conversationID string, userID string, tenantID string, channelIDSystemContext string, userMessage string, apiURL string) ([]MessageOutput, error) {
	// Create a new GraphQL client
	client := graphql.NewClient(apiURL) // Use the passed apiURL

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
	SystemContext       []SystemContextInput   `json:"systemContext"`
	UserContext         UserContextInput       `json:"userContext"`
	Messages            []MessageInput         `json:"messages"`
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

// AnonymizationEntity defines the structure for an anonymization entity.
type AnonymizationEntity struct {
	Replacement string `json:"replacement"`
	Type        string `json:"type"`
	Value       string `json:"value"`
}

// MessageOutput defines the structure for a message in the agent response.
type MessageOutput struct {
	Content string `json:"content"`
	Format  string `json:"format"`
	Role    string `json:"role"`
	TurnID  string `json:"turnId"`
}
