package com.example.mattermost.domain.model;

import java.util.List;

public class ActionEvaluationResult {

    public enum Status {
        COMPLETED,
        WAITING_FOR_INPUT
    }

    private Status status;
    private String reason;
    private List<String> missingOrRequired;

    // Constructors
    public ActionEvaluationResult() {}

    public ActionEvaluationResult(Status status, String reason, List<String> missingOrRequired) {
        this.status = status;
        this.reason = reason;
        this.missingOrRequired = missingOrRequired;
    }

    // Getters and Setters
    public Status getStatus() {
        return status;
    }

    public void setStatus(Status status) {
        this.status = status;
    }

    public String getReason() {
        return reason;
    }

    public void setReason(String reason) {
        this.reason = reason;
    }

    public List<String> getMissingOrRequired() {
        return missingOrRequired;
    }

    public void setMissingOrRequired(List<String> missingOrRequired) {
        this.missingOrRequired = missingOrRequired;
    }

    @Override
    public String toString() {
        return "ActionEvaluationResult{" +
                "status=" + status +
                ", reason='" + reason + '\'' +
                ", missingOrRequired=" + missingOrRequired +
                '}';
    }
}
