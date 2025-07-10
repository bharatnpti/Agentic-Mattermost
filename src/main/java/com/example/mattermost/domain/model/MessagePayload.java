package com.example.mattermost.domain.model;

import jakarta.validation.constraints.NotBlank;

public class MessagePayload {
    @NotBlank(message = "Channel ID is required")
    private String channelId;

    @NotBlank(message = "Message content is required")
    private String message;

    @NotBlank(message = "User ID is required")
    private String userId;

    private String threadId;

    // Constructors
    public MessagePayload() {}

    public MessagePayload(String channelId, String message, String userId, String threadId) {
        this.channelId = channelId;
        this.message = message;
        this.userId = userId;
        this.threadId = threadId;
    }

    // Getters and setters
    public String getChannelId() {
        return channelId;
    }

    public void setChannelId(String channelId) {
        this.channelId = channelId;
    }

    public String getMessage() {
        return message;
    }

    public void setMessage(String message) {
        this.message = message;
    }

    public String getUserId() {
        return userId;
    }

    public void setUserId(String userId) {
        this.userId = userId;
    }

    public String getThreadId() {
        return threadId;
    }

    public void setThreadId(String threadId) {
        this.threadId = threadId;
    }
} 