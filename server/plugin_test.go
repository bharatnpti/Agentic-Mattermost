package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/mattermost/mattermost-plugin-starter-template/server/command" // For DefaultNumMessages
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// TestParseMaestroArgs tests the argument parsing for !maestro commands.
// Adapted from the former TestParseOpenAICommandArgs.
func TestParseMaestroArgs(t *testing.T) {
	// p.parseMaestroArgs doesn't actually use any fields from p, so a nil receiver or minimal struct is fine.
	// However, to make it a valid method call, we need an instance.
	p := &Plugin{} 

	tests := []struct {
		name             string
		argsString       string // This is the string *after* "!maestro "
		expectedTask     string
		expectedN        int
		expectError      bool
		expectedErrorMsg string
	}{
		{
			name:          "valid: task_name and num_messages",
			argsString:    "summarize -n 10",
			expectedTask:  "summarize",
			expectedN:     10,
			expectError:   false,
		},
		{
			name:          "valid: task_name only",
			argsString:    "summarize",
			expectedTask:  "summarize",
			expectedN:     command.DefaultNumMessages, // Should default
			expectError:   false,
		},
		{
			name:          "valid: multi-word task_name",
			argsString:    "translate to french -n 5",
			expectedTask:  "translate to french",
			expectedN:     5,
			expectError:   false,
		},
		{
			name:          "valid: multi-word task_name and no n value",
			argsString:    "translate to french",
			expectedTask:  "translate to french",
			expectedN:     command.DefaultNumMessages,
			expectError:   false,
		},
		{
			name:             "invalid: missing task_name (empty string)",
			argsString:       "", // Equivalent to just "!maestro"
			expectError:      true,
			expectedErrorMsg: "not enough arguments. Usage: !maestro <task_name> [-n <num_messages>]",
		},
		{
			name:             "invalid: non-integer num_messages",
			argsString:       "summarize -n abc",
			expectError:      true,
			expectedErrorMsg: "invalid value for -n: 'abc'. It must be an integer",
		},
		{
			name:             "invalid: negative num_messages",
			argsString:       "summarize -n -5",
			expectError:      true,
			expectedErrorMsg: "invalid value for -n: -5. It must be a positive integer",
		},
		{
			name:             "invalid: zero num_messages",
			argsString:       "summarize -n 0",
			expectError:      true,
			expectedErrorMsg: "invalid value for -n: 0. It must be a positive integer",
		},
		{
			name:             "invalid: unknown flag after -n value",
			argsString:       "summarize -n 10 -x",
			expectError:      true,
			expectedErrorMsg: "unknown or misplaced flag: -x. Flags like -n must come after the task name",
		},
		{
			name:             "invalid: unknown flag instead of task name part or -n",
			argsString:       "summarize -x 10",
			expectError:      true,
			expectedErrorMsg: "unknown or misplaced flag: -x. Only -n is a recognized flag",
		},
		{
			name:             "invalid: non-flag arguments after -n value",
			argsString:       "summarize -n 10 somethingelse",
			expectError:      true,
			expectedErrorMsg: "unexpected argument: somethingelse. No arguments are allowed after -n <value>",
		},
		{
			name:          "valid: task_name and -n at the end",
			argsString:    "do this thing -n 3",
			expectedTask:  "do this thing",
			expectedN:     3,
			expectError:   false,
		},
		{
			name:             "invalid: -n missing value",
			argsString:       "summarize -n",
			expectError:      true,
			expectedErrorMsg: "missing value for -n argument",
		},
		{
			name:          "valid: task name with leading/trailing spaces in parts (handled by strings.Fields)",
			argsString:    "  task  with   spaces  -n  7  ", // strings.Fields will handle outer spaces
			expectedTask:  "task with spaces", // Inner spaces preserved by Join
			expectedN:     7,
			expectError:   false,
		},
		{
			name:          "valid: task name is just one word before -n",
			argsString:    "task -n 1",
			expectedTask:  "task",
			expectedN:     1,
			expectError:   false,
		},
		{
			name:             "invalid: only -n and value",
			argsString:       "-n 10",
			expectError:      true,
			expectedErrorMsg: "task_name cannot be empty. Usage: !maestro <task_name> [-n <num_messages>]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Since parseMaestroArgs is a method on p, call it like p.parseMaestroArgs
			task, n, err := p.parseMaestroArgs(tt.argsString)

			if tt.expectError {
				assert.Error(t, err)
				if tt.expectedErrorMsg != "" {
					assert.EqualError(t, err, tt.expectedErrorMsg)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedTask, task)
				assert.Equal(t, tt.expectedN, n)
			}
		})
	}
}

