package com.example.mattermost.domain.model;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;
import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.concurrent.ConcurrentHashMap;

@JsonIgnoreProperties(ignoreUnknown = true)
public class Goal {

    private String workflowId;
    private String goal;
    private List<ActionNode> nodes;
    private List<Relationship> relationships;

    // Field to store action outputs, should be part of JSON for workflow state
    private Map<String, String> actionOutputs;


    // Constructors
    public Goal() {
        this.actionOutputs = new ConcurrentHashMap<>();
    }

    public Goal(String goal, List<ActionNode> nodes, List<Relationship> relationships) {
        this.goal = goal;
        this.nodes = nodes;
        this.relationships = relationships;
        this.actionOutputs = new ConcurrentHashMap<>();
    }

    // Getters and Setters
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

    public Map<String, String> getActionOutputs() {
        if (this.actionOutputs == null) {
            this.actionOutputs = new ConcurrentHashMap<>();
        }
        return actionOutputs;
    }

    public void setActionOutputs(Map<String, String> actionOutputs) {
        this.actionOutputs = actionOutputs;
    }

    // Helper method to get an ActionNode by its ID
    public ActionNode getNodeById(String actionId) {
        if (nodes == null) {
            return null;
        }
        return nodes.stream()
                    .filter(node -> Objects.equals(node.getActionId(), actionId))
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
               ", nodesCount=" + (nodes != null ? nodes.size() : 0) +
               ", relationshipsCount=" + (relationships != null ? relationships.size() : 0) +
               '}';
    }
}
