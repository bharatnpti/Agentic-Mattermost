package com.example.mattermost;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;
import java.util.Objects;

@JsonIgnoreProperties(ignoreUnknown = true)
public class Relationship {
    private String sourceActionId;
    private String targetActionId;
    private String type; // e.g., DEPENDS_ON

    // Constructors
    public Relationship() {}

    public Relationship(String sourceActionId, String targetActionId, String type) {
        this.sourceActionId = sourceActionId;
        this.targetActionId = targetActionId;
        this.type = type;
    }

    // Getters and Setters
    public String getSourceActionId() {
        return sourceActionId;
    }

    public void setSourceActionId(String sourceActionId) {
        this.sourceActionId = sourceActionId;
    }

    public String getTargetActionId() {
        return targetActionId;
    }

    public void setTargetActionId(String targetActionId) {
        this.targetActionId = targetActionId;
    }

    public String getType() {
        return type;
    }

    public void setType(String type) {
        this.type = type;
    }

    @Override
    public String toString() {
        return "Relationship{" +
               "sourceActionId='" + sourceActionId + '\'' +
               ", targetActionId='" + targetActionId + '\'' +
               ", type='" + type + '\'' +
               '}';
    }
}
