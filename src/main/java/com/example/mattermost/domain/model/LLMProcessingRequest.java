package com.example.mattermost.domain.model;

public class LLMProcessingRequest {
    private String goal;
    private String convHistory;
    private ActionNode action;

    public LLMProcessingRequest(String goal, String convHistory, ActionNode action) {
        this.goal = goal;
        this.convHistory = convHistory;
        this.action = action;
    }

    public LLMProcessingRequest() {
    }

    // Getters
    public String getGoal() { return goal; }
    public String getConvHistory() { return convHistory; }
    public ActionNode getAction() { return action; }

    public void setGoal(String goal) {
        this.goal = goal;
    }

    public void setConvHistory(String convHistory) {
        this.convHistory = convHistory;
    }

    public void setAction(ActionNode action) {
        this.action = action;
    }
}
