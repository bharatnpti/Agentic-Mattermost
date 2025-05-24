package main

import (
	"testing" // "errors" import removed as it's unused after test cleanup

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
	defer func() { CallOpenAIAPIFunc = originalCallOpenAIAPIFunc }() // Restore original function

	t.Run("TestBotMessageIgnored", func(t *testing.T) {
		p, api := setupPlugin(t, "test-api-key") // API key doesn't matter here
		defer api.AssertExpectations(t)

		post := &model.Post{UserId: botUserID, ChannelId: "testchannelid", Message: "I am a bot"}

		// CallOpenAIAPIFunc should not be involved at all.
		// LogInfo for the message details should NOT be called if it's a bot message.
		// No LogError or CreatePost should be called.

		p.MessageHasBeenPosted(&plugin.Context{}, post)
		// No mocks should be called, asserted by AssertExpectations.
	})

	t.Run("TestRegularMessageIsLogged", func(t *testing.T) {
		p, api := setupPlugin(t, "test-api-key") // API key doesn't matter here
		defer api.AssertExpectations(t)
		
		post := &model.Post{UserId: "regularUserID", ChannelId: "testchannelid", Message: "A regular message"}

		// Expect LogInfo to be called with the message details
		api.On("LogInfo", 
			"MessageHasBeenPosted hook triggered (OpenAI call removed)", 
			"user_id", post.UserId, 
			"message", post.Message, 
			"channel_id", post.ChannelId,
		).Times(1)
		
		// CallOpenAIAPIFunc should not be involved.
		// No LogError or CreatePost should be called.

		p.MessageHasBeenPosted(&plugin.Context{}, post)
	})

	// TestSuccessfulResponse, TestAPIKeyMissing, TestOpenAICallFailure are removed
	// as MessageHasBeenPosted no longer handles OpenAI calls directly.
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
