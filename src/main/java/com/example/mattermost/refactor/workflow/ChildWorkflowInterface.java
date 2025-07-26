package com.example.mattermost.refactor.workflow;

import com.example.mattermost.domain.CurrentContext;
import com.example.mattermost.domain.model.ActionNode;
import com.example.mattermost.domain.model.ActionStatus;
import com.example.mattermost.domain.model.Goal;
import io.temporal.workflow.QueryMethod;
import io.temporal.workflow.SignalMethod;
import io.temporal.workflow.WorkflowInterface;
import io.temporal.workflow.WorkflowMethod;

import java.util.List;

@WorkflowInterface
public interface ChildWorkflowInterface {
    @WorkflowMethod
    String executeAction(CurrentContext context);

    @SignalMethod
    void updateStatus(String status);

    @SignalMethod
    void setActionResponse(String response);

    @SignalMethod
    void completeAction();

    @SignalMethod
    void failAction(String reason);

    @QueryMethod
    List<String> getStatus();

    @QueryMethod
    ActionStatus getActionStatus();

    @QueryMethod
    String getActionResponse();
}