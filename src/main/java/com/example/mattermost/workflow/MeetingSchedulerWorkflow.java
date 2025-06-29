package com.example.mattermost.workflow;

import com.example.mattermost.domain.model.Goal;
import io.temporal.workflow.SignalMethod;
import io.temporal.workflow.WorkflowInterface;
import io.temporal.workflow.WorkflowMethod;
import java.util.Map;

@WorkflowInterface
public interface MeetingSchedulerWorkflow {

    @WorkflowMethod
    void scheduleMeeting(Goal goal, String channelId, String userId, String threadId);

    @SignalMethod
    void onUserResponse(String actionId, String userInput, String threadId, String channelId);
}
