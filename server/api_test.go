package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockPlugin is a simplified mock for our plugin.
// We'll use this to mock the behavior of methods like p.MessageUser, p.FindTeamAndMembers etc.
// For methods that call p.API (like p.FindUser directly calling p.API.GetUser),
// we will continue to use plugintest.API for mocking the underlying Mattermost API calls.
type MockPlugin struct {
	Plugin // Embed the real plugin to get ServeHTTP and other methods
	// Add fields here to control mock behavior for plugin's own methods
	mockMessageUser               func(userID, message string) error
	mockFindTeamAndMembers        func(teamID string) (*model.Team, []*model.TeamMember, error)
	mockFindChannelAndMembers     func(channelID string) (*model.Channel, model.ChannelMembers, error)
	mockReplyToMessage            func(postID, message string) (*model.Post, error)
	mockCreateTeamAndAddMembers   func(teamName, teamDisplayName string, userIDs []string) (*model.Team, error)
	mockCreateChannelAndAddMembers func(teamID, channelName, channelDisplayName, channelType string, userIDs []string) (*model.Channel, error)
	mockPostMessageToChannel      func(channelID, message string) (*model.Post, error)
}

// Override plugin methods that we want to mock
func (mp *MockPlugin) MessageUser(userID, message string) error {
	if mp.mockMessageUser != nil {
		return mp.mockMessageUser(userID, message)
	}
	return errors.New("MessageUser mock not implemented")
}

func (mp *MockPlugin) FindTeamAndMembers(teamID string) (*model.Team, []*model.TeamMember, error) {
	if mp.mockFindTeamAndMembers != nil {
		return mp.mockFindTeamAndMembers(teamID)
	}
	return nil, nil, errors.New("FindTeamAndMembers mock not implemented")
}

func (mp *MockPlugin) FindChannelAndMembers(channelID string) (*model.Channel, model.ChannelMembers, error) {
	if mp.mockFindChannelAndMembers != nil {
		return mp.mockFindChannelAndMembers(channelID)
	}
	return nil, nil, errors.New("FindChannelAndMembers mock not implemented")
}

func (mp *MockPlugin) ReplyToMessage(postID, message string) (*model.Post, error) {
	if mp.mockReplyToMessage != nil {
		return mp.mockReplyToMessage(postID, message)
	}
	return nil, errors.New("ReplyToMessage mock not implemented")
}

func (mp *MockPlugin) CreateTeamAndAddMembers(teamName, teamDisplayName string, userIDs []string) (*model.Team, error) {
	if mp.mockCreateTeamAndAddMembers != nil {
		return mp.mockCreateTeamAndAddMembers(teamName, teamDisplayName, userIDs)
	}
	return nil, errors.New("CreateTeamAndAddMembers mock not implemented")
}

func (mp *MockPlugin) CreateChannelAndAddMembers(teamID, channelName, channelDisplayName, channelType string, userIDs []string) (*model.Channel, error) {
	if mp.mockCreateChannelAndAddMembers != nil {
		return mp.mockCreateChannelAndAddMembers(teamID, channelName, channelDisplayName, channelType, userIDs)
	}
	return nil, errors.New("CreateChannelAndAddMembers mock not implemented")
}

func (mp *MockPlugin) PostMessageToChannel(channelID, message string) (*model.Post, error) {
	if mp.mockPostMessageToChannel != nil {
		return mp.mockPostMessageToChannel(channelID, message)
	}
	return nil, errors.New("PostMessageToChannel mock not implemented")
}


// TestAPI_FindUser tests the /api/v1/find_user HTTP handler.
func TestAPI_FindUser(t *testing.T) {
	p := &Plugin{} // Use the real plugin for this test as p.FindUser calls p.API
	mockAPI := &plugintest.API{}
	
	p.SetAPI(mockAPI)

	mockAPI.On("LogError", mock.AnythingOfType("string"), mock.Anything).Maybe()
	mockAPI.On("LogInfo", mock.AnythingOfType("string"), mock.Anything).Maybe()


	validUserID := "testuserid"
	existingUser := &model.User{Id: validUserID, Username: "testuser"}

	tests := []struct {
		name               string
		requestBody        interface{}
		setupMock          func(mAPI *plugintest.API, userID string) 
		expectedStatusCode int
		expectedResponse   string 
		checkJSONResponse  bool   
	}{
		{
			name: "Valid request, user found",
			requestBody: FindUserRequest{UserID: validUserID},
			setupMock: func(mAPI *plugintest.API, userID string) {
				mAPI.On("GetUser", userID).Return(existingUser, nil).Once()
			},
			expectedStatusCode: http.StatusOK,
			expectedResponse:   `{"id":"testuserid","username":"testuser"`, 
			checkJSONResponse:  true,
		},
		{
			name: "User not found",
			requestBody: FindUserRequest{UserID: "nonexistentuser"},
			setupMock: func(mAPI *plugintest.API, userID string) {
				appErr := model.NewAppError("GetUser", "store.sql_user.get.app_error", nil, "User not found", http.StatusNotFound)
				mAPI.On("GetUser", userID).Return(nil, appErr).Once()
			},
			expectedStatusCode: http.StatusNotFound,
			expectedResponse:   "User not found",
		},
		{
			name: "Internal server error from GetUser",
			requestBody: FindUserRequest{UserID: "erroruser"},
			setupMock: func(mAPI *plugintest.API, userID string) {
				appErr := model.NewAppError("GetUser", "some.error", nil, "Database error", http.StatusInternalServerError)
				mAPI.On("GetUser", userID).Return(nil, appErr).Once()
			},
			expectedStatusCode: http.StatusInternalServerError,
			expectedResponse:   "Database error",
		},
		{
			name:               "Invalid JSON request body",
			requestBody:        "this is not json",
			setupMock:          func(mAPI *plugintest.API, userID string) {}, 
			expectedStatusCode: http.StatusBadRequest,
			expectedResponse:   "Invalid request body",
		},
		{
			name:               "Missing user_id in request",
			requestBody:        FindUserRequest{UserID: ""}, 
			setupMock:          func(mAPI *plugintest.API, userID string) {},      
			expectedStatusCode: http.StatusBadRequest,
			expectedResponse:   "Missing user_id in request body",
		},
		{
			name:               "Empty request body",
			requestBody:        nil, 
			setupMock:          func(mAPI *plugintest.API, userID string) {},
			expectedStatusCode: http.StatusBadRequest,
			expectedResponse:   "Invalid request body", 
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset API mocks for each run
			currentMockAPI := &plugintest.API{}
			currentMockAPI.On("LogError", mock.AnythingOfType("string"), mock.Anything).Maybe()
			// Ensure LogError is mocked with various argument counts if necessary, or use more flexible mock.Anything
			currentMockAPI.On("LogError", mock.AnythingOfType("string"), mock.Anything, mock.Anything, mock.Anything, mock.Anything).Maybe()
			p.SetAPI(currentMockAPI)


			userIDToMock := ""
			if req, ok := tt.requestBody.(FindUserRequest); ok {
				userIDToMock = req.UserID
			}
			
			if tt.setupMock != nil {
				tt.setupMock(currentMockAPI, userIDToMock) 
			}

			var reqBodyReader *bytes.Reader
			if tt.requestBody != nil {
				if strBody, ok := tt.requestBody.(string); ok {
					reqBodyReader = bytes.NewReader([]byte(strBody))
				} else {
					jsonBody, _ := json.Marshal(tt.requestBody)
					reqBodyReader = bytes.NewReader(jsonBody)
				}
			} else {
				reqBodyReader = bytes.NewReader([]byte{}) 
			}


			req, err := http.NewRequest("POST", "/api/v1/find_user", reqBodyReader)
			assert.NoError(t, err)
			req.Header.Set("Mattermost-User-ID", "testauthtoken") 

			rr := httptest.NewRecorder()
			p.ServeHTTP(&plugin.Context{}, rr, req)

			assert.Equal(t, tt.expectedStatusCode, rr.Code)

			responseBody := strings.TrimSpace(rr.Body.String())
			if tt.checkJSONResponse && tt.expectedStatusCode == http.StatusOK {
				var userResponse model.User
				err := json.Unmarshal(rr.Body.Bytes(), &userResponse)
				assert.NoError(t, err, "Response body should be a valid User JSON")
				assert.Equal(t, userIDToMock, userResponse.Id) 
				if !strings.HasPrefix(responseBody, tt.expectedResponse) {
					t.Errorf("handler returned unexpected body: got %v want prefix %v",
						responseBody, tt.expectedResponse)
				}
			} else {
				assert.Contains(t, responseBody, tt.expectedResponse, "Response body mismatch")
			}
			currentMockAPI.AssertExpectations(t)
		})
	}
}

