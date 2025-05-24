package command

import (
	"fmt"
	_ "strconv"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin" // For plugin.API
	// Removed direct import of "github.com/mattermost/mattermost-plugin-starter-template/server/main"
)

// HandlerDependencies defines the dependencies needed by the command Handler from the main plugin.
type HandlerDependencies struct {
	API             plugin.API
	BotUserID       string
	GetOpenAIAPIKey func() string                                                         // Kept for now, in case other commands might use it
	CallOpenAIFunc  func(apiKey string, message string, apiURL string) (string, error)    // Kept for now
	OpenAIAPIURL    string                                                                // Kept for now
	ParseArguments  func(argsString string) (taskName string, numMessages int, err error) // Kept for now
}

const DefaultNumMessages = 10 // Exported for use in plugin.go

type Handler struct {
	dependencies HandlerDependencies
}

// Command interface now only needs to list methods used by remaining commands.
type Command interface {
	Handle(args *model.CommandArgs) (*model.CommandResponse, error)
	executeHelloCommand(args *model.CommandArgs) *model.CommandResponse
	// executeOpenAICommand removed
}

const helloCommandTrigger = "hello"

// const openaiCommandTrigger = "maestro" // Removed

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

	// Registration for openaiCommandTrigger removed

	return &Handler{
		dependencies: deps,
	}
}

func (h *Handler) Handle(args *model.CommandArgs) (*model.CommandResponse, error) {
	trigger := strings.TrimPrefix(strings.Fields(args.Command)[0], "/")
	switch trigger {
	case helloCommandTrigger:
		return h.executeHelloCommand(args), nil
	// Case for openaiCommandTrigger removed
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

// executeOpenAICommand function has been removed.
