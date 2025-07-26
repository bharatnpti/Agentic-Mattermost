package com.example.mattermost.refactor.workflow;

import com.example.mattermost.domain.CurrentContext;
import com.example.mattermost.domain.model.ActionStatus;
import com.example.mattermost.domain.model.Goal;
import com.example.mattermost.integration.mattermost.model.User;
import io.temporal.workflow.QueryMethod;
import io.temporal.workflow.SignalMethod;
import io.temporal.workflow.WorkflowInterface;
import io.temporal.workflow.WorkflowMethod;

import java.util.List;
import java.util.Map;

@WorkflowInterface
public interface DagExecutorWorkflow {

    @WorkflowMethod
    String executeGoal(String workflowId, String task, String channelId, String threadId, User user);

    @QueryMethod
    Map<String, ActionStatus> getChildWorkflowStatuses();

    @QueryMethod
    Map<String, List<String>> getChildWorkflowStatusMessages();

    @QueryMethod
    String getOverallStatus();

    @SignalMethod
    void onChildCompleted(String actionId, ActionStatus actionStatus, String message);

    @SignalMethod
    void onUserResponse(String actionId, String userInput, String threadId, String channelId);
}
