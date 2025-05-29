package main

// Message is a struct to represent a message in the chat, including fields expected by the GraphQL agent.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	Format  string `json:"format"` // Added: Format of the message, e.g., "text"
	TurnID  string `json:"turnId"` // Added: Unique identifier for the turn in the conversation
}
