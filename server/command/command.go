package command

import (
	"fmt"
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
		AutoCompleteDesc: "Enter your prompt for OpenAI.",
		AutoCompleteHint: "[your prompt]",
		AutocompleteData: model.NewAutocompleteData(openaiCommandTrigger, "[your prompt]", "Enter your prompt for OpenAI."),
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

func (h *Handler) executeOpenAICommand(args *model.CommandArgs) (*model.CommandResponse, error) {
	prompt := strings.TrimSpace(strings.TrimPrefix(args.Command, "/"+openaiCommandTrigger))

	if prompt == "" {
		h.dependencies.API.SendEphemeralPost(args.UserId, &model.Post{
			ChannelId: args.ChannelId,
			Message:   "Please provide a prompt after the `/maestro` command.",
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

	openAIResponse, err := h.dependencies.CallOpenAIFunc(apiKey, prompt, h.dependencies.OpenAIAPIURL)
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
