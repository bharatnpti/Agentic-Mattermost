package com.example.mattermost.refactor.workflow;

import com.example.mattermost.domain.CurrentContext;
import com.example.mattermost.domain.MessageList;
import com.example.mattermost.domain.model.*;
import com.example.mattermost.workflow.activity.ActiveTaskActivity;
import com.example.mattermost.workflow.activity.LLMActivity;
import io.temporal.activity.ActivityOptions;
import io.temporal.common.RetryOptions;
import io.temporal.workflow.Async;
import io.temporal.workflow.CompletablePromise;
import io.temporal.workflow.Promise;
import io.temporal.workflow.Workflow;
import lombok.extern.slf4j.Slf4j;

import java.time.Duration;
import java.util.ArrayList;
import java.util.List;
import java.util.stream.Collectors;

@Slf4j
public class ChildWorkflowImpl implements ChildWorkflowInterface {

    private final List<String> status = new ArrayList<>();
    private ActionNode actionNode;
    private String actionResponse;
    private boolean isCompleted = false;
    private boolean isFailed = false;
    private String failureReason;

    private CurrentContext context;
    private ActionStatus currentActionStatus = ActionStatus.PENDING;

    private final CompletablePromise<String> proceedSignal = Workflow.newPromise();

    RetryOptions retryOptions = RetryOptions.newBuilder()
            .setInitialInterval(Duration.ofSeconds(1))
            .setMaximumInterval(Duration.ofSeconds(30))
            .setBackoffCoefficient(2)
            .setMaximumAttempts(3)
            .build();

    ActivityOptions defaultActivityOptions = ActivityOptions.newBuilder()
            .setStartToCloseTimeout(Duration.ofMinutes(3))
            .setScheduleToCloseTimeout(Duration.ofMinutes(5))
            .setRetryOptions(retryOptions)
            .build();

    private final LLMActivity llmActivity = Workflow.newActivityStub(LLMActivity.class, defaultActivityOptions);

    private final ActiveTaskActivity activeTaskActivity = Workflow.newActivityStub(ActiveTaskActivity.class, defaultActivityOptions);

    private final MessageHistoryActivity messageHistoryActivity = Workflow.newActivityStub(MessageHistoryActivity.class, defaultActivityOptions);

    @Override
    public String executeAction(CurrentContext context) {
        this.context = context;
        log.info("=== CHILD WORKFLOW STARTING ===");
        log.info("executeAction called with context: {}", context);

        try {
            actionNode = context.getCurrentActionNode();

            log.info("Child workflow started for action: {}", actionNode != null ? actionNode.getActionId() : "NULL");
            log.info("Action node details: {}", actionNode);

            if (actionNode == null) {
                log.error("ActionNode is null! Cannot proceed.");
                throw new IllegalArgumentException("ActionNode cannot be null");
            }

            updateStatus("Child workflow started for action: " + actionNode.getActionId());
            updateStatus("Action Description: " + actionNode.getActionDescription());

            // Set action status to PROCESSING
            log.info("Setting action status to PROCESSING" + actionNode.getActionId());
            currentActionStatus = ActionStatus.PROCESSING;
            actionNode.setActionStatus(ActionStatus.PROCESSING);
            updateStatus("Status: PROCESSING");

                    log.info("About to call determineActionType activity" + actionNode.getActionId());
                    log.info("Context for activity: {}", context);

                    // Step 1: Determine action type
                    log.info("Calling llmActivity.determineActionType...");
            ActionStatus actionType = evaluateAndExecuteAction(context);
            if(actionType == ActionStatus.COMPLETED || actionType == ActionStatus.AUTOMATED) {
                        return context.getCurrentActionNode().getConvHistory();
                    } else {
                        return context.getCurrentActionNode().getConvHistory();
                    }
                } catch (Exception e) {
            log.error("Exception occurred during action execution", e);
                    updateStatus("Error in action processing: " + e.getMessage());
                    currentActionStatus = ActionStatus.FAILED;
                    actionNode.setActionStatus(ActionStatus.FAILED);
                    this.failureReason = e.getMessage();
                    this.isFailed = true;
                    throw e;
                }
    }

