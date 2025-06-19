package com.example.mattermost;

import com.fasterxml.jackson.databind.ObjectMapper;
import io.temporal.activity.ActivityOptions;
import io.temporal.common.RetryOptions;
import io.temporal.workflow.Async;
import io.temporal.workflow.Promise;
import io.temporal.workflow.Workflow;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.time.Duration;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.Set;
import java.util.HashSet;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.TimeUnit;
import java.util.stream.Collectors;

public class MeetingSchedulerWorkflowImpl implements MeetingSchedulerWorkflow {

    private static final Logger logger = LoggerFactory.getLogger(MeetingSchedulerWorkflowImpl.class);

    private Goal currentGoal;
    private final Map<String, ActionStatus> actionStatuses = new ConcurrentHashMap<>();
    private final Map<String, String> actionOutputs = new ConcurrentHashMap<>();
    private final Map<String, String> waitingForUserInputMap = new ConcurrentHashMap<>();

    // Track processing state to avoid duplicate processing
    private final Set<String> currentlyProcessing = new HashSet<>();
    private volatile boolean workflowActive = true;

    // Activities
    private final AskUserActivity askUserActivity;
    private final ValidateInputActivity validateInputActivity;
    private final LLMActivity llmActivity;

    public MeetingSchedulerWorkflowImpl() {
        // Define retry options for activities
        RetryOptions retryOptions = RetryOptions.newBuilder()
                .setInitialInterval(Duration.ofSeconds(100))
                .setMaximumInterval(Duration.ofSeconds(100))
                .setBackoffCoefficient(2)
                .setMaximumAttempts(3)
                .build();

        ActivityOptions defaultActivityOptions = ActivityOptions.newBuilder()
                .setStartToCloseTimeout(Duration.ofMinutes(5))
                .setRetryOptions(retryOptions)
                .build();

        this.askUserActivity = Workflow.newActivityStub(AskUserActivity.class, defaultActivityOptions);
        this.validateInputActivity = Workflow.newActivityStub(ValidateInputActivity.class, defaultActivityOptions);
        this.llmActivity = Workflow.newActivityStub(LLMActivity.class, defaultActivityOptions);
    }

    @Override
    public void scheduleMeeting(Goal goal) {
        logger.info("Workflow started for goal: {}. Initializing action statuses.", goal.getGoal());
        this.currentGoal = goal;

        // Initialize action statuses and outputs
        initializeWorkflowState();

        // Start the main execution loop
        executeWorkflowLoop();

        // Handle workflow completion
        handleWorkflowCompletion();
    }

    private void initializeWorkflowState() {
        currentGoal.getNodes().forEach(node -> {
            actionStatuses.put(node.getActionId(), node.getActionStatus());
            actionOutputs.put(node.getActionId(), "");
        });

        if (currentGoal.getActionOutputs() == null) {
            currentGoal.setActionOutputs(new ConcurrentHashMap<>());
        }

        logger.info("Initialized actionStatuses: {}", actionStatuses);
    }

    private void executeWorkflowLoop() {
        while (workflowActive && !isWorkflowComplete()) {
            logger.debug("Starting workflow execution cycle");

            // Process all currently processable actions
            boolean actionsProcessed = processAllProcessableActions();

            if (!actionsProcessed && hasWaitingActions()) {
                logger.debug("No processable actions, but have waiting actions. Awaiting signals...");
                // Add timeout to prevent infinite waiting
                Workflow.await(Duration.ofMinutes(30), () -> hasNewProcessableActions() || isWorkflowComplete());
            }

//            else if (!actionsProcessed && !hasWaitingActions()) {
//                logger.info("No processable actions and no waiting actions. Checking for completion...");
//                break; // Exit loop to check completion
//            }

            // Add a small yield to prevent tight loops
            Workflow.sleep(Duration.ofMillis(100));
        }
    }

