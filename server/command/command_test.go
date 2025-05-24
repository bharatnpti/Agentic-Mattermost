package command

import (
	"bytes" // Required for template execution if we were to test it here, but not for this file's direct responsibility
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"text/template" // Required if we were to manually execute templates in tests for verification

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

const testBotUserIDGlobal = "testbotuserid"
const testOpenAIAPIURLGlobal = "https://api.openai.com/v1/chat/completions"
const defaultTestModelGlobal = "gpt-3.5-turbo"
const configuredTestModelGlobal = "test-configured-model"
const testTaskName = "summarize_chat"
// Updated testPromptTemplate to be more realistic for {{.Messages}}
const testPromptTemplateValid = "Summarize the following conversation: {{.Messages}}"
const testPromptTemplateMalformed = "Summarize: {{.Messages" // Malformed for testing parsing errors

var mockCallOpenAIFuncGlobal func(apiKey string, modelName string, message string, apiURL string) (string, error)

func TestHelloCommand(t *testing.T) {
	mockAPI := &plugintest.API{}
	mockAPI.On("RegisterCommand", mock.MatchedBy(func(cmd *model.Command) bool { return cmd.Trigger == helloCommandTrigger })).Return(nil).Once()
	mockAPI.On("RegisterCommand", mock.MatchedBy(func(cmd *model.Command) bool { return cmd.Trigger == maestroCommandTrigger })).Return(nil).Once()

	deps := HandlerDependencies{
		API:            mockAPI,
		BotUserID:      testBotUserIDGlobal,
		GetOpenAIModel: func() string { return defaultTestModelGlobal },
		GetOpenAITasks: func() map[string]string { return make(map[string]string) },
	}
	handler := NewCommandHandler(deps)
	args := &model.CommandArgs{Command: "/hello world", UserId: "testuserid", ChannelId: "testchannelid"}
	response, err := handler.Handle(args)
	assert.Nil(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, "Hello, world", response.Text)
	assert.Equal(t, model.CommandResponseTypeInChannel, response.ResponseType)
	mockAPI.AssertExpectations(t)
}

func TestMaestroCommand(t *testing.T) {
	originalGlobalMock := mockCallOpenAIFuncGlobal
	defer func() { mockCallOpenAIFuncGlobal = originalGlobalMock }()

	baseArgs := &model.CommandArgs{UserId: "testuserid", ChannelId: "testchannelid"}

	argParsingTestCases := []struct{ name, command, expectedMsg string }{
		{"Missing all arguments", "/maestro", "Usage: /maestro <task_name> <num_messages>"},
		{"Missing num_messages argument", "/maestro summarize_chat", "Usage: /maestro <task_name> <num_messages>"},
		{"num_messages not a number", "/maestro summarize_chat five", "Number of messages must be a positive integer."},
		{"num_messages is zero", "/maestro summarize_chat 0", "Number of messages must be a positive integer."},
		{"num_messages is negative", "/maestro summarize_chat -1", "Number of messages must be a positive integer."},
	}
	for _, tc := range argParsingTestCases {
		t.Run(tc.name, func(t *testing.T) {
			mockAPI := &plugintest.API{}
			mockAPI.On("RegisterCommand", mock.AnythingOfType("*model.Command")).Return(nil).Times(2)
			deps := HandlerDependencies{
				API:            mockAPI,
				GetOpenAIModel: func() string { return defaultTestModelGlobal },
				GetOpenAITasks: func() map[string]string { return make(map[string]string) },
			}
			handler := NewCommandHandler(deps)
			args := *baseArgs; args.Command = tc.command
			mockAPI.On("SendEphemeralPost", args.UserId, mock.MatchedBy(func(post *model.Post) bool {
				return strings.Contains(post.Message, tc.expectedMsg)
			})).Return(nil).Once()
			_, err := handler.Handle(&args)
			assert.Nil(t, err)
			mockAPI.AssertExpectations(t)
		})
	}

	t.Run("TestMissingAPIKey", func(t *testing.T) {
		mockAPI := &plugintest.API{}
		mockAPI.On("RegisterCommand", mock.AnythingOfType("*model.Command")).Return(nil).Times(2)
		deps := HandlerDependencies{
			API:             mockAPI, BotUserID: testBotUserIDGlobal,
			GetOpenAIAPIKey: func() string { return "" }, 
			GetOpenAIModel:  func() string { return defaultTestModelGlobal },
			GetOpenAITasks:  func() map[string]string { return map[string]string{testTaskName: testPromptTemplateValid} },
			CallOpenAIFunc:  func(a, mn, m, u string) (string, error) { t.Error("CallOpenAIFunc should not be called"); return "", nil },
		}
		handler := NewCommandHandler(deps)
		args := *baseArgs; args.Command = fmt.Sprintf("/maestro %s 5", testTaskName)
		mockAPI.On("SendEphemeralPost", args.UserId, mock.MatchedBy(func(post *model.Post) bool {
			return strings.Contains(post.Message, "_Maestro is processing your request..._")
		})).Return(nil).Once()
		mockAPI.On("SendEphemeralPost", args.UserId, mock.MatchedBy(func(post *model.Post) bool {
			return strings.Contains(post.Message, "API Key is not configured")
		})).Return(nil).Once()
		_, err := handler.Handle(&args)
		assert.Nil(t, err)
		mockAPI.AssertExpectations(t)
	})

	t.Run("TestValidMaestroTaskExecution_FullHistory", func(t *testing.T) {
		mockAPI := &plugintest.API{}
		mockAPI.On("RegisterCommand", mock.AnythingOfType("*model.Command")).Return(nil).Times(2)
		numMessages := 3
		cmd := fmt.Sprintf("/maestro %s %d", testTaskName, numMessages)
		openAIResp := "This is the summary."

		samplePostsData := []*model.Post{ // Oldest first for easy concatenation expectation
			{Id: "post1", UserId: "userA", Message: "Hello A", CreateAt: 1000},
			{Id: "post2", UserId: "userB", Message: "Hello B", CreateAt: 2000},
			{Id: "post3", UserId: "userC", Message: "Hello C", CreateAt: 3000},
		}
		postsMap := make(map[string]*model.Post)
		postOrder := make([]string, len(samplePostsData))
		var expectedFormattedMessagesBuilder strings.Builder
		for i, p := range samplePostsData {
			postsMap[p.Id] = p
			postOrder[len(samplePostsData)-1-i] = p.Id // Newest first in Order
			if i > 0 {
				expectedFormattedMessagesBuilder.WriteString("\n")
			}
			expectedFormattedMessagesBuilder.WriteString(fmt.Sprintf("User %s: %s", p.UserId, p.Message))
		}
		expectedFormattedMessages := expectedFormattedMessagesBuilder.String()
		postList := &model.PostList{Posts: postsMap, Order: postOrder}
		
		mockAPI.On("GetPostsForChannel", baseArgs.ChannelId, 0, numMessages).Return(postList, nil).Once()
		mockAPI.On("LogInfo", "Task name specified", "task", testTaskName).Once()

		// Manually execute template for expected prompt
		tmpl, _ := template.New("expected").Parse(testPromptTemplateValid)
		var expectedPromptBuf bytes.Buffer
		_ = tmpl.Execute(&expectedPromptBuf, struct{ Messages string }{Messages: expectedFormattedMessages})
		expectedPrompt := expectedPromptBuf.String()

		mockCallOpenAIFuncGlobal = func(apiKey, modelName, prompt, apiURL string) (string, error) {
			assert.Equal(t, configuredTestModelGlobal, modelName)
			assert.Equal(t, expectedPrompt, prompt) // Expecting the processed template
			return openAIResp, nil
		}
		defer func() { mockCallOpenAIFuncGlobal = nil }()

		deps := HandlerDependencies{
			API:             mockAPI, BotUserID: testBotUserIDGlobal, GetOpenAIAPIKey: func() string { return "test-api-key" },
			GetOpenAIModel:  func() string { return configuredTestModelGlobal },
			GetOpenAITasks:  func() map[string]string { return map[string]string{testTaskName: testPromptTemplateValid} },
			CallOpenAIFunc:  func(a, mn, m, u string) (string, error) { return mockCallOpenAIFuncGlobal(a, mn, m, u) },
			OpenAIAPIURL:    testOpenAIAPIURLGlobal,
		}
		handler := NewCommandHandler(deps)
		args := *baseArgs; args.Command = cmd

		mockAPI.On("SendEphemeralPost", args.UserId, mock.AnythingOfType("*model.Post")).Return(nil).Once() // Ack
		mockAPI.On("CreatePost", mock.MatchedBy(func(p *model.Post) bool { return p.Message == openAIResp })).Return(nil, nil).Once()
		_, err := handler.Handle(&args)
		assert.Nil(t, err)
		mockAPI.AssertExpectations(t)
	})

	t.Run("TestMaestroCommand_PromptTemplateParsingError", func(t *testing.T) {
		mockAPI := &plugintest.API{}
		mockAPI.On("RegisterCommand", mock.AnythingOfType("*model.Command")).Return(nil).Times(2)
		
		taskNameWithBadTemplate := "bad_template_task"
		cmd := fmt.Sprintf("/maestro %s 3", taskNameWithBadTemplate)

		// Mock GetPostsForChannel to return some valid posts, as parsing happens after fetching
		samplePostsData := []*model.Post{{Id: "post1", UserId: "userA", Message: "Hello A"}}
		postsMap := make(map[string]*model.Post); postOrder := make([]string, 0)
		for _, p := range samplePostsData { postsMap[p.Id] = p; postOrder = append(postOrder, p.Id) }
		postList := &model.PostList{Posts: postsMap, Order: postOrder}; postList.MakeNonNil()
		mockAPI.On("GetPostsForChannel", baseArgs.ChannelId, 0, 3).Return(postList, nil).Once()
		mockAPI.On("LogInfo", "Task name specified", "task", taskNameWithBadTemplate).Once()


		deps := HandlerDependencies{
			API:            mockAPI, BotUserID: testBotUserIDGlobal, GetOpenAIAPIKey: func() string { return "test-api-key" },
			GetOpenAIModel: func() string { return defaultTestModelGlobal },
			GetOpenAITasks: func() map[string]string { return map[string]string{taskNameWithBadTemplate: testPromptTemplateMalformed} },
			CallOpenAIFunc: func(a,mn,m,u string) (string,error) { t.Error("CallOpenAIFunc should not be called"); return "", nil; },
		}
		handler := NewCommandHandler(deps)
		args := *baseArgs; args.Command = cmd

		mockAPI.On("SendEphemeralPost", args.UserId, mock.MatchedBy(func(post *model.Post) bool { // Ack
			return strings.Contains(post.Message, "_Maestro is processing your request..._")
		})).Return(nil).Once()
		// Corrected LogError mock to expect 5 arguments (msg string + 2 pairs of keyval strings)
		mockAPI.On(
			"LogError",
			"Error parsing prompt template",        // arg 0: msg
			"task", taskNameWithBadTemplate,        // arg 1 & 2: keyval[0]
			"error", mock.AnythingOfType("string"), // arg 3 & 4: keyval[1] - value can be any string
		).Return(nil).Once()
		mockAPI.On("SendEphemeralPost", args.UserId, mock.MatchedBy(func(post *model.Post) bool { // Error message
			return strings.Contains(post.Message, fmt.Sprintf("Error parsing prompt template for task '%s'. Please check plugin configuration.", taskNameWithBadTemplate))
		})).Return(nil).Once()
		
		_, err := handler.Handle(&args)
		assert.Nil(t, err)
		mockAPI.AssertExpectations(t)
	})
	
	// TestMaestroTaskExecution_FewerPostsThanRequested - adapted
	t.Run("TestMaestroTaskExecution_FewerPostsThanRequested", func(t *testing.T) {
		mockAPI := &plugintest.API{}
		mockAPI.On("RegisterCommand", mock.AnythingOfType("*model.Command")).Return(nil).Times(2)
		taskName, numMessages := "analyze", 5
		cmd := fmt.Sprintf("/maestro %s %d", taskName, numMessages)
		openAIResp := "Sentiment positive."
		
		samplePostsData := []*model.Post{{Id: "post1", UserId: "userA", Message: "Great day!", CreateAt: 1000}}
		postsMap := make(map[string]*model.Post); postOrder := make([]string, 0)
		for _, p := range samplePostsData { postsMap[p.Id] = p; postOrder = append(postOrder, p.Id) }
		postList := &model.PostList{Posts: postsMap, Order: postOrder}; postList.MakeNonNil()
		
		mockAPI.On("GetPostsForChannel", baseArgs.ChannelId, 0, numMessages).Return(postList, nil).Once()
		mockAPI.On("LogInfo", "Task name specified", "task", taskName).Once()
		
		expectedFormattedMessages := "User userA: Great day!"
		currentPromptTemplate := "Analyze sentiment: {{.Messages}}"
		tmpl, _ := template.New("expected").Parse(currentPromptTemplate)
		var expectedPromptBuf bytes.Buffer
		_ = tmpl.Execute(&expectedPromptBuf, struct{ Messages string }{Messages: expectedFormattedMessages})
		expectedPrompt := expectedPromptBuf.String()

		mockCallOpenAIFuncGlobal = func(apiKey, modelName, prompt, apiURL string) (string,error) { 
			assert.Equal(t, configuredTestModelGlobal, modelName) 
			assert.Equal(t, expectedPrompt, prompt)
			return openAIResp,nil 
		}
		defer func() { mockCallOpenAIFuncGlobal = nil }()

		deps := HandlerDependencies{
			API: mockAPI, BotUserID: testBotUserIDGlobal, GetOpenAIAPIKey: func() string { return "test-api-key" },
			GetOpenAIModel: func() string { return configuredTestModelGlobal },
			GetOpenAITasks: func() map[string]string { return map[string]string{taskName: currentPromptTemplate} }, 
			CallOpenAIFunc: func(a,mn,m,u string) (string,error){ return mockCallOpenAIFuncGlobal(a,mn,m,u) }, 
			OpenAIAPIURL: testOpenAIAPIURLGlobal,
		}
		handler := NewCommandHandler(deps)
		args := *baseArgs; args.Command = cmd
		mockAPI.On("SendEphemeralPost", args.UserId, mock.AnythingOfType("*model.Post")).Return(nil).Once() 
		mockAPI.On("CreatePost", mock.MatchedBy(func(p *model.Post)bool{ return p.Message == openAIResp })).Return(nil,nil).Once()
		_, err := handler.Handle(&args)
		assert.Nil(t, err)
		mockAPI.AssertExpectations(t)
	})

	// Other tests (NoPostsInChannel, OpenAICallError, CreatePostError, DefaultModelUsed) need similar updates
	// to their GetOpenAITasks mock and expected prompt string in CallOpenAIFunc mock if they reach that point.

	t.Run("TestMaestroTaskExecution_NoPostsInChannel", func(t *testing.T) {
		mockAPI := &plugintest.API{}
		mockAPI.On("RegisterCommand", mock.AnythingOfType("*model.Command")).Return(nil).Times(2)
		taskName, numMessages := "find_topics", 3
		cmd := fmt.Sprintf("/maestro %s %d", taskName, numMessages)
		postList := &model.PostList{Posts: make(map[string]*model.Post), Order: []string{}} 
		
		mockAPI.On("GetPostsForChannel", baseArgs.ChannelId, 0, numMessages).Return(postList, nil).Once()
		deps := HandlerDependencies{
			API: mockAPI, BotUserID: testBotUserIDGlobal, GetOpenAIAPIKey: func() string { return "test-api-key" },
			GetOpenAIModel: func() string { return defaultTestModelGlobal },
			GetOpenAITasks: func() map[string]string { return map[string]string{taskName: testPromptTemplateValid} }, 
			CallOpenAIFunc: func(a,mn,m,u string) (string,error) { t.Error("CallOpenAIFunc should not be called"); return "",nil },
		}
		handler := NewCommandHandler(deps)
		args := *baseArgs; args.Command = cmd
		mockAPI.On("SendEphemeralPost", args.UserId, mock.MatchedBy(func(p *model.Post)bool{ return strings.Contains(p.Message, "_Maestro is processing your request..._")})).Return(nil).Once() 
		mockAPI.On("SendEphemeralPost", args.UserId, mock.MatchedBy(func(p *model.Post)bool{ return strings.Contains(p.Message, "No messages found to process")})).Return(nil).Once() 
		_, err := handler.Handle(&args)
		assert.Nil(t, err)
		mockAPI.AssertExpectations(t)
	})

	t.Run("TestOpenAICallError", func(t *testing.T) {
		mockAPI := &plugintest.API{}
		mockAPI.On("RegisterCommand", mock.AnythingOfType("*model.Command")).Return(nil).Times(2)
		mockError := errors.New("OpenAI API error")
		mockCallOpenAIFuncGlobal = func(k,mn,p,u string) (string,error) { return "", mockError }
		defer func() { mockCallOpenAIFuncGlobal = nil }()
		
		posts := map[string]*model.Post{"p1": {Id: "p1", UserId:"u1", Message:"m1", CreateAt:1000}}
		postList := &model.PostList{Posts: posts, Order: []string{"p1"}}
		mockAPI.On("GetPostsForChannel", mock.Anything, mock.Anything, mock.Anything).Return(postList, nil).Once()
		mockAPI.On("LogInfo", mock.Anything, mock.Anything, mock.Anything).Return().Maybe()

		deps := HandlerDependencies{
			API: mockAPI, BotUserID: testBotUserIDGlobal, GetOpenAIAPIKey: func() string { return "test-api-key" },
			GetOpenAIModel: func() string { return defaultTestModelGlobal },
			GetOpenAITasks: func() map[string]string { return map[string]string{testTaskName: testPromptTemplateValid} }, 
			CallOpenAIFunc: func(a,mn,m,u string) (string,error){ return mockCallOpenAIFuncGlobal(a,mn,m,u) }, 
			OpenAIAPIURL: testOpenAIAPIURLGlobal,
		}
		handler := NewCommandHandler(deps)
		args := *baseArgs; args.Command = fmt.Sprintf("/maestro %s 5", testTaskName)
		mockAPI.On("SendEphemeralPost", args.UserId, mock.MatchedBy(func(p *model.Post)bool{ return strings.Contains(p.Message, "_Maestro is processing your request..._")})).Return(nil).Once() 
		mockAPI.On("LogError", "Error calling OpenAI API for slash command", "error", mockError.Error(), "model", defaultTestModelGlobal).Once()
		mockAPI.On("SendEphemeralPost", args.UserId, mock.MatchedBy(func(p *model.Post)bool{ return strings.Contains(p.Message, "error occurred while contacting OpenAI")})).Return(nil).Once() 
		_, err := handler.Handle(&args)
		assert.Nil(t, err)
		mockAPI.AssertExpectations(t)
	})

	t.Run("TestCreatePostError", func(t *testing.T) {
		mockAPI := &plugintest.API{}
		mockAPI.On("RegisterCommand", mock.AnythingOfType("*model.Command")).Return(nil).Times(2)
		openAIResp := "This is a valid response."
		mockCallOpenAIFuncGlobal = func(k,mn,p,u string) (string,error) { return openAIResp, nil }
		defer func() { mockCallOpenAIFuncGlobal = nil }()

		posts := map[string]*model.Post{"p1": {Id: "p1", UserId:"u1", Message:"m1", CreateAt:1000}}
		postList := &model.PostList{Posts: posts, Order: []string{"p1"}}
		mockAPI.On("GetPostsForChannel", mock.Anything, mock.Anything, mock.Anything).Return(postList, nil).Once()
		mockAPI.On("LogInfo", mock.Anything, mock.Anything, mock.Anything).Return().Maybe()

		deps := HandlerDependencies{
			API: mockAPI, BotUserID: testBotUserIDGlobal, GetOpenAIAPIKey: func() string { return "test-api-key" },
			GetOpenAIModel: func() string { return defaultTestModelGlobal },
			GetOpenAITasks: func() map[string]string { return map[string]string{testTaskName: testPromptTemplateValid} }, 
			CallOpenAIFunc: func(a,mn,m,u string) (string,error){ return mockCallOpenAIFuncGlobal(a,mn,m,u) }, 
			OpenAIAPIURL: testOpenAIAPIURLGlobal,
		}
		handler := NewCommandHandler(deps)
		args := *baseArgs; args.Command = fmt.Sprintf("/maestro %s 5", testTaskName)
		mockAppError := model.NewAppError("CreatePost", "id", nil, "failed to create post", http.StatusInternalServerError)
		
		mockAPI.On("SendEphemeralPost", args.UserId, mock.MatchedBy(func(p *model.Post)bool{ return strings.Contains(p.Message, "_Maestro is processing your request..._")})).Return(nil).Once() 
		mockAPI.On("CreatePost", mock.MatchedBy(func(p *model.Post)bool{ return p.Message == openAIResp })).Return(nil, mockAppError).Once()
		mockAPI.On("LogError", "Failed to post OpenAI response for slash command", "error", mockAppError.Error()).Once()
		mockAPI.On("SendEphemeralPost", args.UserId, mock.MatchedBy(func(p *model.Post)bool{ return strings.Contains(p.Message, "error occurred while trying to post the OpenAI response")})).Return(nil).Once() 
		_, err := handler.Handle(&args)
		assert.Nil(t, err)
		mockAPI.AssertExpectations(t)
	})

	t.Run("TestMaestroCommand_DefaultModelUsed", func(t *testing.T) {
		mockAPI := &plugintest.API{}
		mockAPI.On("RegisterCommand", mock.AnythingOfType("*model.Command")).Return(nil).Times(2)
		
		taskName, numMessages := "summarize_default", 1
		cmd := fmt.Sprintf("/maestro %s %d", taskName, numMessages)
		openAIResp := "Response using default model."

		posts := map[string]*model.Post{"p1": {Id: "p1", UserId:"u1", Message:"m1", CreateAt:1000}}
		postList := &model.PostList{Posts: posts, Order: []string{"p1"}}
		mockAPI.On("GetPostsForChannel", baseArgs.ChannelId, 0, numMessages).Return(postList, nil).Once()
		mockAPI.On("LogInfo", "Task name specified", "task", taskName).Once()
		mockAPI.On("LogWarn", "OpenAI Model not configured, using default.", "model", defaultTestModelGlobal).Once()

		expectedFormattedMessages := "User u1: m1"
		currentPromptTemplate := "Default model task: {{.Messages}}"
		tmpl, _ := template.New("expected").Parse(currentPromptTemplate)
		var expectedPromptBuf bytes.Buffer
		_ = tmpl.Execute(&expectedPromptBuf, struct{ Messages string }{Messages: expectedFormattedMessages})
		expectedPrompt := expectedPromptBuf.String()

		mockCallOpenAIFuncGlobal = func(apiKey, modelName, prompt, apiURL string) (string,error) { 
			assert.Equal(t, defaultTestModelGlobal, modelName) 
			assert.Equal(t, expectedPrompt, prompt) 
			return openAIResp, nil 
		}
		defer func() { mockCallOpenAIFuncGlobal = nil }()

		deps := HandlerDependencies{
			API: mockAPI, BotUserID: testBotUserIDGlobal, GetOpenAIAPIKey: func() string { return "test-api-key" },
			GetOpenAIModel: func() string { return "" }, 
			GetOpenAITasks: func() map[string]string { return map[string]string{taskName: currentPromptTemplate} }, 
			CallOpenAIFunc: func(a,mn,m,u string) (string,error){ return mockCallOpenAIFuncGlobal(a,mn,m,u) }, 
			OpenAIAPIURL: testOpenAIAPIURLGlobal,
		}
		handler := NewCommandHandler(deps)
		args := *baseArgs; args.Command = cmd

		mockAPI.On("SendEphemeralPost", args.UserId, mock.AnythingOfType("*model.Post")).Return(nil).Once() 
		mockAPI.On("CreatePost", mock.MatchedBy(func(p *model.Post)bool{ return p.Message == openAIResp })).Return(nil,nil).Once()
		_, err := handler.Handle(&args)
		assert.Nil(t, err)
		mockAPI.AssertExpectations(t)
	})
}
