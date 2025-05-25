package main

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/pkg/errors"
)

// ServeHTTP demonstrates a plugin that handles HTTP requests by greeting the world.
// The root URL is currently <siteUrl>/plugins/com.mattermost.plugin-starter-template/api/v1/. Replace com.mattermost.plugin-starter-template with the plugin ID.
func (p *Plugin) ServeHTTP(c *plugin.Context, w http.ResponseWriter, r *http.Request) {
	router := mux.NewRouter()

	// Middleware to require that the user is logged in
	router.Use(p.MattermostAuthorizationRequired)

	apiRouter := router.PathPrefix("/api/v1").Subrouter()

	apiRouter.HandleFunc("/hello", p.HelloWorld).Methods(http.MethodGet)
	apiRouter.HandleFunc("/find_user", p.FindUser).Methods(http.MethodPost)
	apiRouter.HandleFunc("/message_user", p.MessageUser).Methods(http.MethodPost)
	apiRouter.HandleFunc("/find_group_channels_teams", p.FindGroupChannelsTeams).Methods(http.MethodPost)
	apiRouter.HandleFunc("/reply_to_message", p.ReplyToMessage).Methods(http.MethodPost)
	apiRouter.HandleFunc("/create_group_channels_teams", p.CreateGroupChannelsTeams).Methods(http.MethodPost)
	apiRouter.HandleFunc("/post_message_to_group_channels_teams", p.PostMessageToGroupChannelsTeams).Methods(http.MethodPost)

	router.ServeHTTP(w, r)
}