    private boolean processAllProcessableActions() {
        List<ActionNode> processableActions = getProcessableActions();

        if (processableActions.isEmpty()) {
            logger.debug("No processable actions available");
            return false;
        }

        logger.info("Found {} processable actions", processableActions.size());
        logger.debug("Processable action IDs: {}",
                processableActions.stream().map(ActionNode::getActionId).collect(Collectors.toList()));

        // Process actions in parallel for better performance
        List<Promise<ProcessingResult>> actionPromises = new ArrayList<>();

        for (ActionNode action : processableActions) {
            // Mark as currently processing to avoid duplicate processing
            if (currentlyProcessing.add(action.getActionId())) {
                logger.debug("Starting async processing for action: {}", action.getActionId());
                actionPromises.add(Async.function(this::processActionAsync, action));
            }
        }

        // Wait for all actions to complete their processing
        if (!actionPromises.isEmpty()) {
            // Add timeout to prevent indefinite blocking
            try {
                Promise.allOf(actionPromises).get(10, TimeUnit.MINUTES);
            } catch (Exception e) {
                logger.error("Timeout or error waiting for action processing: {}", e.getMessage());
                // Clean up processing tracking for failed actions
                actionPromises.forEach(promise -> {
                    try {
                        ProcessingResult result = promise.get();
                        currentlyProcessing.remove(result.actionId);
                    } catch (Exception ex) {
                        // Handle individual promise failures
                        logger.error("Error getting result from promise: {}", ex.getMessage());
                    }
                });
                return false;
            }

            // Check results
            boolean anyCompleted = false;
            for (Promise<ProcessingResult> promise : actionPromises) {
                try {
                    ProcessingResult result = promise.get();
                    currentlyProcessing.remove(result.actionId);
                    if (result.completed) {
                        anyCompleted = true;
                    }
                } catch (Exception e) {
                    logger.error("Error processing action result: {}", e.getMessage());
                }
            }

            return anyCompleted;
        }

        return false;
    }

    private ProcessingResult processActionAsync(ActionNode action) {
        String actionId = action.getActionId();

        try {
            // Update status to processing
            updateActionStatus(actionId, ActionStatus.PROCESSING);

            // Build conversation history
            List<ActionNode> completedActions = getAllCompletedActions();
            StringBuilder convHistory = new StringBuilder();

            completedActions.forEach(completedAction -> {
                String actionText = completedAction.getActionDescription();
                String result = completedAction.getActionResponse();
                convHistory.append("**ACTION:**").append(System.lineSeparator());
                convHistory.append(actionText).append(System.lineSeparator());
                convHistory.append("**ACTION RESPONSE:**").append(System.lineSeparator());
                convHistory.append(result).append(System.lineSeparator());
                convHistory.append(System.lineSeparator());
            });

        logger.debug("Processing action: {}, with conv history length: {}", actionId, convHistory.length());

            // Try LLM completion using activity (moved from direct bean access)
            if (tryLLMCompletionViaActivity(action, convHistory.toString())) {
            // This specific log was already debug, no change needed here from previous step
            // logger.debug("Action {} completed by LLM activity", actionId);
                return new ProcessingResult(actionId, true);
            }

            // If LLM didn't complete, try user input
            if (tryUserInputRequest(action)) {
            // This specific log was already debug, no change needed here from previous step
            // logger.debug("Action {} waiting for user input", actionId);
                return new ProcessingResult(actionId, false);
            }

            return new ProcessingResult(actionId, false);

        } catch (Exception e) {
            logger.error("Error processing action {}: {}", actionId, e.getMessage(), e);
            updateActionStatus(actionId, ActionStatus.FAILED);
            return new ProcessingResult(actionId, false);
        }  finally {
            // Always remove from currently processing set
            currentlyProcessing.remove(actionId);
        }
    }

