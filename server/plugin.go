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
	// Since /maestro is removed, ParseArguments, GetOpenAIAPIKey, etc., are not strictly needed by command.go
	// However, keeping them here as per instructions in case other (future) commands might use them.
	// If no other command uses them, these could be cleaned up from HandlerDependencies struct too.
	dependencies := command.HandlerDependencies{
		API:       p.API,
		BotUserID: p.botUserID,
		// The following fields are likely unused by 'hello' command but kept for now.
		GetOpenAIAPIKey: func() string {
			return p.getConfiguration().OpenAIAPIKey
		},
		CallOpenAIFunc: CallOpenAIAPIFunc,
		OpenAIAPIURL:   OpenAIAPIURL,
		ParseArguments: func(argsString string) (string, int, error) {
			return p.parseMaestroArgs(argsString)
		},
	}
	// NewCommandHandler will now only register the 'hello' command, as the /maestro registration
	// has been removed from command.go's NewCommandHandler.
	// No explicit unregistration call is needed here if the registration is removed from the command package.
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

	// 3. Extract the arguments string
	// Trim the prefix and then any leading/trailing whitespace from the arguments.
	// Important: Use the original post.Message for slicing to preserve case in argumentsString if needed later,
	// though parsing logic like parseOpenAICommandArgs might handle case for task_name.
	argumentsString := strings.TrimSpace(post.Message[len(triggerPrefix):])

	// 5. Add logging
	p.API.LogInfo(
		"Detected '!maestro' prefix.",
		"user_id", post.UserId,
		"channel_id", post.ChannelId,
		"original_message", post.Message,
		"arguments_string", argumentsString,
	)

	// 4. Placeholder for parsing and calling refactored logic
	// 4. Placeholder for parsing and calling refactored logic
	taskName, numMessages, err := p.parseMaestroArgs(argumentsString)
	if err != nil {
		p.API.SendEphemeralPost(post.UserId, &model.Post{
			ChannelId: post.ChannelId,
			Message:   err.Error(),
			RootId:    post.Id, // Thread the error message to the command
		})
		p.API.LogError("Failed to parse arguments for !maestro command", "error", err.Error(), "user_id", post.UserId, "arguments", argumentsString)
		return
	}

	if err := p.processMaestroTask(taskName, numMessages, post.ChannelId, post.UserId, post.Id); err != nil {
		// processMaestroTask is responsible for its own user-facing error posts and logging critical errors.
		// We can log an additional message here if needed, e.g.
		p.API.LogError("Error processing !maestro task", "error", err.Error(), "user_id", post.UserId, "task_name", taskName)
		// No need to send another ephemeral post if processMaestroTask already did.
	}
}

// parseMaestroArgs parses the arguments for the !maestro command.
// It's adapted from parseOpenAICommandArgs.
func (p *Plugin) parseMaestroArgs(argsString string) (taskName string, numMessages int, err error) {
	fields := strings.Fields(argsString)
	// For !maestro, the argsString is everything after "!maestro ".
	// So, fields[0] is the first part of the task_name or -n.
	if len(fields) < 1 && argsString != "summarize" { // Need at least a task_name, unless it's summarize with no further args
		// If argsString is exactly "summarize", it's valid. Otherwise, if fields is empty, it's an error.
		if argsString != "summarize" || len(fields) > 0 {
			return "", 0, fmt.Errorf("not enough arguments. Usage: !maestro <task_name> [-n <num_messages>]")
		}
	}
	
	if argsString == "summarize" && len(fields) == 0 { // Edge case: "!maestro summarize"
		return "summarize", command.DefaultNumMessages, nil
	}
	if len(fields) == 0 { // Should be caught above, but as a safeguard
		return "",0, fmt.Errorf("task_name cannot be empty. Usage: !maestro <task_name> [-n <num_messages>]")
	}


	numMessages = command.DefaultNumMessages // Default value
	taskNameParts := []string{}
	taskNameEndIndex := -1

	// Iterate through fields to find task_name and -n parameter
	// For prefix commands, fields are the arguments themselves.
	for i := 0; i < len(fields); i++ {
		if fields[i] == "-n" {
			taskNameEndIndex = i - 1 // Task name ends before -n
			if i+1 < len(fields) {
				val, convErr := strconv.Atoi(fields[i+1])
				if convErr != nil {
					return "", 0, fmt.Errorf("invalid value for -n: '%s'. It must be an integer", fields[i+1])
				}
				if val <= 0 {
					return "", 0, fmt.Errorf("invalid value for -n: %d. It must be a positive integer", val)
				}
				numMessages = val
				i++ // Skip the value part of -n
			} else {
				return "", 0, fmt.Errorf("missing value for -n argument")
			}
		} else if strings.HasPrefix(fields[i], "-") && taskNameEndIndex != -1 {
			// If we have already parsed -n (i.e. taskNameEndIndex is set), another flag is an error
			return "", 0, fmt.Errorf("unknown or misplaced flag: %s. Flags like -n must come after the task name", fields[i])
		} else if taskNameEndIndex == -1 { // Still collecting task name, or expecting -n
			if strings.HasPrefix(fields[i], "-") && fields[i] != "-n" { // It's a flag, but not -n, and we are expecting task name or -n
				return "", 0, fmt.Errorf("unknown or misplaced flag: %s. Only -n is a recognized flag", fields[i])
			}
			taskNameParts = append(taskNameParts, fields[i]) // Append if part of task name
		} else {
			// This case means we have arguments after -n <value> which are not allowed (and not flags handled above)
			return "", 0, fmt.Errorf("unexpected argument: %s. No arguments are allowed after -n <value>", fields[i])
		}
	}

	if len(taskNameParts) == 0 {
         // This case can be hit if only "-n" "10" is provided, which is invalid.
         if argsString == "summarize" { // "!maestro summarize" is valid and handled earlier.
             return "summarize", numMessages, nil // If -n was parsed for summarize
         }
		return "", 0, fmt.Errorf("task_name cannot be empty. Usage: !maestro <task_name> [-n <num_messages>]")
	}
	taskName = strings.Join(taskNameParts, " ")

	return taskName, numMessages, nil
}

