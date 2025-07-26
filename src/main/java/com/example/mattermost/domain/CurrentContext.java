package com.example.mattermost.domain;

import com.example.mattermost.domain.model.ActionNode;
import com.example.mattermost.domain.model.Goal;
import com.example.mattermost.integration.mattermost.model.User;

public class CurrentContext {

    private Goal goal;

    private ActionNode currentActionNode;
    private String currentThreadId;

    private String currentChannelId;

    private User user;

    public CurrentContext() {
    }

    public CurrentContext(Goal goal, ActionNode action, String currentThreadId, String currentChannelId, User user) {
        this.goal = goal;
        this.currentActionNode = action;
        this.currentThreadId = currentThreadId;
        this.currentChannelId = currentChannelId;
        this.user = user;
    }

    public ActionNode getCurrentActionNode() {
        return currentActionNode;
    }

    public void setCurrentActionNode(ActionNode currentActionNode) {
        this.currentActionNode = currentActionNode;
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

    public User getUser() {
        return user;
    }

    public void setUser(User user) {
        this.user = user;
    }

    public Goal getGoal() {
        return goal;
    }

    public void setGoal(Goal goal) {
        this.goal = goal;
    }

    @Override
    public String toString() {
        return "CurrentContext{" +
                "goal=" + goal +
                ", currentActionNode=" + currentActionNode +
                ", currentThreadId='" + currentThreadId + '\'' +
                ", currentChannelId='" + currentChannelId + '\'' +
                ", currentUserId='" + user.getId() + "-" + user.getUsername() + '\'' +
                '}';
    }
}