func TestAPI_PostMessageToGroupChannelsTeams(t *testing.T) {
	mockP := &MockPlugin{}
	apiForLogging := &plugintest.API{}
	mockP.SetAPI(apiForLogging)

	apiForLogging.On("LogError", mock.AnythingOfType("string"), mock.Anything).Maybe()
	apiForLogging.On("LogError", mock.AnythingOfType("string"), mock.Anything, mock.Anything, mock.Anything, mock.Anything).Maybe()
	apiForLogging.On("LogError", mock.AnythingOfType("string"), mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Maybe()


	authedUserID := "authuserid"
	targetChannelID := "targetchannelid"
	messageToPost := "Hello from API test!"
	mockedCreatedPost := &model.Post{Id: "newlycreatedpostid", ChannelId: targetChannelID, Message: messageToPost}

	tests := []struct {
		name               string
		requestBody        interface{}
		setupMockPlugin    func(mp *MockPlugin)
		expectedStatusCode int
		expectedResponse   string // JSON string for exact match, or substring for error messages
		exactJSONMatch     bool
	}{
		{
			name:        "Valid request, message posted successfully",
			requestBody: PostMessageRequest{ChannelID: targetChannelID, Message: messageToPost},
			setupMockPlugin: func(mp *MockPlugin) {
				mp.mockPostMessageToChannel = func(channelID, message string) (*model.Post, error) {
					assert.Equal(t, targetChannelID, channelID)
					assert.Equal(t, messageToPost, message)
					return mockedCreatedPost, nil
				}
			},
			expectedStatusCode: http.StatusCreated,
			expectedResponse:   `{"id":"newlycreatedpostid","create_at":0,"update_at":0,"edit_at":0,"delete_at":0,"is_pinned":false,"user_id":"","channel_id":"targetchannelid","root_id":"","original_id":"","message":"Hello from API test!","type":"","props":null,"hashtag":"","filenames":null,"file_ids":null,"pending_post_id":"","has_reactions":false,"reply_count":0,"is_following":null,"is_liked":null,"last_reply_at":0,"participants":null,"extra_update_at":0,"metadata":null}`,
			exactJSONMatch:     true,
		},
		{
			name:        "Plugin's PostMessageToChannel returns AppError (e.g., channel not found)",
			requestBody: PostMessageRequest{ChannelID: "unknownchannelid", Message: messageToPost},
			setupMockPlugin: func(mp *MockPlugin) {
				mp.mockPostMessageToChannel = func(channelID, message string) (*model.Post, error) {
					return nil, model.NewAppError("p.PostMessageToChannel", "app.channel.get.app_error", nil, "Channel not found.", http.StatusNotFound)
				}
			},
			expectedStatusCode: http.StatusNotFound,
			expectedResponse:   "Channel not found.",
		},
		{
			name:        "Plugin's PostMessageToChannel returns specific validation error (channelID empty)",
			requestBody: PostMessageRequest{ChannelID: targetChannelID, Message: messageToPost}, // Valid request, but mock returns validation error
			setupMockPlugin: func(mp *MockPlugin) {
				mp.mockPostMessageToChannel = func(channelID, message string) (*model.Post, error) {
					return nil, errors.New("PostMessageToChannel: channelID cannot be empty")
				}
			},
			expectedStatusCode: http.StatusBadRequest, // Handler should convert this specific error
			expectedResponse:   "PostMessageToChannel: channelID cannot be empty",
		},
		{
			name:        "Plugin's PostMessageToChannel returns other generic error",
			requestBody: PostMessageRequest{ChannelID: targetChannelID, Message: messageToPost},
			setupMockPlugin: func(mp *MockPlugin) {
				mp.mockPostMessageToChannel = func(channelID, message string) (*model.Post, error) {
					return nil, errors.New("some other internal plugin error")
				}
			},
			expectedStatusCode: http.StatusInternalServerError,
			expectedResponse:   "Internal server error",
		},
		{
			name:               "Invalid JSON request body",
			requestBody:        "this is not valid json",
			setupMockPlugin:    func(mp *MockPlugin) {},
			expectedStatusCode: http.StatusBadRequest,
			expectedResponse:   "Invalid request body",
		},
		{
			name:               "Missing channel_id in request body",
			requestBody:        PostMessageRequest{Message: messageToPost}, // ChannelID is empty
			setupMockPlugin:    func(mp *MockPlugin) {},
			expectedStatusCode: http.StatusBadRequest,
			expectedResponse:   "Missing channel_id in request body",
		},
		{
			name:               "Missing message in request body",
			requestBody:        PostMessageRequest{ChannelID: targetChannelID}, // Message is empty
			setupMockPlugin:    func(mp *MockPlugin) {},
			expectedStatusCode: http.StatusBadRequest,
			expectedResponse:   "Missing message in request body",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockP.mockPostMessageToChannel = nil // Reset mock function
			tt.setupMockPlugin(mockP)

			var reqBodyReader *bytes.Reader
			if strBody, ok := tt.requestBody.(string); ok {
				reqBodyReader = bytes.NewReader([]byte(strBody))
			} else {
				jsonBody, errMarshal := json.Marshal(tt.requestBody)
				assert.NoError(t, errMarshal)
				reqBodyReader = bytes.NewReader(jsonBody)
			}
			
			req, errReq := http.NewRequest("POST", "/api/v1/post_message_to_group_channels_teams", reqBodyReader)
			assert.NoError(t, errReq)
			req.Header.Set("Mattermost-User-ID", authedUserID)

			rr := httptest.NewRecorder()
			mockP.ServeHTTP(&plugin.Context{}, rr, req)

			assert.Equal(t, tt.expectedStatusCode, rr.Code, "HTTP status code mismatch for test: "+tt.name)
			responseBody := strings.TrimSpace(rr.Body.String())

			if tt.exactJSONMatch {
				assert.JSONEq(t, tt.expectedResponse, responseBody, "Response body JSON mismatch for test: "+tt.name)
			} else {
				assert.Contains(t, responseBody, tt.expectedResponse, "Response body should contain expected string for test: "+tt.name)
			}
		})
	}
}

