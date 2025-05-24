package command

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

const testBotUserIDGlobal = "testbotuserid"
const testOpenAIAPIURLGlobal = "https://api.openai.com/v1/chat/completions"

// mockCallOpenAIFuncGlobal is a shared mock that can be set by tests.
var mockCallOpenAIFuncGlobal func(apiKey string, message string, apiURL string) (string, error)

func TestHelloCommand(t *testing.T) {
	mockAPI := &plugintest.API{}
	// Expect RegisterCommand to be called for "hello" and "openai"
	mockAPI.On("RegisterCommand", mock.MatchedBy(func(cmd *model.Command) bool { return cmd.Trigger == helloCommandTrigger })).Return(nil).Once()
	mockAPI.On("RegisterCommand", mock.MatchedBy(func(cmd *model.Command) bool { return cmd.Trigger == openaiCommandTrigger })).Return(nil).Once()

	deps := HandlerDependencies{
		API:       mockAPI,
		BotUserID: testBotUserIDGlobal,
		// Other dependencies are not used by hello command, so default/nil is fine.
	}
	handler := NewCommandHandler(deps)

	args := &model.CommandArgs{
		Command:   "/hello world",
		UserId:    "testuserid",
		ChannelId: "testchannelid",
	}

	response, err := handler.Handle(args)
	assert.Nil(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, "Hello, world", response.Text)
	assert.Equal(t, model.CommandResponseTypeInChannel, response.ResponseType)
	mockAPI.AssertExpectations(t)
}

