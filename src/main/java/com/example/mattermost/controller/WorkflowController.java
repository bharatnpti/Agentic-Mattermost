package com.example.mattermost.controller;

import com.example.mattermost.MeetingSchedulerAppMain;
import com.example.mattermost.domain.model.*;
import com.example.mattermost.domain.repository.ActiveTaskRepository;
import com.example.mattermost.integration.mattermost.MattermostService;
import com.example.mattermost.integration.mattermost.model.User;
import com.example.mattermost.refactor.workflow.*;
import com.example.mattermost.service.GoalExtractionActivity;
import com.example.mattermost.workflow.MeetingSchedulerWorkflow;
import com.example.mattermost.workflow.activity.ActiveTaskActivity;
import com.example.mattermost.workflow.activity.LLMActivity;
import io.temporal.client.WorkflowClient;
import io.temporal.client.WorkflowOptions;
import io.temporal.serviceclient.WorkflowServiceStubs;
import io.temporal.worker.Worker;
import io.temporal.worker.WorkerFactory;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

import java.time.Duration;
import java.util.*;

@RestController
@RequestMapping("/api/v1/workflow")
public class WorkflowController {

    private static final Logger logger = LoggerFactory.getLogger(WorkflowController.class);
    private final WorkflowClient workflowClient;
    private final ActiveTaskRepository activeTaskRepository;
    private final GoalExtractionActivity goalExtractionActivity;

    private final WorkflowQueryActivity workflowQueryActivity;

    private final MattermostService mattermostService;

    // Using constant from MeetingSchedulerAppMain, consider moving to application properties or TemporalConfig
    private static final String TASK_QUEUE = MeetingSchedulerAppMain.TASK_QUEUE;

    private LLMActivity llmActivity;

    private ActiveTaskActivity activeTaskActivity;

    private MessageHistoryActivityImpl messageHistoryActivityImpl;

    private static final String TASK_QUEUE_PARENT = "parent-workflow-queue";
    private static final String TASK_QUEUE_CHILD = "child-workflow-queue";

    public static final String PREFIX = "Meeting_Workflow_";

