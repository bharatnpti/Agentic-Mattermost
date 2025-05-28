package main

//
//import (
//	"errors"
//	"fmt"
//	"net/http" // Added import
//	"strings"
//	"testing"
//
//	"github.com/mattermost/mattermost/server/public/model"
//	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
//	"github.com/stretchr/testify/assert"
//	"github.com/stretchr/testify/mock"
//)
//
//func TestProcessMaestroTask(t *testing.T) {
//	originalCallGraphQLAgentFunc := CallGraphQLAgentFunc
//	defer func() { CallGraphQLAgentFunc = originalCallGraphQLAgentFunc }()
//
//	defaultTestTaskName := "Test task"
//	defaultNumMessages := 5
//	defaultChannelID := "testChannelID"
//	defaultUserID := "testUserID"
//	defaultRootID := "testRootID"
//	defaultBotUserID := "testBotID"
//	defaultAPIKey := "test_key"
//
//	setupAPI := func() *plugintest.API {
//		api := &plugintest.API{}
//		// Default mocks - can be overridden by specific tests
//		api.On("GetConfiguration").Return(&model.Config{LogSettings: model.LogSettings{EnableFile: new(bool)}}, nil).Maybe() // Part of p.getConfiguration()
//		api.On("GetConfig").Return(&model.Config{LogSettings: model.LogSettings{EnableFile: new(bool)}}).Maybe()             // If API.GetConfig() is directly used
//
//		api.On("LogError", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.Anything, mock.AnythingOfType("string"), mock.Anything, mock.AnythingOfType("string"), mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(nil).Maybe()
//		api.On("LogError", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(nil).Maybe()
//		api.On("LogError", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.Anything).Return(nil).Maybe() // Simpler LogError
//		api.On("LogError", mock.Anything).Return(nil).Maybe()                                                              // Simplest LogError
//
//		// Default GetPostsForChannel mock
//		mockPostList := &model.PostList{}
//		mockPostList.AddPost(&model.Post{Id: "post_before_1", UserId: "another_user", Message: "Some previous message", CreateAt: model.GetMillis() - 1000})
//		mockPostList.AddPost(&model.Post{Id: "post_before_2", UserId: "another_user", Message: "Another previous message", CreateAt: model.GetMillis() - 2000})
//		mockPostList.AddOrder("post_before_1")
//		mockPostList.AddOrder("post_before_2")
//		api.On("GetPostsForChannel", defaultChannelID, 0, defaultNumMessages+10).Return(mockPostList, nil).Maybe()
//		return api
//	}
//
//	setupPlugin := func(api *plugintest.API, apiKey string) *Plugin {
//		p := &Plugin{}
//		p.SetAPI(api)
//		// Simulate the configuration being set, as getConfiguration() reads from p.configuration
//		p.setConfiguration(&configuration{OpenAIAPIKey: apiKey})
//		p.botUserID = defaultBotUserID
//		return p
//	}
//
//	t.Run("Success - Single Message from Agent", func(t *testing.T) {
//		api := setupAPI()
//		p := setupPlugin(api, defaultAPIKey)
//
//		// Setup mock for this specific test
//		CallGraphQLAgentFunc = func(apiKey string, conversationID string, userID string, tenantID string, channelID string, userMessage string, apiURL string) ([]MessageOutput, error) {
//			assert.Equal(t, defaultAPIKey, apiKey)
//			assert.Contains(t, userMessage, defaultTestTaskName) // Check that taskName is in prompt
//			assert.Equal(t, defaultUserID, userID)
//			// apiURL is now a parameter, if you need to assert its value (e.g. against a mock server URL) do it here
//			return []MessageOutput{{Content: "Agent response 1", Role: "agent", Format: "text", TurnID: "turn1"}}, nil
//		}
//
//		api.On("SendEphemeralPost", defaultUserID, mock.MatchedBy(func(post *model.Post) bool {
//			return post.ChannelId == defaultChannelID &&
//				post.Message == "Agent response 1" &&
//				post.RootId == defaultRootID
//		})).Return(&model.Post{}).Once() // Expect it to be called once
//
//		err := p.processMaestroTask(defaultTestTaskName, defaultNumMessages, defaultChannelID, defaultUserID, defaultRootID)
//
//		assert.NoError(t, err)
//		api.AssertExpectations(t)
//	})
//
//	t.Run("Success - Multiple Messages from Agent", func(t *testing.T) {
//		api := setupAPI()
//		p := setupPlugin(api, defaultAPIKey)
//
//		// Setup mock for this specific test
//		expectedMessages := []MessageOutput{
//			{Content: "Agent response 1", Role: "agent", Format: "text", TurnID: "turn1"},
//			{Content: "Agent response 2", Role: "agent", Format: "markdown", TurnID: "turn2"},
//		}
//
//		CallGraphQLAgentFunc = func(apiKey string, conversationID string, userID string, tenantID string, channelID string, userMessage string, apiURL string) ([]MessageOutput, error) {
//			assert.Equal(t, defaultAPIKey, apiKey)
//			return expectedMessages, nil
//		}
//
//		// Expect SendEphemeralPost to be called for each message
//		api.On("SendEphemeralPost", defaultUserID, mock.MatchedBy(func(post *model.Post) bool {
//			return post.ChannelId == defaultChannelID && post.Message == "Agent response 1" && post.RootId == defaultRootID
//		})).Return(&model.Post{}).Once()
//		api.On("SendEphemeralPost", defaultUserID, mock.MatchedBy(func(post *model.Post) bool {
//			return post.ChannelId == defaultChannelID && post.Message == "Agent response 2" && post.RootId == defaultRootID
//		})).Return(&model.Post{}).Once()
//
//		err := p.processMaestroTask(defaultTestTaskName, defaultNumMessages, defaultChannelID, defaultUserID, defaultRootID)
//
//		assert.NoError(t, err)
//		api.AssertExpectations(t)
//	})
//
//	t.Run("Success - summarize task", func(t *testing.T) {
//		api := setupAPI()
//		p := setupPlugin(api, defaultAPIKey)
//		summarizeTaskName := "summarize"
//
//		// Reset mocks for this specific test to ensure clean behavior
//		api.Mock = mock.Mock{}
//		// p.getConfiguration() is called to get the API key. If p.configuration is set (which it is by setupPlugin),
//		// no API calls like GetConfiguration() or GetConfig() are made by p.getConfiguration().
//		// So, no mock for GetConfiguration() is needed here if p.configuration is pre-set.
//
//		// Specific GetPostsForChannel mock for this test case
//		specificMockPostList := &model.PostList{}
//		post2 := &model.Post{Id: "p2", Message: "Second message.", UserId: "otherUser", CreateAt: 2000, ChannelId: defaultChannelID}
//		post1 := &model.Post{Id: "p1", Message: "First message to summarize.", UserId: "otherUser", CreateAt: 1000, ChannelId: defaultChannelID}
//		specificMockPostList.AddPost(post2) // Newest
//		specificMockPostList.AddPost(post1) // Older
//		specificMockPostList.AddOrder(post2.Id)
//		specificMockPostList.AddOrder(post1.Id)
//		api.On("GetPostsForChannel", defaultChannelID, 0, defaultNumMessages+10).Return(specificMockPostList, nil).Once()
//
//		// Setup mock for CallGraphQLAgentFunc for this specific test
//		CallGraphQLAgentFunc = func(apiKey string, conversationID string, userID string, tenantID string, channelID string, userMessage string, apiURL string) ([]MessageOutput, error) {
//			assert.Equal(t, defaultAPIKey, apiKey)
//			expectedPrompt := fmt.Sprintf("Summarize the following messages:\n%s\n%s", "First message to summarize.", "Second message.")
//			assert.Equal(t, expectedPrompt, userMessage, "The prompt for summarization is not as expected.")
//			return []MessageOutput{{Content: "Summary response", Role: "agent"}}, nil
//		}
//
//		// Expected SendEphemeralPost call for the agent's response
//		api.On("SendEphemeralPost", defaultUserID, mock.MatchedBy(func(post *model.Post) bool {
//			return post.Message == "Summary response" && post.ChannelId == defaultChannelID && post.RootId == defaultRootID
//		})).Return(&model.Post{}).Once()
//
//		err := p.processMaestroTask(summarizeTaskName, defaultNumMessages, defaultChannelID, defaultUserID, defaultRootID)
//
//		assert.NoError(t, err)
//		api.AssertExpectations(t)
//	})
//
//	t.Run("Error - API Key Not Configured", func(t *testing.T) {
//		api := setupAPI()
//		p := setupPlugin(api, "") // Empty API Key
//
//		// This test should not call CallGraphQLAgentFunc
//		CallGraphQLAgentFunc = func(apiKey string, conversationID string, userID string, tenantID string, channelID string, userMessage string, apiURL string) ([]MessageOutput, error) {
//			t.Fatalf("CallGraphQLAgentFunc was not expected to be called in %s", t.Name())
//			return nil, errors.New("unexpected call")
//		}
//
//		api.On("SendEphemeralPost", defaultUserID, mock.MatchedBy(func(post *model.Post) bool {
//			return post.ChannelId == defaultChannelID &&
//				strings.Contains(post.Message, "API Key is not configured") &&
//				post.RootId == defaultRootID
//		})).Return(&model.Post{}).Once()
//
//		err := p.processMaestroTask(defaultTestTaskName, defaultNumMessages, defaultChannelID, defaultUserID, defaultRootID)
//
//		assert.Error(t, err)
//		assert.Contains(t, err.Error(), "OpenAI API Key is not configured")
//		api.AssertExpectations(t)
//		// LogError should not be called here as it's a configuration issue known before API calls
//	})
//
//	t.Run("Error - GetPostsForChannel Fails", func(t *testing.T) {
//		api := setupAPI()
//		p := setupPlugin(api, defaultAPIKey)
//
//		// Reset mocks for this specific test
//		api.Mock = mock.Mock{}
//		// p.getConfiguration() is called. If p.configuration is set, no API call is made by it.
//		// So, no mock for GetConfiguration() is needed here.
//
//		// This test should not call CallGraphQLAgentFunc
//		CallGraphQLAgentFunc = func(apiKey string, conversationID string, userID string, tenantID string, channelID string, userMessage string, apiURL string) ([]MessageOutput, error) {
//			t.Fatalf("CallGraphQLAgentFunc was not expected to be called in %s", t.Name())
//			return nil, errors.New("unexpected call")
//		}
//
//		// Specific mock for GetPostsForChannel to return an error
//		appErr := model.NewAppError("GetPostsForChannel", "some.error.id", nil, "failed to get posts", http.StatusInternalServerError)
//		api.On("GetPostsForChannel", defaultChannelID, 0, defaultNumMessages+10).Return(nil, appErr).Once()
//
//		// Expected LogError call - matching the string form of the error
//		api.On("LogError", "Failed to fetch posts for channel", "channel_id", defaultChannelID, "error", appErr.Error()).Return(nil).Once()
//
//		// Expected SendEphemeralPost call to inform the user
//		api.On("SendEphemeralPost", defaultUserID, mock.MatchedBy(func(post *model.Post) bool {
//			return post.ChannelId == defaultChannelID &&
//				strings.Contains(post.Message, "An error occurred while fetching messages") &&
//				post.RootId == defaultRootID
//		})).Return(&model.Post{}).Once()
//
//		err := p.processMaestroTask(defaultTestTaskName, defaultNumMessages, defaultChannelID, defaultUserID, defaultRootID)
//
//		assert.Error(t, err)
//		assert.Contains(t, err.Error(), "failed to fetch posts")
//		api.AssertExpectations(t)
//	})
//
//	t.Run("Error - CallGraphQLAgentFunc Returns Error", func(t *testing.T) {
//		api := setupAPI()
//		p := setupPlugin(api, defaultAPIKey)
//		agentError := errors.New("agent call failed") // Keep this specific error for assertion
//
//		// Reset mocks for this specific test to ensure clean behavior for LogError
//		api.Mock = mock.Mock{}
//		// p.getConfiguration() is called. If p.configuration is set, no API call is made by it.
//		// So, no mock for GetConfiguration() is needed here.
//		// Default GetPostsForChannel for this path (successful, to reach CallGraphQLAgentFunc)
//		// This uses the general mockPostList from the outer scope; ensure it's suitable or make specific.
//		// For this test, the content of fetched messages doesn't critically affect the error path,
//		// as long as CallGraphQLAgentFunc is reached.
//		defaultMockPostList := &model.PostList{} // Using a simple one for this error test
//		defaultMockPostList.AddPost(&model.Post{Id: "any_post", Message: "any message", UserId: "another_user"})
//		defaultMockPostList.AddOrder("any_post")
//		api.On("GetPostsForChannel", defaultChannelID, 0, defaultNumMessages+10).Return(defaultMockPostList, nil).Once()
//
//
//		// Setup mock for CallGraphQLAgentFunc for this specific test
//		CallGraphQLAgentFunc = func(apiKey string, conversationID string, userID string, tenantID string, channelID string, userMessage string, apiURL string) ([]MessageOutput, error) {
//			// This assertion is not strictly needed here as we are testing the error path, but good for consistency
//			assert.Equal(t, defaultAPIKey, apiKey)
//			return nil, agentError // Return the predefined error
//		}
//
//		// Expected LogError call
//		// Ensure the error message in LogError matches exactly what's logged by the plugin
//		api.On("LogError", "Error calling GraphQL Agent for !maestro task", "error", agentError.Error(), "user_id", defaultUserID, "task_name", defaultTestTaskName).Return(nil).Once()
//
//		// Expected SendEphemeralPost call to inform the user
//		api.On("SendEphemeralPost", defaultUserID, mock.MatchedBy(func(post *model.Post) bool {
//			return post.ChannelId == defaultChannelID &&
//				strings.Contains(post.Message, "An error occurred while contacting the agent") &&
//				post.RootId == defaultRootID
//		})).Return(&model.Post{}).Once()
//
//
//		err := p.processMaestroTask(defaultTestTaskName, defaultNumMessages, defaultChannelID, defaultUserID, defaultRootID)
//
//		assert.Error(t, err)
//		assert.Equal(t, agentError, err)
//		api.AssertExpectations(t)
//	})
//
//	t.Run("No Messages from Agent", func(t *testing.T) {
//		api := setupAPI()
//		p := setupPlugin(api, defaultAPIKey)
//
//		// Setup mock for this specific test
//		CallGraphQLAgentFunc = func(apiKey string, conversationID string, userID string, tenantID string, channelID string, userMessage string, apiURL string) ([]MessageOutput, error) {
//			return []MessageOutput{}, nil // Empty slice, no error
//		}
//
//		api.On("SendEphemeralPost", defaultUserID, mock.MatchedBy(func(post *model.Post) bool {
//			return strings.Contains(post.Message, "The agent processed your request but returned no messages.")
//		})).Return(&model.Post{}).Once()
//
//		// LogInfo should be called
//		api.On("LogInfo", "GraphQL Agent returned no messages for !maestro task", "user_id", defaultUserID, "task_name", defaultTestTaskName).Return(nil).Once()
//
//
//		err := p.processMaestroTask(defaultTestTaskName, defaultNumMessages, defaultChannelID, defaultUserID, defaultRootID)
//
//		assert.NoError(t, err)
//		api.AssertExpectations(t)
//	})
//}