    private ActionStatus evaluateAndExecuteAction(CurrentContext context) {

        List<MessageHistory> messageHistory = messageHistoryActivity.getMessageHistory(actionNode.getWorkflowId());

        actionNode.setActionResponses(messageHistory.stream().map(MessageHistory::getMessage).collect(Collectors.toList()));

        ActionStatus actionType = llmActivity.determineActionType(context);
        log.info("Activity returned action type: {}, action id: {}", actionType,  actionNode.getActionId());

        updateStatus("Determined action type: " + actionType);
        executeActionByStatus(actionType, context);
        return actionType;
    }

    private void handleWaitingForInput(CurrentContext context) {
        log.info("=== Handling WAITING_FOR_INPUT action ===");
        updateStatus("Handling WAITING_FOR_INPUT action");

        try {
            log.info("About to call formulate_user_message activity" + context.getCurrentActionNode().getActionId());
            // Step 1: Formulate user message
            MessageList messageList = llmActivity.formulate_user_message(context);
            log.info("Got message list with {} messages", messageList.getMessages().size());

            messageList.getMessages().forEach((message) -> {
                log.info("Sending message to: {}", message.getUser().getUsername());
                updateStatus("Sending message to: " + message.getUser().getUsername());
                llmActivity.checkAndAskUser(message, context);
            });

            // Set action status to WAITING_FOR_INPUT
            currentActionStatus = ActionStatus.WAITING_FOR_INPUT;
            actionNode.setActionStatus(ActionStatus.WAITING_FOR_INPUT);
            updateStatus("Status updated to WAITING_FOR_INPUT");
            log.info("Action status set to WAITING_FOR_INPUT: " + context.getCurrentActionNode().getActionId());

            String userResponse = proceedSignal.get();
            log.info("User sent response to signal from handleWaitingForInput: {}, context: {}", userResponse, context);
            evaluateAndExecuteAction(context);

        } catch (Exception e) {
            log.error("Error in handleWaitingForInput: {}", e.getMessage(), e);
            updateStatus("Error in handleWaitingForInput: " + e.getMessage());
            throw e;
        }
    }

    private void handleAutomatedAction(CurrentContext context) {
        log.info("=== Handling AUTOMATED action ===");
        updateStatus("Handling AUTOMATED action");

        try {
            log.info("Starting async LLM completion" + context.getCurrentActionNode().getActionId());
            Promise<LLMProcessingResult> llmCompletionPromise = Async.function(this::tryLLMCompletionViaActivity, context);
            log.info("Waiting for LLM completion result");
            LLMProcessingResult llmProcessingResult = llmCompletionPromise.get();

            log.info("LLM processing completed with status: {}, actionID: {}", llmProcessingResult.getActionStatus(), context.getCurrentActionNode().getActionId());
            updateStatus("LLM processing completed with status: " + llmProcessingResult.getActionStatus());
            actionNode.setActionResponse("Action processing latest response: " + llmProcessingResult.getActionResult());

            executeActionByStatus(llmProcessingResult.getActionStatus(), context);
        } catch (Exception e) {
            log.error("Error in handleAutomatedAction: {}", e.getMessage(), e);
            updateStatus("Error in handleAutomatedAction: " + e.getMessage());
            throw e;
        }
    }

    private LLMProcessingResult tryLLMCompletionViaActivity(CurrentContext context) {
        try {
            log.info("=== Calling LLM activity for processing ===");
            updateStatus("Calling LLM activity for processing");
            LLMProcessingResult result = llmActivity.processActionWithLLM(context);
            log.info("LLM activity returned result: {}", result);
            return result;
        } catch (Exception e) {
            log.error("LLM processing failed: {}", e.getMessage(), e);
            updateStatus("LLM processing failed: " + e.getMessage());
            throw new RuntimeException("LLM processing failed", e);
        }
    }

