package com.example.mattermost.domain.model;

import com.fasterxml.jackson.annotation.JsonIgnore;
import com.fasterxml.jackson.annotation.JsonProperty;
import jakarta.validation.Valid;
import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.NotNull;
import jakarta.validation.constraints.Size;

import java.util.List;
import java.util.Map;
import java.util.concurrent.ConcurrentHashMap;

public class Goal {
    @NotBlank(message = "Goal description cannot be blank")
    private String goal;

    @NotNull(message = "Nodes list cannot be null")
    @Size(min = 1, message = "At least one action node is required")
    @Valid
    private List<ActionNode> nodes;

    @Valid
    private List<Relationship> relationships;

    @JsonIgnore
    private Map<String, String> actionOutputs = new ConcurrentHashMap<>();

    private String workflowId;

    // Getters and setters
    public String getGoal() {
        return goal;
    }

    public void setGoal(String goal) {
        this.goal = goal;
    }

    public List<ActionNode> getNodes() {
        return nodes;
    }

    public void setNodes(List<ActionNode> nodes) {
        this.nodes = nodes;
    }

    public List<Relationship> getRelationships() {
        return relationships;
    }

    public void setRelationships(List<Relationship> relationships) {
        this.relationships = relationships;
    }

    @JsonIgnore
    public Map<String, String> getActionOutputs() {
        return actionOutputs;
    }

    @JsonProperty
    public void setActionOutputs(Map<String, String> actionOutputs) {
        this.actionOutputs = actionOutputs != null ? actionOutputs : new ConcurrentHashMap<>();
    }

    public ActionNode getNodeById(String actionId) {
        if (nodes == null || actionId == null) {
            return null;
        }
        return nodes.stream()
                .filter(node -> actionId.equals(node.getActionId()))
                .findFirst()
                .orElse(null);
    }

    public String getWorkflowId() {
        return workflowId;
    }

    public void setWorkflowId(String workflowId) {
        this.workflowId = workflowId;
    }

    @Override
    public String toString() {
        return "Goal{" +
                "goal='" + goal + '\'' +
                ", nodes=" + nodes +
                ", relationships=" + relationships +
                ", workflowId='" + workflowId + '\'' +
                '}';
    }
}