    private List<ActionNode> getAllCompletedActions() {
        return currentGoal.getNodes().stream()
                .filter(node -> node.getActionStatus() == ActionStatus.COMPLETED)
                .collect(Collectors.toList());
    }

    // FIXED: Use activity instead of direct bean access
    private boolean tryLLMCompletionViaActivity(ActionNode action, String convHistory) {
        try {
            logger.debug("Attempting LLM completion via activity for action: {}", action.getActionId());

            // Delegate to LLM activity instead of direct bean access
            LLMProcessingRequest request = new LLMProcessingRequest(
                    currentGoal.getGoal(),
                    convHistory,
                    action
            );

            LLMProcessingResult result = llmActivity.processActionWithLLM(request);

            if (result != null && result.isSuccess()) {
                updateActionOutput(action.getActionId(), result.getActionResult());
                updateActionStatus(action.getActionId(), result.getActionStatus());
                return result.getActionStatus() == ActionStatus.COMPLETED;
            }

            return false;

        } catch (Exception e) {
            logger.error("Error in LLM completion for action {}: {}", action.getActionId(), e.getMessage(), e);
            return false;
        }
    }

    private boolean tryUserInputRequest(ActionNode action) {
        String prompt = generatePrompt(action);

        // Check if we have a meaningful prompt
        if (prompt == null || prompt.startsWith("Please provide input for action:")) {
            Map<String, Object> actionParams = action.getActionParams();
            if (actionParams == null || !actionParams.containsKey("prompt_message")) {
                return false; // No meaningful prompt available
            }
        }

        try {
            logger.debug("Requesting user input for action {} with prompt: '{}'", action.getActionId(), prompt);

            // Set state before calling activity to avoid race conditions
            updateActionStatus(action.getActionId(), ActionStatus.WAITING_FOR_INPUT);
            waitingForUserInputMap.put(action.getActionId(), prompt);

            // Request user input asynchronously
            askUserActivity.ask(action.getActionId(), prompt);

            logger.debug("User input requested for action: {}", action.getActionId());
            return true;

        } catch (Exception e) {
            logger.error("Error requesting user input for action {}: {}", action.getActionId(), e.getMessage(), e);
            return false;
        }
    }

    @Override
    public void onUserResponse(String actionId, String userInput) {
        // This specific log was already debug, no change needed here from previous step
        // logger.debug("Received user response for actionId: {} with input: {}", actionId, userInput);

        // Validate that we're expecting input for this action
//        if (!waitingForUserInputMap.containsKey(actionId) ||
//                actionStatuses.get(actionId) != ActionStatus.WAITING_FOR_INPUT) {
//            logger.warn("Unexpected user response for actionId: {} - not waiting for input. Current status: {}",
//                    actionId, actionStatuses.get(actionId));
//            return;
//        }

        ActionNode action = currentGoal.getNodeById(actionId);
        if (action == null || action.getActionStatus() != ActionStatus.WAITING_FOR_INPUT) {
            logger.warn("Unexpected user response for actionId: {} - not waiting for input. Current status: {}",
                    actionId, actionStatuses.get(actionId));
            return;
        }

        // Process the user input asynchronously to avoid blocking the signal handler
        processUserInput(actionId, userInput, action);
    }

