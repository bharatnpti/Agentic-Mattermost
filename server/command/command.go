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
// Simplified to only include what's necessary for the remaining 'hello' command.
type HandlerDependencies struct {
	API       plugin.API
	BotUserID string // BotUserID might still be useful for logging or future simple commands
}

type Handler struct {
	dependencies HandlerDependencies
}

// Command interface defines how the plugin interacts with command handlers.
type Command interface {
	Handle(args *model.CommandArgs) (*model.CommandResponse, error)
	// executeHelloCommand removed as it's an internal implementation detail
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

// executeHelloCommand is now a private method of Handler.
func (h *Handler) executeHelloCommand(args *model.CommandArgs) *model.CommandResponse {
	// Example of using a dependency: Log the action.
	// h.dependencies.API.LogInfo("Executing hello command", "user_id", args.UserId, "command", args.Command)

	commandFields := strings.Fields(args.Command)
	if len(commandFields) < 2 { // Ensure there's a second part for the username
		return &model.CommandResponse{
			ResponseType: model.CommandResponseTypeEphemeral,
			Text:         "Please specify a username to say hello to.",
		}
	}
	username := commandFields[1]
	return &model.CommandResponse{
		ResponseType: model.CommandResponseTypeInChannel,
		Text:         "Hello, " + username + "!",
	}
}

func (h *Handler) Handle(args *model.CommandArgs) (*model.CommandResponse, error) {
	trigger := strings.TrimPrefix(strings.Fields(args.Command)[0], "/")
	switch trigger {
	case helloCommandTrigger:
		return h.executeHelloCommand(args), nil
	// Case for openaiCommandTrigger removed
	default:
		// This default case should ideally not be reached if Mattermost only sends registered commands.
		// However, it's a good safeguard.
		h.dependencies.API.LogWarn("Received unknown command trigger", "trigger", trigger, "full_command", args.Command)
		return &model.CommandResponse{
			ResponseType: model.CommandResponseTypeEphemeral,
			Text:         fmt.Sprintf("Unknown command: %s. Currently, only '/%s' is supported.", args.Command, helloCommandTrigger),
		}, nil
	}
}

// executeOpenAICommand function has been removed.
