package com.example.mattermost.domain.model;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;

import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.Objects;

@JsonIgnoreProperties(ignoreUnknown = true)
public class ActionNode {
    private String actionId;

    private String workflowId;
    private String actionName;
    private String actionDescription;
    private Map<String, Object> actionParams;
    private ActionStatus actionStatus; // PENDING, COMPLETED, FAILED, WAITING_FOR_INPUT

    private String actionResponse;

    private List<String> actionResponses = new ArrayList<>();

    // Constructors
    public ActionNode() {
        this.actionStatus = ActionStatus.PENDING;
    }

    public ActionNode(String actionId, String actionName, String actionDescription, Map<String, Object> actionParams, ActionStatus actionStatus) {
        this.actionId = actionId;
        this.actionName = actionName;
        this.actionDescription = actionDescription;
        this.actionParams = actionParams;
        this.actionStatus = actionStatus != null ? actionStatus : ActionStatus.PENDING;
    }

    // Getters and Setters
    public String getActionId() {
        return actionId;
    }

    public void setActionId(String actionId) {
        this.actionId = actionId;
    }

    public String getActionName() {
        return actionName;
    }

    public void setActionName(String actionName) {
        this.actionName = actionName;
    }

    public String getActionDescription() {
        return actionDescription;
    }

    public void setActionDescription(String actionDescription) {
        this.actionDescription = actionDescription;
    }

    public Map<String, Object> getActionParams() {
        return actionParams;
    }

    public void setActionParams(Map<String, Object> actionParams) {
        this.actionParams = actionParams;
    }

    public ActionStatus getActionStatus() {
        return actionStatus;
    }

    public void setActionStatus(ActionStatus actionStatus) {
        this.actionStatus = actionStatus;
    }

    public String getActionResponse() {
        return actionResponse;
    }

    public void setActionResponse(String actionResponse) {
        this.actionResponse = actionResponse;
        addActionResponse(actionResponse);
    }

    public String getWorkflowId() {
        return workflowId;
    }

    public void setWorkflowId(String workflowId) {
        this.workflowId = workflowId;
    }

    public List<String> getActionResponses() {
        return actionResponses;
    }

    public void setActionResponses(List<String> actionResponses) {
        this.actionResponses = actionResponses;
    }

    public void addActionResponse(String actionResponse) {
        this.actionResponses.add(actionResponse);
    }

    @Override
    public boolean equals(Object o) {
        if (this == o) return true;
        if (o == null || getClass() != o.getClass()) return false;
        ActionNode that = (ActionNode) o;
        return Objects.equals(actionId, that.actionId);
    }

    @Override
    public int hashCode() {
        return Objects.hash(actionId);
    }

    @Override
    public String toString() {
        return "ActionNode{" +
                "actionId='" + actionId + '\'' +
                ", workflowId='" + workflowId + '\'' +
                ", actionName='" + actionName + '\'' +
                ", actionDescription='" + actionDescription + '\'' +
                ", actionParams=" + actionParams +
                ", actionStatus=" + actionStatus +
                ", actionResponse='" + actionResponse + '\'' +
                '}';
    }
}