// MockPluginAPI is a helper struct to mock plugin.API for plugin tests
type MockPluginAPI struct {
	plugintest.API // Embeds plugintest.API for default behaviors
	Mock           mock.Mock    // testify's mock for custom expectations
}

func (m *MockPluginAPI) GetPostsForChannel(channelID string, page, perPage int) (*model.PostList, *model.AppError) {
	args := m.Mock.Called(channelID, page, perPage)
	var postList *model.PostList
	if args.Get(0) != nil {
		postList = args.Get(0).(*model.PostList)
	}
	var appErr *model.AppError
	if args.Get(1) != nil {
		appErr = args.Get(1).(*model.AppError)
	}
	return postList, appErr
}

func (m *MockPluginAPI) CreatePost(post *model.Post) (*model.Post, *model.AppError) {
	args := m.Mock.Called(post)
	var createdPost *model.Post
	if args.Get(0) != nil {
		createdPost = args.Get(0).(*model.Post)
	}
	var appErr *model.AppError
	if args.Get(1) != nil {
		appErr = args.Get(1).(*model.AppError)
	}
	return createdPost, appErr
}

func (m *MockPluginAPI) SendEphemeralPost(userID string, post *model.Post) *model.Post {
	args := m.Mock.Called(userID, post)
	if args.Get(0) != nil {
		return args.Get(0).(*model.Post)
	}
	return nil // Return nil if the mock is configured to return nil for the first argument
}


func (m *MockPluginAPI) LogError(msg string, keyValuePairs ...interface{}) {
	allArgs := append([]interface{}{msg}, keyValuePairs...)
	m.Mock.Called(allArgs...)
}

func (m *MockPluginAPI) LogInfo(msg string, keyValuePairs ...interface{}) {
	allArgs := append([]interface{}{msg}, keyValuePairs...)
	m.Mock.Called(allArgs...)
}


