package com.example.mattermost.domain.model;

import com.fasterxml.jackson.annotation.JsonIgnore;
import com.fasterxml.jackson.annotation.JsonIgnoreProperties;
import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.NotNull;

import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.Objects;

@JsonIgnoreProperties(ignoreUnknown = true)
public class ActionNode {
    @NotBlank(message = "Action ID cannot be blank")
    private String actionId;

    @NotBlank(message = "Action name cannot be blank")
    private String actionName;

    @NotBlank(message = "Action description cannot be blank")
    private String actionDescription;

    private Map<String, Object> actionParams;

    @NotNull(message = "Action status cannot be null")
    private ActionStatus actionStatus = ActionStatus.PENDING;

    private String workflowId;

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
        if (actionResponse != null && !actionResponse.isEmpty()) {
            this.actionResponses.add(actionResponse);
        }
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
        this.actionResponses = actionResponses != null ? actionResponses : new ArrayList<>();
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
                ", actionName='" + actionName + '\'' +
                ", actionDescription='" + actionDescription + '\'' +
                ", actionStatus=" + actionStatus +
                ", workflowId='" + workflowId + '\'' +
                '}';
    }
}