func TestAPI_CreateGroupChannelsTeams(t *testing.T) {
	mockP := &MockPlugin{}
	apiForLogging := &plugintest.API{}
	mockP.SetAPI(apiForLogging) // For LogError calls in the handler

	apiForLogging.On("LogError", mock.AnythingOfType("string"), mock.Anything).Maybe()
	apiForLogging.On("LogError", mock.AnythingOfType("string"), mock.Anything, mock.Anything, mock.Anything, mock.Anything).Maybe()
	apiForLogging.On("LogError", mock.AnythingOfType("string"), mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Maybe()

	authedUserID := "authuserid"

	// Team specific mock data
	mockTeam := &model.Team{Id: "newteamid", Name: "new-team", DisplayName: "New Team"}
	// Channel specific mock data
	mockChannel := &model.Channel{Id: "newchannelid", Name: "new-channel", DisplayName: "New Channel", TeamId: "teamid"}

	tests := []struct {
		name               string
		requestBody        interface{}
		setupMockPlugin    func(mp *MockPlugin)
		expectedStatusCode int
		expectedResponse   string // JSON string for exact match, or substring for error messages
		exactJSONMatch     bool
	}{
		// Team Creation Test Cases
		{
			name: "Valid request to create team - success",
			requestBody: CreateGroupRequest{Type: "team", TeamName: "new-team", TeamDisplayName: "New Team", UserIDs: []string{"user1"}},
			setupMockPlugin: func(mp *MockPlugin) {
				mp.mockCreateTeamAndAddMembers = func(name, displayName string, userIDs []string) (*model.Team, error) {
					assert.Equal(t, "new-team", name)
					assert.Equal(t, "New Team", displayName)
					assert.Equal(t, []string{"user1"}, userIDs)
					return mockTeam, nil
				}
			},
			expectedStatusCode: http.StatusCreated,
			expectedResponse:   `{"team":{"id":"newteamid","name":"new-team","display_name":"New Team"}}`,
			exactJSONMatch:     true,
		},
		{
			name: "Create team - plugin returns AppError",
			requestBody: CreateGroupRequest{Type: "team", TeamName: "fail-team", TeamDisplayName: "Fail Team"},
			setupMockPlugin: func(mp *MockPlugin) {
				mp.mockCreateTeamAndAddMembers = func(name, displayName string, userIDs []string) (*model.Team, error) {
					return nil, model.NewAppError("CreateTeamAndAddMembers", "app.team.create.app_error", nil, "Cannot create team, name exists", http.StatusBadRequest)
				}
			},
			expectedStatusCode: http.StatusBadRequest,
			expectedResponse:   "Cannot create team, name exists",
		},
		{
			name: "Create team - plugin returns generic error",
			requestBody: CreateGroupRequest{Type: "team", TeamName: "generic-fail-team", TeamDisplayName: "Generic Fail Team"},
			setupMockPlugin: func(mp *MockPlugin) {
				mp.mockCreateTeamAndAddMembers = func(name, displayName string, userIDs []string) (*model.Team, error) {
					return nil, errors.New("some internal plugin error creating team")
				}
			},
			expectedStatusCode: http.StatusInternalServerError,
			expectedResponse:   "Internal server error",
		},
		{
			name:               "Create team - missing team_name",
			requestBody:        CreateGroupRequest{Type: "team", TeamDisplayName: "New Team"},
			setupMockPlugin:    func(mp *MockPlugin) {},
			expectedStatusCode: http.StatusBadRequest,
			expectedResponse:   "Missing team_name or team_display_name for type 'team'",
		},
		// Channel Creation Test Cases
		{
			name: "Valid request to create channel - success",
			requestBody: CreateGroupRequest{Type: "channel", TeamID: "teamid", ChannelName: "new-channel", ChannelDisplayName: "New Channel", ChannelType: model.ChannelTypeOpen, UserIDs: []string{"user1"}},
			setupMockPlugin: func(mp *MockPlugin) {
				mp.mockCreateChannelAndAddMembers = func(teamID, name, displayName, chType string, userIDs []string) (*model.Channel, error) {
					assert.Equal(t, "teamid", teamID)
					assert.Equal(t, "new-channel", name)
					// ... other assertions
					return mockChannel, nil
				}
			},
			expectedStatusCode: http.StatusCreated,
			expectedResponse:   `{"channel":{"id":"newchannelid","name":"new-channel","display_name":"New Channel","team_id":"teamid"}}`,
			exactJSONMatch:     true,
		},
		{
			name: "Create channel - plugin returns AppError",
			requestBody: CreateGroupRequest{Type: "channel", TeamID: "teamid", ChannelName: "fail-channel", ChannelDisplayName: "Fail Channel", ChannelType: model.ChannelTypeOpen},
			setupMockPlugin: func(mp *MockPlugin) {
				mp.mockCreateChannelAndAddMembers = func(teamID, name, displayName, chType string, userIDs []string) (*model.Channel, error) {
					return nil, model.NewAppError("CreateChannelAndAddMembers", "app.channel.create.app_error", nil, "Cannot create channel, name exists", http.StatusBadRequest)
				}
			},
			expectedStatusCode: http.StatusBadRequest,
			expectedResponse:   "Cannot create channel, name exists",
		},
		{
			name: "Create channel - plugin returns generic error",
			requestBody: CreateGroupRequest{Type: "channel", TeamID: "teamid", ChannelName: "generic-fail-channel", ChannelDisplayName: "Generic Fail Channel", ChannelType: model.ChannelTypeOpen},
			setupMockPlugin: func(mp *MockPlugin) {
				mp.mockCreateChannelAndAddMembers = func(teamID, name, displayName, chType string, userIDs []string) (*model.Channel, error) {
					return nil, errors.New("some internal plugin error creating channel")
				}
			},
			expectedStatusCode: http.StatusInternalServerError,
			expectedResponse:   "Internal server error",
		},
		{
			name:               "Create channel - missing team_id",
			requestBody:        CreateGroupRequest{Type: "channel", ChannelName: "new-channel", ChannelDisplayName: "New Channel", ChannelType: model.ChannelTypeOpen},
			setupMockPlugin:    func(mp *MockPlugin) {},
			expectedStatusCode: http.StatusBadRequest,
			expectedResponse:   "Missing team_id, channel_name, channel_display_name, or channel_type for type 'channel'",
		},
		{
			name:               "Create channel - invalid channel_type",
			requestBody:        CreateGroupRequest{Type: "channel", TeamID: "teamid", ChannelName: "new-channel", ChannelDisplayName: "New Channel", ChannelType: "X"},
			setupMockPlugin:    func(mp *MockPlugin) {},
			expectedStatusCode: http.StatusBadRequest,
			expectedResponse:   "Invalid channel_type. Must be 'O' (Open) or 'P' (Private).",
		},
		// General Handler Logic Test Cases
		{
			name:               "Invalid type in request",
			requestBody:        CreateGroupRequest{Type: "invalidtype"},
			setupMockPlugin:    func(mp *MockPlugin) {},
			expectedStatusCode: http.StatusBadRequest,
			expectedResponse:   "Invalid type specified in request. Must be 'team' or 'channel'.",
		},
		{
			name:               "Invalid JSON request body",
			requestBody:        "this is not valid json",
			setupMockPlugin:    func(mp *MockPlugin) {},
			expectedStatusCode: http.StatusBadRequest,
			expectedResponse:   "Invalid request body",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockP.mockCreateTeamAndAddMembers = nil // Reset mocks
			mockP.mockCreateChannelAndAddMembers = nil
			tt.setupMockPlugin(mockP)

			var reqBodyReader *bytes.Reader
			if strBody, ok := tt.requestBody.(string); ok {
				reqBodyReader = bytes.NewReader([]byte(strBody))
			} else {
				jsonBody, _ := json.Marshal(tt.requestBody)
				reqBodyReader = bytes.NewReader(jsonBody)
			}
			
			req, err := http.NewRequest("POST", "/api/v1/create_group_channels_teams", reqBodyReader)
			assert.NoError(t, err)
			req.Header.Set("Mattermost-User-ID", authedUserID)

			rr := httptest.NewRecorder()
			mockP.ServeHTTP(&plugin.Context{}, rr, req)

			assert.Equal(t, tt.expectedStatusCode, rr.Code, "HTTP status code mismatch for test: "+tt.name)
			responseBody := strings.TrimSpace(rr.Body.String())

			if tt.exactJSONMatch {
				assert.JSONEq(t, tt.expectedResponse, responseBody, "Response body JSON mismatch for test: "+tt.name)
			} else {
				assert.Contains(t, responseBody, tt.expectedResponse, "Response body should contain expected string for test: "+tt.name)
			}
		})
	}
}

