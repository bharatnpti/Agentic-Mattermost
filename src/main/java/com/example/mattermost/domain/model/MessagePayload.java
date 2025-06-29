package com.example.mattermost.domain.model;

public class MessagePayload {
    private String channelId;
    private String message;
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