// processMaestroTask contains the core logic for handling a maestro task.
func (p *Plugin) processMaestroTask(taskName string, numMessages int, channelID string, userID string, rootID string) error {
	if taskName == "" { // Should be caught by parser, but defense in depth
		p.API.SendEphemeralPost(userID, &model.Post{
			ChannelId: channelID,
			Message:   "Please provide a task name. Usage: !maestro <task_name> [-n <num_messages>]",
			RootId:    rootID,
		})
		return fmt.Errorf("taskName is empty")
	}

	apiKey := p.getConfiguration().OpenAIAPIKey
	if apiKey == "" {
		p.API.SendEphemeralPost(userID, &model.Post{
			ChannelId: channelID,
			Message:   "The OpenAI API Key is not configured. Please contact your system administrator.",
			RootId:    rootID,
		})
		return fmt.Errorf("OpenAI API Key is not configured")
	}

	// Fetch messages
	// For a prefix command, the rootID is the ID of the '!maestro' post itself.
	// We want messages *before* this post. GetPostsBefore() is suitable.
	// If we use GetPostsForChannel, we need to be careful with pagination and filtering.
	// Let's stick to GetPostsForChannel for consistency with slash command, and filter.
	postList, appErr := p.API.GetPostsForChannel(channelID, 0, numMessages+10) // Fetch more to ensure we can filter and still get numMessages
	if appErr != nil {
		p.API.LogError("Failed to fetch posts for channel", "channel_id", channelID, "error", appErr.Error())
		p.API.SendEphemeralPost(userID, &model.Post{
			ChannelId: channelID,
			Message:   "An error occurred while fetching messages. Please try again later.",
			RootId:    rootID,
		})
		return errors.Wrap(appErr, "failed to fetch posts")
	}

	var fetchedMessages []string
	count := 0
	// Posts are returned newest first. We iterate to find messages *before* the rootID post.
	for _, postID := range postList.Order {
		if count >= numMessages {
			break
		}
		post := postList.Posts[postID]
		if post.Id == rootID { // Stop if we reach the command message itself
			continue // Don't include the command message itself in the context
		}
		// Skip posts from the bot itself to avoid loops.
		// Also skip other !maestro commands to prevent weird recursion if context is large.
		if post.UserId == p.botUserID || strings.HasPrefix(strings.ToLower(post.Message), "!maestro") {
			continue
		}
		fetchedMessages = append([]string{post.Message}, fetchedMessages...) // Prepend to reverse order to chronological
		count++
	}
	// If fetchedMessages is still longer than numMessages due to GetPostsBefore behavior or initial fetch, truncate.
	// This logic is slightly different because we are explicitly iterating and counting.
	// The above loop already limits to numMessages.

	messagesString := strings.Join(fetchedMessages, "\n")
	var finalPrompt string
	if strings.ToLower(taskName) == "summarize" {
		finalPrompt = fmt.Sprintf("Summarize the following messages:\n%s", messagesString)
	} else {
		finalPrompt = fmt.Sprintf("User query: %s\n%s", taskName, messagesString)
	}

	openAIResponse, err := CallOpenAIAPIFunc(apiKey, finalPrompt, OpenAIAPIURL) // Global vars
	if err != nil {
		p.API.LogError("Error calling OpenAI API for !maestro task", "error", err.Error(), "user_id", userID, "task_name", taskName)
		p.API.SendEphemeralPost(userID, &model.Post{
			ChannelId: channelID,
			Message:   "An error occurred while contacting OpenAI. Please try again later or contact an administrator.",
			RootId:    rootID,
		})
		return err
	}

	responsePost := &model.Post{
		UserId:    p.botUserID,
		ChannelId: channelID,
		Message:   openAIResponse,
		RootId:    rootID, // Thread the response to the !maestro message
		ParentId:  rootID, // Also set ParentId for threading
	}

	if _, appErr := p.API.CreatePost(responsePost); appErr != nil {
		p.API.LogError("Failed to post OpenAI response for !maestro task", "error", appErr.Error(), "user_id", userID)
		p.API.SendEphemeralPost(userID, &model.Post{
			ChannelId: channelID,
			Message:   "An error occurred while trying to post the OpenAI response.",
			RootId:    rootID,
		})
		return errors.Wrap(appErr, "failed to create response post")
	}
	return nil
}