func (p *Plugin) MattermostAuthorizationRequired(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := r.Header.Get("Mattermost-User-ID")
		if userID == "" {
			http.Error(w, "Not authorized", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (p.Plugin) HelloWorld(w http.ResponseWriter, r *http.Request) {
	if _, err := w.Write([]byte("Hello, world!")); err != nil {
		p.API.LogError("Failed to write response", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

type FindUserRequest struct {
	UserID string `json:"user_id"`
}

func (p *Plugin) FindUser(w http.ResponseWriter, r *http.Request) {
	var request FindUserRequest
	// Ensure the request body is closed
	defer r.Body.Close()

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		p.API.LogError("Failed to decode request body for FindUser", "error", err.Error())
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if request.UserID == "" {
		http.Error(w, "Missing user_id in request body", http.StatusBadRequest)
		return
	}

	user, err := p.FindUser(request.UserID) // This calls the method in plugin.go
	if err != nil {
		var appErr *model.AppError
		// Check if the error from p.FindUser is or wraps a *model.AppError
		if errors.As(err, &appErr) {
			// Use the status code from the AppError if available
			if appErr.StatusCode == http.StatusNotFound {
				http.Error(w, "User not found", http.StatusNotFound)
			} else {
				p.API.LogError("AppError while finding user", "error", appErr.Error(), "user_id", request.UserID, "status_code", appErr.StatusCode)
				// Respond with the status code from the AppError
				http.Error(w, appErr.Message, appErr.StatusCode)
			}
		} else {
			// Generic error if it's not an AppError
			p.API.LogError("Failed to find user", "error", err.Error(), "user_id", request.UserID)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(user); err != nil {
		p.API.LogError("Failed to write user response", "error", err.Error(), "user_id", request.UserID)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

type MessageUserRequest struct {
	UserID  string `json:"user_id"`
	Message string `json:"message"`
}

type MessageUserResponse struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

func (p *Plugin) MessageUser(w http.ResponseWriter, r *http.Request) {
	var request MessageUserRequest
	defer r.Body.Close()

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		p.API.LogError("Failed to decode request body for MessageUser", "error", err.Error())
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if request.UserID == "" {
		http.Error(w, "Missing user_id in request body", http.StatusBadRequest)
		return
	}
	if request.Message == "" {
		http.Error(w, "Missing message in request body", http.StatusBadRequest)
		return
	}

	err := p.MessageUser(request.UserID, request.Message) // Calls the method in plugin.go
	if err != nil {
		var appErr *model.AppError
		if errors.As(err, &appErr) {
			// Log the detailed AppError
			p.API.LogError("AppError while sending message to user",
				"error", appErr.Error(),
				"user_id", request.UserID,
				"status_code", appErr.StatusCode,
				"details", appErr.DetailedError)

			// Respond with a user-friendly message and the status code from AppError
			http.Error(w, appErr.Message, appErr.StatusCode)
		} else {
			// Generic error
			p.API.LogError("Failed to send message to user", "error", err.Error(), "user_id", request.UserID)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	response := MessageUserResponse{Status: "ok", Message: "Message sent successfully"}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		p.API.LogError("Failed to write MessageUser success response", "error", err.Error(), "user_id", request.UserID)
		// Header already sent, so can't send http.Error here easily. Log is most important.
	}
}

// FindGroupRequest accommodates finding teams or channels.
type FindGroupRequest struct {
	TeamID    string `json:"team_id,omitempty"`
	ChannelID string `json:"channel_id,omitempty"`
}

// FindGroupResponse can hold details for a team or a channel.
type FindGroupResponse struct {
	Team           *model.Team          `json:"team,omitempty"`
	TeamMembers    []*model.TeamMember  `json:"team_members,omitempty"`
	Channel        *model.Channel       `json:"channel,omitempty"`
	ChannelMembers model.ChannelMembers `json:"channel_members,omitempty"` // Note: model.ChannelMembers is already a slice
}

func (p *Plugin) FindGroupChannelsTeams(w http.ResponseWriter, r *http.Request) {
	var request FindGroupRequest
	defer r.Body.Close()

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		p.API.LogError("Failed to decode request body for FindGroupChannelsTeams", "error", err.Error())
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	response := FindGroupResponse{}
	var err error // Declare error variable to be used by different blocks
	var action string

	if request.TeamID != "" && request.ChannelID != "" {
		http.Error(w, "Please provide either team_id or channel_id, not both.", http.StatusBadRequest)
		return
	}

	if request.TeamID != "" {
		action = "team"
		team, members, teamErr := p.FindTeamAndMembers(request.TeamID)
		if teamErr != nil {
			err = teamErr // Assign to common error variable
		} else {
			response.Team = team
			response.TeamMembers = members
		}
	} else if request.ChannelID != "" {
		action = "channel"
		channel, members, channelErr := p.FindChannelAndMembers(request.ChannelID)
		if channelErr != nil {
			err = channelErr // Assign to common error variable
		} else {
			response.Channel = channel
			response.ChannelMembers = members
		}
	} else {
		http.Error(w, "Missing team_id or channel_id in request body", http.StatusBadRequest)
		return
	}

	if err != nil { // Check error from FindTeamAndMembers or FindChannelAndMembers
		var appErr *model.AppError
		logMessage := "Failed to find " + action
		logContext := []interface{}{"error", err.Error()}
		if action == "team" {
			logContext = append(logContext, "team_id", request.TeamID)
		} else {
			logContext = append(logContext, "channel_id", request.ChannelID)
		}

		if errors.As(err, &appErr) {
			p.API.LogError("AppError while "+logMessage, append(logContext, "status_code", appErr.StatusCode, "details", appErr.DetailedError)...)
			http.Error(w, appErr.Message, appErr.StatusCode)
		} else {
			p.API.LogError(logMessage, logContext...)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		logCtx := []interface{}{"error", err.Error()}
		if request.TeamID != "" {
			logCtx = append(logCtx, "team_id", request.TeamID)
		} else if request.ChannelID != "" {
			logCtx = append(logCtx, "channel_id", request.ChannelID)
		}
		p.API.LogError("Failed to write FindGroupChannelsTeams response", logCtx...)
	}
}

type ReplyToMessageRequest struct {
	PostID  string `json:"post_id"`
	Message string `json:"message"`
}

func (p *Plugin) ReplyToMessage(w http.ResponseWriter, r *http.Request) {
	var request ReplyToMessageRequest
	defer r.Body.Close()

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		p.API.LogError("Failed to decode request body for ReplyToMessage", "error", err.Error())
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if request.PostID == "" {
		http.Error(w, "Missing post_id in request body", http.StatusBadRequest)
		return
	}
	if request.Message == "" {
		http.Error(w, "Missing message in request body", http.StatusBadRequest)
		return
	}

	createdPost, err := p.ReplyToMessage(request.PostID, request.Message)
	if err != nil {
		var appErr *model.AppError
		if errors.As(err, &appErr) {
			p.API.LogError("AppError while replying to message",
				"error", appErr.Error(),
				"post_id", request.PostID,
				"status_code", appErr.StatusCode,
				"details", appErr.DetailedError)
			http.Error(w, appErr.Message, appErr.StatusCode)
		} else {
			// Check for specific validation errors from plugin method
			if err.Error() == "ReplyToMessage: postID cannot be empty" ||
				err.Error() == "ReplyToMessage: message cannot be empty" ||
				err.Error() == "ReplyToMessage: botUserID is not set" { // botUserID error is less likely to be triggered by API user
				http.Error(w, err.Error(), http.StatusBadRequest)
			} else {
				p.API.LogError("Failed to reply to message", "error", err.Error(), "post_id", request.PostID)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
			}
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK) // Typically 200 OK for a reply, or 201 if considered a new resource
	if err := json.NewEncoder(w).Encode(createdPost); err != nil {
		p.API.LogError("Failed to write ReplyToMessage response", "error", err.Error(), "post_id", request.PostID)
	}
}

// CreateGroupRequest is a generic request for creating teams or channels.
type CreateGroupRequest struct {
	Type               string   `json:"type"` // "team" or "channel"
	TeamName           string   `json:"team_name,omitempty"`
	TeamDisplayName    string   `json:"team_display_name,omitempty"`
	TeamID             string   `json:"team_id,omitempty"` // Required for channel creation
	ChannelName        string   `json:"channel_name,omitempty"`
	ChannelDisplayName string   `json:"channel_display_name,omitempty"`
	ChannelType        string   `json:"channel_type,omitempty"` // 'O' for Open, 'P' for Private
	UserIDs            []string `json:"user_ids,omitempty"`
}

// CreateGroupResponse can hold the created team or channel.
type CreateGroupResponse struct {
	Team    *model.Team    `json:"team,omitempty"`
	Channel *model.Channel `json:"channel,omitempty"`
}

func (p *Plugin) CreateGroupChannelsTeams(w http.ResponseWriter, r *http.Request) {
	var request CreateGroupRequest
	defer r.Body.Close()

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		p.API.LogError("Failed to decode request body for CreateGroupChannelsTeams", "error", err.Error())
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	var createdEntity interface{}
	var err error
	action := request.Type // For logging context

	switch request.Type {
	case "team":
		if request.TeamName == "" || request.TeamDisplayName == "" {
			http.Error(w, "Missing team_name or team_display_name for type 'team'", http.StatusBadRequest)
			return
		}
		createdEntity, err = p.CreateTeamAndAddMembers(request.TeamName, request.TeamDisplayName, request.UserIDs)
	case "channel":
		if request.TeamID == "" || request.ChannelName == "" || request.ChannelDisplayName == "" || request.ChannelType == "" {
			http.Error(w, "Missing team_id, channel_name, channel_display_name, or channel_type for type 'channel'", http.StatusBadRequest)
			return
		}
		if request.ChannelType != model.ChannelTypeOpen && request.ChannelType != model.ChannelTypePrivate {
			http.Error(w, "Invalid channel_type. Must be 'O' (Open) or 'P' (Private).", http.StatusBadRequest)
			return
		}
		createdEntity, err = p.CreateChannelAndAddMembers(request.TeamID, request.ChannelName, request.ChannelDisplayName, request.ChannelType, request.UserIDs)
	default:
		http.Error(w, "Invalid type specified in request. Must be 'team' or 'channel'.", http.StatusBadRequest)
		return
	}

	if err != nil {
		var appErr *model.AppError
		logMessage := "Failed to create " + action
		logContext := []interface{}{"error", err.Error()}
		if action == "team" {
			logContext = append(logContext, "team_name", request.TeamName)
		} else { // channel
			logContext = append(logContext, "channel_name", request.ChannelName, "team_id", request.TeamID)
		}

		if errors.As(err, &appErr) {
			p.API.LogError("AppError while "+logMessage, append(logContext, "status_code", appErr.StatusCode, "details", appErr.DetailedError)...)
			http.Error(w, appErr.Message, appErr.StatusCode)
		} else {
			// Check for specific validation errors from the plugin method
			if strings.HasPrefix(err.Error(), "CreateTeamAndAddMembers:") || strings.HasPrefix(err.Error(), "CreateChannelAndAddMembers:") {
				http.Error(w, err.Error(), http.StatusBadRequest)
			} else {
				p.API.LogError(logMessage, logContext...)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
			}
		}
		return
	}

	response := CreateGroupResponse{}
	if team, ok := createdEntity.(*model.Team); ok {
		response.Team = team
	} else if channel, ok := createdEntity.(*model.Channel); ok {
		response.Channel = channel
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated) // 201 Created for successful creation
	if err := json.NewEncoder(w).Encode(response); err != nil {
		logCtx := []interface{}{"error", err.Error()}
		if request.Type == "team" && response.Team != nil {
			logCtx = append(logCtx, "team_id", response.Team.Id)
		} else if request.Type == "channel" && response.Channel != nil {
			logCtx = append(logCtx, "channel_id", response.Channel.Id)
		}
		p.API.LogError("Failed to write CreateGroupChannelsTeams response", logCtx...)
	}
}

// PostMessageRequest defines the structure for a request to post a message.
// Currently, it only supports posting to a channel.
type PostMessageRequest struct {
	ChannelID string `json:"channel_id"`
	Message   string `json:"message"`
	// Future: Could add ParentId or RootId for threading, Type ("channel", "group" - though groups are channels)
}

func (p *Plugin) PostMessageToGroupChannelsTeams(w http.ResponseWriter, r *http.Request) {
	var request PostMessageRequest
	defer r.Body.Close()

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		p.API.LogError("Failed to decode request body for PostMessageToGroupChannelsTeams", "error", err.Error())
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if request.ChannelID == "" {
		http.Error(w, "Missing channel_id in request body", http.StatusBadRequest)
		return
	}
	if request.Message == "" {
		http.Error(w, "Missing message in request body", http.StatusBadRequest)
		return
	}

	// Call the plugin method
	createdPost, err := p.PostMessageToChannel(request.ChannelID, request.Message)
	if err != nil {
		var appErr *model.AppError
		if errors.As(err, &appErr) {
			p.API.LogError("AppError while posting message to channel",
				"error", appErr.Error(),
				"channel_id", request.ChannelID,
				"status_code", appErr.StatusCode,
				"details", appErr.DetailedError)
			http.Error(w, appErr.Message, appErr.StatusCode)
		} else {
			// Check for specific validation errors from the plugin method
			if err.Error() == "PostMessageToChannel: channelID cannot be empty" ||
				err.Error() == "PostMessageToChannel: message cannot be empty" ||
				err.Error() == "PostMessageToChannel: botUserID is not set" {
				http.Error(w, err.Error(), http.StatusBadRequest)
			} else {
				p.API.LogError("Failed to post message to channel", "error", err.Error(), "channel_id", request.ChannelID)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
			}
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated) // 201 Created for successful message posting
	if err := json.NewEncoder(w).Encode(createdPost); err != nil {
		p.API.LogError("Failed to write PostMessageToGroupChannelsTeams response", "error", err.Error(), "channel_id", request.ChannelID)
	}
}