func TestAPI_ReplyToMessage(t *testing.T) {
	mockP := &MockPlugin{}
	apiForLogging := &plugintest.API{}
	mockP.SetAPI(apiForLogging)

	// Generic logging mocks
	apiForLogging.On("LogError", mock.AnythingOfType("string"), mock.Anything).Maybe()
	apiForLogging.On("LogError", mock.AnythingOfType("string"), mock.Anything, mock.Anything, mock.Anything, mock.Anything).Maybe()
	apiForLogging.On("LogError", mock.AnythingOfType("string"), mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Maybe()

	authedUserID := "authuserid"
	targetPostID := "targetpostid"
	replyMessage := "This is a reply."

	mockCreatedPost := &model.Post{Id: "newreplypostid", Message: replyMessage, RootId: targetPostID, ChannelId: "channelid"}

	tests := []struct {
		name               string
		requestBody        interface{}
		setupMockPlugin    func(mp *MockPlugin)
		expectedStatusCode int
		expectedResponse   string // JSON string for exact match, or substring for error messages
		exactJSONMatch     bool
	}{
		{
			name:        "Valid request, reply successful",
			requestBody: ReplyToMessageRequest{PostID: targetPostID, Message: replyMessage},
			setupMockPlugin: func(mp *MockPlugin) {
				mp.mockReplyToMessage = func(postID, message string) (*model.Post, error) {
					assert.Equal(t, targetPostID, postID)
					assert.Equal(t, replyMessage, message)
					return mockCreatedPost, nil
				}
			},
			expectedStatusCode: http.StatusOK, // As per current handler implementation for success
			expectedResponse:   `{"id":"newreplypostid","create_at":0,"update_at":0,"edit_at":0,"delete_at":0,"is_pinned":false,"user_id":"","channel_id":"channelid","root_id":"targetpostid","original_id":"","message":"This is a reply.","type":"","props":null,"hashtag":"","filenames":null,"file_ids":null,"pending_post_id":"","has_reactions":false,"reply_count":0,"is_following":null,"is_liked":null,"last_reply_at":0,"participants":null,"extra_update_at":0,"metadata":null}`,
			exactJSONMatch:     true,
		},
		{
			name:        "Plugin's ReplyToMessage returns AppError (e.g., original post not found)",
			requestBody: ReplyToMessageRequest{PostID: "unknownpostid", Message: replyMessage},
			setupMockPlugin: func(mp *MockPlugin) {
				mp.mockReplyToMessage = func(postID, message string) (*model.Post, error) {
					return nil, model.NewAppError("p.ReplyToMessage", "app.post.get.app_error", nil, "Original post not found.", http.StatusNotFound)
				}
			},
			expectedStatusCode: http.StatusNotFound,
			expectedResponse:   "Original post not found.",
		},
		{
			name:        "Plugin's ReplyToMessage returns generic error",
			requestBody: ReplyToMessageRequest{PostID: targetPostID, Message: replyMessage},
			setupMockPlugin: func(mp *MockPlugin) {
				mp.mockReplyToMessage = func(postID, message string) (*model.Post, error) {
					return nil, errors.New("some internal plugin error")
				}
			},
			expectedStatusCode: http.StatusInternalServerError,
			expectedResponse:   "Internal server error",
		},
		{
			name:        "Plugin's ReplyToMessage returns specific validation error (postID empty)",
			requestBody: ReplyToMessageRequest{PostID: targetPostID, Message: replyMessage}, // Request is valid, but mock returns validation error
			setupMockPlugin: func(mp *MockPlugin) {
				mp.mockReplyToMessage = func(postID, message string) (*model.Post, error) {
					return nil, errors.New("ReplyToMessage: postID cannot be empty")
				}
			},
			expectedStatusCode: http.StatusBadRequest, // Handler converts this specific error string
			expectedResponse:   "ReplyToMessage: postID cannot be empty",
		},
		{
			name:        "Plugin's ReplyToMessage returns specific validation error (message empty)",
			requestBody: ReplyToMessageRequest{PostID: targetPostID, Message: replyMessage}, 
			setupMockPlugin: func(mp *MockPlugin) {
				mp.mockReplyToMessage = func(postID, message string) (*model.Post, error) {
					return nil, errors.New("ReplyToMessage: message cannot be empty")
				}
			},
			expectedStatusCode: http.StatusBadRequest, 
			expectedResponse:   "ReplyToMessage: message cannot be empty",
		},
		{
			name:               "Invalid JSON request body",
			requestBody:        "this is not valid json",
			setupMockPlugin:    func(mp *MockPlugin) {},
			expectedStatusCode: http.StatusBadRequest,
			expectedResponse:   "Invalid request body",
		},
		{
			name:               "Missing post_id in request body",
			requestBody:        ReplyToMessageRequest{Message: replyMessage}, // PostID is empty
			setupMockPlugin:    func(mp *MockPlugin) {},
			expectedStatusCode: http.StatusBadRequest,
			expectedResponse:   "Missing post_id in request body",
		},
		{
			name:               "Missing message in request body",
			requestBody:        ReplyToMessageRequest{PostID: targetPostID}, // Message is empty
			setupMockPlugin:    func(mp *MockPlugin) {},
			expectedStatusCode: http.StatusBadRequest,
			expectedResponse:   "Missing message in request body",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockP.mockReplyToMessage = nil // Reset mock function
			tt.setupMockPlugin(mockP)

			var reqBodyReader *bytes.Reader
			if strBody, ok := tt.requestBody.(string); ok {
				reqBodyReader = bytes.NewReader([]byte(strBody))
			} else {
				jsonBody, errMarshal := json.Marshal(tt.requestBody)
				assert.NoError(t, errMarshal)
				reqBodyReader = bytes.NewReader(jsonBody)
			}
			
			req, errReq := http.NewRequest("POST", "/api/v1/reply_to_message", reqBodyReader)
			assert.NoError(t, errReq)
			req.Header.Set("Mattermost-User-ID", authedUserID)

			rr := httptest.NewRecorder()
			mockP.ServeHTTP(&plugin.Context{}, rr, req)

			assert.Equal(t, tt.expectedStatusCode, rr.Code, "HTTP status code mismatch for test: "+tt.name)
			responseBody := strings.TrimSpace(rr.Body.String())

			if tt.exactJSONMatch {
				assert.JSONEq(t, tt.expectedResponse, responseBody, "Response body JSON mismatch for test: "+tt.name)
			} else {
				assert.Contains(t, responseBody, tt.expectedResponse, "Response body should contain expected string for test: "+tt.name)
			}
		})
	}
}

