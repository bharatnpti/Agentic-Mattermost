package command

import (
	"bytes" // Added for template execution
	"fmt"
	"strconv"
	"strings"
	"text/template" // Added for template parsing

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
)

// HandlerDependencies defines the dependencies needed by the command Handler from the main plugin.
type HandlerDependencies struct {
	API             plugin.API
	BotUserID       string
	GetOpenAIAPIKey func() string
	GetOpenAIModel  func() string
	GetOpenAITasks  func() map[string]string
	CallOpenAIFunc  func(apiKey string, modelName string, message string, apiURL string) (string, error)
	OpenAIAPIURL    string
}

type Handler struct {
	dependencies HandlerDependencies
}

type Command interface {
	Handle(args *model.CommandArgs) (*model.CommandResponse, error)
	executeHelloCommand(args *model.CommandArgs) *model.CommandResponse
	executeMaestroCommand(args *model.CommandArgs) (*model.CommandResponse, error)
}

const helloCommandTrigger = "hello"
const maestroCommandTrigger = "maestro"

func NewCommandHandler(deps HandlerDependencies) Command {
	if err := deps.API.RegisterCommand(&model.Command{
		Trigger:          helloCommandTrigger,
		DisplayName:      "Hello Command",
		Description:      "A simple hello command.",
		AutoComplete:     true,
		AutoCompleteDesc: "Say hello to someone",
		AutoCompleteHint: "[@username]",
		AutocompleteData: model.NewAutocompleteData(helloCommandTrigger, "[@username]", "Username to say hello to"),
	}); err != nil {
		deps.API.LogError("Failed to register hello command", "error", err)
	}

	if err := deps.API.RegisterCommand(&model.Command{
		Trigger:          maestroCommandTrigger,
		DisplayName:      "Maestro AI Task",
		Description:      "Executes a configured AI task with context from chat messages.",
		AutoComplete:     true,
		AutoCompleteDesc: "Executes a Maestro AI task.",
		AutoCompleteHint: "[task_name] [num_messages]",
		AutocompleteData: model.NewAutocompleteData(maestroCommandTrigger, "[task_name] [num_messages]", "Executes a Maestro AI task. Usage: /maestro <task_name> <num_messages>"),
	}); err != nil {
		deps.API.LogError("Failed to register maestro command", "error", err)
	}

	return &Handler{
		dependencies: deps,
	}
}

func (h *Handler) Handle(args *model.CommandArgs) (*model.CommandResponse, error) {
	trigger := strings.TrimPrefix(strings.Fields(args.Command)[0], "/")
	switch trigger {
	case helloCommandTrigger:
		return h.executeHelloCommand(args), nil
	case maestroCommandTrigger:
		_, err := h.executeMaestroCommand(args)
		if err != nil {
			h.dependencies.API.SendEphemeralPost(args.UserId, &model.Post{
				ChannelId: args.ChannelId,
				Message:   "An unexpected error occurred while executing the Maestro command.",
			})
			// It's generally better for executeMaestroCommand to handle its own ephemeral error messages for specific failures.
			// If err is still not nil here, it implies a more fundamental issue that wasn't caught by executeMaestroCommand's error handling.
		}
		// Return an empty response to make the command visible, as executeMaestroCommand sends its own posts.
		return &model.CommandResponse{ResponseType: model.CommandResponseTypeInChannel, SkipSlackParsing: true}, nil
	default:
		return &model.CommandResponse{
			ResponseType: model.CommandResponseTypeEphemeral,
			Text:         fmt.Sprintf("Unknown command: %s", args.Command),
		}, nil
	}
}

func (h *Handler) executeHelloCommand(args *model.CommandArgs) *model.CommandResponse {
	if len(strings.Fields(args.Command)) < 2 {
		return &model.CommandResponse{
			ResponseType: model.CommandResponseTypeEphemeral,
			Text:         "Please specify a username",
		}
	}
	username := strings.Fields(args.Command)[1]
	return &model.CommandResponse{
		ResponseType: model.CommandResponseTypeInChannel,
		Text:         "Hello, " + username,
	}
}