func TestOpenAICommand(t *testing.T) {
	// Store and restore the global mock function to ensure test isolation
	originalGlobalMock := mockCallOpenAIFuncGlobal
	defer func() { mockCallOpenAIFuncGlobal = originalGlobalMock }()

	baseArgs := &model.CommandArgs{
		UserId:    "testuserid",
		ChannelId: "testchannelid",
	}

	t.Run("TestValidCommand", func(t *testing.T) {
		mockAPI := &plugintest.API{}
		mockAPI.On("RegisterCommand", mock.AnythingOfType("*model.Command")).Return(nil).Times(2)

		expectedJoke := "Why did the chicken cross the road?"
		mockCallOpenAIFuncGlobal = func(apiKey, message, apiURL string) (string, error) {
			assert.Equal(t, "test-api-key", apiKey)
			assert.Equal(t, "tell me a joke", message)
			assert.Equal(t, testOpenAIAPIURLGlobal, apiURL)
			return expectedJoke, nil
		}
		defer func() { mockCallOpenAIFuncGlobal = nil }()

		deps := HandlerDependencies{
			API:             mockAPI,
			BotUserID:       testBotUserIDGlobal,
			GetOpenAIAPIKey: func() string { return "test-api-key" },
			CallOpenAIFunc:  func(apiKey, msg, url string) (string, error) { return mockCallOpenAIFuncGlobal(apiKey, msg, url) },
			OpenAIAPIURL:    testOpenAIAPIURLGlobal,
		}
		handler := NewCommandHandler(deps)
		args := *baseArgs // copy
		args.Command = "/maestro tell me a joke"

		mockAPI.On("CreatePost", mock.MatchedBy(func(post *model.Post) bool {
			return post.UserId == testBotUserIDGlobal &&
				post.ChannelId == args.ChannelId &&
				post.Message == expectedJoke
		})).Return(nil, nil).Once()

		_, err := handler.Handle(&args)
		assert.Nil(t, err)
		mockAPI.AssertExpectations(t)
	})

	t.Run("TestEmptyPrompt", func(t *testing.T) {
		mockAPI := &plugintest.API{}
		mockAPI.On("RegisterCommand", mock.AnythingOfType("*model.Command")).Return(nil).Times(2)

		mockCallOpenAIFuncGlobal = func(apiKey, message, apiURL string) (string, error) {
			t.Errorf("CallOpenAIFunc should not be called for empty prompt")
			return "", errors.New("should not be called")
		}
		defer func() { mockCallOpenAIFuncGlobal = nil }()

		deps := HandlerDependencies{
			API:             mockAPI,
			BotUserID:       testBotUserIDGlobal,                     // Not strictly needed but good to have
			GetOpenAIAPIKey: func() string { return "test-api-key" }, // Should not be called
			CallOpenAIFunc:  func(apiKey, msg, url string) (string, error) { return mockCallOpenAIFuncGlobal(apiKey, msg, url) },
			OpenAIAPIURL:    testOpenAIAPIURLGlobal,
		}
		handler := NewCommandHandler(deps)
		args := *baseArgs // copy
		args.Command = "/maestro "

		mockAPI.On("SendEphemeralPost", args.UserId, mock.MatchedBy(func(post *model.Post) bool {
			return post.ChannelId == args.ChannelId &&
				strings.Contains(post.Message, "Please provide a prompt")
		})).Return(nil).Once() // Changed from *model.Post to nil, as SendEphemeralPost returns *model.Post

		_, err := handler.Handle(&args)
		assert.Nil(t, err)
		mockAPI.AssertExpectations(t)
	})

	t.Run("TestMissingAPIKey", func(t *testing.T) {
		mockAPI := &plugintest.API{}
		mockAPI.On("RegisterCommand", mock.AnythingOfType("*model.Command")).Return(nil).Times(2)

		mockCallOpenAIFuncGlobal = func(apiKey, message, apiURL string) (string, error) {
			t.Errorf("CallOpenAIFunc should not be called when API key is missing")
			return "", errors.New("should not be called")
		}
		defer func() { mockCallOpenAIFuncGlobal = nil }()

		deps := HandlerDependencies{
			API:             mockAPI,
			BotUserID:       testBotUserIDGlobal,
			GetOpenAIAPIKey: func() string { return "" }, // Key is missing
			CallOpenAIFunc:  func(apiKey, msg, url string) (string, error) { return mockCallOpenAIFuncGlobal(apiKey, msg, url) },
			OpenAIAPIURL:    testOpenAIAPIURLGlobal,
		}
		handler := NewCommandHandler(deps)
		args := *baseArgs // copy
		args.Command = "/maestro tell me a joke"

		mockAPI.On("SendEphemeralPost", args.UserId, mock.MatchedBy(func(post *model.Post) bool {
			return post.ChannelId == args.ChannelId &&
				strings.Contains(post.Message, "API Key is not configured")
		})).Return(nil).Once() // Changed from *model.Post to nil

		_, err := handler.Handle(&args)
		assert.Nil(t, err)
		mockAPI.AssertExpectations(t)
	})

	t.Run("TestOpenAICallError", func(t *testing.T) {
		mockAPI := &plugintest.API{}
		mockAPI.On("RegisterCommand", mock.AnythingOfType("*model.Command")).Return(nil).Times(2)

		mockError := errors.New("OpenAI API error")
		mockCallOpenAIFuncGlobal = func(apiKey, message, apiURL string) (string, error) {
			return "", mockError
		}
		defer func() { mockCallOpenAIFuncGlobal = nil }()

		deps := HandlerDependencies{
			API:             mockAPI,
			BotUserID:       testBotUserIDGlobal,
			GetOpenAIAPIKey: func() string { return "test-api-key" },
			CallOpenAIFunc:  func(apiKey, msg, url string) (string, error) { return mockCallOpenAIFuncGlobal(apiKey, msg, url) },
			OpenAIAPIURL:    testOpenAIAPIURLGlobal,
		}
		handler := NewCommandHandler(deps)
		args := *baseArgs // copy
		args.Command = "/maestro tell me a joke"

		mockAPI.On("LogError", "Error calling OpenAI API for slash command", "error", mockError.Error()).Once()
		mockAPI.On("SendEphemeralPost", args.UserId, mock.MatchedBy(func(post *model.Post) bool {
			return post.ChannelId == args.ChannelId &&
				strings.Contains(post.Message, "error occurred while contacting OpenAI")
		})).Return(nil).Once() // Changed from *model.Post to nil

		_, err := handler.Handle(&args)
		assert.Nil(t, err)
		mockAPI.AssertExpectations(t)
	})

	t.Run("TestCreatePostError", func(t *testing.T) {
		mockAPI := &plugintest.API{}
		mockAPI.On("RegisterCommand", mock.AnythingOfType("*model.Command")).Return(nil).Times(2)

		expectedJoke := "Why did the chicken cross the road?"
		mockCallOpenAIFuncGlobal = func(apiKey, message, apiURL string) (string, error) {
			return expectedJoke, nil
		}
		defer func() { mockCallOpenAIFuncGlobal = nil }()

		deps := HandlerDependencies{
			API:             mockAPI,
			BotUserID:       testBotUserIDGlobal,
			GetOpenAIAPIKey: func() string { return "test-api-key" },
			CallOpenAIFunc:  func(apiKey, msg, url string) (string, error) { return mockCallOpenAIFuncGlobal(apiKey, msg, url) },
			OpenAIAPIURL:    testOpenAIAPIURLGlobal,
		}
		handler := NewCommandHandler(deps)
		args := *baseArgs // copy
		args.Command = "/maestro tell me a joke"

		mockAppError := model.NewAppError("CreatePost", "id", nil, "failed to create post", http.StatusInternalServerError)
		mockAPI.On("CreatePost", mock.MatchedBy(func(post *model.Post) bool {
			return post.Message == expectedJoke
		})).Return(nil, mockAppError).Once()

		mockAPI.On("LogError", "Failed to post OpenAI response for slash command", "error", mockAppError.Error()).Once()
		mockAPI.On("SendEphemeralPost", args.UserId, mock.MatchedBy(func(post *model.Post) bool {
			return post.ChannelId == args.ChannelId &&
				strings.Contains(post.Message, "error occurred while trying to post the OpenAI response")
		})).Return(nil).Once() // Changed from *model.Post to nil

		_, err := handler.Handle(&args)
		assert.Nil(t, err)
		mockAPI.AssertExpectations(t)
	})
}
