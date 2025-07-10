package com.example.mattermost.domain;

import com.example.mattermost.domain.model.ActionNode;
import com.example.mattermost.domain.model.Goal;

public class CurrentContext {

    private Goal goal;

    private ActionNode actionNode;
    private String currentThreadId;

    private String currentChannelId;

    private String currentUserId;

    public CurrentContext() {
    }

    public CurrentContext(Goal goal, ActionNode action, String currentThreadId, String currentChannelId, String currentUserId) {
        this.goal = goal;
        this.actionNode = action;
        this.currentThreadId = currentThreadId;
        this.currentChannelId = currentChannelId;
        this.currentUserId = currentUserId;
    }

    public ActionNode getActionNode() {
        return actionNode;
    }

    public void setActionNode(ActionNode actionNode) {
        this.actionNode = actionNode;
    }

    public String getCurrentThreadId() {
        return currentThreadId;
    }

    public void setCurrentThreadId(String currentThreadId) {
        this.currentThreadId = currentThreadId;
    }

    public String getCurrentChannelId() {
        return currentChannelId;
    }

    public void setCurrentChannelId(String currentChannelId) {
        this.currentChannelId = currentChannelId;
    }

    public String getCurrentUserId() {
        return currentUserId;
    }

    public void setCurrentUserId(String currentUserId) {
        this.currentUserId = currentUserId;
    }

    public Goal getGoal() {
        return goal;
    }

    public void setGoal(Goal goal) {
        this.goal = goal;
    }
}