func TestProcessMaestroTask(t *testing.T) {
	botUserID := "testbotuserid"
	channelID := "testchannelid"
	userID := "testuserid"
	rootID := "rootpostid"
	
	// Store original CallOpenAIAPIFunc and OpenAIAPIURL to restore them later
    originalCallFunc := CallOpenAIAPIFunc
    originalAPIURL := OpenAIAPIURL
    defer func() {
        CallOpenAIAPIFunc = originalCallFunc
        OpenAIAPIURL = originalAPIURL
    }()


	setupPlugin := func(apiKey string) (*Plugin, *MockPluginAPI) {
		apiMock := &MockPluginAPI{}
		p := &Plugin{
			MattermostPlugin: plugin.MattermostPlugin{
				API: apiMock,
			},
			botUserID: botUserID,
			configuration: &configuration{
				OpenAIAPIKey: apiKey,
			},
		}
		return p, apiMock
	}

	t.Run("successful execution for summarize", func(t *testing.T) {
		p, apiMock := setupPlugin("test-key")
		defer apiMock.Mock.AssertExpectations(t)

		taskName := "summarize"
		numMessages := 2
		expectedOpenAIResponse := "This is a summary."
		
		// Mock CallOpenAIAPIFunc
		var capturedPrompt string
        CallOpenAIAPIFunc = func(apiKey, message, apiURL string) (string, error) {
            capturedPrompt = message
            assert.Equal(t, "test-key", apiKey)
            // You could also assert apiURL if it's configurable and important
            return expectedOpenAIResponse, nil
        }

		postList := model.NewPostList()
		postList.AddPost(&model.Post{Id: "post2", UserId: "user1", Message: "message one", CreateAt: 2000, ChannelId: channelID})
		postList.AddPost(&model.Post{Id: "post1", UserId: "user2", Message: "message two", CreateAt: 1000, ChannelId: channelID})
		postList.AddOrder("post2", "post1") // Newest first

		// numMessages+10 is used in processMaestroTask
		apiMock.Mock.On("GetPostsForChannel", channelID, 0, numMessages+10).Return(postList, (*model.AppError)(nil)).Once()

		apiMock.Mock.On("CreatePost", mock.MatchedBy(func(post *model.Post) bool {
			return post.UserId == botUserID &&
				post.ChannelId == channelID &&
				post.Message == expectedOpenAIResponse &&
				post.RootId == rootID &&
				post.ParentId == rootID
		})).Return(&model.Post{Id: "responsepostid"}, (*model.AppError)(nil)).Once()

		err := p.processMaestroTask(taskName, numMessages, channelID, userID, rootID)
		assert.NoError(t, err)
		assert.Equal(t, "Summarize the following messages:\nmessage two\nmessage one", capturedPrompt)
	})
	
	t.Run("successful execution for generic task", func(t *testing.T) {
		p, apiMock := setupPlugin("test-key")
		defer apiMock.Mock.AssertExpectations(t)

		taskName := "explain this concept"
		numMessages := 1
		expectedOpenAIResponse := "This is an explanation."
		
		var capturedPrompt string
        CallOpenAIAPIFunc = func(apiKey, message, apiURL string) (string, error) {
            capturedPrompt = message
            return expectedOpenAIResponse, nil
        }

		postList := model.NewPostList()
		postList.AddPost(&model.Post{Id: "post1", UserId: "userA", Message: "Some context", CreateAt: 1000, ChannelId: channelID})
		postList.AddOrder("post1")

		apiMock.Mock.On("GetPostsForChannel", channelID, 0, numMessages+10).Return(postList, (*model.AppError)(nil)).Once()

		apiMock.Mock.On("CreatePost", mock.MatchedBy(func(post *model.Post) bool {
			return post.Message == expectedOpenAIResponse && post.RootId == rootID
		})).Return(&model.Post{}, (*model.AppError)(nil)).Once()

		err := p.processMaestroTask(taskName, numMessages, channelID, userID, rootID)
		assert.NoError(t, err)
		assert.Equal(t, fmt.Sprintf("User query: %s\nSome context", taskName), capturedPrompt)
	})


	t.Run("error: no API key", func(t *testing.T) {
		p, apiMock := setupPlugin("") // Empty API key
		defer apiMock.Mock.AssertExpectations(t)

		apiMock.Mock.On("SendEphemeralPost", userID, mock.AnythingOfType("*model.Post")).Return(&model.Post{}).Once()
		// LogError is not directly called by processMaestroTask for empty API key, it returns an error.
		// The calling function (MessageHasBeenPosted) would log.

		err := p.processMaestroTask("summarize", 5, channelID, userID, rootID)
		assert.Error(t, err)
		assert.Equal(t, "OpenAI API Key is not configured", err.Error())
	})

	t.Run("error: GetPostsForChannel fails", func(t *testing.T) {
		p, apiMock := setupPlugin("test-key")
		defer apiMock.Mock.AssertExpectations(t)
		
		appErr := model.NewAppError("GetPostsForChannel", "some.error", nil, "db error", 500)
		apiMock.Mock.On("GetPostsForChannel", channelID, 0, 5+10).Return((*model.PostList)(nil), appErr).Once()
		apiMock.Mock.On("LogError", "Failed to fetch posts for channel", "channel_id", channelID, "error", appErr.Error()).Once()
		apiMock.Mock.On("SendEphemeralPost", userID, mock.AnythingOfType("*model.Post")).Return(&model.Post{}).Once()

		err := p.processMaestroTask("summarize", 5, channelID, userID, rootID)
		assert.Error(t, err)
		assert.True(t, strings.HasPrefix(err.Error(), "failed to fetch posts: "))
	})

	t.Run("error: CallOpenAIAPIFunc fails", func(t *testing.T) {
		p, apiMock := setupPlugin("test-key")
		defer apiMock.Mock.AssertExpectations(t)

		openAIError := errors.New("OpenAI API down")
        CallOpenAIAPIFunc = func(apiKey, message, apiURL string) (string, error) {
            return "", openAIError
        }
		
		postList := model.NewPostList() // Empty is fine, it will proceed to call OpenAI
		apiMock.Mock.On("GetPostsForChannel", channelID, 0, 5+10).Return(postList, (*model.AppError)(nil)).Once()
		apiMock.Mock.On("LogError", "Error calling OpenAI API for !maestro task", "error", openAIError.Error(), "user_id", userID, "task_name", "summarize").Once()
		apiMock.Mock.On("SendEphemeralPost", userID, mock.AnythingOfType("*model.Post")).Return(&model.Post{}).Once()
		
		err := p.processMaestroTask("summarize", 5, channelID, userID, rootID)
		assert.Error(t, err)
		assert.Equal(t, openAIError, err)
	})

	t.Run("error: CreatePost fails", func(t *testing.T) {
		p, apiMock := setupPlugin("test-key")
		defer apiMock.Mock.AssertExpectations(t)

        CallOpenAIAPIFunc = func(apiKey, message, apiURL string) (string, error) {
            return "AI response", nil
        }
		
		postList := model.NewPostList()
		apiMock.Mock.On("GetPostsForChannel", channelID, 0, 5+10).Return(postList, (*model.AppError)(nil)).Once()
		
		createPostErr := model.NewAppError("CreatePost", "some.error", nil, "failed to save post", 500)
		apiMock.Mock.On("CreatePost", mock.AnythingOfType("*model.Post")).Return((*model.Post)(nil), createPostErr).Once()
		apiMock.Mock.On("LogError", "Failed to post OpenAI response for !maestro task", "error", createPostErr.Error(), "user_id", userID).Once()
		apiMock.Mock.On("SendEphemeralPost", userID, mock.AnythingOfType("*model.Post")).Return(&model.Post{}).Once()

		err := p.processMaestroTask("summarize", 5, channelID, userID, rootID)
		assert.Error(t, err)
		assert.True(t, strings.HasPrefix(err.Error(), "failed to create response post: "))
	})
	
	t.Run("message filtering in processMaestroTask", func(t *testing.T) {
		p, apiMock := setupPlugin("test-key")
		defer apiMock.Mock.AssertExpectations(t)
		
		taskName := "analyze"
		numMessages := 3
		
		var capturedPrompt string
        CallOpenAIAPIFunc = func(apiKey, message, apiURL string) (string, error) {
            capturedPrompt = message
            return "Analysis done.", nil
        }

		postList := model.NewPostList()
		// Order: newest to oldest as API returns
		postList.AddPost(&model.Post{Id: "post5", UserId: "userA", Message: "user message 3", CreateAt: 5000})
		postList.AddPost(&model.Post{Id: "post4", UserId: botUserID, Message: "bot message", CreateAt: 4000})      // Should be filtered
		postList.AddPost(&model.Post{Id: rootID, UserId: userID, Message: "!maestro " + taskName , CreateAt: 3500}) // The trigger message
		postList.AddPost(&model.Post{Id: "post3", UserId: "userB", Message: "user message 2", CreateAt: 3000})
		postList.AddPost(&model.Post{Id: "post2", UserId: "userC", Message: "!maestro other task", CreateAt: 2000}) // Should be filtered
		postList.AddPost(&model.Post{Id: "post1", UserId: "userD", Message: "user message 1", CreateAt: 1000})
		postList.AddOrder("post5", "post4", rootID, "post3", "post2", "post1")


		apiMock.Mock.On("GetPostsForChannel", channelID, 0, numMessages+10).Return(postList, (*model.AppError)(nil)).Once()
		apiMock.Mock.On("CreatePost", mock.AnythingOfType("*model.Post")).Return(&model.Post{}, (*model.AppError)(nil)).Once()

		err := p.processMaestroTask(taskName, numMessages, channelID, userID, rootID)
		assert.NoError(t, err)
		
		expectedMessages := "user message 1\nuser message 2\nuser message 3"
		expectedFullPrompt := fmt.Sprintf("User query: %s\n%s", taskName, expectedMessages)
		assert.Equal(t, expectedFullPrompt, capturedPrompt)
	})
}


