package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
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
const GraphQLAgentAPIURL = "https://ia-platform-contract-agent.dev.apps.oneai.yo-digital.com/graphiql?path=/graphql"

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
var CallGraphQLAgentFunc = func(apiKey string, conversationID string, userID string, tenantID string, channelID string, userMessage string, apiURL string) ([]MessageOutput, error) {
	graphQLQuery := `
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

	requestPayload := AgentRequestInput{
		ConversationContext: ConversationContextInput{
			ConversationID: conversationID,
		},
		SystemContext: []SystemContextInput{
			{Key: "channelId", Value: channelID},
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

	graphqlVariables := map[string]interface{}{
		"request": requestPayload,
	}

	httpRequestPayload := GraphQLRequest{
		Query:     graphQLQuery,
		Variables: graphqlVariables,
	}

	jsonBody, err := json.Marshal(httpRequestPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal GraphQL request body: %w", err)
	}

	// Use the provided apiURL parameter instead of the global constant
	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create GraphQL request: %w", err)
	}

	// If the GraphQL API requires a specific API key or token, add it here.
	// For example, if it uses a Bearer token like OpenAI:
	// if apiKey != "" {
	//  req.Header.Set("Authorization", "Bearer " + apiKey)
	// }
	// If it uses a different auth mechanism (e.g. x-api-key), adjust accordingly.
	// For now, we are not adding any specific auth header beyond Content-Type.
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send GraphQL request: %w", err)
	}
	defer resp.Body.Close()

	responseBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read GraphQL response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GraphQL API request failed with status %d: %s", resp.StatusCode, string(responseBody))
	}

	var gqlResponse GraphQLResponse
	err = json.Unmarshal(responseBody, &gqlResponse)
	if err != nil {
		// Attempt to log more details if unmarshalling fails
		// Check if it's a direct agent response for non-subscription HTTP calls
		var agentResp AgentResponseData
		if errDirect := json.Unmarshal(responseBody, &agentResp); errDirect == nil && len(agentResp.Messages) > 0 {
			// This might be the case if the server returns the 'agent' data directly under 'data' or even at root for non-subscription queries over HTTP
			// For now, we stick to the defined GraphQLResponse structure
			// This block is more for debugging if the primary unmarshal fails.
		}
		return nil, fmt.Errorf("failed to unmarshal GraphQL response body: %w. Body: %s", err, string(responseBody))
	}

	if len(gqlResponse.Errors) > 0 {
		var errorMessages []string
		for _, gqlErr := range gqlResponse.Errors {
			errorMessages = append(errorMessages, gqlErr.Message)
		}
		return nil, fmt.Errorf("GraphQL query returned errors: %s", strings.Join(errorMessages, "; "))
	}

	// Check if the nested structure is as expected
	if gqlResponse.Data.Agent.Messages == nil {
		// This case handles when 'agent' is present but 'messages' is null.
		// Consider if anonymizationEntities alone is a valid response.
		// Based on the issue, we need messages.
		return nil, fmt.Errorf("no messages found in GraphQL response. Response body: %s", string(responseBody))
	}

	return gqlResponse.Data.Agent.Messages, nil
}

// GraphQLRequest wraps the query and variables for a GraphQL request.
type GraphQLRequest struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables"`
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

// GraphQLResponse wraps the data received from a GraphQL query.
type GraphQLResponse struct {
	Data   AgentResponseWrapper `json:"data"`
	Errors []GraphQLError       `json:"errors,omitempty"` // To capture potential GraphQL errors
}

// GraphQLError defines the structure for an error returned by GraphQL.
type GraphQLError struct {
	Message    string        `json:"message"`
	Locations  []Location    `json:"locations,omitempty"`
	Path       []interface{} `json:"path,omitempty"`
	Extensions interface{}   `json:"extensions,omitempty"`
}

// Location defines the structure for error locations in GraphQL.
type Location struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

// AgentResponseWrapper wraps the 'agent' field in the GraphQL response.
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
