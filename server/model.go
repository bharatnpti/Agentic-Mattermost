package main

// Message is a struct to represent a message in the chat, including fields expected by the GraphQL agent.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	Format  string `json:"format"` // Added: Format of the message, e.g., "text"
	TurnID  string `json:"turnId"` // Added: Unique identifier for the turn in the conversation
}

// WorkflowMessageRequest defines the structure for the workflow message API request.
type WorkflowMessageRequest struct {
	ChannelId string `json:"channelId"`
	Message   string `json:"message"`
	UserId    string `json:"userId"`
}

// WorkflowMessageResponse defines the structure for the workflow message API response.
// Assuming a simple response, can be expanded later if needed.
type WorkflowMessageResponse struct {
	Status  string `json:"status"`
	Error   string `json:"error,omitempty"`
}