func TestAPI_FindGroupChannelsTeams(t *testing.T) {
	mockP := &MockPlugin{} // Use the MockPlugin to mock p.FindTeamAndMembers and p.FindChannelAndMembers
	apiForLogging := &plugintest.API{}
	mockP.SetAPI(apiForLogging) // Set the API for the embedded real Plugin, used for LogError, etc.

	// Setup generic logging mocks on apiForLogging, as the handler might call p.API.LogError
	apiForLogging.On("LogError", mock.AnythingOfType("string"), mock.Anything).Maybe()
	apiForLogging.On("LogError", mock.AnythingOfType("string"), mock.Anything, mock.Anything, mock.Anything, mock.Anything).Maybe()
	apiForLogging.On("LogError", mock.AnythingOfType("string"), mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Maybe()


	teamID := "testteamid"
	channelID := "testchannelid"
	authedUserID := "authuserid" // For Mattermost-User-ID header

	mockTeam := &model.Team{Id: teamID, DisplayName: "Test Team", Name: "test-team"}
	mockTeamMembers := []*model.TeamMember{{TeamId: teamID, UserId: "user1"}, {TeamId: teamID, UserId: "user2"}}

	mockChannel := &model.Channel{Id: channelID, DisplayName: "Test Channel", Name: "test-channel", TeamId: teamID}
	mockChannelMembers := model.ChannelMembers{
		{ChannelId: channelID, UserId: "user1"},
		{ChannelId: channelID, UserId: "user2"},
	}

	tests := []struct {
		name               string
		requestBody        interface{}
		setupMockPlugin    func(mp *MockPlugin) // To set the behavior of mocked plugin methods
		expectedStatusCode int
		expectedResponse   string // JSON string for exact match, or substring for error messages
		exactJSONMatch     bool
	}{
		// Team Test Cases
		{
			name:        "Valid request for team - success",
			requestBody: FindGroupRequest{TeamID: teamID},
			setupMockPlugin: func(mp *MockPlugin) {
				mp.mockFindTeamAndMembers = func(id string) (*model.Team, []*model.TeamMember, error) {
					assert.Equal(t, teamID, id)
					return mockTeam, mockTeamMembers, nil
				}
			},
			expectedStatusCode: http.StatusOK,
			expectedResponse:   `{"team":{"id":"testteamid","display_name":"Test Team","name":"test-team"},"team_members":[{"team_id":"testteamid","user_id":"user1"},{"team_id":"testteamid","user_id":"user2"}]}`,
			exactJSONMatch:     true,
		},
		{
			name:        "Team not found - AppError from plugin",
			requestBody: FindGroupRequest{TeamID: "unknownteam"},
			setupMockPlugin: func(mp *MockPlugin) {
				mp.mockFindTeamAndMembers = func(id string) (*model.Team, []*model.TeamMember, error) {
					return nil, nil, model.NewAppError("p.FindTeamAndMembers", "app.team.get.app_error", nil, "Team not found here", http.StatusNotFound)
				}
			},
			expectedStatusCode: http.StatusNotFound,
			expectedResponse:   "Team not found here",
		},
		{
			name:        "Generic error from plugin when finding team",
			requestBody: FindGroupRequest{TeamID: teamID},
			setupMockPlugin: func(mp *MockPlugin) {
				mp.mockFindTeamAndMembers = func(id string) (*model.Team, []*model.TeamMember, error) {
					return nil, nil, errors.New("internal plugin error finding team")
				}
			},
			expectedStatusCode: http.StatusInternalServerError,
			expectedResponse:   "Internal server error",
		},
		// Channel Test Cases
		{
			name:        "Valid request for channel - success",
			requestBody: FindGroupRequest{ChannelID: channelID},
			setupMockPlugin: func(mp *MockPlugin) {
				mp.mockFindChannelAndMembers = func(id string) (*model.Channel, model.ChannelMembers, error) {
					assert.Equal(t, channelID, id)
					return mockChannel, mockChannelMembers, nil
				}
			},
			expectedStatusCode: http.StatusOK,
			expectedResponse:   `{"channel":{"id":"testchannelid","display_name":"Test Channel","name":"test-channel","team_id":"testteamid"},"channel_members":[{"channel_id":"testchannelid","user_id":"user1"},{"channel_id":"testchannelid","user_id":"user2"}]}`,
			exactJSONMatch:     true,
		},
		{
			name:        "Channel not found - AppError from plugin",
			requestBody: FindGroupRequest{ChannelID: "unknownchannel"},
			setupMockPlugin: func(mp *MockPlugin) {
				mp.mockFindChannelAndMembers = func(id string) (*model.Channel, model.ChannelMembers, error) {
					return nil, nil, model.NewAppError("p.FindChannelAndMembers", "app.channel.get.app_error", nil, "Channel not found here", http.StatusNotFound)
				}
			},
			expectedStatusCode: http.StatusNotFound,
			expectedResponse:   "Channel not found here",
		},
		{
			name:        "Generic error from plugin when finding channel",
			requestBody: FindGroupRequest{ChannelID: channelID},
			setupMockPlugin: func(mp *MockPlugin) {
				mp.mockFindChannelAndMembers = func(id string) (*model.Channel, model.ChannelMembers, error) {
					return nil, nil, errors.New("internal plugin error finding channel")
				}
			},
			expectedStatusCode: http.StatusInternalServerError,
			expectedResponse:   "Internal server error",
		},
		// General Handler Logic Test Cases
		{
			name:               "Request with both team_id and channel_id",
			requestBody:        FindGroupRequest{TeamID: teamID, ChannelID: channelID},
			setupMockPlugin:    func(mp *MockPlugin) {}, // No plugin method call expected
			expectedStatusCode: http.StatusBadRequest,
			expectedResponse:   "Please provide either team_id or channel_id, not both.",
		},
		{
			name:               "Request with neither team_id nor channel_id",
			requestBody:        FindGroupRequest{},
			setupMockPlugin:    func(mp *MockPlugin) {},
			expectedStatusCode: http.StatusBadRequest,
			expectedResponse:   "Missing team_id or channel_id in request body",
		},
		{
			name:               "Invalid JSON request body",
			requestBody:        "this is not valid json",
			setupMockPlugin:    func(mp *MockPlugin) {},
			expectedStatusCode: http.StatusBadRequest,
			expectedResponse:   "Invalid request body",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset mock functions to nil to avoid interference between tests
			mockP.mockFindTeamAndMembers = nil
			mockP.mockFindChannelAndMembers = nil
			
			tt.setupMockPlugin(mockP)

			var reqBodyReader *bytes.Reader
			if strBody, ok := tt.requestBody.(string); ok {
				reqBodyReader = bytes.NewReader([]byte(strBody))
			} else {
				jsonBody, errMarshal := json.Marshal(tt.requestBody)
				assert.NoError(t, errMarshal)
				reqBodyReader = bytes.NewReader(jsonBody)
			}
			
			req, errReq := http.NewRequest("POST", "/api/v1/find_group_channels_teams", reqBodyReader)
			assert.NoError(t, errReq)
			req.Header.Set("Mattermost-User-ID", authedUserID)

			rr := httptest.NewRecorder()
			mockP.ServeHTTP(&plugin.Context{}, rr, req) // Use the mockP which has overridden methods

			assert.Equal(t, tt.expectedStatusCode, rr.Code, "HTTP status code mismatch for test: "+tt.name)
			responseBody := strings.TrimSpace(rr.Body.String())

			if tt.exactJSONMatch {
				// For successful cases, ensure the JSON structure is as expected.
				// Zero out fields that are not relevant or change often (like CreateAt, UpdateAt etc.) before comparison if necessary.
				// For this test, the mock data is simple enough.
				assert.JSONEq(t, tt.expectedResponse, responseBody, "Response body JSON mismatch for test: "+tt.name)
			} else {
				assert.Contains(t, responseBody, tt.expectedResponse, "Response body should contain expected string for test: "+tt.name)
			}
		})
	}
}

