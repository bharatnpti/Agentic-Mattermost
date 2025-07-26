package com.example.mattermost.refactor.model;

import java.io.Serializable;
import java.util.Map;

public class ActionResult implements Serializable {

    public enum Status {
        COMPLETED,
        FAILED,
        WAITING,    // Waiting for user signal
        RETRY,      // Needs to be retried
        SKIPPED     // Optional logic: if action was skipped
    }

    private String actionId;
    private Status status;
    private String message;         // human-readable or debug info
    private String output;          // raw output like response text, final meeting time, etc.
    private Map<String, Object> metadata; // optional map for structured values (e.g., parsed response)

    public ActionResult() {
    }

    public ActionResult(String actionId, Status status, String message, String output, Map<String, Object> metadata) {
        this.actionId = actionId;
        this.status = status;
        this.message = message;
        this.output = output;
        this.metadata = metadata;
    }

    // --- Getters and Setters ---

    public String getActionId() {
        return actionId;
    }

    public void setActionId(String actionId) {
        this.actionId = actionId;
    }

    public Status getStatus() {
        return status;
    }

    public void setStatus(Status status) {
        this.status = status;
    }

    public String getMessage() {
        return message;
    }

    public void setMessage(String message) {
        this.message = message;
    }

    public String getOutput() {
        return output;
    }

    public void setOutput(String output) {
        this.output = output;
    }

    public Map<String, Object> getMetadata() {
        return metadata;
    }

    public void setMetadata(Map<String, Object> metadata) {
        this.metadata = metadata;
    }

    // --- Convenience Methods ---

    public static ActionResult completed(String actionId, String output, Map<String, Object> metadata) {
        return new ActionResult(actionId, Status.COMPLETED, "Success", output, metadata);
    }

    public static ActionResult waiting(String actionId, String message) {
        return new ActionResult(actionId, Status.WAITING, message, null, null);
    }

    public static ActionResult failed(String actionId, String message) {
        return new ActionResult(actionId, Status.FAILED, message, null, null);
    }

    public static ActionResult retry(String actionId, String message) {
        return new ActionResult(actionId, Status.RETRY, message, null, null);
    }

    public static ActionResult skipped(String actionId, String reason) {
        return new ActionResult(actionId, Status.SKIPPED, reason, null, null);
    }
}
