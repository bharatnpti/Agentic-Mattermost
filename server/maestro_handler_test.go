package main

import (
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockMaestroHandler struct {
	MaestroHandler
	processCalled bool
	lastArgs      []interface{}
}

func (m *mockMaestroHandler) mockProcessMaestroTask(agentName string, numMessages int, taskText string, channelID string, userID string, rootID string) error {
	m.processCalled = true
	m.lastArgs = []interface{}{agentName, numMessages, taskText, channelID, userID, rootID}
	return nil
}

func TestMessageHasBeenPosted(t *testing.T) {
	botUserID := "botid"
	api := &plugintest.API{}
	api.On("LogInfo", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	api.On("LogError", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	api.On("LogDebug", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	api.On("SendEphemeralPost", mock.Anything, mock.Anything).Return(&model.Post{}, nil)
	api.On("GetPost", mock.Anything).Return(&model.Post{}, nil)
	api.On("GetPostsForChannel", mock.Anything, mock.Anything, mock.Anything).Return(&model.PostList{Order: []string{}, Posts: map[string]*model.Post{}}, nil)

	h := &mockMaestroHandler{
		MaestroHandler: MaestroHandler{
			API:       api,
			BotUserID: botUserID,
			GetConfig: func() *configuration {
				return &configuration{CustomEndpoints: []CustomEndpoint{}, MaestroURL: "http://maestro", GraphQLPingIntervalSeconds: new(int)}
			},
		},
	}

	// Patch processMaestroTask
	h.ProcessTaskFunc = h.mockProcessMaestroTask

	t.Run("ignores bot user", func(t *testing.T) {
		post := &model.Post{UserId: botUserID, Message: "!hey-foo"}
		h.processCalled = false
		h.MessageHasBeenPosted(nil, post)
		assert.False(t, h.processCalled)
	})

	t.Run("ignores non-trigger message", func(t *testing.T) {
		post := &model.Post{UserId: "user1", Message: "hello world"}
		h.processCalled = false
		h.MessageHasBeenPosted(nil, post)
		assert.False(t, h.processCalled)
	})

	t.Run("invalid arguments sends ephemeral", func(t *testing.T) {
		post := &model.Post{UserId: "user1", Message: "!hey-"} // missing args
		h.processCalled = false
		h.MessageHasBeenPosted(nil, post)
		assert.False(t, h.processCalled)
		api.AssertCalled(t, "SendEphemeralPost", "user1", mock.AnythingOfType("*model.Post"))
	})

	t.Run("valid arguments calls processMaestroTask", func(t *testing.T) {
		post := &model.Post{UserId: "user1", Message: "!hey-meeting-agent -n 2 do something"}
		h.processCalled = false
		h.MessageHasBeenPosted(nil, post)
		assert.True(t, h.processCalled)
		assert.Equal(t, []interface{}{"meeting-agent", 2, "do something", post.ChannelId, post.UserId, post.Id}, h.lastArgs)
	})

	t.Run("valid arguments calls processMaestroTask without n", func(t *testing.T) {
		post := &model.Post{UserId: "user1", Message: "!hey-weather-agent do something"}
		h.processCalled = false
		h.MessageHasBeenPosted(nil, post)
		assert.True(t, h.processCalled)
		assert.Equal(t, []interface{}{"weather-agent", 0, "do something", post.ChannelId, post.UserId, post.Id}, h.lastArgs)
	})
}