func TestAPI_FindGroupChannelsTeams(t *testing.T) {
	mockP := &MockPlugin{}
	apiForLogging := &plugintest.API{}
	mockP.SetAPI(apiForLogging)

	apiForLogging.On("LogError", mock.AnythingOfType("string"), mock.Anything).Maybe()
	apiForLogging.On("LogError", mock.AnythingOfType("string"), mock.Anything, mock.Anything, mock.Anything, mock.Anything).Maybe()
	apiForLogging.On("LogError", mock.AnythingOfType("string"), mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Maybe()


	teamID := "testteamid"
	channelID := "testchannelid"
	authedUserID := "authuserid"

	mockTeam := &model.Team{Id: teamID, DisplayName: "Test Team"}
	mockTeamMembers := []*model.TeamMember{{TeamId: teamID, UserId: "user1"}}

	mockChannel := &model.Channel{Id: channelID, DisplayName: "Test Channel", TeamId: teamID}
	mockChannelMembers := model.ChannelMembers{{ChannelId: channelID, UserId: "user1"}}

	tests := []struct {
		name               string
		requestBody        interface{}
		setupMock          func(mp *MockPlugin)
		expectedStatusCode int
		expectedResponse   string // Can be a JSON string for exact match or substring for errors
		exactJSONMatch     bool
	}{
		// Team Test Cases
		{
			name:        "Valid request for team",
			requestBody: FindGroupRequest{TeamID: teamID},
			setupMock: func(mp *MockPlugin) {
				mp.mockFindTeamAndMembers = func(id string) (*model.Team, []*model.TeamMember, error) {
					assert.Equal(t, teamID, id)
					return mockTeam, mockTeamMembers, nil
				}
			},
			expectedStatusCode: http.StatusOK,
			expectedResponse:   `{"team":{"id":"testteamid","display_name":"Test Team"},"team_members":[{"team_id":"testteamid","user_id":"user1"}]}`,
			exactJSONMatch:     true,
		},
		{
			name:        "Team not found (AppError)",
			requestBody: FindGroupRequest{TeamID: "unknownteam"},
			setupMock: func(mp *MockPlugin) {
				mp.mockFindTeamAndMembers = func(id string) (*model.Team, []*model.TeamMember, error) {
					return nil, nil, model.NewAppError("FindTeamAndMembers", "app.team.get.app_error", nil, "Team not found", http.StatusNotFound)
				}
			},
			expectedStatusCode: http.StatusNotFound,
			expectedResponse:   "Team not found",
		},
		{
			name:        "Generic error finding team",
			requestBody: FindGroupRequest{TeamID: teamID},
			setupMock: func(mp *MockPlugin) {
				mp.mockFindTeamAndMembers = func(id string) (*model.Team, []*model.TeamMember, error) {
					return nil, nil, errors.New("some internal error finding team")
				}
			},
			expectedStatusCode: http.StatusInternalServerError,
			expectedResponse:   "Internal server error",
		},
		// Channel Test Cases
		{
			name:        "Valid request for channel",
			requestBody: FindGroupRequest{ChannelID: channelID},
			setupMock: func(mp *MockPlugin) {
				mp.mockFindChannelAndMembers = func(id string) (*model.Channel, model.ChannelMembers, error) {
					assert.Equal(t, channelID, id)
					return mockChannel, mockChannelMembers, nil
				}
			},
			expectedStatusCode: http.StatusOK,
			expectedResponse:   `{"channel":{"id":"testchannelid","display_name":"Test Channel","team_id":"testteamid"},"channel_members":[{"channel_id":"testchannelid","user_id":"user1"}]}`,
			exactJSONMatch:     true,
		},
		{
			name:        "Channel not found (AppError)",
			requestBody: FindGroupRequest{ChannelID: "unknownchannel"},
			setupMock: func(mp *MockPlugin) {
				mp.mockFindChannelAndMembers = func(id string) (*model.Channel, model.ChannelMembers, error) {
					return nil, nil, model.NewAppError("FindChannelAndMembers", "app.channel.get.app_error", nil, "Channel not found", http.StatusNotFound)
				}
			},
			expectedStatusCode: http.StatusNotFound,
			expectedResponse:   "Channel not found",
		},
		{
			name:        "Generic error finding channel",
			requestBody: FindGroupRequest{ChannelID: channelID},
			setupMock: func(mp *MockPlugin) {
				mp.mockFindChannelAndMembers = func(id string) (*model.Channel, model.ChannelMembers, error) {
					return nil, nil, errors.New("some internal error finding channel")
				}
			},
			expectedStatusCode: http.StatusInternalServerError,
			expectedResponse:   "Internal server error",
		},
		// General Handler Test Cases
		{
			name:               "Request with both team_id and channel_id",
			requestBody:        FindGroupRequest{TeamID: teamID, ChannelID: channelID},
			setupMock:          func(mp *MockPlugin) {},
			expectedStatusCode: http.StatusBadRequest,
			expectedResponse:   "Please provide either team_id or channel_id, not both.",
		},
		{
			name:               "Request with neither team_id nor channel_id",
			requestBody:        FindGroupRequest{},
			setupMock:          func(mp *MockPlugin) {},
			expectedStatusCode: http.StatusBadRequest,
			expectedResponse:   "Missing team_id or channel_id in request body",
		},
		{
			name:               "Invalid JSON request body",
			requestBody:        "not valid json",
			setupMock:          func(mp *MockPlugin) {},
			expectedStatusCode: http.StatusBadRequest,
			expectedResponse:   "Invalid request body",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMock(mockP)

			var reqBodyReader *bytes.Reader
			if strBody, ok := tt.requestBody.(string); ok {
				reqBodyReader = bytes.NewReader([]byte(strBody))
			} else {
				jsonBody, _ := json.Marshal(tt.requestBody)
				reqBodyReader = bytes.NewReader(jsonBody)
			}
			
			req, err := http.NewRequest("POST", "/api/v1/find_group_channels_teams", reqBodyReader)
			assert.NoError(t, err)
			req.Header.Set("Mattermost-User-ID", authedUserID)

			rr := httptest.NewRecorder()
			mockP.ServeHTTP(&plugin.Context{}, rr, req)

			assert.Equal(t, tt.expectedStatusCode, rr.Code, "HTTP status code mismatch for test: "+tt.name)
			responseBody := strings.TrimSpace(rr.Body.String())

			if tt.exactJSONMatch {
				assert.JSONEq(t, tt.expectedResponse, responseBody, "Response body mismatch for test: "+tt.name)
			} else {
				assert.Contains(t, responseBody, tt.expectedResponse, "Response body should contain expected string for test: "+tt.name)
			}
		})
	}
}

