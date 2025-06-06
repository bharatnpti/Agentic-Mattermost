package main

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
)

type MaestroHandler struct {
	API                  plugin.API
	BotUserID            string
	GetConfig            func() *configuration
	CallGraphQLAgentFunc func(parentCtx context.Context, apiKey string, conversationID string, userID string, tenantID string, channelIDSystemContext string, messages []Message, apiURL string, pingInterval time.Duration, messageChan chan<- string, errorChan chan<- error)
}

func NewMaestroHandler(api plugin.API, botUserID string, getConfig func() *configuration, callGraphQLAgentFunc func(parentCtx context.Context, apiKey string, conversationID string, userID string, tenantID string, channelIDSystemContext string, messages []Message, apiURL string, pingInterval time.Duration, messageChan chan<- string, errorChan chan<- error)) *MaestroHandler {
	return &MaestroHandler{
		API:                  api,
		BotUserID:            botUserID,
		GetConfig:            getConfig,
		CallGraphQLAgentFunc: callGraphQLAgentFunc,
	}
}

// MessageHasBeenPosted is invoked when a message is posted to a channel.
func (h *MaestroHandler) MessageHasBeenPosted(c *plugin.Context, post *model.Post) {
	// 1. Ignore posts made by the bot itself
	if post.UserId == h.BotUserID {
		return
	}

	// 2. Check if the message starts with !maestro (case-insensitive)
	messageLowercase := strings.ToLower(post.Message)
	triggerPrefix := "!maestro"

	if !strings.HasPrefix(messageLowercase, triggerPrefix) {
		return
	}

	// 3. Extract the arguments string
	argumentsString := strings.TrimSpace(post.Message[len(triggerPrefix):])

	// 5. Add logging
	h.API.LogInfo(
		"Detected '!maestro' prefix.",
		"user_id", post.UserId,
		"channel_id", post.ChannelId,
		"original_message", post.Message,
		"arguments_string", argumentsString,
	)

	agentName, numMessages, text, err := h.parseMaestroArgsNewFormat(argumentsString)
	// Test LogDebug call immediately after parseMaestroArgsNewFormat
	h.API.LogDebug("DEBUG: MessageHasBeenPosted: After parseMaestroArgsNewFormat", "agentName", agentName, "numMessages", numMessages, "message: ", text, "err", fmt.Sprintf("%v", err))
	if err != nil {
		h.API.SendEphemeralPost(post.UserId, &model.Post{
			ChannelId: post.ChannelId,
			Message:   err.Error(),
			RootId:    post.Id, // Thread the error message to the command
		})
		h.API.LogError("Failed to parse arguments for !maestro command", "error", err.Error(), "user_id", post.UserId, "arguments", argumentsString)
		return
	}

	if err := h.processMaestroTask(agentName, numMessages, post.ChannelId, post.UserId, post.Id); err != nil {
		// Add a simpler debug log to ensure it prints, using standard fmt.Println
		fmt.Printf("STANDARD_DEBUG: MessageHasBeenPosted: processMaestroTask returned non-nil error. Error: %#v\n", err)
		// These LogDebug calls might not be showing up, keeping them for now.
		h.API.LogDebug("DEBUG: MessageHasBeenPosted: Entered processMaestroTask error block.")
		h.API.LogDebug("DEBUG: MessageHasBeenPosted: err value is", "err_val_sprintf", fmt.Sprintf("%#v", err))
		h.API.LogDebug("DEBUG: MessageHasBeenPosted: err string is", "err_str", err.Error())
		h.API.LogError("Error processing !maestro task", "error", err.Error(), "user_id", post.UserId, "task_name", agentName)
	}
}

func (h *MaestroHandler) parseMaestroArgsNewFormat(argsString string) (agentName string, numMessages int, taskText string, err error) {
	h.API.LogInfo("Parsing Maestro arguments", "args", argsString)

	fields := strings.Fields(argsString)
	const DefaultNumMessages = 10
	numMessages = DefaultNumMessages

	taskTextStart := 0

	// Check for optional agent name
	if len(fields) > 0 && strings.HasPrefix(fields[0], "$") {
		agentName = fields[0][1:] // strip leading $
		taskTextStart = 1
	}

	// Check for optional -n <int>
	if len(fields) > taskTextStart+1 && fields[taskTextStart] == "-n" {
		val, convErr := strconv.Atoi(fields[taskTextStart+1])
		if convErr != nil {
			return "", 0, "", fmt.Errorf("invalid value for -n: '%s'. It must be an integer", fields[taskTextStart+1])
		}
		if val <= 0 {
			return "", 0, "", fmt.Errorf("invalid value for -n: %d. It must be a positive integer", val)
		}
		numMessages = val
		taskTextStart += 2
	}

	// Ensure there's task text remaining
	if len(fields) <= taskTextStart {
		return "", 0, "", fmt.Errorf("missing task text")
	}

	taskText = strings.Join(fields[taskTextStart:], " ")
	return agentName, numMessages, taskText, nil
}

