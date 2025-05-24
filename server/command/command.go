package command

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin" // For plugin.API
	// Removed direct import of "github.com/mattermost/mattermost-plugin-starter-template/server/main"
)

// HandlerDependencies defines the dependencies needed by the command Handler from the main plugin.
type HandlerDependencies struct {
	API             plugin.API
	BotUserID       string
	GetOpenAIAPIKey func() string
	CallOpenAIFunc  func(apiKey string, message string, apiURL string) (string, error)
	OpenAIAPIURL    string
}

const defaultNumMessages = 10

type Handler struct {
	dependencies HandlerDependencies
}

type Command interface {
	Handle(args *model.CommandArgs) (*model.CommandResponse, error)
	executeHelloCommand(args *model.CommandArgs) *model.CommandResponse
	executeOpenAICommand(args *model.CommandArgs) (*model.CommandResponse, error)
}

const helloCommandTrigger = "hello"
const openaiCommandTrigger = "maestro"

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
		Trigger:          openaiCommandTrigger,
		DisplayName:      "OpenAI Prompt",
		Description:      "Sends a prompt to OpenAI and returns the response.",
		AutoComplete:     true,
		AutoCompleteDesc: "Run a task with context from channel messages. Specify task and optionally number of messages. Ex: summarize -n 10",
		AutoCompleteHint: "<task_name> [-n num_messages]",
		AutocompleteData: model.NewAutocompleteData(openaiCommandTrigger, "<task_name> [-n num_messages]", "Run a task with context from channel messages. Ex: summarize -n 10"),
	}); err != nil {
		deps.API.LogError("Failed to register openai command", "error", err)
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
	case openaiCommandTrigger:
		return h.executeOpenAICommand(args)
	default:
		return &model.CommandResponse{
			ResponseType: model.CommandResponseTypeEphemeral,
			Text:         fmt.Sprintf("Unknown command: %s", args.Command),
		}, nil
	}
}

func (h *Handler) executeHelloCommand(args *model.CommandArgs) *model.CommandResponse {

	//h.dependencies.API.LogInfo("executing: executeHelloCommand with " + args.UserId + ", command: " + args.Command + ", channel: " + args.ChannelId)

	if len(strings.Fields(args.Command)) < 2 {
		return &model.CommandResponse{
			ResponseType: model.CommandResponseTypeEphemeral,
			Text:         "Please specify a username",
		}
	}
	username := strings.Fields(args.Command)[1]
	return &model.CommandResponse{
		ResponseType: model.CommandResponseTypeInChannel, // Explicitly set response type
		Text:         "Hello, " + username,
	}
}

func parseOpenAICommandArgs(command string) (taskName string, numMessages int, err error) {
	fields := strings.Fields(command)
	if len(fields) < 2 { // Expect at least /maestro <task_name>
		return "", 0, fmt.Errorf("not enough arguments. Usage: /%s <task_name> [-n <num_messages>]", openaiCommandTrigger)
	}

	numMessages = defaultNumMessages // Default value
	taskNameParts := []string{}
	taskNameEndIndex := -1

	// Iterate through fields to find task_name and -n parameter
	for i := 1; i < len(fields); i++ { // Start from index 1 (after /maestro)
		if fields[i] == "-n" {
			taskNameEndIndex = i - 1 // Task name ends before -n
			if i+1 < len(fields) {
				val, convErr := strconv.Atoi(fields[i+1])
				if convErr != nil {
					return "", 0, fmt.Errorf("invalid value for -n: '%s'. It must be an integer", fields[i+1])
				}
				if val <= 0 {
					return "", 0, fmt.Errorf("invalid value for -n: %d. It must be a positive integer", val)
				}
				numMessages = val
				i++ // Skip the value part of -n
			} else {
				return "", 0, fmt.Errorf("missing value for -n argument")
			}
		} else if strings.HasPrefix(fields[i], "-") && taskNameEndIndex != -1 {
			// If we have already parsed -n (i.e. taskNameEndIndex is set), another flag is an error
			return "", 0, fmt.Errorf("unknown or misplaced flag: %s. Flags like -n must come after the task name", fields[i])
		} else if taskNameEndIndex == -1 { // Still collecting task name, or expecting -n
			if strings.HasPrefix(fields[i], "-") && fields[i] != "-n" { // It's a flag, but not -n, and we are expecting task name or -n
				return "", 0, fmt.Errorf("unknown or misplaced flag: %s. Only -n is a recognized flag", fields[i])
			}
			taskNameParts = append(taskNameParts, fields[i]) // Append if part of task name
		} else {
			// This case means we have arguments after -n <value> which are not allowed (and not flags handled above)
			return "", 0, fmt.Errorf("unexpected argument: %s. No arguments are allowed after -n <value>", fields[i])
		}
	}

	if len(taskNameParts) == 0 {
		return "", 0, fmt.Errorf("task_name cannot be empty. Usage: /%s <task_name> [-n <num_messages>]", openaiCommandTrigger)
	}
	taskName = strings.Join(taskNameParts, " ")

	return taskName, numMessages, nil
}

