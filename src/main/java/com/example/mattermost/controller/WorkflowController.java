package com.example.mattermost.controller;

import com.example.mattermost.MeetingSchedulerAppMain;
import com.example.mattermost.domain.model.*;
import com.example.mattermost.domain.repository.ActiveTaskRepository;
import com.example.mattermost.service.GoalExtractionService;
import com.example.mattermost.workflow.MeetingSchedulerWorkflow;
import io.temporal.client.WorkflowClient;
import io.temporal.client.WorkflowOptions;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

import java.util.*;

@RestController
@RequestMapping("/api/v1/workflow")
public class WorkflowController {

    private static final Logger logger = LoggerFactory.getLogger(WorkflowController.class);
    private final WorkflowClient workflowClient;
    private final ActiveTaskRepository activeTaskRepository;
    private final GoalExtractionService goalExtractionService;

    // Using constant from MeetingSchedulerAppMain, consider moving to application properties or TemporalConfig
    private static final String TASK_QUEUE = MeetingSchedulerAppMain.TASK_QUEUE;

    @Autowired
    public WorkflowController(WorkflowClient workflowClient,
                              ActiveTaskRepository activeTaskRepository,
                              GoalExtractionService goalExtractionService) {
        this.workflowClient = workflowClient;
        this.activeTaskRepository = activeTaskRepository;
        this.goalExtractionService = goalExtractionService;
    }

    @PostMapping("/start")
    public ResponseEntity<Map<String, String>> startWorkflow(@RequestBody Goal goal, String channelId, String userId, String threadId) {
        String workflowId = "MeetingSchedulerWorkflow_" + UUID.randomUUID().toString().substring(0, 6);
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
            goal.setWorkflowId(workflowId);
            WorkflowClient.start(workflow::scheduleMeeting, goal, channelId, userId, threadId);
            logger.info("Successfully initiated workflow {} for goal: '{}'", workflowId, goal.getGoal());

            Map<String, String> response = new HashMap<>();
            response.put("workflowId", workflowId);
            return ResponseEntity.ok(response);

        } catch (Exception e) {
            logger.error("Error starting workflow for goal: '{}', workflowId: {}", goal.getGoal(), workflowId, e);
            Map<String, String> errorResponse = new HashMap<>();
            errorResponse.put("error", "Failed to start workflow: " + e.getMessage());
            // Consider using a more specific error status code if appropriate
            return ResponseEntity.status(HttpStatus.INTERNAL_SERVER_ERROR).body(errorResponse);
        }
    }

    @PostMapping("/user-response")
    public ResponseEntity<Map<String, String>> handleUserResponse(@RequestBody UserResponsePayload payload) {
        String workflowId = payload.getWorkflowId();
        String actionId = payload.getActionId();
        String userInput = payload.getUserInput();

        logger.info("Received user response for workflowId: {} and actionId: {}", workflowId, actionId);

        try {
            // Get a stub for the existing workflow instance
            MeetingSchedulerWorkflow workflow = workflowClient.newWorkflowStub(MeetingSchedulerWorkflow.class, workflowId);

            // Signal the workflow
            workflow.onUserResponse(actionId, userInput, payload.threadId, payload.channelId);

            logger.info("Signal onUserResponse sent successfully to workflowId: {}", workflowId);
            Map<String, String> response = new HashMap<>();
            response.put("message", "Signal onUserResponse sent successfully to workflowId_" + workflowId);
            return ResponseEntity.ok(response);
        } catch (Exception e) {
            logger.error("Error sending signal to workflowId: " + workflowId, e);
            Map<String, String> response = new HashMap<>();
            response.put("error", "Failed to send signal: " + e.getMessage());
            return ResponseEntity.status(HttpStatus.INTERNAL_SERVER_ERROR).body(response);
        }
    }

    @PostMapping("/message")
    public ResponseEntity<Map<String, String>> handleMessage(@RequestBody MessagePayload messagePayload) {
        String channelId = messagePayload.getChannelId();
        String message = messagePayload.getMessage();
        String userId = messagePayload.getUserId(); // Optional: for tracking user context
        String threadId = messagePayload.getThreadId();

        logger.info("Received message from channelId: {}, userId: {}, with content: '{}'", channelId, userId, message);

        try {
            // Check if there's already an active task for this channel
            List<ActiveTask> existingTask = activeTaskRepository.findByChannelIdAndUserId(channelId, userId);

            if (!existingTask.isEmpty()) {
                // There's an active task, treat this as a user response
                ActiveTask activeTask = existingTask.stream().filter(task -> task.getStatus() == ActionStatus.WAITING_FOR_INPUT).findFirst().orElseThrow();
                String workflowId = activeTask.getWorkflowId();
                String currentActionId = activeTask.getCurrentActionId();

                logger.info("Found active task for channelId: {}, workflowId: {}, treating message as user response",
                        channelId, workflowId);

                // Create UserResponsePayload and call existing handleUserResponse method
                UserResponsePayload responsePayload = new UserResponsePayload();
                responsePayload.setWorkflowId(workflowId);
                responsePayload.setActionId(currentActionId);
                responsePayload.setUserInput(message);
                responsePayload.setThreadId(threadId);
                responsePayload.setChannelId(channelId);

                // Update the active task with the latest interaction
                activeTask.setLastInteraction(java.time.LocalDateTime.now());
                activeTaskRepository.save(activeTask);

                return handleUserResponse(responsePayload);

            } else {
                // No active task, extract goal from message and start new workflow
                logger.info("No active task found for channelId: {}, extracting goal from message", channelId);

                Goal extractedGoal = goalExtractionService.extractGoalFromMessage(message);

                if (extractedGoal == null || extractedGoal.getGoal() == null || extractedGoal.getGoal().trim().isEmpty()) {
                    logger.warn("Could not extract valid goal from message: '{}'", message);
                    Map<String, String> response = new HashMap<>();
                    response.put("error", "Could not understand your request. Please provide more details about what you'd like to schedule.");
                    return ResponseEntity.badRequest().body(response);
                }

                logger.info("Extracted goal: '{}' from message", extractedGoal.getGoal());

                // Start new workflow
                ResponseEntity<Map<String, String>> workflowResponse = startWorkflow(extractedGoal, channelId, userId, threadId);

                // If workflow started successfully, create and save active task record
                if (workflowResponse.getStatusCode() == HttpStatus.OK) {
                    String workflowId = workflowResponse.getBody().get("workflowId");

//                    ActiveTask newTask = new ActiveTask();
//                    newTask.setChannelId(channelId);
//                    newTask.setWorkflowId(workflowId);
//                    newTask.setUserId(userId);
//                    newTask.setGoal(extractedGoal.getGoal());
//                    newTask.setStatus(ActionStatus.PROCESSING);
//                    newTask.setCreatedAt(java.time.LocalDateTime.now());
//                    newTask.setLastInteraction(java.time.LocalDateTime.now());
//                    newTask.setCurrentActionId("INITIAL"); // Set initial action ID
//
//                    activeTaskRepository.save(newTask);
                    logger.info("Created new active task record for channelId: {}, workflowId: {}", channelId, workflowId);
                }

                return workflowResponse;
            }

        } catch (Exception e) {
            logger.error("Error handling message for channelId: {}", channelId, e);
            Map<String, String> errorResponse = new HashMap<>();
            errorResponse.put("error", "Failed to process message: " + e.getMessage());
            return ResponseEntity.status(HttpStatus.INTERNAL_SERVER_ERROR).body(errorResponse);
        }
    }
}