func (h *MaestroHandler) processMaestroTask(agentName string, numMessages int, channelID string, userID string, rootID string) error {
	h.API.LogInfo("Processing Maestro task", "agentName", agentName)

	originalPost, appErr := h.API.GetPost(rootID)
	if appErr != nil {
		h.API.LogError("Failed to get original post", "post_id", rootID, "error", appErr.Error())
	}

	var threadRootID string
	if originalPost != nil && originalPost.RootId != "" {
		threadRootID = originalPost.RootId
	} else {
		threadRootID = rootID
	}

	h.API.LogInfo("Threading info", "originalPostId", rootID, "threadRootID", threadRootID)

	postList, appErr := h.API.GetPostsForChannel(channelID, 0, numMessages+10)
	if appErr != nil {
		h.API.LogDebug("DEBUG: processMaestroTask returning error after GetPostsForChannel failed", "error", appErr.Error())
		h.API.LogError("Failed to fetch posts for channel", "channel_id", channelID, "error", appErr.Error())
		errorPost := &model.Post{
			ChannelId: channelID,
			Message:   "An error occurred while fetching messages. Please try again later.",
			RootId:    threadRootID,
			UserId:    h.BotUserID,
		}
		_, createErr := h.API.CreatePost(errorPost)
		if createErr != nil {
			h.API.LogError("Failed to create error post", "error", createErr.Error())
		}
		return fmt.Errorf("failed to fetch posts: %w", appErr)
	}
	h.API.LogDebug("DEBUG: processMaestroTask: Fetched posts successfully")

	h.API.LogInfo("Posts fetched for channel", "channelID", channelID, "postListOrderLength", len(postList.Order), "postListPostsLength", len(postList.Posts))

	fetchedMessages := h.buildMessages(postList)

	for _, m := range fetchedMessages {
		h.API.LogInfo("Processing Maestro task", "Role", m.Role, "message", m.Content)
	}

	graphQLConversationID := "1"
	graphQLTenantID := "de"
	graphQLSystemChannelID := "ONEAPPWEB"

	config := h.GetConfig()
	endpoints := config.CustomEndpoints

	var found *CustomEndpoint

	for i, e := range endpoints {
		fmt.Printf("Iterating Agents: %d: %s -> %s\n", i, e.Name, e.Endpoint)
	}

	for i, e := range endpoints {
		if e.Name == agentName {
			found = &endpoints[i]
			break
		}
	}

	if found != nil {
		fmt.Printf("Found: %s -> %s\n", found.Name, found.Endpoint)
	} else {
		fmt.Println("Not found")
		found = &CustomEndpoint{"Maestro", config.MaestroURL}
	}

	webSocketURL := found.Endpoint
	// Assuming GraphQLPingIntervalSeconds is guaranteed to be non-nil due to defaulting in configuration.go
	pingIntervalSeconds := *config.GraphQLPingIntervalSeconds
	pingIntervalDuration := time.Duration(pingIntervalSeconds) * time.Second

	if webSocketURL == "" {
		h.API.LogDebug("DEBUG: processMaestroTask returning error because webSocketURL is empty")
		h.API.LogError("GraphQL Agent WebSocket URL is not configured.", "user_id", userID, "task_name", agentName)
		errorPost := &model.Post{
			ChannelId: channelID,
			Message:   "The GraphQL Agent WebSocket URL is not configured. Please contact an administrator.",
			RootId:    threadRootID,
			UserId:    h.BotUserID,
		}
		_, createErr := h.API.CreatePost(errorPost)
		if createErr != nil {
			h.API.LogError("Failed to create error post", "error", createErr.Error())
		}
		return fmt.Errorf("GraphQL Agent WebSocket URL is not configured")
	}
	h.API.LogDebug("DEBUG: processMaestroTask: WebSocket URL is configured", "url", webSocketURL)

	h.API.LogInfo("agent url is ", "webSocketURL", webSocketURL)

	// Create channels for continuous message processing
	messageChan := make(chan string, 10) // Buffered channel for messages
	errorChan := make(chan error, 5)     // Buffered channel for errors

	appCtx, _ := context.WithCancel(context.Background())

	// Start the GraphQL agent function in a goroutine
	go h.CallGraphQLAgentFunc(appCtx, found.Name, graphQLConversationID, userID, graphQLTenantID, graphQLSystemChannelID, fetchedMessages, webSocketURL, pingIntervalDuration, messageChan, errorChan)

	// Process messages continuously
	go h.processIncomingMessages(messageChan, errorChan, channelID, threadRootID, userID, agentName)

	return nil
}