func TestAPI_MessageUser(t *testing.T) {
	mockP := &MockPlugin{} // Use the MockPlugin
	// This mockAPI is for the embedded Plugin's API, used for LogError etc. by the handler.
	apiForLogging := &plugintest.API{} 
	mockP.SetAPI(apiForLogging)    

	// Mock LogError to prevent nil pointer dereference if they are called by the handler
	apiForLogging.On("LogError", mock.AnythingOfType("string"), mock.Anything).Maybe()
	apiForLogging.On("LogError", mock.AnythingOfType("string"), mock.Anything, mock.Anything, mock.Anything, mock.Anything).Maybe()
	apiForLogging.On("LogError", mock.AnythingOfType("string"), mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Maybe()


	targetUserID := "targetUser"
	message := "Hello there"

	tests := []struct {
		name               string
		requestBody        interface{}
		setupMock          func(mp *MockPlugin) // Function to set up the mock for p.MessageUser
		expectedStatusCode int
		expectedResponse   string // Expected substring in response body
		exactJSONMatch     bool   // If true, use JSONEq for response body comparison
	}{
		{
			name:        "Valid request, message sent successfully",
			requestBody: MessageUserRequest{UserID: targetUserID, Message: message},
			setupMock: func(mp *MockPlugin) {
				mp.mockMessageUser = func(userID, msg string) error {
					assert.Equal(t, targetUserID, userID)
					assert.Equal(t, message, msg)
					return nil // Success
				}
			},
			expectedStatusCode: http.StatusOK,
			expectedResponse:   `{"status":"ok","message":"Message sent successfully"}`,
			exactJSONMatch:     true,
		},
		{
			name:        "Plugin returns AppError (e.g., user not found by plugin logic)",
			requestBody: MessageUserRequest{UserID: "nonexistentUser", Message: message},
			setupMock: func(mp *MockPlugin) {
				mp.mockMessageUser = func(userID, msg string) error {
					return model.NewAppError("MessageUserPluginLogic", "app.user.get.app_error", nil, "User not found in plugin", http.StatusNotFound)
				}
			},
			expectedStatusCode: http.StatusNotFound,
			expectedResponse:   "User not found in plugin",
		},
		{
			name:        "Plugin returns generic error",
			requestBody: MessageUserRequest{UserID: targetUserID, Message: message},
			setupMock: func(mp *MockPlugin) {
				mp.mockMessageUser = func(userID, msg string) error {
					return errors.New("some internal plugin error")
				}
			},
			expectedStatusCode: http.StatusInternalServerError,
			expectedResponse:   "Internal server error",
		},
		{
			name:               "Invalid JSON request body",
			requestBody:        "this is not json",
			setupMock:          func(mp *MockPlugin) {},
			expectedStatusCode: http.StatusBadRequest,
			expectedResponse:   "Invalid request body",
		},
		{
			name:               "Missing user_id in request",
			requestBody:        MessageUserRequest{UserID: "", Message: message},
			setupMock:          func(mp *MockPlugin) {},
			expectedStatusCode: http.StatusBadRequest,
			expectedResponse:   "Missing user_id in request body",
		},
		{
			name:               "Missing message in request",
			requestBody:        MessageUserRequest{UserID: targetUserID, Message: ""},
			setupMock:          func(mp *MockPlugin) {},
			expectedStatusCode: http.StatusBadRequest,
			expectedResponse:   "Missing message in request body",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup the mock for the specific test case
			tt.setupMock(mockP)

			var reqBodyReader *bytes.Reader
			if strBody, ok := tt.requestBody.(string); ok {
				reqBodyReader = bytes.NewReader([]byte(strBody))
			} else {
				jsonBody, _ := json.Marshal(tt.requestBody)
				reqBodyReader = bytes.NewReader(jsonBody)
			}

			req, err := http.NewRequest("POST", "/api/v1/message_user", reqBodyReader)
			assert.NoError(t, err)
			req.Header.Set("Mattermost-User-ID", "testauthtoken") // For MattermostAuthorizationRequired

			rr := httptest.NewRecorder()
			// Use mockP.ServeHTTP which will invoke the overridden MessageUser if /message_user is hit
			mockP.ServeHTTP(&plugin.Context{}, rr, req)

			assert.Equal(t, tt.expectedStatusCode, rr.Code)
			responseBody := strings.TrimSpace(rr.Body.String())
			
			if tt.exactJSONMatch {
				assert.JSONEq(t, tt.expectedResponse, responseBody, "Response body mismatch for test: "+tt.name)
			} else {
				assert.Contains(t, responseBody, tt.expectedResponse, "Response body mismatch for test: "+tt.name)
			}
			// We are not using testify's mock.Mock for mp.mockMessageUser, so AssertExpectations is not applicable here.
			// The assertions are within the mockMessageUser func itself.
		})
	}
}

```
