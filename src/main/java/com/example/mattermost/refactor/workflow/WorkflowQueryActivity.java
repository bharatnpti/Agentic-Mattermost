package com.example.mattermost.refactor.workflow;

import com.example.mattermost.domain.model.ActionStatus;
import io.temporal.activity.ActivityInterface;
import io.temporal.activity.ActivityMethod;
import io.temporal.client.WorkflowClient;

import java.util.Arrays;
import java.util.List;

@ActivityInterface
public interface WorkflowQueryActivity {
    @ActivityMethod
    List<String> queryChildWorkflowStatus(String workflowId);
    
    @ActivityMethod
    ActionStatus queryChildWorkflowActionStatus(String workflowId);
}