    private void processUserInput(String actionId, String userInput, ActionNode action) {
        // This specific log was already debug, no change needed here from previous step
        // logger.debug("Processing user input for action: {}", actionId);

        try {
            // Update status to processing
            updateActionStatus(actionId, ActionStatus.PROCESSING);

            // Process with LLM using the user input as context
//            tryLLMCompletionViaActivity(action, userInput);

            ActionEvaluationResult actionEvaluationResult = new ObjectMapper().readValue(llmActivity.evaluateAndProcessUserInput(currentGoal, action, userInput), ActionEvaluationResult.class);

            if(actionEvaluationResult.getStatus() == ActionEvaluationResult.Status.COMPLETED ) {
                // This specific log was already debug, no change needed here from previous step
                // logger.debug("User input status: {} successfully for actionId: {}", actionEvaluationResult.getStatus(), actionId);
                updateActionOutput(actionId, userInput);
                action.setActionResponse(userInput);
                updateActionStatus(action.getActionId(), ActionStatus.valueOf(actionEvaluationResult.getStatus().name()));
                waitingForUserInputMap.remove(actionId);
            } if(actionEvaluationResult.getStatus() == ActionEvaluationResult.Status.WAITING_FOR_INPUT ) {
                //ask user for more info
            }
            else {
                // This specific log was already warn, no change needed here from previous step
                // logger.warn("User input failed for actionId: {}", actionId);
                updateActionStatus(action.getActionId(), ActionStatus.valueOf(actionEvaluationResult.getStatus().name()));
            }



        } catch (Exception e) {
            logger.error("Error processing user input for action {}: {}", actionId, e.getMessage(), e);
            updateActionStatus(actionId, ActionStatus.FAILED);
            waitingForUserInputMap.remove(actionId);
        } finally {
            currentlyProcessing.remove(actionId);
        }
    }

    private void handleInvalidInput(String actionId, ActionNode action) {
        try {
            String originalPrompt = waitingForUserInputMap.get(actionId);
            String refinedPrompt = "Your previous input was not sufficient. " + originalPrompt;

            // This specific log was already debug, no change needed here from previous step
            // logger.debug("Re-prompting for action {} with refined prompt", actionId);

            // Reset to waiting state and re-prompt
            updateActionStatus(actionId, ActionStatus.WAITING_FOR_INPUT);
            askUserActivity.ask(actionId, refinedPrompt);

        } catch (Exception e) {
            logger.error("Error re-prompting for action {}: {}", actionId, e.getMessage(), e);
            updateActionStatus(actionId, ActionStatus.FAILED);
            waitingForUserInputMap.remove(actionId);
        }
    }

    private void updateActionStatus(String actionId, ActionStatus status) {
        actionStatuses.put(actionId, status);
        ActionNode node = currentGoal.getNodeById(actionId);
        if (node != null) {
            node.setActionStatus(status);
        }
        logger.debug("Updated action {} status to {}", actionId, status);
    }

    private void updateActionOutput(String actionId, String output) {
        actionOutputs.put(actionId, output);
        currentGoal.getActionOutputs().put(actionId, output);
        logger.debug("Updated action {} output: {}", actionId, output);
    }

    private List<ActionNode> getProcessableActions() {
        return currentGoal.getNodes().stream()
                .filter(node -> {
                    String actionId = node.getActionId();
                    ActionStatus currentStatus = actionStatuses.get(actionId);

                    // Only process PENDING actions that aren't currently being processed
                    boolean isPending = currentStatus == ActionStatus.PENDING;
                    boolean notCurrentlyProcessing = !currentlyProcessing.contains(actionId);
                    boolean depsMet = dependenciesMet(actionId);

                    logger.trace("Action {}: status={}, pending={}, notProcessing={}, depsMet={}",
                            actionId, currentStatus, isPending, notCurrentlyProcessing, depsMet);

                    return isPending && notCurrentlyProcessing && depsMet;
                })
                .collect(Collectors.toList());
    }

    private boolean hasWaitingActions() {
        return actionStatuses.values().stream()
                .anyMatch(status -> status == ActionStatus.WAITING_FOR_INPUT || status == ActionStatus.PENDING || status == ActionStatus.PROCESSING);
    }

    private boolean hasNewProcessableActions() {
        return !getProcessableActions().isEmpty();
    }

