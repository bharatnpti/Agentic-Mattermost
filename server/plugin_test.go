package main

import (
	"fmt"
	"github.com/mattermost/mattermost/server/public/plugin"
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
			name:         "valid: task_name and num_messages",
			argsString:   "summarize -n 10",
			expectedTask: "summarize",
			expectedN:    10,
			expectError:  false,
		},
		{
			name:         "valid: task_name only",
			argsString:   "summarize",
			expectedTask: "summarize",
			expectedN:    command.DefaultNumMessages, // Should default
			expectError:  false,
		},
		{
			name:         "valid: multi-word task_name",
			argsString:   "translate to french -n 5",
			expectedTask: "translate to french",
			expectedN:    5,
			expectError:  false,
		},
		{
			name:         "valid: multi-word task_name and no n value",
			argsString:   "translate to french",
			expectedTask: "translate to french",
			expectedN:    command.DefaultNumMessages,
			expectError:  false,
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
			name:         "valid: task_name and -n at the end",
			argsString:   "do this thing -n 3",
			expectedTask: "do this thing",
			expectedN:    3,
			expectError:  false,
		},
		{
			name:             "invalid: -n missing value",
			argsString:       "summarize -n",
			expectError:      true,
			expectedErrorMsg: "missing value for -n argument",
		},
		{
			name:         "valid: task name with leading/trailing spaces in parts (handled by strings.Fields)",
			argsString:   "  task  with   spaces  -n  7  ", // strings.Fields will handle outer spaces
			expectedTask: "task with spaces",               // Inner spaces preserved by Join
			expectedN:    7,
			expectError:  false,
		},
		{
			name:         "valid: task name is just one word before -n",
			argsString:   "task -n 1",
			expectedTask: "task",
			expectedN:    1,
			expectError:  false,
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
	plugintest.API           // Embeds plugintest.API for default behaviors
	Mock           mock.Mock // testify's mock for custom expectations
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

// Add GetUser to the mock
func (m *MockPluginAPI) GetUser(userID string) (*model.User, *model.AppError) {
	args := m.Mock.Called(userID)
	var user *model.User
	if args.Get(0) != nil {
		user = args.Get(0).(*model.User)
	}
	var appErr *model.AppError
	if args.Get(1) != nil {
		appErr = args.Get(1).(*model.AppError)
	}
	return user, appErr
}

// Add GetDirectChannel to the mock
func (m *MockPluginAPI) GetDirectChannel(userID1, userID2 string) (*model.Channel, *model.AppError) {
	args := m.Mock.Called(userID1, userID2)
	var channel *model.Channel
	if args.Get(0) != nil {
		channel = args.Get(0).(*model.Channel)
	}
	var appErr *model.AppError
	if args.Get(1) != nil {
		appErr = args.Get(1).(*model.AppError)
	}
	return channel, appErr
}

func TestProcessMaestroTask(t *testing.T) {
	botUserID := "testbotuserid"
	channelID := "testchannelid"
	userID := "testuserid"
	rootID := "rootpostid"

	// Store original CallOpenAIAPIFunc and OpenAIAPIURL to restore them later
	originalCallFunc := CallOpenAIAPIFunc
	defer func() {
		CallOpenAIAPIFunc = originalCallFunc
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
		postList.AddOrder("post2")
		postList.AddOrder("post1") // Newest first

		// numMessages+10 is used in processMaestroTask
		apiMock.Mock.On("GetPostsForChannel", channelID, 0, numMessages+10).Return(postList, (*model.AppError)(nil)).Once()

		apiMock.Mock.On("CreatePost", mock.MatchedBy(func(post *model.Post) bool {
			return post.UserId == botUserID &&
				post.ChannelId == channelID &&
				post.Message == expectedOpenAIResponse
			//post.RootId == rootID
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
			return post.Message == expectedOpenAIResponse
			//&& post.RootId == rootID
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
		postList.AddPost(&model.Post{Id: rootID, UserId: userID, Message: "!maestro " + taskName, CreateAt: 3500}) // The trigger message
		postList.AddPost(&model.Post{Id: "post3", UserId: "userB", Message: "user message 2", CreateAt: 3000})
		postList.AddPost(&model.Post{Id: "post2", UserId: "userC", Message: "!maestro other task", CreateAt: 2000}) // Should be filtered
		postList.AddPost(&model.Post{Id: "post1", UserId: "userD", Message: "user message 1", CreateAt: 1000})
		postList.AddOrder("post5")
		postList.AddOrder("post4")
		postList.AddOrder(rootID)
		postList.AddOrder("post3")
		postList.AddOrder("post2")
		postList.AddOrder("post1")

		apiMock.Mock.On("GetPostsForChannel", channelID, 0, numMessages+10).Return(postList, (*model.AppError)(nil)).Once()
		apiMock.Mock.On("CreatePost", mock.AnythingOfType("*model.Post")).Return(&model.Post{}, (*model.AppError)(nil)).Once()

		err := p.processMaestroTask(taskName, numMessages, channelID, userID, rootID)
		assert.NoError(t, err)

		expectedMessages := "user message 1\nuser message 2\nuser message 3"
		expectedFullPrompt := fmt.Sprintf("User query: %s\n%s", taskName, expectedMessages)
		assert.Equal(t, expectedFullPrompt, capturedPrompt)
	})
}

func TestPostMessageToChannel_Plugin(t *testing.T) {
	const (
		botUserID   = "testbotuserid"
		channelID   = "testchannelid"
		message     = "Hello, channel!"
		newPostID   = "newpostid"
	)

	p := &Plugin{
		botUserID: botUserID,
	}
	mockAPI := &plugintest.API{}
	p.SetAPI(mockAPI)

	mockAPI.On("LogError", mock.AnythingOfType("string"), mock.Anything).Maybe() // Generic mock for logging

	t.Run("Successful message posting", func(t *testing.T) {
		expectedPost := &model.Post{Id: newPostID, ChannelId: channelID, Message: message, UserId: botUserID}
		mockAPI.On("CreatePost", mock.MatchedBy(func(post *model.Post) bool {
			return post.ChannelId == channelID && post.Message == message && post.UserId == botUserID
		})).Return(expectedPost, nil).Once()

		createdPost, err := p.PostMessageToChannel(channelID, message)
		assert.NoError(t, err)
		assert.Equal(t, expectedPost, createdPost)
		mockAPI.AssertExpectations(t)
	})

	t.Run("CreatePost fails", func(t *testing.T) {
		appErr := model.NewAppError("CreatePost", "api.post.create_post.app_error", nil, "Cannot create post", http.StatusInternalServerError)
		mockAPI.On("CreatePost", mock.MatchedBy(func(post *model.Post) bool {
			return post.ChannelId == channelID && post.Message == message && post.UserId == botUserID
		})).Return(nil, appErr).Once()

		_, err := p.PostMessageToChannel(channelID, message)
		assert.Error(t, err)
		assert.True(t, errors.Is(err, appErr))
		mockAPI.AssertExpectations(t)
	})

	t.Run("Empty channelID", func(t *testing.T) {
		_, err := p.PostMessageToChannel("", message)
		assert.Error(t, err)
		assert.EqualError(t, err, "PostMessageToChannel: channelID cannot be empty")
	})

	t.Run("Empty message", func(t *testing.T) {
		_, err := p.PostMessageToChannel(channelID, "")
		assert.Error(t, err)
		assert.EqualError(t, err, "PostMessageToChannel: message cannot be empty")
	})

	t.Run("botUserID not set", func(t *testing.T) {
		originalBotID := p.botUserID
		p.botUserID = ""
		defer func() { p.botUserID = originalBotID }()

		_, err := p.PostMessageToChannel(channelID, message)
		assert.Error(t, err)
		assert.EqualError(t, err, "PostMessageToChannel: botUserID is not set")
	})
}

func TestCreateTeamAndAddMembers_Plugin(t *testing.T) {
	p := &Plugin{}
	mockAPI := &plugintest.API{}
	p.SetAPI(mockAPI)
	// p.botUserID = "testbotuserid" // Not strictly required by CreateTeamAndAddMembers logic itself

	teamName := "test-team"
	teamDisplayName := "Test Team Display Name"
	userID1 := "userid1"
	userID2 := "userid2"
	createdTeamID := "newteamid"

	mockCreatedTeam := &model.Team{Id: createdTeamID, Name: teamName, DisplayName: teamDisplayName}

	// Generic mocks for logging
	mockAPI.On("LogWarn", mock.AnythingOfType("string"), mock.Anything, mock.Anything).Maybe()
	mockAPI.On("LogError", mock.AnythingOfType("string"), mock.Anything, mock.Anything, mock.Anything, mock.Anything).Maybe()


	t.Run("Successful team creation and all members added", func(t *testing.T) {
		mockAPI.On("CreateTeam", mock.MatchedBy(func(team *model.Team) bool {
			return team.Name == teamName && team.DisplayName == teamDisplayName
		})).Return(mockCreatedTeam, nil).Once()
		mockAPI.On("AddTeamMember", createdTeamID, userID1).Return(&model.TeamMember{TeamId: createdTeamID, UserId: userID1}, nil).Once()
		mockAPI.On("AddTeamMember", createdTeamID, userID2).Return(&model.TeamMember{TeamId: createdTeamID, UserId: userID2}, nil).Once()

		team, err := p.CreateTeamAndAddMembers(teamName, teamDisplayName, []string{userID1, userID2})
		assert.NoError(t, err)
		assert.Equal(t, mockCreatedTeam, team)
		mockAPI.AssertExpectations(t)
	})

	t.Run("CreateTeam fails", func(t *testing.T) {
		appErr := model.NewAppError("CreateTeam", "app.team.create_team.app_error", nil, "Cannot create team", http.StatusInternalServerError)
		mockAPI.On("CreateTeam", mock.AnythingOfType("*model.Team")).Return(nil, appErr).Once()

		team, err := p.CreateTeamAndAddMembers(teamName, teamDisplayName, []string{userID1})
		assert.Error(t, err)
		assert.True(t, errors.Is(err, appErr))
		assert.Nil(t, team)
		mockAPI.AssertExpectations(t)
	})

	t.Run("AddTeamMember fails for one user, succeeds for another", func(t *testing.T) {
		mockAPI.On("CreateTeam", mock.MatchedBy(func(team *model.Team) bool {
			return team.Name == teamName && team.DisplayName == teamDisplayName
		})).Return(mockCreatedTeam, nil).Once()
		mockAPI.On("AddTeamMember", createdTeamID, userID1).Return(&model.TeamMember{TeamId: createdTeamID, UserId: userID1}, nil).Once()
		appErrAdd := model.NewAppError("AddTeamMember", "app.team.add_user.app_error", nil, "Cannot add user", http.StatusInternalServerError)
		mockAPI.On("AddTeamMember", createdTeamID, userID2).Return(nil, appErrAdd).Once()
		// Expect LogError to be called for the failed addition
		mockAPI.On("LogError", "Failed to add user to team", "user_id", userID2, "team_id", createdTeamID, "error", appErrAdd.Error()).Once()


		team, err := p.CreateTeamAndAddMembers(teamName, teamDisplayName, []string{userID1, userID2})
		assert.NoError(t, err) // Overall operation is success (best-effort)
		assert.Equal(t, mockCreatedTeam, team)
		mockAPI.AssertExpectations(t)
	})

	t.Run("Empty teamName", func(t *testing.T) {
		team, err := p.CreateTeamAndAddMembers("", teamDisplayName, []string{})
		assert.Error(t, err)
		assert.EqualError(t, err, "CreateTeamAndAddMembers: teamName cannot be empty")
		assert.Nil(t, team)
	})

	t.Run("Empty teamDisplayName", func(t *testing.T) {
		team, err := p.CreateTeamAndAddMembers(teamName, "", []string{})
		assert.Error(t, err)
		assert.EqualError(t, err, "CreateTeamAndAddMembers: teamDisplayName cannot be empty")
		assert.Nil(t, team)
	})
	
	t.Run("Empty userID in list is skipped with warning", func(t *testing.T) {
		mockAPI.On("CreateTeam", mock.AnythingOfType("*model.Team")).Return(mockCreatedTeam, nil).Once()
		mockAPI.On("AddTeamMember", createdTeamID, userID1).Return(&model.TeamMember{TeamId: createdTeamID, UserId: userID1}, nil).Once()
		// Expect LogWarn to be called for the empty userID
		mockAPI.On("LogWarn", "CreateTeamAndAddMembers: Skipping empty userID.", "team_id", createdTeamID).Once()
		mockAPI.On("AddTeamMember", createdTeamID, userID2).Return(&model.TeamMember{TeamId: createdTeamID, UserId: userID2}, nil).Once()


		team, err := p.CreateTeamAndAddMembers(teamName, teamDisplayName, []string{userID1, "", userID2})
		assert.NoError(t, err)
		assert.Equal(t, mockCreatedTeam, team)
		mockAPI.AssertExpectations(t)
		// Ensure AddTeamMember was not called with an empty string
		// AssertNotCalled is tricky with current setup if it was never expected.
		// The fact that specific LogWarn is called and other AddTeamMember calls are asserted implies it was skipped.
	})
}

func TestCreateChannelAndAddMembers_Plugin(t *testing.T) {
	p := &Plugin{}
	mockAPI := &plugintest.API{}
	p.SetAPI(mockAPI)

	teamID := "testteamid"
	channelName := "test-channel"
	channelDisplayName := "Test Channel Display Name"
	userID1 := "userid1"
	userID2 := "userid2"
	createdChannelID := "newchannelid"

	mockCreatedPublicChannel := &model.Channel{Id: createdChannelID, Name: channelName, DisplayName: channelDisplayName, Type: model.ChannelTypeOpen, TeamId: teamID}
	mockCreatedPrivateChannel := &model.Channel{Id: createdChannelID, Name: channelName, DisplayName: channelDisplayName, Type: model.ChannelTypePrivate, TeamId: teamID}

	// Generic mocks for logging
	mockAPI.On("LogWarn", mock.AnythingOfType("string"), mock.Anything, mock.Anything).Maybe()
	mockAPI.On("LogError", mock.AnythingOfType("string"), mock.Anything, mock.Anything, mock.Anything, mock.Anything).Maybe()


	t.Run("Successful public channel creation and member addition", func(t *testing.T) {
		mockAPI.On("CreateChannel", mock.MatchedBy(func(ch *model.Channel) bool {
			return ch.TeamId == teamID && ch.Name == channelName && ch.DisplayName == channelDisplayName && ch.Type == model.ChannelTypeOpen
		})).Return(mockCreatedPublicChannel, nil).Once()
		mockAPI.On("AddChannelMember", createdChannelID, userID1).Return(&model.ChannelMember{}, nil).Once()
		mockAPI.On("AddChannelMember", createdChannelID, userID2).Return(&model.ChannelMember{}, nil).Once()

		channel, err := p.CreateChannelAndAddMembers(teamID, channelName, channelDisplayName, model.ChannelTypeOpen, []string{userID1, userID2})
		assert.NoError(t, err)
		assert.Equal(t, mockCreatedPublicChannel, channel)
		mockAPI.AssertExpectations(t)
	})

	t.Run("Successful private channel creation", func(t *testing.T) {
		mockAPI.On("CreateChannel", mock.MatchedBy(func(ch *model.Channel) bool {
			return ch.TeamId == teamID && ch.Type == model.ChannelTypePrivate
		})).Return(mockCreatedPrivateChannel, nil).Once()
		
		// No members to add in this specific sub-test for simplicity, focusing on type
		channel, err := p.CreateChannelAndAddMembers(teamID, channelName, channelDisplayName, model.ChannelTypePrivate, []string{})
		assert.NoError(t, err)
		assert.Equal(t, mockCreatedPrivateChannel, channel)
		mockAPI.AssertExpectations(t)
	})

	t.Run("CreateChannel fails", func(t *testing.T) {
		appErr := model.NewAppError("CreateChannel", "app.channel.create_channel.app_error", nil, "Cannot create channel", http.StatusInternalServerError)
		mockAPI.On("CreateChannel", mock.AnythingOfType("*model.Channel")).Return(nil, appErr).Once()

		channel, err := p.CreateChannelAndAddMembers(teamID, channelName, channelDisplayName, model.ChannelTypeOpen, []string{userID1})
		assert.Error(t, err)
		assert.True(t, errors.Is(err, appErr))
		assert.Nil(t, channel)
		mockAPI.AssertExpectations(t)
	})

	t.Run("AddChannelMember fails for one user", func(t *testing.T) {
		mockAPI.On("CreateChannel", mock.AnythingOfType("*model.Channel")).Return(mockCreatedPublicChannel, nil).Once()
		mockAPI.On("AddChannelMember", createdChannelID, userID1).Return(&model.ChannelMember{}, nil).Once()
		appErrAdd := model.NewAppError("AddChannelMember", "app.channel.add_user.app_error", nil, "Cannot add user to channel", http.StatusInternalServerError)
		mockAPI.On("AddChannelMember", createdChannelID, userID2).Return(nil, appErrAdd).Once()
		mockAPI.On("LogError", "Failed to add user to channel", "user_id", userID2, "channel_id", createdChannelID, "error", appErrAdd.Error()).Once()


		channel, err := p.CreateChannelAndAddMembers(teamID, channelName, channelDisplayName, model.ChannelTypeOpen, []string{userID1, userID2})
		assert.NoError(t, err) // Best-effort
		assert.Equal(t, mockCreatedPublicChannel, channel)
		mockAPI.AssertExpectations(t)
	})

	t.Run("Empty teamID", func(t *testing.T) {
		_, err := p.CreateChannelAndAddMembers("", channelName, channelDisplayName, model.ChannelTypeOpen, []string{})
		assert.Error(t, err)
		assert.EqualError(t, err, "CreateChannelAndAddMembers: teamID cannot be empty")
	})
	t.Run("Empty channelName", func(t *testing.T) {
		_, err := p.CreateChannelAndAddMembers(teamID, "", channelDisplayName, model.ChannelTypeOpen, []string{})
		assert.Error(t, err)
		assert.EqualError(t, err, "CreateChannelAndAddMembers: channelName cannot be empty")
	})
	t.Run("Empty channelDisplayName", func(t *testing.T) {
		_, err := p.CreateChannelAndAddMembers(teamID, channelName, "", model.ChannelTypeOpen, []string{})
		assert.Error(t, err)
		assert.EqualError(t, err, "CreateChannelAndAddMembers: channelDisplayName cannot be empty")
	})
	t.Run("Invalid channelType", func(t *testing.T) {
		_, err := p.CreateChannelAndAddMembers(teamID, channelName, channelDisplayName, "X", []string{}) // Invalid type
		assert.Error(t, err)
		assert.EqualError(t, err, "CreateChannelAndAddMembers: invalid channelType 'X'. Must be 'O' or 'P'")
	})
	
	t.Run("Empty userID in list is skipped with warning for channel", func(t *testing.T) {
		mockAPI.On("CreateChannel", mock.AnythingOfType("*model.Channel")).Return(mockCreatedPublicChannel, nil).Once()
		mockAPI.On("AddChannelMember", createdChannelID, userID1).Return(&model.ChannelMember{}, nil).Once()
		mockAPI.On("LogWarn", "CreateChannelAndAddMembers: Skipping empty userID.", "channel_id", createdChannelID).Once()
		mockAPI.On("AddChannelMember", createdChannelID, userID2).Return(&model.ChannelMember{}, nil).Once()

		channel, err := p.CreateChannelAndAddMembers(teamID, channelName, channelDisplayName, model.ChannelTypeOpen, []string{userID1, "", userID2})
		assert.NoError(t, err)
		assert.Equal(t, mockCreatedPublicChannel, channel)
		mockAPI.AssertExpectations(t)
	})
}

func TestReplyToMessage_Plugin(t *testing.T) {
	const (
		botUserID        = "testbotuserid"
		channelID        = "testchannelid"
		rootPostID       = "rootpostid"
		childPostID      = "childpostid"
		actualRootID     = "actualrootid" // Root ID for the thread childPostID belongs to
		replyMessage     = "This is a reply."
		createdReplyID   = "createdreplyid"
	)

	p := &Plugin{
		botUserID: botUserID,
	}
	mockAPI := &plugintest.API{}
	p.SetAPI(mockAPI)

	mockAPI.On("LogError", mock.AnythingOfType("string"), mock.Anything).Maybe() // Generic mock for logging

	t.Run("Successful reply to a root post", func(t *testing.T) {
		originalPost := &model.Post{Id: rootPostID, ChannelId: channelID, RootId: ""}
		mockAPI.On("GetPost", rootPostID).Return(originalPost, nil).Once()
		mockAPI.On("CreatePost", mock.MatchedBy(func(post *model.Post) bool {
			return post.UserId == botUserID &&
				post.ChannelId == channelID &&
				post.Message == replyMessage &&
				post.RootId == rootPostID // Reply should be to the rootPostID itself
		})).Return(&model.Post{Id: createdReplyID, RootId: rootPostID}, nil).Once()

		createdPost, err := p.ReplyToMessage(rootPostID, replyMessage)
		assert.NoError(t, err)
		assert.NotNil(t, createdPost)
		assert.Equal(t, createdReplyID, createdPost.Id)
		assert.Equal(t, rootPostID, createdPost.RootId)
		mockAPI.AssertExpectations(t)
	})

	t.Run("Successful reply to a thread post (child post)", func(t *testing.T) {
		originalPost := &model.Post{Id: childPostID, ChannelId: channelID, RootId: actualRootID}
		mockAPI.On("GetPost", childPostID).Return(originalPost, nil).Once()
		mockAPI.On("CreatePost", mock.MatchedBy(func(post *model.Post) bool {
			return post.UserId == botUserID &&
				post.ChannelId == channelID &&
				post.Message == replyMessage &&
				post.RootId == actualRootID // Reply should be to the actualRootID of the thread
		})).Return(&model.Post{Id: createdReplyID, RootId: actualRootID}, nil).Once()

		createdPost, err := p.ReplyToMessage(childPostID, replyMessage)
		assert.NoError(t, err)
		assert.NotNil(t, createdPost)
		assert.Equal(t, createdReplyID, createdPost.Id)
		assert.Equal(t, actualRootID, createdPost.RootId)
		mockAPI.AssertExpectations(t)
	})

	t.Run("GetPost fails (post not found)", func(t *testing.T) {
		appErr := model.NewAppError("GetPost", "store.sql_post.get.app_error", nil, "Post not found", http.StatusNotFound)
		mockAPI.On("GetPost", "nonexistentpostid").Return(nil, appErr).Once()

		_, err := p.ReplyToMessage("nonexistentpostid", replyMessage)
		assert.Error(t, err)
		assert.True(t, errors.Is(err, appErr))
		mockAPI.AssertExpectations(t)
	})

	t.Run("CreatePost fails", func(t *testing.T) {
		originalPost := &model.Post{Id: rootPostID, ChannelId: channelID, RootId: ""}
		mockAPI.On("GetPost", rootPostID).Return(originalPost, nil).Once()
		appErr := model.NewAppError("CreatePost", "api.post.create_post.app_error", nil, "Cannot create post", http.StatusInternalServerError)
		mockAPI.On("CreatePost", mock.AnythingOfType("*model.Post")).Return(nil, appErr).Once()

		_, err := p.ReplyToMessage(rootPostID, replyMessage)
		assert.Error(t, err)
		assert.True(t, errors.Is(err, appErr))
		mockAPI.AssertExpectations(t)
	})

	t.Run("Empty postID", func(t *testing.T) {
		_, err := p.ReplyToMessage("", replyMessage)
		assert.Error(t, err)
		assert.EqualError(t, err, "ReplyToMessage: postID cannot be empty")
	})

	t.Run("Empty message", func(t *testing.T) {
		_, err := p.ReplyToMessage(rootPostID, "")
		assert.Error(t, err)
		assert.EqualError(t, err, "ReplyToMessage: message cannot be empty")
	})

	t.Run("botUserID not set", func(t *testing.T) {
		originalBotID := p.botUserID
		p.botUserID = ""
		defer func() { p.botUserID = originalBotID }()

		_, err := p.ReplyToMessage(rootPostID, replyMessage)
		assert.Error(t, err)
		assert.EqualError(t, err, "ReplyToMessage: botUserID is not set")
	})
}

func TestFindTeamAndMembers_Plugin(t *testing.T) {
	p := &Plugin{}
	mockAPI := &plugintest.API{}
	p.SetAPI(mockAPI)

	teamID := "testteamid"
	existingTeam := &model.Team{Id: teamID, DisplayName: "Test Team"}
	teamMembers := []*model.TeamMember{
		{TeamId: teamID, UserId: "user1"},
		{TeamId: teamID, UserId: "user2"},
	}

	mockAPI.On("LogError", mock.AnythingOfType("string"), mock.Anything).Maybe()


	t.Run("Team and members found successfully", func(t *testing.T) {
		mockAPI.On("GetTeam", teamID).Return(existingTeam, nil).Once()
		mockAPI.On("GetTeamMembers", teamID, 0, 100).Return(teamMembers, nil).Once()

		team, members, err := p.FindTeamAndMembers(teamID)

		assert.NoError(t, err)
		assert.Equal(t, existingTeam, team)
		assert.Equal(t, teamMembers, members)
		mockAPI.AssertExpectations(t)
	})

	t.Run("Team not found", func(t *testing.T) {
		appErr := model.NewAppError("GetTeam", "store.sql_team.get.app_error", nil, "team not found", http.StatusNotFound)
		mockAPI.On("GetTeam", teamID).Return(nil, appErr).Once()

		team, members, err := p.FindTeamAndMembers(teamID)

		assert.Error(t, err)
		assert.Nil(t, team)
		assert.Nil(t, members)
		assert.True(t, errors.Is(err, appErr))
		mockAPI.AssertExpectations(t)
	})

	t.Run("GetTeam returns other AppError", func(t *testing.T) {
		appErr := model.NewAppError("GetTeam", "some.error", nil, "internal error", http.StatusInternalServerError)
		mockAPI.On("GetTeam", teamID).Return(nil, appErr).Once()

		team, members, err := p.FindTeamAndMembers(teamID)
		assert.Error(t, err)
		assert.Nil(t, team)
		assert.Nil(t, members)
		assert.True(t, errors.Is(err, appErr))
		mockAPI.AssertExpectations(t)
	})
	
	t.Run("GetTeamMembers returns AppError", func(t *testing.T) {
		mockAPI.On("GetTeam", teamID).Return(existingTeam, nil).Once()
		appErr := model.NewAppError("GetTeamMembers", "store.sql_team.get_members.app_error", nil, "error getting members", http.StatusInternalServerError)
		mockAPI.On("GetTeamMembers", teamID, 0, 100).Return(nil, appErr).Once()

		team, members, err := p.FindTeamAndMembers(teamID)

		assert.Error(t, err)
		assert.NotNil(t, team) // Team was found
		assert.Equal(t, existingTeam, team)
		assert.Nil(t, members)
		assert.True(t, errors.Is(err, appErr))
		mockAPI.AssertExpectations(t)
	})

	t.Run("Empty teamID", func(t *testing.T) {
		team, members, err := p.FindTeamAndMembers("")
		assert.Error(t, err)
		assert.EqualError(t, err, "FindTeamAndMembers: teamID cannot be empty")
		assert.Nil(t, team)
		assert.Nil(t, members)
	})
}

func TestFindChannelAndMembers_Plugin(t *testing.T) {
	p := &Plugin{}
	mockAPI := &plugintest.API{}
	p.SetAPI(mockAPI)

	channelID := "testchannelid"
	existingChannel := &model.Channel{Id: channelID, DisplayName: "Test Channel", TeamId: "testteamid"}
	channelMembers := model.ChannelMembers{
		{ChannelId: channelID, UserId: "user1"},
		{ChannelId: channelID, UserId: "user2"},
	}
	
	mockAPI.On("LogError", mock.AnythingOfType("string"), mock.Anything).Maybe()


	t.Run("Channel and members found successfully", func(t *testing.T) {
		mockAPI.On("GetChannel", channelID).Return(existingChannel, nil).Once()
		mockAPI.On("GetChannelMembers", channelID, 0, 100).Return(&channelMembers, nil).Once()

		channel, members, err := p.FindChannelAndMembers(channelID)

		assert.NoError(t, err)
		assert.Equal(t, existingChannel, channel)
		assert.Equal(t, channelMembers, members)
		mockAPI.AssertExpectations(t)
	})

	t.Run("Channel not found", func(t *testing.T) {
		appErr := model.NewAppError("GetChannel", "store.sql_channel.get.app_error", nil, "channel not found", http.StatusNotFound)
		mockAPI.On("GetChannel", channelID).Return(nil, appErr).Once()

		channel, members, err := p.FindChannelAndMembers(channelID)

		assert.Error(t, err)
		assert.Nil(t, channel)
		assert.Nil(t, members)
		assert.True(t, errors.Is(err, appErr))
		mockAPI.AssertExpectations(t)
	})
	
	t.Run("GetChannel returns other AppError", func(t *testing.T) {
		appErr := model.NewAppError("GetChannel", "some.error", nil, "internal error", http.StatusInternalServerError)
		mockAPI.On("GetChannel", channelID).Return(nil, appErr).Once()
		
		channel, members, err := p.FindChannelAndMembers(channelID)
		assert.Error(t, err)
		assert.Nil(t, channel)
		assert.Nil(t, members)
		assert.True(t, errors.Is(err, appErr))
		mockAPI.AssertExpectations(t)
	})

	t.Run("GetChannelMembers returns AppError", func(t *testing.T) {
		mockAPI.On("GetChannel", channelID).Return(existingChannel, nil).Once()
		appErr := model.NewAppError("GetChannelMembers", "store.sql_channel.get_members.app_error", nil, "error getting members", http.StatusInternalServerError)
		mockAPI.On("GetChannelMembers", channelID, 0, 100).Return(nil, appErr).Once()

		channel, members, err := p.FindChannelAndMembers(channelID)

		assert.Error(t, err)
		assert.NotNil(t, channel) // Channel was found
		assert.Equal(t, existingChannel, channel)
		assert.Nil(t, members)
		assert.True(t, errors.Is(err, appErr))
		mockAPI.AssertExpectations(t)
	})
	
	t.Run("GetChannelMembers returns nil, nil (no members, no error)", func(t *testing.T) {
		mockAPI.On("GetChannel", channelID).Return(existingChannel, nil).Once()
		mockAPI.On("GetChannelMembers", channelID, 0, 100).Return(nil, nil).Once()

		channel, members, err := p.FindChannelAndMembers(channelID)

		assert.NoError(t, err)
		assert.NotNil(t, channel)
		assert.Equal(t, existingChannel, channel)
		assert.Empty(t, members) // Should be an empty slice, not nil
		mockAPI.AssertExpectations(t)
	})

	t.Run("Empty channelID", func(t *testing.T) {
		channel, members, err := p.FindChannelAndMembers("")
		assert.Error(t, err)
		assert.EqualError(t, err, "FindChannelAndMembers: channelID cannot be empty")
		assert.Nil(t, channel)
		assert.Nil(t, members)
	})
}

func TestMessageUser_Plugin(t *testing.T) {
	const (
		testBotUserID = "testbotuserid"
		targetUserID  = "targetuserid"
		testChannelID = "testchannelid"
		validMessage  = "Hello, user!"
	)

	p := &Plugin{
		botUserID: testBotUserID,
	}
	// Use plugintest.API for mocking API calls directly
	mockAPI := &plugintest.API{}
	p.SetAPI(mockAPI)

	// Default mock for LogError to prevent test failures if it's called unexpectedly
	mockAPI.On("LogError", mock.AnythingOfType("string"), mock.Anything).Maybe()


	t.Run("Successful message sending", func(t *testing.T) {
		// Reset mocks for this specific subtest to ensure clean state
		mockAPI.On("GetDirectChannel", testBotUserID, targetUserID).Return(&model.Channel{Id: testChannelID}, nil).Once()
		mockAPI.On("CreatePost", mock.MatchedBy(func(post *model.Post) bool {
			return post.ChannelId == testChannelID && post.Message == validMessage && post.UserId == testBotUserID
		})).Return(&model.Post{}, nil).Once()

		err := p.MessageUser(targetUserID, validMessage)
		assert.NoError(t, err)
		mockAPI.AssertExpectations(t)
	})

	t.Run("GetDirectChannel fails", func(t *testing.T) {
		appErr := model.NewAppError("GetDirectChannel", "some.error", nil, "could not get direct channel", http.StatusInternalServerError)
		mockAPI.On("GetDirectChannel", testBotUserID, targetUserID).Return(nil, appErr).Once()

		err := p.MessageUser(targetUserID, validMessage)
		assert.Error(t, err)
		assert.True(t, errors.Is(err, appErr), "Error should wrap the AppError from GetDirectChannel")
		mockAPI.AssertExpectations(t) // Ensure GetDirectChannel was called
		mockAPI.AssertNotCalled(t, "CreatePost", mock.Anything) // Ensure CreatePost was not called
	})

	t.Run("CreatePost fails", func(t *testing.T) {
		mockAPI.On("GetDirectChannel", testBotUserID, targetUserID).Return(&model.Channel{Id: testChannelID}, nil).Once()
		appErr := model.NewAppError("CreatePost", "some.error", nil, "could not create post", http.StatusInternalServerError)
		// Need to ensure the mock for CreatePost is specific enough or reset if it was called in other tests.
		// Using a fresh mock for CreatePost within this sub-test if needed, or ensuring .Once() is respected.
		// For plugintest.API, the expectation set by On() is generally for a single call unless Times() is used.
		mockAPI.On("CreatePost", mock.MatchedBy(func(post *model.Post) bool {
			return post.ChannelId == testChannelID && post.Message == validMessage && post.UserId == testBotUserID
		})).Return(nil, appErr).Once()


		err := p.MessageUser(targetUserID, validMessage)
		assert.Error(t, err)
		assert.True(t, errors.Is(err, appErr), "Error should wrap the AppError from CreatePost")
		mockAPI.AssertExpectations(t)
	})

	t.Run("Empty userID", func(t *testing.T) {
		err := p.MessageUser("", validMessage)
		assert.Error(t, err)
		assert.EqualError(t, err, "MessageUser: userID cannot be empty")
		mockAPI.AssertNotCalled(t, "GetDirectChannel", mock.Anything, mock.Anything)
		mockAPI.AssertNotCalled(t, "CreatePost", mock.Anything)
	})

	t.Run("Empty message", func(t *testing.T) {
		err := p.MessageUser(targetUserID, "")
		assert.Error(t, err)
		assert.EqualError(t, err, "MessageUser: message cannot be empty")
		mockAPI.AssertNotCalled(t, "GetDirectChannel", mock.Anything, mock.Anything)
		mockAPI.AssertNotCalled(t, "CreatePost", mock.Anything)
	})

	t.Run("botUserID not set", func(t *testing.T) {
		originalBotID := p.botUserID
		p.botUserID = ""
		defer func() { p.botUserID = originalBotID }() // Restore

		err := p.MessageUser(targetUserID, validMessage)
		assert.Error(t, err)
		assert.EqualError(t, err, "MessageUser: botUserID is not set")
		mockAPI.AssertNotCalled(t, "GetDirectChannel", mock.Anything, mock.Anything)
		mockAPI.AssertNotCalled(t, "CreatePost", mock.Anything)
	})
}

func TestMessageHasBeenPosted(t *testing.T) {
	botUserID := "testbotuserid"
	userID := "testactiveuserid"
	channelID := "testchannelid"

	// Store original CallOpenAIAPIFunc and OpenAIAPIURL to restore them later
	originalCallFunc := CallOpenAIAPIFunc
	defer func() {
		CallOpenAIAPIFunc = originalCallFunc
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
			return p.Message == "AI summary."
			//&& p.RootId == post.Id
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

	//t.Run("!maestro command with parsing error", func(t *testing.T) {
	//	p, apiMock, post := setupPluginAndPost("!maestro -n abc", userID) // Invalid -n value
	//	defer apiMock.Mock.AssertExpectations(t)
	//
	//	apiMock.Mock.On("LogInfo", "Detected '!maestro' prefix.", "user_id", userID, "channel_id", channelID, "original_message", post.Message, "arguments_string", "-n abc").Once()
	//
	//	// Ephemeral post for parsing error
	//	expectedErrorMsg := "task_name cannot be empty. Usage: !maestro <task_name> [-n <num_messages>]"
	//	// The actual error from parseMaestroArgs for "-n abc" would be "task_name cannot be empty..."
	//	// because "-n" itself isn't a task name. If it was "task -n abc", it would be "invalid value for -n..."
	//	// Let's use a command that gives a more direct parsing error for -n:
	//	post.Message = "!maestro mytask -n abc"
	//	argumentsString := "mytask -n abc"
	//	apiMock.Mock.On("LogInfo", "Detected '!maestro' prefix.", "user_id", userID, "channel_id", channelID, "original_message", post.Message, "arguments_string", argumentsString).Once()
	//	expectedErrorMsg = "invalid value for -n: 'abc'. It must be an integer"
	//
	//	apiMock.Mock.On("SendEphemeralPost", userID, mock.MatchedBy(func(ephemeralPost *model.Post) bool {
	//		return ephemeralPost.ChannelId == channelID &&
	//			strings.Contains(ephemeralPost.Message, expectedErrorMsg) &&
	//			ephemeralPost.RootId == post.Id
	//	})).Return(&model.Post{}).Once()
	//
	//	apiMock.Mock.On("LogError", "Failed to parse arguments for !maestro command", "error", expectedErrorMsg, "user_id", userID, "arguments", argumentsString).Once()
	//
	//	p.MessageHasBeenPosted(nil, post)
	//})

	t.Run("!maestro command where processMaestroTask returns an error (e.g. API key empty)", func(t *testing.T) {
		p, apiMock, post := setupPluginAndPost("!maestro summarize", userID)
		p.configuration.OpenAIAPIKey = "" // Ensure API key is empty for this test
		defer apiMock.Mock.AssertExpectations(t)

		apiMock.Mock.On("LogInfo", "Detected '!maestro' prefix.", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Once()

		// processMaestroTask will send its own ephemeral post
		apiMock.Mock.On("SendEphemeralPost", userID, mock.MatchedBy(func(ephemeralPost *model.Post) bool {
			return strings.Contains(ephemeralPost.Message, "OpenAI API Key is not configured")
			//&& ephemeralPost.RootId == post.Id
		})).Return(&model.Post{}).Once()

		// And MessageHasBeenPosted will log the error from processMaestroTask
		apiMock.Mock.On("LogError", "Error processing !maestro task", "error", "OpenAI API Key is not configured", "user_id", userID, "task_name", "summarize").Once()

		p.MessageHasBeenPosted(nil, post)
	})
}

func TestFindUser_Plugin(t *testing.T) {
	p := &Plugin{}
	mockAPI := &plugintest.API{} // Using plugintest.API for simplicity if methods are already there
	                             // Or, if we added GetUser to our MockPluginAPI, we'd use that:
	                             // mockAPI := &MockPluginAPI{}
	p.SetAPI(mockAPI)          // Make sure plugin instance uses the mock

	userIDExists := "exists"
	userIDNotExists := "notexists"
	userIDError := "error"

	expectedUser := &model.User{Id: userIDExists, Username: "testuser"}

	// Test Case 1: User Found
	t.Run("User Found", func(t *testing.T) {
		// If using plugintest.API directly:
		mockAPI.On("GetUser", userIDExists).Return(expectedUser, nil).Once()
		// If using extended MockPluginAPI:
		// mockAPI.Mock.On("GetUser", userIDExists).Return(expectedUser, (*model.AppError)(nil)).Once()


		user, err := p.FindUser(userIDExists)

		assert.NoError(t, err)
		assert.NotNil(t, user)
		assert.Equal(t, expectedUser.Id, user.Id)
		assert.Equal(t, expectedUser.Username, user.Username)
		mockAPI.AssertExpectations(t) // Verify that the mock was called as expected
	})

	// Test Case 2: User Not Found (AppError)
	t.Run("User Not Found", func(t *testing.T) {
		appErrNotFound := model.NewAppError("GetUser", "store.sql_user.get.app_error", nil, "user_id="+userIDNotExists, http.StatusNotFound)
		mockAPI.On("GetUser", userIDNotExists).Return(nil, appErrNotFound).Once()
		// mockAPI.Mock.On("GetUser", userIDNotExists).Return((*model.User)(nil), appErrNotFound).Once()

		user, err := p.FindUser(userIDNotExists)

		assert.Error(t, err)
		assert.Nil(t, user)
		// Check if the returned error wraps the AppError
		assert.True(t, errors.Is(err, appErrNotFound), "Error should wrap the AppError from GetUser")
		var actualAppErr *model.AppError
		if errors.As(err, &actualAppErr) {
			assert.Equal(t, http.StatusNotFound, actualAppErr.StatusCode)
		} else {
			t.Errorf("Expected error to be or wrap a *model.AppError")
		}
		mockAPI.AssertExpectations(t)
	})

	// Test Case 3: Other API Error (AppError)
	t.Run("Other API Error", func(t *testing.T) {
		appErrInternal := model.NewAppError("GetUser", "some.internal.error", nil, "internal server error", http.StatusInternalServerError)
		mockAPI.On("GetUser", userIDError).Return(nil, appErrInternal).Once()
		// mockAPI.Mock.On("GetUser", userIDError).Return((*model.User)(nil), appErrInternal).Once()

		user, err := p.FindUser(userIDError)

		assert.Error(t, err)
		assert.Nil(t, user)
		assert.True(t, errors.Is(err, appErrInternal), "Error should wrap the AppError from GetUser")
		var actualAppErr *model.AppError
		if errors.As(err, &actualAppErr) {
			assert.Equal(t, http.StatusInternalServerError, actualAppErr.StatusCode)
		} else {
			t.Errorf("Expected error to be or wrap a *model.AppError")
		}
		mockAPI.AssertExpectations(t)
	})
}
