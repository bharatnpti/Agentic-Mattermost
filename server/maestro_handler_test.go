package main

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

const DefaultTestNumMessages = 10 // Matching the one in maestro_handler.go

func TestParseMaestroArgsNewFormat(t *testing.T) {
	mockAPI := &plugintest.API{}
	// For LogInfo, we can use On("LogInfo", ...).Return() if we want to assert logging.
	// Or, if LogInfo is not critical to the parsing logic, we can allow any call.
	mockAPI.On("LogInfo", mock.Anything, mock.Anything, mock.Anything).Maybe()

	// Need a minimal handler instance to call the method.
	// Only API is used by parseMaestroArgsNewFormat for logging.
	handler := &MaestroHandler{API: mockAPI}

	tests := []struct {
		name            string
		argsString      string
		expectedTask    string
		expectedNumMsgs int
		expectedErr     bool
		expectedErrMsg  string
	}{
		{
			name:            "Simple task text",
			argsString:      "do something",
			expectedTask:    "do something",
			expectedNumMsgs: DefaultTestNumMessages,
			expectedErr:     false,
		},
		{
			name:            "Task text with -n flag first",
			argsString:      "-n 5 summarize this",
			expectedTask:    "summarize this",
			expectedNumMsgs: 5,
			expectedErr:     false,
		},
		{
			name:            "Task text with -n flag at end (should be ignored as not first)",
			argsString:      "summarize this -n 5",
			expectedTask:    "summarize this -n 5",
			expectedNumMsgs: DefaultTestNumMessages, // -n 5 is part of task
			expectedErr:     false,
		},
		{
			name:            "Only -n flag with valid number",
			argsString:      "-n 7",
			expectedTask:    "", // No task text
			expectedNumMsgs: 7,
			expectedErr:     false,
		},
		{
			name:            "Empty string",
			argsString:      "",
			expectedTask:    "",
			expectedNumMsgs: DefaultTestNumMessages,
			expectedErr:     false,
		},
		{
			name:            "-n flag with invalid number",
			argsString:      "-n abc summarize",
			expectedTask:    "",
			expectedNumMsgs: 0,
			expectedErr:     true,
			expectedErrMsg:  "invalid value for -n: 'abc'. It must be an integer",
		},
		{
			name:            "-n flag with negative number",
			argsString:      "-n -2 summarize",
			expectedTask:    "",
			expectedNumMsgs: 0,
			expectedErr:     true,
			expectedErrMsg:  "invalid value for -n: -2. It must be a positive integer",
		},
		{
			name:            "-n flag with zero",
			argsString:      "-n 0 summarize",
			expectedTask:    "",
			expectedNumMsgs: 0,
			expectedErr:     true,
			expectedErrMsg:  "invalid value for -n: 0. It must be a positive integer",
		},
		{
			name:            "-n flag without value",
			argsString:      "-n", // This will be treated as task " -n"
			expectedTask:    "-n",
			expectedNumMsgs: DefaultTestNumMessages,
			expectedErr:     false, // The current logic allows this; -n becomes part of task if not followed by number.
		},
		{
			name:            "Task text with leading/trailing spaces",
			argsString:      "  do task   ",
			expectedTask:    "do task",
			expectedNumMsgs: DefaultTestNumMessages,
			expectedErr:     false,
		},
		{
			name:            "Task text with internal multiple spaces",
			argsString:      "do   this   task",
			expectedTask:    "do this task", // strings.Fields handles multiple spaces
			expectedNumMsgs: DefaultTestNumMessages,
			expectedErr:     false,
		},
		{
            name:            "Only -n flag with valid number and no task",
            argsString:      "-n 15",
            expectedTask:    "",
            expectedNumMsgs: 15,
            expectedErr:     false,
        },
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task, numMsgs, err := handler.parseMaestroArgsNewFormat(tt.argsString)

			assert.Equal(t, tt.expectedTask, task)
			assert.Equal(t, tt.expectedNumMsgs, numMsgs)
			if tt.expectedErr {
				assert.Error(t, err)
				if tt.expectedErrMsg != "" {
					assert.Contains(t, err.Error(), tt.expectedErrMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Mocking CallGraphQLAgentFunc for processMaestroTask tests
type MockCallGraphQLAgentFuncType func(apiKey string, conversationID string, userID string, tenantID string, channelIDSystemContext string, messages []Message, apiURL string) (string, error)

func (m MockCallGraphQLAgentFuncType) Call(apiKey, conversationID, userID, tenantID, channelIDSystemContext string, messages []Message, apiURL string) (string, error) {
	return m(apiKey, conversationID, userID, tenantID, channelIDSystemContext, messages, apiURL)
}
func TestBuildMessages(t *testing.T) {
	botUserID := "botUserID"
	user1ID := "user1ID"
	user2ID := "user2ID"

	posts := &model.PostList{
		Order: []string{"postID1", "postID2", "postID3", "postID4"},
		Posts: map[string]*model.Post{
			"postID1": {Id: "postID1", UserId: user1ID, Message: "Hello"},
			"postID2": {Id: "postID2", UserId: botUserID, Message: "Hi there"},
			"postID3": {Id: "postID3", UserId: user2ID, Message: "Question?"},
			"postID4": {Id: "postID4", UserId: user1ID, Message: "Another one"},
		},
	}

	// No API interactions in buildMessages, so mockAPI is not strictly needed unless LogInfo was there.
	// For consistency, let's assume handler might use API for logging in future.
	mockAPI := &plugintest.API{}
	mockAPI.On("LogInfo", mock.Anything, mock.Anything, mock.Anything).Maybe()
	handler := &MaestroHandler{API: mockAPI, BotUserID: botUserID}

	expectedMessages := []Message{
		{Role: "user", Content: fmt.Sprintf("%s: %s", user1ID, "Hello"), Format: "text"},
		{Role: "assistant", Content: "Hi there", Format: "text"},
		{Role: "user", Content: fmt.Sprintf("%s: %s", user2ID, "Question?"), Format: "text"},
		{Role: "user", Content: fmt.Sprintf("%s: %s", user1ID, "Another one"), Format: "text"},
	}

	actualMessages := handler.buildMessages(posts)

	// The TurnID is random, so we can't directly compare it.
	// We'll check length and then compare other fields.
	assert.Equal(t, len(expectedMessages), len(actualMessages), "Number of messages should match")

	for i, expected := range expectedMessages {
		actual := actualMessages[i]
		assert.Equal(t, expected.Role, actual.Role, "Message %d Role mismatch", i)
		assert.Equal(t, expected.Content, actual.Content, "Message %d Content mismatch", i)
		assert.Equal(t, expected.Format, actual.Format, "Message %d Format mismatch", i)
		assert.NotEmpty(t, actual.TurnID, "Message %d TurnID should not be empty", i) // Check if TurnID is populated
	}

	// Test order (posts are reversed by buildMessages)
	// Original postList has postID1 as the first in Order, which means it's the oldest if GetPostsForChannel returns newest first.
	// buildMessages reverses this, so actualMessages[0] should correspond to posts.Order[len-1] if that's the case.
	// However, GetPostsForChannel typically returns newest first (e.g. page 0, perPage N).
	// The function buildMessages iterates this order and then reverses the resulting slice.
	// So, if posts.Order is ["post1", "post2", "post3"] (post1 newest),
	// initial messages slice before reverse: [msg1 (from post1), msg2, msg3]
	// after reverse: [msg3, msg2, msg1 (from post1)]
	// This means actualMessages[0] should correspond to the OLDEST post in the input list.
	// Let's re-verify the comment in buildMessages:
	// "Reverse messages to have the latest ones last" -> This means oldest first.
	// If posts.Order is ["postID1", "postID2", "postID3", "postID4"] (assume this is newest to oldest from GetPostsForChannel)
	// Then buildMessages processes them in this order.
	// Then it reverses them. So actualMessages[0] should be from postID4.
	// My current posts.Order: ["postID1", "postID2", "postID3", "postID4"]
	// My current expectedMessages: User1, Bot, User2, User1
	// This implies the original order of posts in the test `posts` variable was intended to be oldest to newest.
	// Let's adjust the test data to reflect how GetPostsForChannel would return them (newest first)
	// and then the reversal in buildMessages will make the final slice oldest first.

	postsReversedOrder := &model.PostList{
		Order: []string{"postID4", "postID3", "postID2", "postID1"}, // Newest (postID4) to Oldest (postID1)
		Posts: map[string]*model.Post{
			"postID1": {Id: "postID1", UserId: user1ID, Message: "Oldest message"},
			"postID2": {Id: "postID2", UserId: botUserID, Message: "Bot reply to oldest"},
			"postID3": {Id: "postID3", UserId: user2ID, Message: "Intermediate message"},
			"postID4": {Id: "postID4", UserId: user1ID, Message: "Newest message"},
		},
	}
	// Expected messages after buildMessages (which reverses): Oldest to Newest
	expectedMessagesReversed := []Message{
		{Role: "user", Content: fmt.Sprintf("%s: %s", user1ID, "Oldest message"), Format: "text"},
		{Role: "assistant", Content: "Bot reply to oldest", Format: "text"},
		{Role: "user", Content: fmt.Sprintf("%s: %s", user2ID, "Intermediate message"), Format: "text"},
		{Role: "user", Content: fmt.Sprintf("%s: %s", user1ID, "Newest message"), Format: "text"},
	}

	actualMessagesReversed := handler.buildMessages(postsReversedOrder)
	assert.Equal(t, len(expectedMessagesReversed), len(actualMessagesReversed))
	for i, expected := range expectedMessagesReversed {
		actual := actualMessagesReversed[i]
		assert.Equal(t, expected.Role, actual.Role, "Reversed Test: Message %d Role mismatch", i)
		assert.Equal(t, expected.Content, actual.Content, "Reversed Test: Message %d Content mismatch", i)
		assert.Equal(t, expected.Format, actual.Format, "Reversed Test: Message %d Format mismatch", i)
		assert.NotEmpty(t, actual.TurnID, "Reversed Test: Message %d TurnID should not be empty", i)
	}

	// Test with empty PostList
	emptyPosts := &model.PostList{Order: []string{}, Posts: map[string]*model.Post{}}
	actualEmptyMessages := handler.buildMessages(emptyPosts)
	assert.Empty(t, actualEmptyMessages, "Should return empty slice for empty PostList")
}

// Further tests for processMaestroTask and MessageHasBeenPosted will be added next.
// For processMaestroTask, we need to mock:
// - API.GetPost
// - API.GetPostsForChannel
// - API.CreatePost
// - API.SendEphemeralPost (if CreatePost fails)
// - API.LogError
// - API.LogInfo
// - GetConfig() to return a mock configuration
// - CallGraphQLAgentFunc
//
// For MessageHasBeenPosted, we need to mock:
// - API.SendEphemeralPost
// - API.LogError
// - API.LogInfo
// - (indirectly) processMaestroTask and its mocks

// Let's define a helper for creating a MaestroHandler with mocks for processMaestroTask
type processMaestroTaskMocks struct {
	API                  *plugintest.API
	mockGetConfig        func() *configuration
	mockCallGraphQL      MockCallGraphQLAgentFuncType
}

func setupMaestroHandlerForProcessTask(t *testing.T) (*MaestroHandler, processMaestroTaskMocks) {
	apiMock := &plugintest.API{}
	// Common Setup:
	apiMock.On("LogInfo", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Maybe()
	apiMock.On("LogError", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Maybe()


	mockConfig := &configuration{GraphQLAgentWebSocketURL: "ws://fake-agent.com"}
	getConfigFunc := func() *configuration { return mockConfig }

	// Default mock for CallGraphQLAgentFunc - can be overridden in specific tests
	mockCallGraphQL := MockCallGraphQLAgentFuncType(func(apiKey, conversationID, userID, tenantID, channelIDSystemContext string, messages []Message, apiURL string) (string, error) {
		return "GraphQL response", nil
	})

	handler := NewMaestroHandler(apiMock, "botUserID", getConfigFunc, mockCallGraphQL.Call)

	return handler, processMaestroTaskMocks{
		API: apiMock,
		mockGetConfig: getConfigFunc,
		mockCallGraphQL: mockCallGraphQL,
	}
}

func TestProcessMaestroTask_Success(t *testing.T) {
	handler, mocks := setupMaestroHandlerForProcessTask(t)
	channelID := "channelID"
	userID := "userID"
	rootID := "rootID"

	originalPost := &model.Post{Id: rootID, ChannelId: channelID, UserId: userID}
	postsForChannel := &model.PostList{
		Order: []string{"p1"},
		Posts: map[string]*model.Post{"p1": {Id: "p1", Message: "test message", UserId: userID}},
	}

	mocks.API.On("GetPost", rootID).Return(originalPost, nil).Once()
	mocks.API.On("GetPostsForChannel", channelID, 0, mock.AnythingOfType("int")).Return(postsForChannel, nil).Once()
	mocks.API.On("CreatePost", mock.MatchedBy(func(post *model.Post) bool {
		return post.ChannelId == channelID && post.RootId == rootID && post.Message == "GraphQL response"
	})).Return(&model.Post{}, nil).Once()

	err := handler.processMaestroTask("test task", 5, channelID, userID, rootID)
	assert.NoError(t, err)
	mocks.API.AssertExpectations(t)
}

func TestProcessMaestroTask_ErrorFetchingPosts(t *testing.T) {
	handler, mocks := setupMaestroHandlerForProcessTask(t)
	channelID := "channelID"
	userID := "userID"
	rootID := "rootID"
	
	originalPost := &model.Post{Id: rootID, ChannelId: channelID, UserId: userID}
	mocks.API.On("GetPost", rootID).Return(originalPost, nil).Once()
	mocks.API.On("GetPostsForChannel", channelID, 0, mock.AnythingOfType("int")).Return(nil, model.NewAppError("GetPostsForChannel", "some.error", nil, "Failed to fetch", http.StatusInternalServerError)).Once()
	// Expect a CreatePost for the error message
	mocks.API.On("CreatePost", mock.MatchedBy(func(post *model.Post) bool {
		return post.Message == "An error occurred while fetching messages. Please try again later."
	})).Return(&model.Post{}, nil).Once()


	err := handler.processMaestroTask("test task", 5, channelID, userID, rootID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Failed to fetch posts")
	mocks.API.AssertExpectations(t)
}

func TestProcessMaestroTask_GraphQLUrlNotConfigured(t *testing.T) {
	handler, mocks := setupMaestroHandlerForProcessTask(t)
	channelID := "channelID"
	userID := "userID"
	rootID := "rootID"

	// Override GetConfig to return empty URL
	handler.GetConfig = func() *configuration { return &configuration{GraphQLAgentWebSocketURL: ""} }

	originalPost := &model.Post{Id: rootID, ChannelId: channelID, UserId: userID}
	postsForChannel := &model.PostList{ Order: []string{"p1"}, Posts: map[string]*model.Post{"p1": {Id: "p1", Message: "test msg", UserId: userID}}}
	mocks.API.On("GetPost", rootID).Return(originalPost, nil).Once()
	mocks.API.On("GetPostsForChannel", channelID, 0, mock.AnythingOfType("int")).Return(postsForChannel, nil).Once()
	mocks.API.On("CreatePost", mock.MatchedBy(func(post *model.Post) bool {
		return post.Message == "The GraphQL Agent WebSocket URL is not configured. Please contact an administrator."
	})).Return(&model.Post{}, nil).Once()


	err := handler.processMaestroTask("test task", 5, channelID, userID, rootID)
	assert.Error(t, err)
	assert.EqualError(t, err, "GraphQL Agent WebSocket URL is not configured")
	mocks.API.AssertExpectations(t)
}

func TestProcessMaestroTask_ErrorCallingGraphQL(t *testing.T) {
	handler, mocks := setupMaestroHandlerForProcessTask(t)
	channelID := "channelID"
	userID := "userID"
	rootID := "rootID"

	// Override CallGraphQLAgentFunc to return an error
	handler.CallGraphQLAgentFunc = func(apiKey, conversationID, uID, tenantID, channelIDSystemContext string, messages []Message, apiURL string) (string, error) {
		return "", fmt.Errorf("GraphQL call failed")
	}

	originalPost := &model.Post{Id: rootID, ChannelId: channelID, UserId: userID}
	postsForChannel := &model.PostList{ Order: []string{"p1"}, Posts: map[string]*model.Post{"p1": {Id: "p1", Message: "test msg", UserId: userID}}}
	mocks.API.On("GetPost", rootID).Return(originalPost, nil).Once()
	mocks.API.On("GetPostsForChannel", channelID, 0, mock.AnythingOfType("int")).Return(postsForChannel, nil).Once()
	mocks.API.On("CreatePost", mock.MatchedBy(func(post *model.Post) bool {
		return post.Message == "An error occurred while contacting the agent. Please try again later or contact an administrator."
	})).Return(&model.Post{}, nil).Once()

	err := handler.processMaestroTask("test task", 5, channelID, userID, rootID)
	assert.Error(t, err)
	assert.EqualError(t, err, "GraphQL call failed")
	mocks.API.AssertExpectations(t)
}


func TestProcessMaestroTask_EmptyResponseFromGraphQL(t *testing.T) {
	handler, mocks := setupMaestroHandlerForProcessTask(t)
	channelID := "channelID"
	userID := "userID"
	rootID := "rootID"

	// Override CallGraphQLAgentFunc to return empty message
	handler.CallGraphQLAgentFunc = func(apiKey, conversationID, uID, tenantID, channelIDSystemContext string, messages []Message, apiURL string) (string, error) {
		return "", nil // Empty string, no error
	}
	
	originalPost := &model.Post{Id: rootID, ChannelId: channelID, UserId: userID}
	postsForChannel := &model.PostList{ Order: []string{"p1"}, Posts: map[string]*model.Post{"p1": {Id: "p1", Message: "test msg", UserId: userID}}}
	mocks.API.On("GetPost", rootID).Return(originalPost, nil).Once()
	mocks.API.On("GetPostsForChannel", channelID, 0, mock.AnythingOfType("int")).Return(postsForChannel, nil).Once()
	mocks.API.On("CreatePost", mock.MatchedBy(func(post *model.Post) bool {
		return post.Message == "The agent processed your request but returned no messages."
	})).Return(&model.Post{}, nil).Once()


	err := handler.processMaestroTask("test task", 5, channelID, userID, rootID)
	assert.NoError(t, err)
	mocks.API.AssertExpectations(t)
}

func TestMessageHasBeenPosted_NonMaestroMessage(t *testing.T) {
	apiMock := &plugintest.API{} // No calls expected for non-maestro messages
	handler := &MaestroHandler{API: apiMock, BotUserID: "botUserID"}
	post := &model.Post{UserId: "user1", Message: "Hello there"}

	handler.MessageHasBeenPosted(nil, post)
	apiMock.AssertExpectations(t) // Verify no API calls were made
}

func TestMessageHasBeenPosted_BotMessage(t *testing.T) {
	apiMock := &plugintest.API{}
	botUserID := "botUserID"
	handler := &MaestroHandler{API: apiMock, BotUserID: botUserID}
	post := &model.Post{UserId: botUserID, Message: "!maestro summarize"}

	handler.MessageHasBeenPosted(nil, post)
	apiMock.AssertExpectations(t)
}

func TestMessageHasBeenPosted_ParseError(t *testing.T) {
	apiMock := &plugintest.API{}
	apiMock.On("LogInfo", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Maybe() // For initial detection log
	apiMock.On("SendEphemeralPost", "user1", mock.AnythingOfType("*model.Post")).Return(&model.Post{}).Once()
	apiMock.On("LogError", "Failed to parse arguments for !maestro command", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Once()


	handler := &MaestroHandler{API: apiMock, BotUserID: "botUserID"}
	// This input to parseMaestroArgsNewFormat will cause an error
	post := &model.Post{UserId: "user1", Message: "!maestro -n notanumber"}

	handler.MessageHasBeenPosted(nil, post)
	apiMock.AssertExpectations(t)
}


// Note: Testing the successful path of MessageHasBeenPosted leading to processMaestroTask
// requires setting up all the mocks for processMaestroTask again.
// This can be complex. A more focused test would be to ensure MessageHasBeenPosted
// calls processMaestroTask with correct parameters if parsing is successful.
// We can achieve this by creating a mockable version of processMaestroTask or by
// verifying the first few calls of a real processMaestroTask if it's too hard to mock fully.

// For now, the above tests cover critical paths for MessageHasBeenPosted.
// A full integration test for MessageHasBeenPosted -> processMaestroTask (success)
// would look very similar to TestProcessMaestroTask_Success but invoked via MessageHasBeenPosted.
// Let's add one such test for coverage.

type MockMaestroHandler struct {
	MaestroHandler
	processMaestroTaskCalled bool
	taskNameArg              string
	numMessagesArg           int
	channelIDArg             string
	userIDArg                string
	rootIDArg                string
	processMaestroTaskErr    error
}

func (m *MockMaestroHandler) ProcessMaestroTaskMock(taskName string, numMessages int, channelID, userID, rootID string) error {
	m.processMaestroTaskCalled = true
	m.taskNameArg = taskName
	m.numMessagesArg = numMessages
	m.channelIDArg = channelID
	m.userIDArg = userID
	m.rootIDArg = rootID
	return m.processMaestroTaskErr
}


func TestMessageHasBeenPosted_SuccessfulPathToProcessTask(t *testing.T) {
	apiMock := &plugintest.API{}
	apiMock.On("LogInfo", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Maybe()
	// No SendEphemeralPost or LogError for parsing in this success case.

	// We will replace the real processMaestroTask with a mock.
	// This requires a way to inject this mock.
	// One way is to have processMaestroTask as a field in the handler struct of type func(...) error.
	// Or, for this test, we can use a different approach if the handler isn't easily modifiable.
	// Given current structure, testing the call to the *actual* processMaestroTask
	// means we need all its mocks again.

	// Let's use the actual handler and mock its dependencies as if processMaestroTask was called.
	// This duplicates setup from TestProcessMaestroTask_Success, but ensures MessageHasBeenPosted calls it.

	handler, mocks := setupMaestroHandlerForProcessTask(t) // Re-use the setup
	
	postContent := "!maestro -n 3 do this task"
	// expectedTask := "do this task" // Not directly asserting taskName in this test, but it's parsed
	// expectedNumMsgs := 3
	
	userID := "userTest"
	channelID := "channelTest"
	postID := "postTestID"
	
	post := &model.Post{UserId: userID, Message: postContent, ChannelId: channelID, Id: postID}

	// Mocks for processMaestroTask part:
	originalPost := &model.Post{Id: postID, ChannelId: channelID, UserId: userID}
	postsForChannel := &model.PostList{
		Order: []string{"p1"},
		Posts: map[string]*model.Post{"p1": {Id: "p1", Message: "test message", UserId: userID}},
	}

	mocks.API.On("GetPost", postID).Return(originalPost, nil).Once()
	mocks.API.On("GetPostsForChannel", channelID, 0, mock.AnythingOfType("int")).Return(postsForChannel, nil).Once()
	mocks.API.On("CreatePost", mock.MatchedBy(func(p *model.Post) bool {
		return p.ChannelId == channelID && p.RootId == postID && p.Message == "GraphQL response" // Assuming default mockCallGraphQL
	})).Return(&model.Post{}, nil).Once()
	// Ensure LogError for "Error processing !maestro task" is NOT called
	mocks.API.On("LogError", "Error processing !maestro task", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Times(0)


	handler.MessageHasBeenPosted(nil, post)

	// Assert that LogInfo for "Detected '!maestro' prefix." was called
	foundDetectLog := false
	for _, call := range mocks.API.Calls {
		if call.Method == "LogInfo" {
			if len(call.Arguments) > 0 && call.Arguments.String(0) == "Detected '!maestro' prefix." {
				foundDetectLog = true
				break
			}
		}
	}
	assert.True(t, foundDetectLog, "Expected LogInfo for maestro detection")
	
	mocks.API.AssertExpectations(t)
}
// End of file
