package main

import (
	"errors" // Import for creating specific errors
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestMessageHasBeenPosted(t *testing.T) {
	botUserID := "testbotuserid"
	originalCallOpenAIAPIFunc := CallOpenAIAPIFunc // Store original

	setupPlugin := func(t *testing.T, apiKey string) (*Plugin, *plugintest.API) {
		p := &Plugin{
			botUserID: botUserID,
		}
		api := &plugintest.API{}

		conf := &configuration{OpenAIAPIKey: apiKey}
		p.setConfiguration(conf)
		p.SetAPI(api)
		return p, api
	}

	// Restore original function after each test in this suite
	defer func() { CallOpenAIAPIFunc = originalCallOpenAIAPIFunc }()

	t.Run("TestSuccessfulResponse", func(t *testing.T) {
		p, api := setupPlugin(t, "test-api-key")
		defer api.AssertExpectations(t)

		post := &model.Post{UserId: "testuserid", ChannelId: "testchannelid", Message: "Hello OpenAI"}
		expectedResponse := "OpenAI says hello!"

		CallOpenAIAPIFunc = func(apiKey, message, apiURL string) (string, error) {
			assert.Equal(t, "test-api-key", apiKey)
			assert.Equal(t, "Hello OpenAI", message)
			assert.Equal(t, OpenAIAPIURL, apiURL)
			return expectedResponse, nil
		}

		api.On("LogInfo", "MessageHasBeenPosted hook triggered", "user", post.UserId, "message", post.Message).Times(1)
		api.On("LogInfo", "OpenAI API response", "response", expectedResponse).Times(1)
		api.On("CreatePost", mock.MatchedBy(func(newPost *model.Post) bool {
			return newPost.UserId == botUserID &&
				newPost.ChannelId == post.ChannelId &&
				newPost.Message == expectedResponse
		})).Return(nil, nil).Times(1) // Return (nil, nil) for (*model.Post, *model.AppError)

		p.MessageHasBeenPosted(&plugin.Context{}, post)
	})

	t.Run("TestBotMessageIgnored", func(t *testing.T) {
		p, _ := setupPlugin(t, "test-api-key") // API mock not strictly needed here but setup is harmless

		post := &model.Post{UserId: botUserID, ChannelId: "testchannelid", Message: "I am a bot"}

		CallOpenAIAPIFunc = func(apiKey, message, apiURL string) (string, error) {
			t.Error("CallOpenAIAPIFunc should not be called for bot messages")
			return "", errors.New("should not be called")
		}

		p.MessageHasBeenPosted(&plugin.Context{}, post)
		// No calls to LogInfo, LogError, CreatePost are expected beyond initial trigger if any.
		// Assertions are implicitly checked by AssertExpectations if any unexpected mock calls were made to api.
	})

	t.Run("TestAPIKeyMissing", func(t *testing.T) {
		p, api := setupPlugin(t, "") // API Key is empty
		defer api.AssertExpectations(t)

		post := &model.Post{UserId: "testuserid", ChannelId: "testchannelid", Message: "Hello OpenAI"}

		api.On("LogInfo", "MessageHasBeenPosted hook triggered", "user", post.UserId, "message", post.Message).Times(1)
		api.On("LogError", "OpenAI API Key is not configured.").Times(1)

		CallOpenAIAPIFunc = func(apiKey, message, apiURL string) (string, error) {
			t.Error("CallOpenAIAPIFunc should not be called when API key is missing")
			return "", errors.New("should not be called")
		}

		p.MessageHasBeenPosted(&plugin.Context{}, post)
	})

	t.Run("TestOpenAICallFailure", func(t *testing.T) {
		p, api := setupPlugin(t, "test-api-key")
		defer api.AssertExpectations(t)

		post := &model.Post{UserId: "testuserid", ChannelId: "testchannelid", Message: "Hello OpenAI"}
		simulatedError := errors.New("simulated OpenAI API error")

		CallOpenAIAPIFunc = func(apiKey, message, apiURL string) (string, error) {
			return "", simulatedError
		}

		api.On("LogInfo", "MessageHasBeenPosted hook triggered", "user", post.UserId, "message", post.Message).Times(1)
		api.On("LogError", "Error calling OpenAI API", "error", simulatedError.Error()).Times(1)

		p.MessageHasBeenPosted(&plugin.Context{}, post)
	})
}

func TestConfigurationLoading(t *testing.T) {
	t.Run("OnConfigurationChange loads API key", func(t *testing.T) {
		p := &Plugin{}
		api := &plugintest.API{}
		p.SetAPI(api)

		expectedConfig := &configuration{
			OpenAIAPIKey: "loaded-test-key",
		}

		api.On("LoadPluginConfiguration", mock.AnythingOfType("*main.configuration")).Run(func(args mock.Arguments) {
			configPtr := args.Get(0).(*configuration)
			*configPtr = *expectedConfig
		}).Return(nil).Times(1)

		err := p.OnConfigurationChange()
		require.NoError(t, err)

		loadedConfig := p.getConfiguration()
		assert.Equal(t, expectedConfig.OpenAIAPIKey, loadedConfig.OpenAIAPIKey)

		api.AssertExpectations(t)
	})

	t.Run("getConfiguration returns empty struct when no config", func(t *testing.T) {
		p := &Plugin{}
		// p.configuration is nil by default
		conf := p.getConfiguration()
		require.NotNil(t, conf, "getConfiguration should not return nil")
		assert.Empty(t, conf.OpenAIAPIKey, "OpenAIAPIKey should be empty for default config")
	})
}
