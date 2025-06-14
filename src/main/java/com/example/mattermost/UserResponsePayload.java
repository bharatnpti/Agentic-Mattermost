package com.example.mattermost;

import java.util.Map;

public class UserResponsePayload {

    private String workflowId;
    private String actionId;
    private Map<String, Object> userInput;

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

    public Map<String, Object> getUserInput() {
        return userInput;
    }

    public void setUserInput(Map<String, Object> userInput) {
        this.userInput = userInput;
    }
}
