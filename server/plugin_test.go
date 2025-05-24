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
	defaultSettings := &configuration{
		OpenAIAPIKey: "default-key",
		OpenAIModel:  "default-model",
		Tasks:        `{"default_task": "Default prompt: {{.Messages}}"}`,
		ParsedTasks:  map[string]string{"default_task": "Default prompt: {{.Messages}}"},
	}

	t.Run("OnConfigurationChange loads all settings and parses valid Tasks JSON", func(t *testing.T) {
		p := &Plugin{}
		api := &plugintest.API{}
		p.SetAPI(api)

		validTasksJSON := `{"task1": "Prompt for task1: {{.Messages}}", "task2": "Prompt for task2: {{.Messages}}"}`
		expectedParsedTasks := map[string]string{
			"task1": "Prompt for task1: {{.Messages}}",
			"task2": "Prompt for task2: {{.Messages}}",
		}

		configToLoad := &configuration{
			OpenAIAPIKey: "loaded-test-key",
			OpenAIModel:  "loaded-gpt-4",
			Tasks:        validTasksJSON,
			// ParsedTasks will be set by OnConfigurationChange
		}

		api.On("LoadPluginConfiguration", mock.AnythingOfType("*main.configuration")).Run(func(args mock.Arguments) {
			configPtr := args.Get(0).(*configuration)
			// Simulate LoadPluginConfiguration populating the fields
			configPtr.OpenAIAPIKey = configToLoad.OpenAIAPIKey
			configPtr.OpenAIModel = configToLoad.OpenAIModel
			configPtr.Tasks = configToLoad.Tasks
		}).Return(nil).Once()

		err := p.OnConfigurationChange()
		require.NoError(t, err)

		loadedConfig := p.getConfiguration()
		assert.Equal(t, configToLoad.OpenAIAPIKey, loadedConfig.OpenAIAPIKey)
		assert.Equal(t, configToLoad.OpenAIModel, loadedConfig.OpenAIModel)
		assert.Equal(t, configToLoad.Tasks, loadedConfig.Tasks) // Raw string
		assert.Equal(t, expectedParsedTasks, loadedConfig.ParsedTasks) // Parsed map
		api.AssertExpectations(t)
	})

	t.Run("OnConfigurationChange handles malformed Tasks JSON", func(t *testing.T) {
		p := &Plugin{}
		api := &plugintest.API{}
		p.SetAPI(api)
		// Set a valid initial configuration to see if it's wiped or preserved (current logic wipes to empty)
		p.setConfiguration(defaultSettings)


		malformedTasksJSON := `{"task1": "Prompt for task1: {{.Messages}}",` // Missing closing brace

		configToLoad := &configuration{
			OpenAIAPIKey: "another-key",
			OpenAIModel:  "another-model",
			Tasks:        malformedTasksJSON,
		}
		
		api.On("LoadPluginConfiguration", mock.AnythingOfType("*main.configuration")).Run(func(args mock.Arguments) {
			configPtr := args.Get(0).(*configuration)
			configPtr.OpenAIAPIKey = configToLoad.OpenAIAPIKey
			configPtr.OpenAIModel = configToLoad.OpenAIModel
			configPtr.Tasks = configToLoad.Tasks
		}).Return(nil).Once()

		api.On("LogError", "Failed to parse Tasks JSON string from configuration.", "error", mock.AnythingOfType("string"), "tasks_string", malformedTasksJSON).Once()

		err := p.OnConfigurationChange()
		require.NoError(t, err) // OnConfigurationChange itself doesn't return parsing error

		loadedConfig := p.getConfiguration()
		assert.Equal(t, "another-key", loadedConfig.OpenAIAPIKey) // Other fields should still load
		assert.Equal(t, "another-model", loadedConfig.OpenAIModel)
		assert.Equal(t, malformedTasksJSON, loadedConfig.Tasks)
		assert.NotNil(t, loadedConfig.ParsedTasks, "ParsedTasks should be an empty map, not nil")
		assert.Empty(t, loadedConfig.ParsedTasks, "ParsedTasks should be empty due to parsing error")
		api.AssertExpectations(t)
	})

	t.Run("OnConfigurationChange handles empty Tasks JSON string", func(t *testing.T) {
		p := &Plugin{}
		api := &plugintest.API{}
		p.SetAPI(api)

		configToLoad := &configuration{
			OpenAIAPIKey: "empty-tasks-key",
			OpenAIModel:  "empty-tasks-model",
			Tasks:        "", // Empty Tasks string
		}

		api.On("LoadPluginConfiguration", mock.AnythingOfType("*main.configuration")).Run(func(args mock.Arguments) {
			configPtr := args.Get(0).(*configuration)
			configPtr.OpenAIAPIKey = configToLoad.OpenAIAPIKey
			configPtr.OpenAIModel = configToLoad.OpenAIModel
			configPtr.Tasks = configToLoad.Tasks
		}).Return(nil).Once()
		// No LogError expected for empty Tasks string

		err := p.OnConfigurationChange()
		require.NoError(t, err)

		loadedConfig := p.getConfiguration()
		assert.Equal(t, "empty-tasks-key", loadedConfig.OpenAIAPIKey)
		assert.Equal(t, "empty-tasks-model", loadedConfig.OpenAIModel)
		assert.Equal(t, "", loadedConfig.Tasks)
		assert.NotNil(t, loadedConfig.ParsedTasks, "ParsedTasks should be an empty map, not nil")
		assert.Empty(t, loadedConfig.ParsedTasks, "ParsedTasks should be empty for empty Tasks string")
		api.AssertExpectations(t)
	})


	t.Run("getConfiguration returns default empty struct when no config initially", func(t *testing.T) {
		p := &Plugin{}
		// p.configuration is nil by default
		conf := p.getConfiguration()
		require.NotNil(t, conf, "getConfiguration should not return nil")
		assert.Empty(t, conf.OpenAIAPIKey, "OpenAIAPIKey should be empty for default config")
		assert.Empty(t, conf.OpenAIModel, "OpenAIModel should be empty for default config")
		assert.Empty(t, conf.Tasks, "Tasks string should be empty for default config")
		assert.NotNil(t, conf.ParsedTasks, "ParsedTasks should be an empty map, not nil, for default config")
		assert.Empty(t, conf.ParsedTasks, "ParsedTasks should be empty for default config")
	})
}