func TestMessageHasBeenPosted(t *testing.T) {
	botUserID := "testbotuserid"
	userID := "testactiveuserid"
	channelID := "testchannelid"

	// Store original CallOpenAIAPIFunc and OpenAIAPIURL to restore them later
    originalCallFunc := CallOpenAIAPIFunc
    originalAPIURL := OpenAIAPIURL
    defer func() {
        CallOpenAIAPIFunc = originalCallFunc
        OpenAIAPIURL = originalAPIURL
    }()


	setupPluginAndPost := func(message, postUserID string) (*Plugin, *MockPluginAPI, *model.Post) {
		apiMock := &MockPluginAPI{}
		p := &Plugin{
			MattermostPlugin: plugin.MattermostPlugin{
				API: apiMock,
			},
			botUserID: botUserID,
			configuration: &configuration{
				OpenAIAPIKey: "test-api-key", // Assume valid key for most MessageHasBeenPosted tests
			},
		}
		post := &model.Post{
			UserId:    postUserID,
			Message:   message,
			ChannelId: channelID,
			Id:        model.NewId(), // Unique ID for the post
		}
		return p, apiMock, post
	}

	t.Run("ignore bot's own message", func(t *testing.T) {
		p, apiMock, post := setupPluginAndPost("!maestro summarize", botUserID)
		defer apiMock.Mock.AssertExpectations(t)
		// No API calls should be made if the message is from the bot.
		p.MessageHasBeenPosted(nil, post)
		// Assert that no methods on apiMock.Mock were called, e.g. LogInfo for detected prefix.
		// apiMock.Mock.AssertNotCalled(t, "LogInfo", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
		// More simply, just ensure no unexpected calls by having no expectations set.
	})

	t.Run("ignore message not starting with !maestro", func(t *testing.T) {
		p, apiMock, post := setupPluginAndPost("hello world", userID)
		defer apiMock.Mock.AssertExpectations(t)
		p.MessageHasBeenPosted(nil, post)
	})
	
	t.Run("ignore message not starting with !maestro, but contains it", func(t *testing.T) {
		p, apiMock, post := setupPluginAndPost("hello world !maestro later", userID)
		defer apiMock.Mock.AssertExpectations(t)
		p.MessageHasBeenPosted(nil, post)
	})


	t.Run("valid !maestro command calls parser and processor", func(t *testing.T) {
		p, apiMock, post := setupPluginAndPost("!maestro summarize -n 5", userID)
		defer apiMock.Mock.AssertExpectations(t)

		// LogInfo for detected prefix
		apiMock.Mock.On("LogInfo", "Detected '!maestro' prefix.", "user_id", userID, "channel_id", channelID, "original_message", post.Message, "arguments_string", "summarize -n 5").Once()

		// Mocking dependencies for processMaestroTask part
        CallOpenAIAPIFunc = func(apiKey, message, apiURL string) (string, error) {
            return "AI summary.", nil
        }
		emptyPostList := model.NewPostList() // Assuming no prior messages for simplicity here
		apiMock.Mock.On("GetPostsForChannel", channelID, 0, 5+10).Return(emptyPostList, (*model.AppError)(nil)).Once()
		apiMock.Mock.On("CreatePost", mock.MatchedBy(func(p *model.Post) bool {
			return p.Message == "AI summary." && p.RootId == post.Id
		})).Return(&model.Post{}, (*model.AppError)(nil)).Once()
		
		p.MessageHasBeenPosted(nil, post)
	})
	
	t.Run("valid !Maestro command (case insensitive) calls parser and processor", func(t *testing.T) {
		p, apiMock, post := setupPluginAndPost("!Maestro summarize -n 2", userID) // Case variation
		defer apiMock.Mock.AssertExpectations(t)

		apiMock.Mock.On("LogInfo", "Detected '!maestro' prefix.", "user_id", userID, "channel_id", channelID, "original_message", post.Message, "arguments_string", "summarize -n 2").Once()
        CallOpenAIAPIFunc = func(apiKey, message, apiURL string) (string, error) { return "AI summary.", nil }
		emptyPostList := model.NewPostList()
		apiMock.Mock.On("GetPostsForChannel", channelID, 0, 2+10).Return(emptyPostList, (*model.AppError)(nil)).Once()
		apiMock.Mock.On("CreatePost", mock.AnythingOfType("*model.Post")).Return(&model.Post{}, (*model.AppError)(nil)).Once()
		
		p.MessageHasBeenPosted(nil, post)
	})


	t.Run("!maestro command with parsing error", func(t *testing.T) {
		p, apiMock, post := setupPluginAndPost("!maestro -n abc", userID) // Invalid -n value
		defer apiMock.Mock.AssertExpectations(t)

		apiMock.Mock.On("LogInfo", "Detected '!maestro' prefix.", "user_id", userID, "channel_id", channelID, "original_message", post.Message, "arguments_string", "-n abc").Once()
		
		// Ephemeral post for parsing error
		expectedErrorMsg := "task_name cannot be empty. Usage: !maestro <task_name> [-n <num_messages>]"
		// The actual error from parseMaestroArgs for "-n abc" would be "task_name cannot be empty..."
		// because "-n" itself isn't a task name. If it was "task -n abc", it would be "invalid value for -n..."
		// Let's use a command that gives a more direct parsing error for -n:
		post.Message = "!maestro mytask -n abc"
		argumentsString := "mytask -n abc"
		apiMock.Mock.On("LogInfo", "Detected '!maestro' prefix.", "user_id", userID, "channel_id", channelID, "original_message", post.Message, "arguments_string", argumentsString).Once()
		expectedErrorMsg = "invalid value for -n: 'abc'. It must be an integer"


		apiMock.Mock.On("SendEphemeralPost", userID, mock.MatchedBy(func(ephemeralPost *model.Post) bool {
			return ephemeralPost.ChannelId == channelID &&
				strings.Contains(ephemeralPost.Message, expectedErrorMsg) &&
				ephemeralPost.RootId == post.Id
		})).Return(&model.Post{}).Once()
		
		apiMock.Mock.On("LogError", "Failed to parse arguments for !maestro command", "error", expectedErrorMsg, "user_id", userID, "arguments", argumentsString).Once()
		
		p.MessageHasBeenPosted(nil, post)
	})

	t.Run("!maestro command where processMaestroTask returns an error (e.g. API key empty)", func(t *testing.T) {
		p, apiMock, post := setupPluginAndPost("!maestro summarize", userID)
		p.configuration.OpenAIAPIKey = "" // Ensure API key is empty for this test
		defer apiMock.Mock.AssertExpectations(t)

		apiMock.Mock.On("LogInfo", "Detected '!maestro' prefix.", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Once()
		
		// processMaestroTask will send its own ephemeral post
		apiMock.Mock.On("SendEphemeralPost", userID, mock.MatchedBy(func(ephemeralPost *model.Post) bool {
			return strings.Contains(ephemeralPost.Message, "OpenAI API Key is not configured") && ephemeralPost.RootId == post.Id
		})).Return(&model.Post{}).Once()
		
		// And MessageHasBeenPosted will log the error from processMaestroTask
		apiMock.Mock.On("LogError", "Error processing !maestro task", "error", "OpenAI API Key is not configured", "user_id", userID, "task_name", "summarize").Once()

		p.MessageHasBeenPosted(nil, post)
	})
}
```
