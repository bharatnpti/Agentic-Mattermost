package main

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
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

	// maestroHandler is the handler for !maestro commands.
	maestroHandler *MaestroHandler

	// router is the HTTP router for the plugin.
	router *mux.Router
}

// SetAPI allows to set the API for testing purposes
func (p *Plugin) SetAPI(api plugin.API) {
	p.API = api
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
	// Since /maestro is removed, ParseArguments, GetOpenAIAPIKey, etc., are not strictly needed by command.go
	// However, keeping them here as per instructions in case other (future) commands might use them.
	// If no other command uses them, these could be cleaned up from HandlerDependencies struct too.
	// Updated to reflect simplified HandlerDependencies in command.go
	dependencies := command.HandlerDependencies{
		API:       p.API,
		BotUserID: p.botUserID,
	}
	// NewCommandHandler will now only register the 'hello' command, as the /maestro registration
	// has been removed from command.go's NewCommandHandler.
	// No explicit unregistration call is needed here if the registration is removed from the command package.
	p.commandClient = command.NewCommandHandler(dependencies)

	// Initialize MaestroHandler
	p.maestroHandler = NewMaestroHandler(p.API, p.botUserID, p.getConfiguration, CallGraphQLAgentFunc)

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

	// Initialize the HTTP router
	p.initializeRouter()

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
	// 1. Ignore posts made by the bot itself
	if post.UserId == p.botUserID {
		return
	}

	// 2. Check if the message starts with !maestro (case-insensitive)
	messageLowercase := strings.ToLower(post.Message)
	triggerPrefix := "!maestro"

	if !strings.HasPrefix(messageLowercase, triggerPrefix) {
		return
	}

	// Pass the call to the maestroHandler if it's initialized
	if p.maestroHandler != nil {
		p.maestroHandler.MessageHasBeenPosted(c, post)
	} else {
		p.API.LogError("MaestroHandler not initialized. MessageHasBeenPosted will not be handled for !maestro.")
	}
}

// parseMaestroArgs was used by the old /maestro command and is no longer needed.
// func (p *Plugin) parseMaestroArgs(argsString string) (taskName string, numMessages int, err error) {
// ... (original function content) ...
// }

// ServeHTTP handles HTTP requests to the plugin.
func (p *Plugin) ServeHTTP(c *plugin.Context, w http.ResponseWriter, r *http.Request) {
	if p.router == nil {
		p.API.LogError("HTTP router not initialized")
		http.Error(w, "Plugin router not initialized", http.StatusInternalServerError)
		return
	}
	p.router.ServeHTTP(w, r)
}