func (h *Handler) executeOpenAICommand(args *model.CommandArgs) (*model.CommandResponse, error) {
	taskName, numMessages, err := parseOpenAICommandArgs(args.Command)
	if err != nil {
		h.dependencies.API.SendEphemeralPost(args.UserId, &model.Post{
			ChannelId: args.ChannelId,
			Message:   err.Error(),
		})
		return &model.CommandResponse{}, nil
	}

	if taskName == "" {
		h.dependencies.API.SendEphemeralPost(args.UserId, &model.Post{
			ChannelId: args.ChannelId,
			Message:   "Please provide a task name after the `/maestro` command. Usage: /maestro <task_name> [-n <num_messages>]",
		})
		return &model.CommandResponse{}, nil
	}

	apiKey := h.dependencies.GetOpenAIAPIKey()

	if apiKey == "" {
		h.dependencies.API.SendEphemeralPost(args.UserId, &model.Post{
			ChannelId: args.ChannelId,
			Message:   "The OpenAI API Key is not configured. Please contact your system administrator.",
		})
		return &model.CommandResponse{}, nil
	}

	// Fetch messages
	postList, appErr := h.dependencies.API.GetPostsForChannel(args.ChannelId, 0, numMessages)
	if appErr != nil {
		h.dependencies.API.LogError("Failed to fetch posts for channel", "channel_id", args.ChannelId, "error", appErr.Error())
		h.dependencies.API.SendEphemeralPost(args.UserId, &model.Post{
			ChannelId: args.ChannelId,
			Message:   "An error occurred while fetching messages. Please try again later.",
		})
		return &model.CommandResponse{}, nil
	}

	var fetchedMessages []string
	// Posts are returned newest first, so we reverse them to get chronological order
	for i := len(postList.Posts) - 1; i >= 0; i-- {
		post := postList.Posts[postList.Order[i]]
		// Skip posts from the bot itself to avoid loops and also skip current command post
		// args.PostId is not available in this version of model.CommandArgs.
		// strings.HasPrefix should generally catch the command post.
		if post.UserId == h.dependencies.BotUserID || strings.HasPrefix(post.Message, "/"+openaiCommandTrigger) {
			continue
		}
		fetchedMessages = append(fetchedMessages, post.Message)
	}

	if len(fetchedMessages) == 0 && taskName != "summarize" { // Allow summarize to work with no messages (though it will be trivial)
		// If no messages are fetched (e.g. all messages by bot or current command), and it's not a summarize task
		// It might be better to inform the user if the task is not summarize.
		// For summarize, an empty summary is a valid response.
		// For other tasks, it might indicate an issue or misuse.
		// However, the current logic will proceed with an empty message list for OpenAI.
		// This behavior can be refined based on desired product behavior.
		// For now, we let it proceed. OpenAI will receive an empty message list for context.
	}

	messagesString := strings.Join(fetchedMessages, "\n")
	var finalPrompt string
	if strings.ToLower(taskName) == "summarize" {
		finalPrompt = fmt.Sprintf("Summarize the following messages:\n%s", messagesString)
	} else {
		finalPrompt = fmt.Sprintf("User query: %s\n%s", taskName, messagesString)
	}

	openAIResponse, err := h.dependencies.CallOpenAIFunc(apiKey, finalPrompt, h.dependencies.OpenAIAPIURL)
	if err != nil {
		h.dependencies.API.LogError("Error calling OpenAI API for slash command", "error", err.Error())
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
	}

	if _, appErr := h.dependencies.API.CreatePost(responsePost); appErr != nil {
		h.dependencies.API.LogError("Failed to post OpenAI response for slash command", "error", appErr.Error())
		h.dependencies.API.SendEphemeralPost(args.UserId, &model.Post{
			ChannelId: args.ChannelId,
			Message:   "An error occurred while trying to post the OpenAI response.",
		})
		return &model.CommandResponse{}, nil
	}

	return &model.CommandResponse{}, nil
}