    private boolean dependenciesMet(String actionId) {
        if (currentGoal.getRelationships() == null) {
            return true;
        }

        Set<String> dependencies = currentGoal.getRelationships().stream()
                .filter(r -> Objects.equals(r.getTargetActionId(), actionId) && "DEPENDS_ON".equalsIgnoreCase(r.getType()))
                .map(Relationship::getSourceActionId)
                .collect(Collectors.toSet());

        if (dependencies.isEmpty()) {
            return true;
        }

        return dependencies.stream()
                .allMatch(sourceId -> actionStatuses.get(sourceId) == ActionStatus.COMPLETED);
    }

    private String generatePrompt(ActionNode action) {
        Object promptMessage = action.getActionParams().get("prompt_message");
        if (promptMessage instanceof String) {
            return (String) promptMessage;
        }
        return "Please provide input for action: " + action.getActionName();
    }

    private boolean isWorkflowComplete() {
        if (currentGoal == null || currentGoal.getNodes() == null) {
            return true;
        }

        // Check if all actions are in a terminal state
        boolean allTerminal = currentGoal.getNodes().stream()
                .allMatch(node -> {
                    ActionStatus status = actionStatuses.get(node.getActionId());
                    return status == ActionStatus.COMPLETED ||
                            status == ActionStatus.SKIPPED ||
                            status == ActionStatus.FAILED;
                });

        if (allTerminal) {
            return true;
        }

        // Handle actions that can't progress due to failed dependencies
        markSkippedActionsWithFailedDependencies();

        // Re-check after marking skipped actions
        return currentGoal.getNodes().stream()
                .allMatch(node -> {
                    ActionStatus status = actionStatuses.get(node.getActionId());
                    return status == ActionStatus.COMPLETED ||
                            status == ActionStatus.SKIPPED ||
                            status == ActionStatus.FAILED;
                });
    }

    private void markSkippedActionsWithFailedDependencies() {
        for (ActionNode node : currentGoal.getNodes()) {
            String actionId = node.getActionId();
            if (actionStatuses.get(actionId) == ActionStatus.PENDING &&
                    !dependenciesMet(actionId) &&
                    hasFailedDependencies(actionId)) {

                // This specific log was already warn, no change needed here from previous step
                // logger.warn("Marking action {} as SKIPPED due to failed dependencies", actionId);
                updateActionStatus(actionId, ActionStatus.SKIPPED);
            }
        }
    }

    private boolean hasFailedDependencies(String actionId) {
        if (currentGoal.getRelationships() == null) {
            return false;
        }

        Set<String> dependencies = currentGoal.getRelationships().stream()
                .filter(r -> Objects.equals(r.getTargetActionId(), actionId) && "DEPENDS_ON".equalsIgnoreCase(r.getType()))
                .map(Relationship::getSourceActionId)
                .collect(Collectors.toSet());

        return dependencies.stream()
                .anyMatch(sourceId -> actionStatuses.get(sourceId) == ActionStatus.FAILED);
    }

    private void handleWorkflowCompletion() {
        workflowActive = false;

        boolean anyActionFailed = actionStatuses.values().stream()
                .anyMatch(status -> status == ActionStatus.FAILED);

        if (anyActionFailed) {
            String failedActions = actionStatuses.entrySet().stream()
                    .filter(entry -> entry.getValue() == ActionStatus.FAILED)
                    .map(Map.Entry::getKey)
                    .collect(Collectors.joining(", "));

            logger.error("Workflow completed with FAILED actions: [{}]", failedActions);
            currentGoal.setActionOutputs(actionOutputs);

            throw Workflow.newFailedPromise(new Exception(
                    "Workflow completed with one or more FAILED actions: " + failedActions)).getFailure();
        }

        logger.info("Workflow completed successfully. Final outputs: {}", actionOutputs);
        currentGoal.setActionOutputs(actionOutputs);
    }

    // Helper class to track processing results
    private static class ProcessingResult {
        final String actionId;
        final boolean completed;

        ProcessingResult(String actionId, boolean completed) {
            this.actionId = actionId;
            this.completed = completed;
        }
    }
}

// Additional classes needed for the LLM activity approach

