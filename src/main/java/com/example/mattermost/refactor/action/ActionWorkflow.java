package com.example.mattermost.refactor.action;

import com.example.mattermost.domain.model.ActionNode;
import com.example.mattermost.refactor.model.ActionResult;
import io.temporal.workflow.SignalMethod;
import io.temporal.workflow.WorkflowInterface;
import io.temporal.workflow.WorkflowMethod;

@WorkflowInterface
public interface ActionWorkflow {
    @WorkflowMethod
    ActionResult run(ActionNode node);

    @SignalMethod
    void submitResponse(ActionNode actionNode, String userInput);
}
