package com.example.mattermost;

import io.temporal.activity.ActivityInterface;
import io.temporal.activity.ActivityMethod;

import java.util.Map;

@ActivityInterface
public interface ActiveTaskActivity {

    @ActivityMethod
    void updateActiveTask(String actionId, ActionStatus status, String workflowId, String channelId, String userId);
}