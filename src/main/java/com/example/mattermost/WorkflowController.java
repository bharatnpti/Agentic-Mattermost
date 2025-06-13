package com.example.mattermost;

import io.temporal.client.WorkflowClient;
import io.temporal.client.WorkflowOptions;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.*;

import java.util.HashMap;
import java.util.Map;
import java.util.UUID;

@RestController
@RequestMapping("/api/v1/workflow")
public class WorkflowController {

    private static final Logger logger = LoggerFactory.getLogger(WorkflowController.class);
    private final WorkflowClient workflowClient;

    // Using constant from MeetingSchedulerAppMain, consider moving to application properties or TemporalConfig
    private static final String TASK_QUEUE = MeetingSchedulerAppMain.TASK_QUEUE;

    public WorkflowController(WorkflowClient workflowClient) {
        this.workflowClient = workflowClient;
    }

    @PostMapping("/start")
    public ResponseEntity<Map<String, String>> startWorkflow(@RequestBody Goal goal) {
        String workflowId = "MeetingSchedulerWorkflow_" + UUID.randomUUID().toString();
        logger.info("Received request to start workflow for goal: '{}', generated workflowId: {}", goal.getGoal(), workflowId);

        try {
            MeetingSchedulerWorkflow workflow = workflowClient.newWorkflowStub(
                    MeetingSchedulerWorkflow.class,
                    WorkflowOptions.newBuilder()
                            .setWorkflowId(workflowId)
                            .setTaskQueue(TASK_QUEUE)
                            // Add other necessary options like timeouts if needed
                            .build()
            );

            // Start workflow asynchronously
            WorkflowClient.start(workflow::scheduleMeeting, goal);
            logger.info("Successfully initiated workflow {} for goal: '{}'", workflowId, goal.getGoal());

            Map<String, String> response = new HashMap<>();
            response.put("workflowId", workflowId);
            return ResponseEntity.ok(response);

        } catch (Exception e) {
            logger.error("Error starting workflow for goal: '{}', workflowId: {}", goal.getGoal(), workflowId, e);
            Map<String, String> errorResponse = new HashMap<>();
            errorResponse.put("error", "Failed to start workflow: " + e.getMessage());
            // Consider using a more specific error status code if appropriate
            return ResponseEntity.status(500).body(errorResponse);
        }
    }
}