// processIncomingMessages handles continuous message processing from GraphQL agent
func (h *MaestroHandler) processIncomingMessages(messageChan <-chan string, errorChan <-chan error, channelID, threadRootID, userID, taskName string) {
	h.API.LogInfo("Starting continuous message processing", "channelID", channelID, "userID", userID)

	for {
		select {
		case message, ok := <-messageChan:
			if !ok {
				h.API.LogInfo("Message channel closed, stopping message processing", "channelID", channelID, "userID", userID)
				return
			}

			if message == "" {
				h.API.LogDebug("Received empty message, skipping", "channelID", channelID)
				continue
			}

			h.API.LogInfo("Received message from GraphQL agent", "messageContent", message, "channelID", channelID)

			// Create a post for each message received
			responsePost := &model.Post{
				ChannelId: channelID,
				Message:   message,
				RootId:    threadRootID,
				UserId:    h.BotUserID,
			}

			_, createErr := h.API.CreatePost(responsePost)
			if createErr != nil {
				h.API.LogError("Failed to create response post", "error", createErr.Error(), "channelID", channelID)
				// Try to send as ephemeral message if regular post fails
				h.API.SendEphemeralPost(userID, &model.Post{
					ChannelId: channelID,
					Message:   "I processed your request but couldn't post the response. Here it is privately: " + message,
					RootId:    threadRootID,
				})
			} else {
				h.API.LogInfo("Successfully posted message to channel", "channelID", channelID, "messageLength", len(message))
			}

		case err, ok := <-errorChan:
			if !ok {
				h.API.LogInfo("Error channel closed, stopping message processing", "channelID", channelID, "userID", userID)
				return
			}

			h.API.LogError("Error from GraphQL Agent", "error", err.Error(), "user_id", userID, "task_name", taskName)

			// Post error message to channel
			//errorPost := &model.Post{
			//	ChannelId: channelID,
			//	Message:   "An error occurred while processing your request: " + err.Error(),
			//	RootId:    threadRootID,
			//	UserId:    h.BotUserID,
			//}

			//_, createErr := h.API.CreatePost(errorPost)
			//if createErr != nil {
			//h.API.LogError("Failed to create error post", "error", createErr.Error())
			// Send as ephemeral if regular post fails
			h.API.SendEphemeralPost(userID, &model.Post{
				ChannelId: channelID,
				Message:   "An error occurred: " + err.Error(),
				RootId:    threadRootID,
			})
			//}
		}
	}
}

func (h *MaestroHandler) buildMessages(posts *model.PostList) []Message {
	var messages []Message
	for _, postId := range posts.Order {
		role := "user"
		post := posts.Posts[postId]
		content := post.Message
		if post.UserId == h.BotUserID {
			role = "assistant"
		} else {
			// Attributing message to user ID, could be replaced with display name if available/preferred
			content = fmt.Sprintf("%s: %s", post.UserId, post.Message)
		}

		rand.Seed(time.Now().UnixNano()) // Seed random number generator

		message := Message{
			Role:    role,
			Content: content,
			Format:  "text",                          // Set Format to "text"
			TurnID:  strconv.Itoa(rand.Intn(100000)), // Generate random TurnID
		}
		messages = append(messages, message)
	}

	// Reverse messages to have the latest ones last.
	// This is important because GetPostsForChannel returns posts in descending order of CreateAt (newest first).
	// For a conversation history, we typically want oldest first, newest last.
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	return messages
}
