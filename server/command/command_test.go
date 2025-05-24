package command

import (
	"fmt"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	// "github.com/mattermost/mattermost/server/public/plugin" // Unused
	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestParseOpenAICommandArgs(t *testing.T) {
	tests := []struct {
		name            string
		command         string
		expectedTask    string
		expectedN       int
		expectError     bool
		expectedErrorMsg string
	}{
		{
			name:         "valid command with task_name and num_messages",
			command:      "/maestro summarize -n 10",
			expectedTask: "summarize",
			expectedN:    10,
			expectError:  false,
		},
		{
			name:         "valid command with task_name only",
			command:      "/maestro summarize",
			expectedTask: "summarize",
			expectedN:    defaultNumMessages, // Should default
			expectError:  false,
		},
		{
			name:         "valid command with multi-word task_name",
			command:      "/maestro translate to french -n 5",
			expectedTask: "translate to french",
			expectedN:    5,
			expectError:  false,
		},
		{
			name:         "valid command with multi-word task_name and no n value",
			command:      "/maestro translate to french",
			expectedTask: "translate to french",
			expectedN:    defaultNumMessages,
			expectError:  false,
		},
		{
			name:             "invalid command: missing task_name",
			command:          "/maestro",
			expectError:      true,
			expectedErrorMsg: fmt.Sprintf("not enough arguments. Usage: /%s <task_name> [-n <num_messages>]", openaiCommandTrigger),
		},
		{
			name:             "invalid command: non-integer num_messages",
			command:          "/maestro summarize -n abc",
			expectError:      true,
			expectedErrorMsg: "invalid value for -n: 'abc'. It must be an integer",
		},
		{
			name:             "invalid command: negative num_messages",
			command:          "/maestro summarize -n -5",
			expectError:      true,
			expectedErrorMsg: "invalid value for -n: -5. It must be a positive integer",
		},
		{
			name:             "invalid command: zero num_messages",
			command:          "/maestro summarize -n 0",
			expectError:      true,
			expectedErrorMsg: "invalid value for -n: 0. It must be a positive integer",
		},
		{
			name:             "invalid command: unknown flag after -n value",
			command:          "/maestro summarize -n 10 -x",
			expectError:      true,
			expectedErrorMsg: "unknown or misplaced flag: -x. Flags like -n must come after the task name", // Corrected expectation
		},
		{
			name:             "invalid command: unknown flag instead of task name part or -n",
			command:          "/maestro summarize -x 10",
			expectError:      true,
			expectedErrorMsg: "unknown or misplaced flag: -x. Only -n is a recognized flag", // Corrected expectation
		},
		{
			name:             "invalid command: non-flag arguments after -n value",
			command:          "/maestro summarize -n 10 somethingelse",
			expectError:      true,
			expectedErrorMsg: "unexpected argument: somethingelse. No arguments are allowed after -n <value>",
		},
		{
			name:         "valid command with task_name and -n at the end",
			command:      "/maestro do this thing -n 3",
			expectedTask: "do this thing",
			expectedN:    3,
			expectError:  false,
		},
		{
			name:             "invalid command: -n missing value",
			command:          "/maestro summarize -n",
			expectError:      true,
			expectedErrorMsg: "missing value for -n argument",
		},
		{
			name:             "invalid command: only trigger",
			command:          "/" + openaiCommandTrigger,
			expectError:      true,
			expectedErrorMsg: fmt.Sprintf("not enough arguments. Usage: /%s <task_name> [-n <num_messages>]", openaiCommandTrigger),
		},
		{
			name:             "empty command string",
			command:          "",
			expectError:      true,
			expectedErrorMsg: fmt.Sprintf("not enough arguments. Usage: /%s <task_name> [-n <num_messages>]", openaiCommandTrigger),
		},
		{
			name:         "task name with leading/trailing spaces in parts (should be handled by strings.Fields)",
			command:      "/maestro  task  with   spaces  -n  7",
			expectedTask: "task with spaces",
			expectedN:    7,
			expectError:  false,
		},
		{
			name:         "task name is just one word before -n",
			command:      "/maestro task -n 1",
			expectedTask: "task",
			expectedN:    1,
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task, n, err := parseOpenAICommandArgs(tt.command)

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

// Mock for plugin.API
type MockOpenAIPluginAPI struct {
	plugintest.API // Embeds mock.Mock and provides default implementations
	CustomMock mock.Mock  // We need a separate mock for methods we define ourselves
}

// Override specific methods from plugin.API that we need to control
func (m *MockOpenAIPluginAPI) GetPostsForChannel(channelID string, page, perPage int) (*model.PostList, *model.AppError) {
	args := m.CustomMock.Called(channelID, page, perPage)
	if args.Get(0) == nil && args.Get(1) == nil { // Both nil means GetPostsForChannel was not expected
		return m.API.GetPostsForChannel(channelID, page, perPage) // Call embedded plugintest.API's version
	}
	if args.Get(0) == nil { // If first arg is nil, it implies an error return
		return nil, args.Get(1).(*model.AppError)
	}
	// Ensure that AppError is correctly cast or is nil
	var appErr *model.AppError
	if args.Get(1) != nil {
		appErr = args.Get(1).(*model.AppError)
	}
	return args.Get(0).(*model.PostList), appErr
}

func (m *MockOpenAIPluginAPI) SendEphemeralPost(userID string, post *model.Post) *model.Post {
	args := m.CustomMock.Called(userID, post)
	// Return what was configured for the mock. plugintest.API.SendEphemeralPost returns a fixed value.
	if args.Get(0) == nil { // If the mocked call is configured to return nil
		return nil
	}
	return args.Get(0).(*model.Post)
}

func (m *MockOpenAIPluginAPI) CreatePost(post *model.Post) (*model.Post, *model.AppError) {
	args := m.CustomMock.Called(post)
	if args.Get(0) == nil && args.Get(1) == nil {
		return m.API.CreatePost(post)
	}
	// Ensure that AppError is correctly cast or is nil
    var appErr *model.AppError
    if args.Get(1) != nil {
        appErr = args.Get(1).(*model.AppError)
    }
	if args.Get(0) == nil {
		return nil, appErr
	}
	return args.Get(0).(*model.Post), appErr
}

func (m *MockOpenAIPluginAPI) LogError(msg string, keyValuePairs ...interface{}) {
	// We need to pass msg and then expand keyValuePairs for the mock call.
	mockArgs := make([]interface{}, 0, 1+len(keyValuePairs))
	mockArgs = append(mockArgs, msg)
	mockArgs = append(mockArgs, keyValuePairs...)
	m.CustomMock.Called(mockArgs...) // Call CustomMock's Called
}


func TestExecuteOpenAICommand(t *testing.T) {
	botUserID := "testbotuserid"
	channelID := "testchannelid"
	userID := "testuserid"
	// triggerPostID := "triggerpostid" // No longer used directly in CommandArgs
	openAIAPIURL := "https://api.openai.com/v1/completions"

	// Helper to create default command args
	newDefaultArgs := func() *model.CommandArgs {
		return &model.CommandArgs{
			ChannelId: channelID,
			UserId:    userID,
			Command:   "", // To be set by each test
			RootId:   "", // Can be empty if not a reply
			ParentId: "", // Can be empty if not a reply
		}
	}

	// Helper for GetOpenAIAPIKey mock
	mockGetOpenAIAPIKeyFunc := func(isValid bool) func() string {
		return func() string {
			if isValid {
				return "test-api-key"
			}
			return ""
		}
	}
	
	// Helper for CallOpenAIFunc mock - captures prompt
	var capturedOpenAIPrompt string
	mockCallOpenAIFunc := func(response string, errToReturn error) func(string, string, string) (string, error) {
		return func(apiKey, message, apiURL string) (string, error) {
			capturedOpenAIPrompt = message
			return response, errToReturn
		}
	}


	// ----- Test Cases -----
	t.Run("successful summarize execution with message filtering and ordering", func(t *testing.T) {
		apiMock := &MockOpenAIPluginAPI{}
		defer apiMock.CustomMock.AssertExpectations(t)

		handler := &Handler{
			dependencies: HandlerDependencies{
				API:             apiMock,
				BotUserID:       botUserID,
				GetOpenAIAPIKey: mockGetOpenAIAPIKeyFunc(true),
				CallOpenAIFunc:  mockCallOpenAIFunc("Test summary.", nil),
				OpenAIAPIURL:    openAIAPIURL,
			},
		}

		args := newDefaultArgs()
		args.Command = "/maestro summarize -n 3" // Request 3 messages

		// Mock GetPostsForChannel
		postList := model.NewPostList()
		// Posts are added in reverse chronological order (newest first) as API returns
		// The command post itself should be filtered out by its ID.
		// Bot messages should be filtered out by UserId.
		postList.AddPost(&model.Post{Id: "post4", UserId: "userC", Message: "Recent message", CreateAt: 4000, ChannelId: channelID})
		postList.AddPost(&model.Post{Id: "post3", UserId: botUserID, Message: "Bot spamming", CreateAt: 3000, ChannelId: channelID})     // Bot message
		postList.AddPost(&model.Post{Id: "triggerCmdPost", UserId: userID, Message: args.Command, CreateAt: 2000, ChannelId: channelID}) // Simulating the command post
		postList.AddPost(&model.Post{Id: "post1", UserId: "userA", Message: "Oldest relevant message", CreateAt: 1000, ChannelId: channelID})
		postList.AddOrder("post4")
		postList.AddOrder("post3")
		postList.AddOrder("triggerCmdPost")
		postList.AddOrder("post1") // Order as returned by API (newest first)

		// We requested 3 messages. GetPostsForChannel will be called with 3.
		apiMock.CustomMock.On("GetPostsForChannel", channelID, 0, 3).Return(postList, (*model.AppError)(nil)).Once()
		
		// Expected prompt after filtering and reversing:
		// "Oldest relevant message"
		// "Recent message"
		expectedPrompt := "Summarize the following messages:\nOldest relevant message\nRecent message"
		
		apiMock.CustomMock.On("CreatePost", mock.MatchedBy(func(post *model.Post) bool {
			return post.UserId == botUserID &&
				post.ChannelId == channelID &&
				post.Message == "Test summary."
		})).Return(&model.Post{Id: "newpostid"}, (*model.AppError)(nil)).Once()

		_, err := handler.executeOpenAICommand(args)
		assert.NoError(t, err)
		assert.Equal(t, expectedPrompt, capturedOpenAIPrompt, "OpenAI prompt mismatch")
	})

	t.Run("successful execution for other task", func(t *testing.T) {
		apiMock := &MockOpenAIPluginAPI{}
		defer apiMock.CustomMock.AssertExpectations(t)

		taskName := "explain this code"
		numMessages := 2
		expectedOpenAIResponse := "This code does XYZ."

		handler := &Handler{
			dependencies: HandlerDependencies{
				API:             apiMock,
				BotUserID:       botUserID,
				GetOpenAIAPIKey: mockGetOpenAIAPIKeyFunc(true),
				CallOpenAIFunc:  mockCallOpenAIFunc(expectedOpenAIResponse, nil),
				OpenAIAPIURL:    openAIAPIURL,
			},
		}

		args := newDefaultArgs()
		args.Command = fmt.Sprintf("/maestro %s -n %d", taskName, numMessages)

		postList := model.NewPostList()
		postList.AddPost(&model.Post{Id: "postB", UserId: "user1", Message: "message one", CreateAt: 2000, ChannelId: channelID})
		postList.AddPost(&model.Post{Id: "postA", UserId: "user2", Message: "message two", CreateAt: 1000, ChannelId: channelID})
		postList.AddOrder("postB")
		postList.AddOrder("postA")

		apiMock.CustomMock.On("GetPostsForChannel", channelID, 0, numMessages).Return(postList, (*model.AppError)(nil)).Once()
		
		expectedPrompt := fmt.Sprintf("User query: %s\nmessage two\nmessage one", taskName)
		
		apiMock.CustomMock.On("CreatePost", mock.MatchedBy(func(post *model.Post) bool {
			return post.Message == expectedOpenAIResponse
		})).Return(&model.Post{}, (*model.AppError)(nil)).Once()

		_, err := handler.executeOpenAICommand(args)
		assert.NoError(t, err)
		assert.Equal(t, expectedPrompt, capturedOpenAIPrompt, "OpenAI prompt mismatch")
	})


	t.Run("error when GetOpenAIAPIKey returns empty", func(t *testing.T) {
		apiMock := &MockOpenAIPluginAPI{}
		defer apiMock.CustomMock.AssertExpectations(t)

		handler := &Handler{
			dependencies: HandlerDependencies{
				API:             apiMock,
				BotUserID:       botUserID,
				GetOpenAIAPIKey: mockGetOpenAIAPIKeyFunc(false), 
				CallOpenAIFunc:  mockCallOpenAIFunc("", nil),    
				OpenAIAPIURL:    openAIAPIURL,
			},
		}
		args := newDefaultArgs()
		args.Command = "/maestro summarize"

		expectedEphemeralMsg := "The OpenAI API Key is not configured. Please contact your system administrator."
		apiMock.CustomMock.On("SendEphemeralPost", userID, mock.MatchedBy(func(post *model.Post) bool {
			return post.ChannelId == channelID && post.Message == expectedEphemeralMsg
		})).Return(&model.Post{}).Once() 

		_, err := handler.executeOpenAICommand(args)
		assert.NoError(t, err) 
	})

	t.Run("error when GetPostsForChannel fails", func(t *testing.T) {
		apiMock := &MockOpenAIPluginAPI{}
		defer apiMock.CustomMock.AssertExpectations(t)

		handler := &Handler{
			dependencies: HandlerDependencies{
				API:             apiMock,
				BotUserID:       botUserID,
				GetOpenAIAPIKey: mockGetOpenAIAPIKeyFunc(true),
				CallOpenAIFunc:  mockCallOpenAIFunc("", nil), 
				OpenAIAPIURL:    openAIAPIURL,
			},
		}
		args := newDefaultArgs()
		args.Command = "/maestro summarize -n 5"

		appErr := model.NewAppError("GetPostsForChannel", "some.error.id", nil, "Failed to fetch posts", 500)
		apiMock.CustomMock.On("GetPostsForChannel", channelID, 0, 5).Return((*model.PostList)(nil), appErr).Once()

		apiMock.CustomMock.On("LogError", "Failed to fetch posts for channel", "channel_id", channelID, "error", appErr.Error()).Once()
		
		expectedEphemeralMsg := "An error occurred while fetching messages. Please try again later."
		apiMock.CustomMock.On("SendEphemeralPost", userID, mock.MatchedBy(func(post *model.Post) bool {
			return post.ChannelId == channelID && post.Message == expectedEphemeralMsg
		})).Return(&model.Post{}).Once()

		_, err := handler.executeOpenAICommand(args)
		assert.NoError(t, err)
	})

	t.Run("error when CallOpenAIFunc fails", func(t *testing.T) {
		apiMock := &MockOpenAIPluginAPI{}
		defer apiMock.CustomMock.AssertExpectations(t)
		
		openAICallError := fmt.Errorf("OpenAI API error")

		handler := &Handler{
			dependencies: HandlerDependencies{
				API:             apiMock,
				BotUserID:       botUserID,
				GetOpenAIAPIKey: mockGetOpenAIAPIKeyFunc(true),
				CallOpenAIFunc:  mockCallOpenAIFunc("", openAICallError), // Mocked to return an error
				OpenAIAPIURL:    openAIAPIURL,
			},
		}
		args := newDefaultArgs()
		args.Command = "/maestro summarize -n 1" 

		postList := model.NewPostList()
		postList.AddPost(&model.Post{Id: "post1", UserId: "anotheruser", Message: "Test message for OpenAI call", CreateAt: 1000, ChannelId: channelID})
		postList.AddOrder("post1")
		apiMock.CustomMock.On("GetPostsForChannel", channelID, 0, 1).Return(postList, (*model.AppError)(nil)).Once()

		apiMock.CustomMock.On("LogError", "Error calling OpenAI API for slash command", "error", openAICallError.Error()).Once()

		expectedEphemeralMsg := "An error occurred while contacting OpenAI. Please try again later or contact an administrator."
		apiMock.CustomMock.On("SendEphemeralPost", userID, mock.MatchedBy(func(post *model.Post) bool {
			return post.ChannelId == channelID && post.Message == expectedEphemeralMsg
		})).Return(&model.Post{}).Once()

		_, err := handler.executeOpenAICommand(args)
		assert.NoError(t, err)
		assert.Equal(t, "Summarize the following messages:\nTest message for OpenAI call", capturedOpenAIPrompt, "Prompt passed to CallOpenAIFunc was not as expected.")
	})

	t.Run("error when CreatePost fails after successful OpenAI call", func(t *testing.T) {
		apiMock := &MockOpenAIPluginAPI{}
		defer apiMock.CustomMock.AssertExpectations(t)

		openAIResponse := "Successful OpenAI response"
		createPostAppErr := model.NewAppError("CreatePost", "some.post.error.id", nil, "Failed to create post", 500)
		
		handler := &Handler{
			dependencies: HandlerDependencies{
				API:             apiMock,
				BotUserID:       botUserID,
				GetOpenAIAPIKey: mockGetOpenAIAPIKeyFunc(true),
				CallOpenAIFunc:  mockCallOpenAIFunc(openAIResponse, nil),
				OpenAIAPIURL:    openAIAPIURL,
			},
		}
		args := newDefaultArgs()
		args.Command = "/maestro ask something -n 1"

		postList := model.NewPostList()
		postList.AddPost(&model.Post{Id: "p1", UserId: "someuser", Message: "context message", CreateAt: 123, ChannelId: channelID})
		postList.AddOrder("p1")
		apiMock.CustomMock.On("GetPostsForChannel", channelID, 0, 1).Return(postList, (*model.AppError)(nil)).Once()

		apiMock.CustomMock.On("CreatePost", mock.MatchedBy(func(post *model.Post) bool {
			return post.UserId == botUserID && post.ChannelId == channelID && post.Message == openAIResponse
		})).Return((*model.Post)(nil), createPostAppErr).Once()

		apiMock.CustomMock.On("LogError", "Failed to post OpenAI response for slash command", "error", createPostAppErr.Error()).Once()
		
		expectedEphemeralMsg := "An error occurred while trying to post the OpenAI response."
		apiMock.CustomMock.On("SendEphemeralPost", userID, mock.MatchedBy(func(post *model.Post) bool {
			return post.ChannelId == channelID && post.Message == expectedEphemeralMsg
		})).Return(&model.Post{}).Once()

		_, err := handler.executeOpenAICommand(args)
		assert.NoError(t, err)
		assert.Equal(t, "User query: ask something\ncontext message", capturedOpenAIPrompt, "Prompt for CreatePost test was not as expected.")
	})
	
	t.Run("no messages fetched, non-summarize task, ensure prompt is correct", func(t *testing.T) {
		apiMock := &MockOpenAIPluginAPI{}
		defer apiMock.CustomMock.AssertExpectations(t)

		taskName := "explain this"
		expectedOpenAIResponse := "Explanation based on no context."

		handler := &Handler{
			dependencies: HandlerDependencies{
				API:             apiMock,
				BotUserID:       botUserID,
				GetOpenAIAPIKey: mockGetOpenAIAPIKeyFunc(true),
				CallOpenAIFunc:  mockCallOpenAIFunc(expectedOpenAIResponse, nil),
				OpenAIAPIURL:    openAIAPIURL,
			},
		}

		args := newDefaultArgs()
		args.Command = fmt.Sprintf("/maestro %s -n 5", taskName)

		emptyPostList := model.NewPostList() 
		apiMock.CustomMock.On("GetPostsForChannel", channelID, 0, 5).Return(emptyPostList, (*model.AppError)(nil)).Once()
		
		apiMock.CustomMock.On("CreatePost", mock.MatchedBy(func(post *model.Post) bool {
			return post.Message == expectedOpenAIResponse
		})).Return(&model.Post{}, (*model.AppError)(nil)).Once()

		_, err := handler.executeOpenAICommand(args)
		assert.NoError(t, err)
		// Expect prompt to be just the user query part with a newline, as fetchedMessages is empty.
		assert.Equal(t, fmt.Sprintf("User query: %s\n", taskName), capturedOpenAIPrompt, "Prompt for empty messages was not as expected.")
	})

	t.Run("no messages fetched, summarize task, ensure prompt is correct", func(t *testing.T) {
		apiMock := &MockOpenAIPluginAPI{}
		defer apiMock.CustomMock.AssertExpectations(t)

		expectedOpenAIResponse := "Summary of nothing."
		
		handler := &Handler{
			dependencies: HandlerDependencies{
				API:             apiMock,
				BotUserID:       botUserID,
				GetOpenAIAPIKey: mockGetOpenAIAPIKeyFunc(true),
				CallOpenAIFunc:  mockCallOpenAIFunc(expectedOpenAIResponse, nil),
				OpenAIAPIURL:    openAIAPIURL,
			},
		}

		args := newDefaultArgs()
		args.Command = "/maestro summarize -n 5"

		emptyPostList := model.NewPostList()
		apiMock.CustomMock.On("GetPostsForChannel", channelID, 0, 5).Return(emptyPostList, (*model.AppError)(nil)).Once()

		apiMock.CustomMock.On("CreatePost", mock.MatchedBy(func(post *model.Post) bool {
			return post.Message == expectedOpenAIResponse
		})).Return(&model.Post{}, (*model.AppError)(nil)).Once()

		_, err := handler.executeOpenAICommand(args)
		assert.NoError(t, err)
		// Expect prompt for summarize to also end with a newline if messages are empty.
		assert.Equal(t, "Summarize the following messages:\n", capturedOpenAIPrompt, "Prompt for summarize with empty messages was not as expected.")
	})
}
