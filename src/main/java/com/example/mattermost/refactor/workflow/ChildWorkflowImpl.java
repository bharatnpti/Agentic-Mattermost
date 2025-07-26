package com.example.mattermost.refactor.workflow;

import com.example.mattermost.domain.CurrentContext;
import com.example.mattermost.domain.MessageList;
import com.example.mattermost.domain.model.*;
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

@Slf4j
public class ChildWorkflowImpl implements ChildWorkflowInterface {

    private final List<String> status = new ArrayList<>();
    private ActionNode actionNode;
    private String actionResponse;
    private boolean isCompleted = false;
    private boolean isFailed = false;
    private String failureReason;
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

    @Override
    public String executeAction(CurrentContext context) {
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
            log.info("Setting action status to PROCESSING");
            currentActionStatus = ActionStatus.PROCESSING;
            actionNode.setActionStatus(ActionStatus.PROCESSING);
            updateStatus("Status: PROCESSING");

            int maxIterations = 10; // Prevent infinite loops
            int iteration = 0;

            log.info("Starting main processing loop");

            // Main processing loop with safety bounds
//            while (currentActionStatus != ActionStatus.COMPLETED &&
//                    currentActionStatus != ActionStatus.FAILED &&
//                    iteration < maxIterations) {

                iteration++;
                log.info("=== Processing iteration: {} ===", iteration);
                updateStatus("Processing iteration: " + iteration);

                try {
                    log.info("About to call determineActionType activity");
                    log.info("Context for activity: {}", context);

                    // Step 1: Determine action type
                    log.info("Calling llmActivity.determineActionType...");
                    ActionStatus actionType = llmActivity.determineActionType(context);
                    log.info("Activity returned action type: {}", actionType);

                    updateStatus("Determined action type: " + actionType);

                    log.info("About to execute action by status: {}", actionType);
                    executeActionByStatus(actionType, context);
                    log.info("Completed executeActionByStatus");

                    // Add a small delay to prevent tight loops
//                    if (currentActionStatus == ActionStatus.PROCESSING) {
//                        updateStatus("Still processing, waiting before next iteration...");
//                        log.info("Still processing, sleeping for 1 second");
//                        Workflow.sleep(Duration.ofSeconds(1));
//                    }

                } catch (Exception e) {
                    log.error("Error in action processing iteration {}: {}", iteration, e.getMessage(), e);
                    updateStatus("Error in action processing: " + e.getMessage());
                    currentActionStatus = ActionStatus.FAILED;
                    actionNode.setActionStatus(ActionStatus.FAILED);
                    this.failureReason = e.getMessage();
                    this.isFailed = true;
//                    break;
                }
//            }

            // Check if we hit max iterations
//            if (iteration >= maxIterations && currentActionStatus != ActionStatus.COMPLETED) {
//                log.warn("Maximum iterations ({}) reached, marking as failed", maxIterations);
//                updateStatus("Maximum iterations reached, marking as failed");
//                currentActionStatus = ActionStatus.FAILED;
//                actionNode.setActionStatus(ActionStatus.FAILED);
//                this.failureReason = "Maximum processing iterations exceeded";
//                this.isFailed = true;
//            }

            if (currentActionStatus == ActionStatus.FAILED) {
                log.error("Action failed with reason: {}", failureReason);
                updateStatus("Action failed: " + failureReason);
                throw new RuntimeException("Action failed: " + failureReason);
            }

            // Mark as completed
            log.info("Action processing completed successfully");
            currentActionStatus = ActionStatus.COMPLETED;
            actionNode.setActionStatus(ActionStatus.COMPLETED);
            this.isCompleted = true;

            updateStatus("Action completed successfully");
            if (actionResponse == null) {
                actionResponse = "Action completed successfully for: " + actionNode.getActionId();
            }
            actionNode.setActionResponse(actionResponse);

            log.info("Child workflow returning response: {}", actionResponse);
            return actionResponse;

        } catch (Exception e) {
            log.error("=== CHILD WORKFLOW EXECUTION FAILED ===", e);
            // Ensure we properly set failure state
            currentActionStatus = ActionStatus.FAILED;
            if (actionNode != null) {
                actionNode.setActionStatus(ActionStatus.FAILED);
            }
            this.isFailed = true;
            this.failureReason = e.getMessage();
            updateStatus("Workflow execution failed: " + e.getMessage());
            throw e;
        }
    }

    private void handleWaitingForInput(CurrentContext context) {
        log.info("=== Handling WAITING_FOR_INPUT action ===");
        updateStatus("Handling WAITING_FOR_INPUT action");

        try {
            log.info("About to call formulate_user_message activity");
            // Step 1: Formulate user message
            MessageList messageList = llmActivity.formulate_user_message(context);
            log.info("Got message list with {} messages", messageList.getMessages().size());

            messageList.getMessages().forEach((message) -> {
                log.info("Sending message to: {}", message.getRecipient());
                updateStatus("Sending message to: " + message.getRecipient());
                Async.procedure(llmActivity::checkAndAskUser, message, context);
            });

            // Set action status to WAITING_FOR_INPUT
            currentActionStatus = ActionStatus.WAITING_FOR_INPUT;
            actionNode.setActionStatus(ActionStatus.WAITING_FOR_INPUT);
            updateStatus("Status updated to WAITING_FOR_INPUT");
            log.info("Action status set to WAITING_FOR_INPUT");

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
            log.info("Starting async LLM completion");
            Promise<LLMProcessingResult> llmCompletionPromise = Async.function(this::tryLLMCompletionViaActivity, context);
            log.info("Waiting for LLM completion result");
            LLMProcessingResult llmProcessingResult = llmCompletionPromise.get();

            log.info("LLM processing completed with status: {}", llmProcessingResult.getActionStatus());
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
                break;

            case COMPLETED:
                log.info("Action type is COMPLETED, marking as completed");
                currentActionStatus = ActionStatus.COMPLETED;
                actionNode.setActionStatus(ActionStatus.COMPLETED);
                this.isCompleted = true;
                updateStatus("Action marked as COMPLETED");
                break;

            case FAILED:
                log.info("Action type is FAILED, marking as failed");
                currentActionStatus = ActionStatus.FAILED;
                actionNode.setActionStatus(ActionStatus.FAILED);
                this.isFailed = true;
                updateStatus("Action marked as FAILED");
                break;

            default:
                log.error("Unknown action type: {}", actionType);
                updateStatus("Unknown action type: " + actionType);
                currentActionStatus = ActionStatus.FAILED;
                actionNode.setActionStatus(ActionStatus.FAILED);
                this.isFailed = true;
                this.failureReason = "Unknown action type: " + actionType;
                break;
        }
    }

    @Override
    public void updateStatus(String statusMessage) {
        String message = String.format("[%s] %s",
                Workflow.currentTimeMillis(), statusMessage);
        status.add(message);
        log.info("updateStatus: {}", message);
    }

    @Override
    public void setActionResponse(String response) {
        this.actionResponse = response;
        updateStatus("Response received: " + response);
        log.info("Action response set: {}", response);
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
        log.debug("getActionResponse returning: {}", actionResponse);
        return actionResponse;
    }
}