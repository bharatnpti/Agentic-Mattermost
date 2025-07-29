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

    // Use the Spring-managed WorkflowClient instead of creating new ones
    private final WorkflowClient workflowClient;
    private final ActiveTaskRepository activeTaskRepository;
    private final GoalExtractionActivity goalExtractionActivity;
    private final WorkflowQueryActivity workflowQueryActivity;
    private final MattermostService mattermostService;
    private final LLMActivity llmActivity;
    private final ActiveTaskActivity activeTaskActivity;
    private final MessageHistoryActivityImpl messageHistoryActivityImpl;

    // Using constant from MeetingSchedulerAppMain, consider moving to application properties or TemporalConfig
    private static final String TASK_QUEUE = MeetingSchedulerAppMain.TASK_QUEUE;
    private static final String TASK_QUEUE_PARENT = "parent-workflow-queue";
    private static final String TASK_QUEUE_CHILD = "child-workflow-queue";
    public static final String PREFIX = "Meeting_Workflow_";

    // Track if additional workers are already registered to avoid duplicates
    // Note: Workers are now registered in TemporalConfig, so this is no longer needed

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

                // Use the existing Spring-managed WorkflowClient
                ChildWorkflowInterface workflow = workflowClient.newWorkflowStub(ChildWorkflowInterface.class, workflowId);

                MessageHistory messageHistory = new MessageHistory();
                messageHistory.setMessage(user.getUsername() + ": " + System.lineSeparator() + message);
                messageHistory.setChildWorkFlowId(workflowId);
                messageHistory.setUserId(userId);
                messageHistory.setUserName(user.getUsername());

                messageHistoryActivityImpl.save(messageHistory);

                // Signal the workflow
                workflow.onUserResponse(message, threadRootId, channelId);

                return ResponseEntity.accepted().body(Map.of("workflowId", workflowId));

            } else {
                String workflowId = PREFIX + threadRootId;
                logger.info("No active task found for threadId: {}, workflowId: {}", threadRootId, workflowId);

                // Workers are already registered in TemporalConfig, so we can directly start the workflow
                WorkflowOptions parentOptions = WorkflowOptions.newBuilder()
                        .setWorkflowId(workflowId)
                        .setTaskQueue(TASK_QUEUE_PARENT)
                        .setWorkflowExecutionTimeout(Duration.ofMinutes(720))
                        .setWorkflowRunTimeout(Duration.ofMinutes(120))
                        .build();

                // Use the existing Spring-managed WorkflowClient
                DagExecutorWorkflow workflow = workflowClient.newWorkflowStub(DagExecutorWorkflow.class, parentOptions);

                logger.info("Starting workflow execution...");
                WorkflowClient.start(workflow::executeGoal, workflowId, message, channelId, threadRootId, user);

                return ResponseEntity.accepted().body(Map.of("workflowId", workflowId));
            }

        } catch (Exception e) {
            logger.error("Error handling message for channelId: {}, threadId: {}", channelId, threadRootId, e);
            Map<String, String> errorResponse = new HashMap<>();
            errorResponse.put("error", "Failed to process message: " + e.getMessage());
            return ResponseEntity.status(HttpStatus.INTERNAL_SERVER_ERROR).body(errorResponse);
        }
    }
}