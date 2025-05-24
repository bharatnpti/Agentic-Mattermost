package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
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
