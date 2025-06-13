package com.example.mattermost;

public enum ActionStatus {
    PENDING,
    WAITING_FOR_INPUT,
    PROCESSING, // Indicates that the action is currently being processed after input
    COMPLETED,
    FAILED,
    SKIPPED // If an action cannot be run due to failed dependencies
}
