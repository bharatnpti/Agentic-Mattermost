package com.example.mattermost.refactor.workflow;

import com.example.mattermost.domain.model.ActionStatus;
import io.temporal.client.WorkflowClient;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Service;

import java.util.Arrays;
import java.util.List;

@Slf4j
@Service
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
            log.info("queryChildWorkflowActionStatus: {}", workflowId);
            ChildWorkflowInterface childStub = client.newWorkflowStub(
                ChildWorkflowInterface.class, workflowId);
            return childStub.getActionStatus();
        } catch (Exception e) {
            log.error("Error querying workflow {}: {}", workflowId, e.getMessage());
            return ActionStatus.PENDING;
        }
    }
}