    @Autowired
    public WorkflowController(WorkflowClient workflowClient,
                              ActiveTaskRepository activeTaskRepository,
                              GoalExtractionActivity goalExtractionActivity,
                              LLMActivity llmActivity,
                              MattermostService mattermostService,
                              MessageHistoryActivityImpl messageHistoryActivityImpl,
                              WorkflowQueryActivity workflowQueryActivity,
                              ActiveTaskActivity activeTaskActivity) {
        this.workflowClient = workflowClient;
        this.activeTaskRepository = activeTaskRepository;
        this.goalExtractionActivity = goalExtractionActivity;
        this.llmActivity = llmActivity;
        this.mattermostService = mattermostService;
        this.messageHistoryActivityImpl = messageHistoryActivityImpl;
        this.workflowQueryActivity = workflowQueryActivity;
        this.activeTaskActivity = activeTaskActivity;
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
            workflow.onUserResponse(actionId, userInput, payload.getThreadId(), payload.getChannelId());

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

    @PostMapping("/message_old")
    public ResponseEntity<Map<String, String>> handleMessage_old(@RequestBody MessagePayload messagePayload) {
        String channelId = messagePayload.getChannelId();
        String message = messagePayload.getMessage();
        String userId = messagePayload.getUserId(); // Optional: for tracking user context
        String threadRootId = messagePayload.getThreadId();

        logger.info("Received message from channelId: {}, userId: {}, threadId: {} with content: '{}'", channelId, userId, threadRootId, message);

        try {
            // Check if there's already an active task for this channel
//            List<ActiveTask> existingTask = activeTaskRepository.findByChannelIdAndUserId(channelId, userId);

            List<ActiveTask> existingTask = activeTaskRepository.findByThreadRootId(threadRootId);

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
                responsePayload.setThreadId(threadRootId);
                responsePayload.setChannelId(channelId);

                // Update the active task with the latest interaction
                activeTask.setStatus(ActionStatus.PROCESSING);
                activeTask.setLastInteraction(java.time.LocalDateTime.now());
//                activeTaskRepository.save(activeTask);

                return handleUserResponse(responsePayload);

            } else {
                // No active task, extract goal from message and start new workflow
                logger.info("No active task found for channelId: {}, extracting goal from message", channelId);

                Goal extractedGoal = goalExtractionActivity.extractGoalFromMessage(message);

                if (extractedGoal == null || extractedGoal.getGoal() == null || extractedGoal.getGoal().trim().isEmpty()) {
                    logger.warn("Could not extract valid goal from message: '{}'", message);
                    Map<String, String> response = new HashMap<>();
                    response.put("error", "Could not understand your request. Please provide more details about what you'd like to schedule.");
                    return ResponseEntity.badRequest().body(response);
                }

                logger.info("Extracted goal: '{}' from message", extractedGoal.getGoal());

                // Start new workflow
                ResponseEntity<Map<String, String>> workflowResponse = startWorkflow(extractedGoal, channelId, userId, threadRootId);

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

    @PostMapping("/message")
    public ResponseEntity<Map<String, String>> handleMessage(@RequestBody MessagePayload messagePayload) {
        String channelId = messagePayload.getChannelId();
        String message = messagePayload.getMessage();
        String userId = messagePayload.getUserId();
        String threadRootId = messagePayload.getThreadId();

        User user = mattermostService.getUserById(userId);

        logger.info("Received message from channelId: {}, userId: {}, threadId: {} with content: '{}'",
                channelId, userId, threadRootId, message);

        try {
            List<ActiveTask> existingTask = activeTaskRepository.findByThreadRootId(threadRootId);

            logger.info("existing tasks: {}", existingTask);

            if (!existingTask.isEmpty()) {
                ActiveTask activeTask = existingTask.stream()
                        .filter(task -> task.getStatus() == ActionStatus.WAITING_FOR_INPUT)
                        .findFirst()
                        .orElseThrow();
                String workflowId = activeTask.getWorkflowId();

                ChildWorkflowInterface workflow = workflowClient.newWorkflowStub(ChildWorkflowInterface.class, workflowId);

                MessageHistory messageHistory = new MessageHistory();
                messageHistory.setMessage(user.getUsername() + ": " + System.lineSeparator() + message);
                messageHistory.setChildWorkFlowId(workflowId);
                messageHistory.setUserId(userId);
                messageHistory.setUserName(user.getUsername());

                messageHistoryActivityImpl.save(messageHistory);

                // Signal the workflow
                workflow.onUserResponse(message, threadRootId, channelId);


                // TODO
                return ResponseEntity.accepted().body(Map.of("workflowId", workflowId));

            } else {
                String workflowId = PREFIX + threadRootId;
                logger.info("No active task found for threadId: {}, workflowId: {}", threadRootId, workflowId);

                WorkflowServiceStubs service = WorkflowServiceStubs.newLocalServiceStubs();
                WorkflowClient client = WorkflowClient.newInstance(service);
                WorkerFactory factory = WorkerFactory.newInstance(client);

                // Register activities on PARENT worker since they're called from parent workflow
                Worker parentWorker = factory.newWorker(TASK_QUEUE_PARENT);
                parentWorker.registerWorkflowImplementationTypes(DagExecutorWorkflowImpl.class);
                parentWorker.registerActivitiesImplementations(
                        goalExtractionActivity,
                        workflowQueryActivity
                );

                Worker childWorker = factory.newWorker(TASK_QUEUE_CHILD);
                childWorker.registerWorkflowImplementationTypes(ChildWorkflowImpl.class);
                childWorker.registerActivitiesImplementations(llmActivity, messageHistoryActivityImpl, activeTaskActivity);

                factory.start();
                logger.info("✅ All workers started successfully");

                WorkflowOptions parentOptions = WorkflowOptions.newBuilder()
                        .setWorkflowId(workflowId)
                        .setTaskQueue(TASK_QUEUE_PARENT)
                        .setWorkflowExecutionTimeout(Duration.ofMinutes(30))
                        .setWorkflowRunTimeout(Duration.ofMinutes(15))
                        .build();

                DagExecutorWorkflow workflow = client.newWorkflowStub(DagExecutorWorkflow.class, parentOptions);

                logger.info("Starting workflow execution...");
                WorkflowClient.start(workflow::executeGoal, workflowId, message, channelId, threadRootId, user);

                return ResponseEntity.accepted().body(Map.of("workflowId", workflowId));
            }

        } catch (Exception e) {
            logger.error("Error handling message for channelId: {}", channelId, e);
            Map<String, String> errorResponse = new HashMap<>();
            errorResponse.put("error", "Failed to process message: " + e.getMessage());
            return ResponseEntity.status(HttpStatus.INTERNAL_SERVER_ERROR).body(errorResponse);
        }
    }
}