func (h *Handler) executeMaestroCommand(args *model.CommandArgs) (*model.CommandResponse, error) {
	commandArgsText := strings.TrimSpace(strings.TrimPrefix(args.Command, "/"+maestroCommandTrigger))
	parts := strings.Fields(commandArgsText)

	if len(parts) != 2 {
		h.dependencies.API.SendEphemeralPost(args.UserId, &model.Post{
			ChannelId: args.ChannelId,
			Message:   "Usage: /maestro <task_name> <num_messages>",
		})
		return &model.CommandResponse{}, nil
	}

	taskName := parts[0]
	numMessagesStr := parts[1]
	numMessages, errConv := strconv.Atoi(numMessagesStr)
	if errConv != nil || numMessages <= 0 {
		h.dependencies.API.SendEphemeralPost(args.UserId, &model.Post{
			ChannelId: args.ChannelId,
			Message:   "Number of messages must be a positive integer.",
		})
		return &model.CommandResponse{}, nil
	}

	ackPost := &model.Post{
		ChannelId: args.ChannelId,
		Message:   "_Maestro is processing your request..._",
	}
	h.dependencies.API.SendEphemeralPost(args.UserId, ackPost)

	apiKey := h.dependencies.GetOpenAIAPIKey()
	if apiKey == "" {
		h.dependencies.API.SendEphemeralPost(args.UserId, &model.Post{
			ChannelId: args.ChannelId,
			Message:   "The OpenAI API Key is not configured. Please contact your system administrator.",
		})
		return &model.CommandResponse{}, nil
	}

	tasks := h.dependencies.GetOpenAITasks()
	promptTemplate, taskExists := tasks[taskName]
	if !taskExists {
		h.dependencies.API.SendEphemeralPost(args.UserId, &model.Post{
			ChannelId: args.ChannelId,
			Message:   fmt.Sprintf("Task '%s' not found in configuration.", taskName),
		})
		return &model.CommandResponse{}, nil
	}

	postList, appErr := h.dependencies.API.GetPostsForChannel(args.ChannelId, 0, numMessages)
	if appErr != nil {
		h.dependencies.API.LogError("Failed to fetch posts for channel", "channel_id", args.ChannelId, "error", appErr.Error())
		h.dependencies.API.SendEphemeralPost(args.UserId, &model.Post{
			ChannelId: args.ChannelId,
			Message:   "Error fetching message history. Please try again.",
		})
		return &model.CommandResponse{}, nil
	}

	if postList == nil || len(postList.Order) == 0 {
		h.dependencies.API.SendEphemeralPost(args.UserId, &model.Post{
			ChannelId: args.ChannelId,
			Message:   "No messages found to process for the task.",
		})
		return &model.CommandResponse{}, nil
	}

	h.dependencies.API.LogInfo("Task name specified", "task", taskName)

	orderedPosts := make([]*model.Post, 0, len(postList.Order))
	for _, postID := range postList.Order {
		if post, ok := postList.Posts[postID]; ok {
			if post.UserId == args.UserId && strings.HasPrefix(post.Message, args.Command) {
				continue
			}
			if strings.HasPrefix(post.Type, "system_") {
				continue
			}
			orderedPosts = append(orderedPosts, post)
		}
	}
	for i, j := 0, len(orderedPosts)-1; i < j; i, j = i+1, j-1 {
		orderedPosts[i], orderedPosts[j] = orderedPosts[j], orderedPosts[i]
	}

	var formattedMessages []string
	for _, post := range orderedPosts {
		formattedMessages = append(formattedMessages, fmt.Sprintf("User %s: %s", post.UserId, post.Message))
	}
	concatenatedMessages := strings.Join(formattedMessages, "\n")

	// Placeholder Replacement using text/template
	tmpl, errTmplParse := template.New("taskPrompt").Parse(promptTemplate)
	if errTmplParse != nil {
		h.dependencies.API.LogError("Error parsing prompt template", "task", taskName, "error", errTmplParse.Error())
		h.dependencies.API.SendEphemeralPost(args.UserId, &model.Post{
			ChannelId: args.ChannelId,
			Message:   fmt.Sprintf("Error parsing prompt template for task '%s'. Please check plugin configuration.", taskName),
		})
		return &model.CommandResponse{}, nil
	}

	data := struct{ Messages string }{Messages: concatenatedMessages}
	var processedPrompt bytes.Buffer
	if errTmplExec := tmpl.Execute(&processedPrompt, data); errTmplExec != nil {
		h.dependencies.API.LogError("Error executing prompt template", "task", taskName, "error", errTmplExec.Error())
		h.dependencies.API.SendEphemeralPost(args.UserId, &model.Post{
			ChannelId: args.ChannelId,
			Message:   fmt.Sprintf("Error executing prompt template for task '%s'.", taskName),
		})
		return &model.CommandResponse{}, nil
	}
	finalPrompt := processedPrompt.String()

	modelName := h.dependencies.GetOpenAIModel()
	if modelName == "" {
		modelName = "gpt-3.5-turbo"
		h.dependencies.API.LogWarn("OpenAI Model not configured, using default.", "model", modelName)
	}

	openAIResponse, errOpenAI := h.dependencies.CallOpenAIFunc(apiKey, modelName, finalPrompt, h.dependencies.OpenAIAPIURL)
	if errOpenAI != nil {
		h.dependencies.API.LogError("Error calling OpenAI API for slash command", "error", errOpenAI.Error(), "model", modelName)
		h.dependencies.API.SendEphemeralPost(args.UserId, &model.Post{
			ChannelId: args.ChannelId,
			Message:   "An error occurred while contacting OpenAI. Please try again later or contact an administrator.",
		})
		return &model.CommandResponse{}, nil
	}

	responsePost := &model.Post{
		UserId:    h.dependencies.BotUserID,
		ChannelId: args.ChannelId,
		Message:   openAIResponse,
		RootId:    "",
	}

	if _, appErrCreate := h.dependencies.API.CreatePost(responsePost); appErrCreate != nil {
		h.dependencies.API.LogError("Failed to post OpenAI response for slash command", "error", appErrCreate.Error())
		h.dependencies.API.SendEphemeralPost(args.UserId, &model.Post{
			ChannelId: args.ChannelId,
			Message:   "An error occurred while trying to post the OpenAI response.",
		})
		return &model.CommandResponse{}, nil
	}

	return &model.CommandResponse{}, nil
}
