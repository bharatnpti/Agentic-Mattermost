package com.example.workflowcontroller;

import com.example.mattermost.Goal;
import com.example.mattermost.MeetingSchedulerAppMain;
import com.example.mattermost.workflow.MeetingSchedulerWorkflow;
import io.temporal.client.WorkflowClient;
import io.temporal.client.WorkflowOptions;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

import java.util.UUID;

@RestController
@RequestMapping("/workflow")
public class WorkflowController {

    private final WorkflowClient workflowClient;

    @Autowired
    public WorkflowController(WorkflowClient workflowClient) {
        this.workflowClient = workflowClient;
    }

    @PostMapping("/start")
    public String startWorkflow(@RequestBody Goal goal) {
        // Generate a unique workflow ID
        String workflowId = "MeetingSchedulerWorkflow_" + UUID.randomUUID().toString();

        // Create a new workflow stub
        MeetingSchedulerWorkflow workflow = workflowClient.newWorkflowStub(
                MeetingSchedulerWorkflow.class,
                WorkflowOptions.newBuilder()
                        .setTaskQueue(MeetingSchedulerAppMain.TASK_QUEUE)
                        .setWorkflowId(workflowId)
                        .build()
        );

        // Start the workflow
        WorkflowClient.start(workflow::scheduleMeeting, goal);

        // Return a confirmation message
        return "Workflow started with ID: " + workflowId;
    }
}
