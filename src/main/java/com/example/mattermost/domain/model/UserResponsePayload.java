package com.example.mattermost.domain.model;

import java.util.Map;
import jakarta.validation.constraints.NotBlank;

public class UserResponsePayload {

    private String channelId;

    @NotBlank(message = "Workflow ID is required")
    private String workflowId;

    @NotBlank(message = "Action ID is required")
    private String actionId;

    @NotBlank(message = "User input is required")
    private String userInput;

    private String threadId;

    public String getWorkflowId() {
        return workflowId;
    }

    public void setWorkflowId(String workflowId) {
        this.workflowId = workflowId;
    }

    public String getActionId() {
        return actionId;
    }

    public void setActionId(String actionId) {
        this.actionId = actionId;
    }

    public String getUserInput() {
        return userInput;
    }

    public void setUserInput(String userInput) {
        this.userInput = userInput;
    }

    public String getThreadId() {
        return threadId;
    }

    public void setThreadId(String threadId) {
        this.threadId = threadId;
    }

    public String getChannelId() {
        return channelId;
    }

    public void setChannelId(String channelId) {
        this.channelId = channelId;
    }
}
