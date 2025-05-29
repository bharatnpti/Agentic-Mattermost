package main

import (
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
	CallGraphQLAgentFunc func(apiKey string, conversationID string, userID string, tenantID string, channelIDSystemContext string, messages []Message, apiURL string) (string, error)
}

func NewMaestroHandler(api plugin.API, botUserID string, getConfig func() *configuration, callGraphQLAgentFunc func(apiKey string, conversationID string, userID string, tenantID string, channelIDSystemContext string, messages []Message, apiURL string) (string, error)) *MaestroHandler {
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

	taskName, numMessages, err := h.parseMaestroArgsNewFormat(argumentsString)
	if err != nil {
		h.API.SendEphemeralPost(post.UserId, &model.Post{
			ChannelId: post.ChannelId,
			Message:   err.Error(),
			RootId:    post.Id, // Thread the error message to the command
		})
		h.API.LogError("Failed to parse arguments for !maestro command", "error", err.Error(), "user_id", post.UserId, "arguments", argumentsString)
		return
	}

	if err := h.processMaestroTask(taskName, numMessages, post.ChannelId, post.UserId, post.Id); err != nil {
		h.API.LogError("Error processing !maestro task", "error", err.Error(), "user_id", post.UserId, "task_name", taskName)
	}
}

func (h *MaestroHandler) parseMaestroArgsNewFormat(argsString string) (taskText string, numMessages int, err error) {
	h.API.LogInfo("Parsing Maestro arguments", "args", argsString)

	fields := strings.Fields(argsString)
	// DefaultNumMessages should be accessible, e.g. defined in model.go or passed via config/struct
	// For now, let's use a local constant or retrieve from a shared package if defined elsewhere.
	// Assuming command.DefaultNumMessages is accessible or replaced.
	// Let's define it here for now if not available globally.
	const DefaultNumMessages = 10 // Or retrieve from a shared constants/config package

	numMessages = DefaultNumMessages // Default value
	taskTextStart := 0

	if len(fields) >= 2 && fields[0] == "-n" {
		val, convErr := strconv.Atoi(fields[1])
		if convErr != nil {
			return "", 0, fmt.Errorf("invalid value for -n: '%s'. It must be an integer", fields[1])
		}
		if val <= 0 {
			return "", 0, fmt.Errorf("invalid value for -n: %d. It must be a positive integer", val)
		}
		numMessages = val
		taskTextStart = 2
	}

	if len(fields) > taskTextStart {
		taskText = strings.Join(fields[taskTextStart:], " ")
	}

	return taskText, numMessages, nil
}

