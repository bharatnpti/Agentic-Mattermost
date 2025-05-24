package main

import (
	"net/http"
	"sync"
	"time"

	"github.com/mattermost/mattermost-plugin-starter-template/server/command"
	"github.com/mattermost/mattermost-plugin-starter-template/server/store/kvstore"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/mattermost/mattermost/server/public/pluginapi"
	"github.com/mattermost/mattermost/server/public/pluginapi/cluster"
	"github.com/pkg/errors"
)

// Plugin implements the interface expected by the Mattermost server to communicate between the server and plugin processes.
type Plugin struct {
	plugin.MattermostPlugin

	// kvstore is the client used to read/write KV records for this plugin.
	kvstore kvstore.KVStore

	// client is the Mattermost server API client.
	client *pluginapi.Client

	// commandClient is the client used to register and execute slash commands.
	commandClient command.Command

	backgroundJob *cluster.Job

	// configurationLock synchronizes access to the configuration.
	configurationLock sync.RWMutex

	// configuration is the active plugin configuration. Consult getConfiguration and
	// setConfiguration for usage.
	configuration *configuration

	// botUserID is the user ID of the plugin's bot account.
	botUserID string
}

// OnActivate is invoked when the plugin is activated. If an error is returned, the plugin will be deactivated.
func (p *Plugin) OnActivate() error {
	p.client = pluginapi.NewClient(p.API, p.Driver)

	bot := &model.Bot{
		Username:    "maestro",
		DisplayName: "maestro",
		Description: "Bot for Agentic Capabilities.",
	}
	// Use p.client.Bot.EnsureBot (or p.API.EnsureBot if available in the specific Mattermost version)
	// For pluginapi.Client, it's typically p.client.Bot.EnsureBot
	// However, EnsureBot is also often exposed via p.API.EnsureBot in newer versions.
	// Let's try p.API.EnsureBot first as it's simpler and often preferred if available.
	// If that fails, we can try p.client.Bot.EnsureBot.
	// Based on common plugin patterns, p.API.EnsureBot is the direct way if using plugin.API helpers.
	// The pluginapi.Client is more for remote plugin execution.
	// Let's use p.client.Bot.EnsureBot() as p.API.EnsureBot and p.API.Helpers.EnsureBot failed.
	// The p.client is pluginapi.NewClient(p.API, p.Driver)
	botUserID, err := p.client.Bot.EnsureBot(bot)
	if err != nil {
		return errors.Wrap(err, "failed to ensure bot account using p.client.Bot.EnsureBot")
	}
	p.botUserID = botUserID

	p.kvstore = kvstore.NewKVStore(p.client)

	// Construct HandlerDependencies
	dependencies := command.HandlerDependencies{
		API:       p.API,
		BotUserID: p.botUserID,
		GetOpenAIAPIKey: func() string {
			return p.getConfiguration().OpenAIAPIKey
		},
		CallOpenAIFunc: CallOpenAIAPIFunc, // This is the global var from main package
		OpenAIAPIURL:   OpenAIAPIURL,      // This is the global var from main package
	}
	p.commandClient = command.NewCommandHandler(dependencies)

	job, err := cluster.Schedule(
		p.API,
		"BackgroundJob",
		cluster.MakeWaitForRoundedInterval(1*time.Hour),
		p.runJob,
	)
	if err != nil {
		return errors.Wrap(err, "failed to schedule background job")
	}

	p.backgroundJob = job

	return nil
}

// OnDeactivate is invoked when the plugin is deactivated.
func (p *Plugin) OnDeactivate() error {
	if p.backgroundJob != nil {
		if err := p.backgroundJob.Close(); err != nil {
			p.API.LogError("Failed to close background job", "err", err)
		}
	}
	return nil
}

// This will execute the commands that were registered in the NewCommandHandler function.
func (p *Plugin) ExecuteCommand(c *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	response, err := p.commandClient.Handle(args)
	if err != nil {
		return nil, model.NewAppError("ExecuteCommand", "plugin.command.execute_command.app_error", nil, err.Error(), http.StatusInternalServerError)
	}
	return response, nil
}

// See https://developers.mattermost.com/extend/plugins/server/reference/

// MessageHasBeenPosted is invoked when a message is posted to a channel.
func (p *Plugin) MessageHasBeenPosted(c *plugin.Context, post *model.Post) {
	// Ignore posts made by the bot itself
	if post.UserId == p.botUserID {
		return
	}

	p.API.LogInfo("MessageHasBeenPosted hook triggered (OpenAI call removed)", "user_id", post.UserId, "message", post.Message, "channel_id", post.ChannelId)

	// All OpenAI related logic including API key checks, CallOpenAIAPIFunc,
	// and p.API.CreatePost for the response have been removed as per the subtask.
	// The function now only logs the incoming message details.
}
