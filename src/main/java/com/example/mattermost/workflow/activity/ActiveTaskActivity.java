package com.example.mattermost.workflow.activity;

import com.example.mattermost.domain.model.ActionStatus;
import io.temporal.activity.ActivityInterface;
import io.temporal.activity.ActivityMethod;

import java.util.Map;

@ActivityInterface
public interface ActiveTaskActivity {

    @ActivityMethod
    void updateActiveTask(String actionId, ActionStatus status, String workflowId, String channelId, String userId);
}