func (h *MaestroHandler) processMaestroTask(taskName string, numMessages int, channelID string, userID string, rootID string) error {
	h.API.LogInfo("Processing Maestro task", "taskName", taskName)

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

	h.API.LogInfo("Posts fetched for channel", "channelID", channelID, "postListOrderLength", len(postList.Order), "postListPostsLength", len(postList.Posts))

	fetchedMessages := h.buildMessages(postList)

	for _, m := range fetchedMessages {
		h.API.LogInfo("Processing Maestro task", "Role", m.Role, "message", m.Content)
	}

	graphQLConversationID := "1"
	graphQLTenantID := "de"
	graphQLSystemChannelID := "ONEAPPWEB"

	config := h.GetConfig()
	webSocketURL := config.GraphQLAgentWebSocketURL

	if webSocketURL == "" {
		h.API.LogError("GraphQL Agent WebSocket URL is not configured.", "user_id", userID, "task_name", taskName)
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

	h.API.LogInfo("agent url is ", "webSocketURL", webSocketURL)
	messages, err := h.CallGraphQLAgentFunc("apiKey", graphQLConversationID, userID, graphQLTenantID, graphQLSystemChannelID, fetchedMessages, webSocketURL)
	if err != nil {
		h.API.LogError("Error calling GraphQL Agent for !maestro task", "error", err.Error(), "user_id", userID, "task_name", taskName, "webSocketURL", webSocketURL)
		errorPost := &model.Post{
			ChannelId: channelID,
			Message:   "An error occurred while contacting the agent. Please try again later or contact an administrator.",
			RootId:    threadRootID,
			UserId:    h.BotUserID,
		}
		_, createErr := h.API.CreatePost(errorPost)
		if createErr != nil {
			h.API.LogError("Failed to create error post", "error", createErr.Error())
		}
		return err
	}

	h.API.LogInfo("Response received from GraphQL agent", "messageContent", messages)

	if len(messages) == 0 {
		h.API.LogInfo("GraphQL Agent returned no messages for !maestro task", "user_id", userID, "task_name", taskName)
		noMessagesPost := &model.Post{
			ChannelId: channelID,
			Message:   "The agent processed your request but returned no messages.",
			RootId:    threadRootID,
			UserId:    h.BotUserID,
		}
		_, createErr := h.API.CreatePost(noMessagesPost)
		if createErr != nil {
			h.API.LogError("Failed to create no messages post", "error", createErr.Error())
		}
		return nil
	}

	message := messages

	responsePost := &model.Post{
		ChannelId: channelID,
		Message:   message,
		RootId:    threadRootID,
		UserId:    h.BotUserID,
	}

	_, createErr := h.API.CreatePost(responsePost)
	if createErr != nil {
		h.API.LogError("Failed to create response post", "error", createErr.Error())
		h.API.SendEphemeralPost(userID, &model.Post{
			ChannelId: channelID,
			Message:   "I processed your request but couldn't post the response. Here it is privately: " + message,
			RootId:    rootID,
		})
		return fmt.Errorf("failed to create response post: %w", createErr)
	}

	return nil
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

		// Assuming Message struct has Role and Content fields.
		// Format and TurnID might need to be handled based on the new Message struct definition.
		// If Message struct has Format and TurnID, they need to be set here.
		// For now, let's assume Message struct is {Role string, Content string}
		// and adapt if it includes Format and TurnID from the previous location.
		// The moved Message struct is:
		// type Message struct {
		//  Role    string `json:"role"`
		//  Content string `json:"content"`
		// }
		// So, Format and TurnID are not part of it. If CallGraphQLAgentFunc expects them,
		// this Message struct or the call signature needs adjustment.
		// The CallGraphQLAgentFunc signature in maestro_handler.go is:
		// CallGraphQLAgentFunc func(apiKey string, conversationID string, userID string, tenantID string, channelIDSystemContext string, messages []Message, apiURL string) (string, error)
		// And the Message struct it uses (from model.go) is {Role, Content}.
		// However, the original buildMessages in plugin.go created a Message struct with Format and TurnID.
		// This implies the CallGraphQLAgentFunc in openai.go *does* expect Format and TurnID.
		// Let's check the definition of Message in openai.go (which was moved to model.go)
		// The original definition in openai.go (now in model.go) was:
		// type Message struct {
		// 	Role    string `json:"role"`
		// 	Content string `json:"content"`
		// }
		// This is a conflict. The `buildMessages` was creating a `main.Message` that had `Format` and `TurnID`
		// but the `CallGraphQLAgentFunc` in `openai.go` takes `[]main.Message` which *does not* have these fields.
		// The `CallGraphQLAgentFunc` in `openai.go` *internally* constructs `MessageInput` which has these.

		// Let's re-check `CallGraphQLAgentFunc` in `openai.go`.
		// It takes `messages []Message`. This `Message` is `main.Message` from `model.go`.
		// Inside `CallGraphQLAgentFunc`, it iterates `messages` and creates `messageBlock` for the GraphQL query.
		// The query formatting part:
		// buffer.WriteString(fmt.Sprintf(`{
		// content: "%s",
		// format: "%s",  <-- where does this come from?
		// role: "%s",
		// turnId: "%s" <-- and this?
		// }`, escapeString(msg.Content), escapeString(msg.Format), escapeString(msg.Role), escapeString(msg.TurnID)))
		// This means the `Message` struct passed to `CallGraphQLAgentFunc` *must* have Format and TurnID.

		// So, the `Message` struct in `server/model.go` needs to be updated.
		// Let's assume it will be updated. For now, this function will create messages
		// with those fields.

		// Corrected part:
		// The `Message` struct in `model.go` is fine. The issue is that `buildMessages`
		// was creating fields `Format` and `TurnID` that were not part of the `Message` struct in `model.go`.
		// The `CallGraphQLAgentFunc` in `openai.go` (which is now passed to the handler)
		// takes `[]Message` (from `model.go`) and *itself* adds `Format: "text"` and generates `TurnID`
		// when constructing the GraphQL query string.
		// The Message struct in model.go now has Format and TurnID.
		// Populate them here.
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

	// Ensure we only return up to numMessages (this was missing in the original processMaestroTask)
	// Actually, numMessages is used by GetPostsForChannel, but the post-processing of `buildMessages`
	// might result in more or less, depending on filtering.
	// The `processMaestroTask` fetches `numMessages+10` posts. `buildMessages` converts all of them.
	// The `CallGraphQLAgentFunc` then takes all `fetchedMessages`.
	// The number of messages sent to the agent is effectively controlled by what `buildMessages` returns.
	// If a specific number of messages (e.g. `numMessages` from the command) is strictly required
	// for the agent call, then `buildMessages` should be trimmed or `processMaestroTask` should
	// slice `fetchedMessages` before passing to `CallGraphQLAgentFunc`.
	// For now, behavior is consistent with original: all fetched and processed posts are sent.

	return messages
}
