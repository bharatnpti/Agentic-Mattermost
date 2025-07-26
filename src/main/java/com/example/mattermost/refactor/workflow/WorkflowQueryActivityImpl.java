package com.example.mattermost.refactor.workflow;

import com.example.mattermost.domain.model.ActionStatus;
import io.temporal.client.WorkflowClient;

import java.util.Arrays;
import java.util.List;

public class WorkflowQueryActivityImpl implements WorkflowQueryActivity {
    private final WorkflowClient client;
    
    public WorkflowQueryActivityImpl(WorkflowClient client) {
        this.client = client;
    }
    
    @Override
    public List<String> queryChildWorkflowStatus(String workflowId) {
        try {
            ChildWorkflowInterface childStub = client.newWorkflowStub(
                ChildWorkflowInterface.class, workflowId);
            return childStub.getStatus();
        } catch (Exception e) {
            return Arrays.asList("Error querying workflow " + workflowId + ": " + e.getMessage());
        }
    }
    
    @Override
    public ActionStatus queryChildWorkflowActionStatus(String workflowId) {
        try {
            ChildWorkflowInterface childStub = client.newWorkflowStub(
                ChildWorkflowInterface.class, workflowId);
            return childStub.getActionStatus();
        } catch (Exception e) {
            return ActionStatus.FAILED;
        }
    }
}
