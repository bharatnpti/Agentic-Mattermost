package com.example.mattermost;

public class LLMProcessingResult {
    private boolean success;
    private String actionResult;
    private ActionStatus actionStatus;

    // Getters
    public boolean isSuccess() { return success; }
    public String getActionResult() { return actionResult; }
    public ActionStatus getActionStatus() { return actionStatus; }

    public LLMProcessingResult(boolean success, String actionResult, ActionStatus actionStatus) {
        this.success = success;
        this.actionResult = actionResult;
        this.actionStatus = actionStatus;
    }

    public LLMProcessingResult() {
    }

    public void setSuccess(boolean success) {
        this.success = success;
    }

    public void setActionResult(String actionResult) {
        this.actionResult = actionResult;
    }

    public void setActionStatus(ActionStatus actionStatus) {
        this.actionStatus = actionStatus;
    }
}