    private void executeActionByStatus(ActionStatus actionType, CurrentContext context) {
        log.info("=== Executing action by status: {} ===", actionType);
        updateStatus("Executing action by status: " + actionType);

        switch (actionType) {
            case WAITING_FOR_INPUT:
                handleWaitingForInput(context);
                break;

            case AUTOMATED:
                handleAutomatedAction(context);
                summarizeResponse(context);
                break;

            case COMPLETED:
                log.info("Action {} is COMPLETED", context.getCurrentActionNode().getActionId());
                currentActionStatus = ActionStatus.COMPLETED;
                actionNode.setActionStatus(ActionStatus.COMPLETED);
                this.isCompleted = true;
                updateStatus("Action marked as COMPLETED");
                summarizeResponse(context);
                signalParent(context, currentActionStatus, "");
                activeTaskActivity.updateActiveTask(actionNode.getActionId(), actionType, context.getCurrentActionNode().getWorkflowId(), context.getCurrentChannelId(), context.getUser().getId(), context.getCurrentThreadId());
                break;

            case FAILED:
                log.info("Action type is FAILED, marking as failed" + context.getCurrentActionNode().getActionId());
                currentActionStatus = ActionStatus.FAILED;
                actionNode.setActionStatus(ActionStatus.FAILED);
                this.isFailed = true;
                updateStatus("Action marked as FAILED");
                activeTaskActivity.updateActiveTask(actionNode.getActionId(), actionType, context.getCurrentActionNode().getWorkflowId(), context.getCurrentChannelId(), context.getUser().getId(), context.getCurrentThreadId());
                throw new RuntimeException("Action type is FAILED");
            default:
                log.error("Unknown action type: {}, actionId: {}", actionType, context.getCurrentActionNode().getActionId());
                updateStatus("Unknown action type: " + actionType);
                currentActionStatus = ActionStatus.FAILED;
                actionNode.setActionStatus(ActionStatus.FAILED);
                this.isFailed = true;
                this.failureReason = "Unknown action type: " + actionType;
                activeTaskActivity.updateActiveTask(actionNode.getActionId(), actionType, context.getCurrentActionNode().getWorkflowId(), context.getCurrentChannelId(), context.getUser().getId(), context.getCurrentThreadId());
                throw new RuntimeException("Action type is FAILED");
        }

    }

    private void summarizeResponse(CurrentContext context) {
        actionResponse = llmActivity.summarize(context);
        log.info("LLM activity summarized response: {}", actionResponse);
    }

    @Override
    public void updateStatus(String statusMessage) {
        String message = String.format("[%s] %s",
                Workflow.currentTimeMillis(), statusMessage);
        status.add(message);
        log.info("updateStatus: {}", message);
    }

    @Override
    public void onUserResponse(String response, String threadId, String channelId) {
        context.setCurrentThreadId(threadId);
        context.setCurrentChannelId(channelId);
        this.actionResponse = response;
        updateStatus("Response received: " + response);
        log.info("Action response set: {}", response);

        if (!proceedSignal.isCompleted()) {
            proceedSignal.complete(response);
        }
    }


    @Override
    public void completeAction() {
        this.isCompleted = true;
        this.currentActionStatus = ActionStatus.COMPLETED;
        if (actionNode != null) {
            actionNode.setActionStatus(ActionStatus.COMPLETED);
        }
        updateStatus("Action marked as completed");
        log.info("Action manually marked as completed");
    }

    @Override
    public void failAction(String reason) {
        this.isFailed = true;
        this.failureReason = reason;
        this.currentActionStatus = ActionStatus.FAILED;
        if (actionNode != null) {
            actionNode.setActionStatus(ActionStatus.FAILED);
        }
        updateStatus("Action marked as failed: " + reason);
        log.info("Action manually marked as failed: {}", reason);
    }

    @Override
    public List<String> getStatus() {
        return new ArrayList<>(status);
    }

    @Override
    public ActionStatus getActionStatus() {
        ActionStatus status = currentActionStatus != null ? currentActionStatus :
                (actionNode != null ? actionNode.getActionStatus() : ActionStatus.PENDING);
        log.debug("getActionStatus returning: {}", status);
        return status;
    }

    @Override
    public String getActionResponse() {
        log.info("getActionResponse returning: {}", actionResponse);
        return actionResponse;
    }

    private void signalParent(CurrentContext context, ActionStatus status, String message) {
        String parentWorkflowId = context.getGoal().getWorkflowId();
        if (parentWorkflowId != null && actionNode != null) {
            DagExecutorWorkflow parent = Workflow.newExternalWorkflowStub(DagExecutorWorkflow.class, parentWorkflowId);
            parent.onChildCompleted(actionNode.getActionId(), ActionStatus.COMPLETED, message);
        }
    }
}