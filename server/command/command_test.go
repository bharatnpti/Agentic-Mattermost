package command

import (
	"net/http"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestExecuteHelloCommand(t *testing.T) {
	apiMock := &plugintest.API{}
	// For LogInfo, if used within executeHelloCommand (currently it's not, but good practice)
	apiMock.On("LogInfo", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Maybe()

	deps := HandlerDependencies{
		API:       apiMock,
		BotUserID: "botid",
	}
	handler := &Handler{dependencies: deps}

	tests := []struct {
		name                 string
		commandArgs          *model.CommandArgs
		expectedResponseType string
		expectedText         string
	}{
		{
			name: "Valid username",
			commandArgs: &model.CommandArgs{
				Command: "/hello @testuser",
			},
			expectedResponseType: model.CommandResponseTypeInChannel,
			expectedText:         "Hello, @testuser!",
		},
		{
			name: "No username provided",
			commandArgs: &model.CommandArgs{
				Command: "/hello",
			},
			expectedResponseType: model.CommandResponseTypeEphemeral,
			expectedText:         "Please specify a username to say hello to.",
		},
		{
			name: "Username with extra spaces in command",
			commandArgs: &model.CommandArgs{
				Command: "/hello   @user.name  ",
			},
			expectedResponseType: model.CommandResponseTypeInChannel,
			expectedText:         "Hello, @user.name!",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := handler.executeHelloCommand(tt.commandArgs)
			assert.Equal(t, tt.expectedResponseType, response.ResponseType)
			assert.Equal(t, tt.expectedText, response.Text)
		})
	}
	apiMock.AssertExpectations(t)
}

func TestHandle_HelloCommand(t *testing.T) {
	apiMock := &plugintest.API{}
	// Mock RegisterCommand if it were called within Handle or New, but it's in NewCommandHandler
	// Mock LogInfo if executeHelloCommand were to use it
	apiMock.On("LogInfo", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Maybe()


	deps := HandlerDependencies{
		API:       apiMock,
		BotUserID: "botid",
	}
	// Note: NewCommandHandler also registers the command. For Handle tests,
	// we assume registration has happened or test the handler logic directly.
	// Here, we are testing the Handler's Handle method, not NewCommandHandler.
	handler := &Handler{dependencies: deps}

	commandArgs := &model.CommandArgs{
		Command: "/hello @testuser",
	}

	response, err := handler.Handle(commandArgs)
	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, model.CommandResponseTypeInChannel, response.ResponseType)
	assert.Equal(t, "Hello, @testuser!", response.Text)
	apiMock.AssertExpectations(t)
}

func TestHandle_UnknownCommand(t *testing.T) {
	apiMock := &plugintest.API{}
	// Adjusted to match the actual call: LogWarn(message, key1, value1, key2, value2)
	// Actual call from command.go: LogWarn("Received unknown command trigger", "trigger", triggerValue, "full_command", commandArgsValue)
	// For this test, triggerValue is "unknowncommand" and commandArgsValue is "/unknowncommand arg1"
	apiMock.On("LogWarn", "Received unknown command trigger", "trigger", "unknowncommand", "full_command", "/unknowncommand arg1").Once()


	deps := HandlerDependencies{
		API:       apiMock,
		BotUserID: "botid",
	}
	handler := &Handler{dependencies: deps}

	commandArgs := &model.CommandArgs{
		Command: "/unknowncommand arg1",
	}

	response, err := handler.Handle(commandArgs)
	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, model.CommandResponseTypeEphemeral, response.ResponseType)
	assert.Contains(t, response.Text, "Unknown command: /unknowncommand arg1")
	assert.Contains(t, response.Text, "Currently, only '/hello' and '/agent' are supported.")
	apiMock.AssertExpectations(t)
}

func TestNewCommandHandler(t *testing.T) {
	apiMock := &plugintest.API{}

	// Mock the RegisterCommand call
	apiMock.On("RegisterCommand", mock.MatchedBy(func(cmd *model.Command) bool {
		return cmd.Trigger == "hello"
	})).Return(nil).Once() // Expect it to be called once for 'hello'

	// Expect a call for the 'agent' command
	apiMock.On("RegisterCommand", mock.MatchedBy(func(cmd *model.Command) bool {
		return cmd.Trigger == agentCommandTrigger &&
			cmd.DisplayName == "Agent Command" &&
			cmd.Description == "Interact with the agent." &&
			cmd.AutoCompleteDesc == "Available commands: resume" &&
			cmd.AutoCompleteHint == "[subcommand]"
	})).Return(nil).Once()

	deps := HandlerDependencies{
		API:       apiMock,
		BotUserID: "botid",
	}

	commandHandler := NewCommandHandler(deps)
	assert.NotNil(t, commandHandler)

	// Verify that RegisterCommand was called
	apiMock.AssertExpectations(t)
}

func TestNewCommandHandler_RegisterCommand_Error(t *testing.T) {
	apiMock := &plugintest.API{}

	// Mock the RegisterCommand call to return an error for the first command (hello)
	expectedErr := model.NewAppError("RegisterCommand", "some.id", nil, "failed to register", http.StatusInternalServerError)
	apiMock.On("RegisterCommand", mock.MatchedBy(func(cmd *model.Command) bool {
		return cmd.Trigger == "hello"
	})).Return(expectedErr).Once()
	apiMock.On("LogError", "Failed to register hello command", "error", expectedErr).Once()

	// Also mock the second call (agent), assuming it could also fail or succeed.
	// For this test, let's assume it succeeds to isolate the error logging for the first one.
	// If we wanted to test error on the second, we'd make this one return error and the first nil.
	apiMock.On("RegisterCommand", mock.MatchedBy(func(cmd *model.Command) bool {
		return cmd.Trigger == agentCommandTrigger
	})).Return(nil).Once()


	deps := HandlerDependencies{
		API:       apiMock,
		BotUserID: "botid",
	}

	commandHandler := NewCommandHandler(deps)
	assert.NotNil(t, commandHandler) // Handler is still returned, error is logged

	apiMock.AssertExpectations(t)
}
// End of file
