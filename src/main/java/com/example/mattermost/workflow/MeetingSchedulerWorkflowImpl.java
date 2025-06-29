package com.example.mattermost.workflow;

import com.example.mattermost.domain.model.*;
import com.example.mattermost.workflow.activity.ActiveTaskActivity;
import com.example.mattermost.workflow.activity.AskUserActivity;
import com.example.mattermost.workflow.activity.LLMActivity;
import com.example.mattermost.workflow.activity.ValidateInputActivity;
import com.fasterxml.jackson.databind.ObjectMapper;
import io.temporal.activity.ActivityOptions;
import io.temporal.common.RetryOptions;
import io.temporal.workflow.Async;
import io.temporal.workflow.Promise;
import io.temporal.workflow.Workflow;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.time.Duration;
import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.Set;
import java.util.HashSet;
import java.util.concurrent.ConcurrentHashMap;
import java.util.stream.Collectors;

public class MeetingSchedulerWorkflowImpl implements MeetingSchedulerWorkflow {

    private static final Logger logger = LoggerFactory.getLogger(MeetingSchedulerWorkflowImpl.class);
    public static boolean debug = false;

    private Goal currentGoal;

    private String currentChannelId;

    private String currentUserId;

    private String currentThreadId;

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
    private final ActiveTaskActivity activeTaskActivity;

    public MeetingSchedulerWorkflowImpl() {
        // Define retry options for activities
        RetryOptions retryOptions = RetryOptions.newBuilder()
                .setInitialInterval(Duration.ofSeconds(1)) // Reduced from 100 seconds
                .setMaximumInterval(Duration.ofSeconds(30)) // Reduced from 100 seconds
                .setBackoffCoefficient(2)
                .setMaximumAttempts(3)
                .build();

        ActivityOptions defaultActivityOptions = ActivityOptions.newBuilder()
                .setStartToCloseTimeout(Duration.ofMinutes(5))
                .setRetryOptions(retryOptions)
                .build();

        // Separate options for activeTaskActivity with shorter timeout
        ActivityOptions activeTaskOptions = ActivityOptions.newBuilder()
                .setStartToCloseTimeout(Duration.ofSeconds(30))
                .setRetryOptions(retryOptions)
                .build();

        this.askUserActivity = Workflow.newActivityStub(AskUserActivity.class, defaultActivityOptions);
        this.validateInputActivity = Workflow.newActivityStub(ValidateInputActivity.class, defaultActivityOptions);
        this.llmActivity = Workflow.newActivityStub(LLMActivity.class, defaultActivityOptions);
        this.activeTaskActivity = Workflow.newActivityStub(ActiveTaskActivity.class, activeTaskOptions);
    }

    @Override
    public void scheduleMeeting(Goal goal, String channelId, String userId,  String threadId) {
        logger.info("Workflow started for goal: {}. Initializing action statuses.", goal.getGoal());
        this.currentGoal = goal;
        this.currentChannelId = channelId;
        this.currentUserId = userId;
        this.currentThreadId = threadId;

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
            logger.info("Starting workflow execution cycle");

            // Process all currently processable actions
            boolean actionsProcessed = processAllProcessableActions();

            if (!actionsProcessed && hasWaitingActions()) {
                logger.info("No processable actions, but have waiting actions. Awaiting signals...");
                // Add timeout to prevent infinite waiting
                Workflow.await(Duration.ofMinutes(30), () -> hasNewProcessableActions() || isWorkflowComplete());
            }

            // Add a small yield to prevent tight loops
            Workflow.sleep(Duration.ofMillis(100));
        }
    }

    private boolean processAllProcessableActions() {
        List<ActionNode> processableActions = getProcessableActions();

        if (processableActions.isEmpty()) {
            logger.info("No processable actions available");
            return false;
        }

        logger.info("Found {} processable actions", processableActions.size());
        logger.info("Processable action IDs: {}",
                processableActions.stream().map(ActionNode::getActionId).collect(Collectors.toList()));

        // Process actions sequentially to avoid deadlock issues
        boolean anyCompleted = false;
        for (ActionNode action : processableActions) {
            if (currentlyProcessing.add(action.getActionId())) {
                logger.info("Processing action: {}", action.getActionId());
                try {
                    ProcessingResult result = processActionSync(action);
                    if (result.completed) {
                        anyCompleted = true;
                    }
                } finally {
                    currentlyProcessing.remove(action.getActionId());
                }
            }
        }

        return anyCompleted;
    }

    private ProcessingResult processActionSync(ActionNode action) {
        String actionId = action.getActionId();

        try {
            updateActionStatusLocal(actionId, ActionStatus.PROCESSING);
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

            logger.info("Processing action: {}, with conv history length: {}", actionId, convHistory.length());

            // --- CHANGE STARTS HERE ---
            // Asynchronously determine action type
            Promise<ActionStatus> determineTypePromise = Async.function(llmActivity::determineActionType, currentGoal, action, convHistory.toString());

            // Wait for the promise to complete and get the result
            ActionStatus actionStatus = determineTypePromise.get(); // This will yield the workflow thread

            processBasedOnActionStatus(actionId, action, actionStatus, convHistory.toString());

//            if(actionStatus == ActionStatus.WAITING_FOR_INPUT) {
//                // Asynchronously ask user
//                Async.procedure(llmActivity::ask_user, currentGoal, action, convHistory.toString(), currentThreadId, currentChannelId, currentUserId); // Assuming generatePrompt is deterministic
//                updateActionStatusLocal(actionId, ActionStatus.WAITING_FOR_INPUT);
//                waitingForUserInputMap.put(action.getActionId(), generatePrompt(action)); // Assuming generatePrompt is deterministic
////                Async.procedure(this::updateActiveTaskAsync, action.getActionId(), ActionStatus.WAITING_FOR_INPUT);
//            } else if(actionStatus != ActionStatus.COMPLETED) {
//                // Asynchronously try LLM completion
//                Promise<LLMProcessingResult> llmCompletionPromise = Async.function(this::tryLLMCompletionViaActivity, action, convHistory.toString());
//                llmCompletionPromise.get(); // This will yield the workflow thread
//            }
            // --- CHANGE ENDS HERE ---

            return new ProcessingResult(actionId, false);

        } catch (Exception e) {
            logger.error("Error processing action {}: {}", actionId, e.getMessage(), e);
            updateActionStatusLocal(actionId, ActionStatus.FAILED);
            return new ProcessingResult(actionId, false);
        }
    }

    private LLMProcessingResult tryLLMCompletionViaActivity(ActionNode action, String convHistory) {
        try {
            logger.info("Attempting LLM completion via activity for action: {}", action.getActionId());

            LLMProcessingRequest request = new LLMProcessingRequest(
                    currentGoal.getGoal(),
                    convHistory,
                    action
            );

            // This call within tryLLMCompletionViaActivity is still synchronous relative to this method.
            // The previous change ensures that processActionSync itself doesn't block waiting for this.
            LLMProcessingResult result = llmActivity.processActionWithLLM(request, currentThreadId, currentUserId, currentChannelId);

            if (result != null && result.isSuccess()) {
                updateActionOutput(action.getActionId(), result.getActionResult());
                updateActionStatusLocal(action.getActionId(), result.getActionStatus());
                Async.procedure(this::updateActiveTaskAsync, action.getActionId(), result.getActionStatus());
            }
            return result;

        } catch (Exception e) {
            logger.error("Error in LLM completion for action {}: {}", action.getActionId(), e.getMessage(), e);
            throw e;
        }
    }

    private List<ActionNode> getAllCompletedActions() {
        return currentGoal.getNodes().stream()
                .filter(node -> node.getActionStatus() == ActionStatus.COMPLETED)
                .collect(Collectors.toList());
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
            logger.info("Requesting user input for action {} with prompt: '{}'", action.getActionId(), prompt);

            // Set state before calling activity to avoid race conditions
            updateActionStatusLocal(action.getActionId(), ActionStatus.WAITING_FOR_INPUT);
            waitingForUserInputMap.put(action.getActionId(), prompt);

            // Request user input asynchronously
            askUserActivity.ask(action.getActionId(), prompt);

            // Async update to external system
            Async.procedure(this::updateActiveTaskAsync, action.getActionId(), ActionStatus.WAITING_FOR_INPUT);

            logger.info("User input requested for action: {}", action.getActionId());
            return true;

        } catch (Exception e) {
            logger.error("Error requesting user input for action {}: {}", action.getActionId(), e.getMessage(), e);
            return false;
        }
    }

    @Override
    public void onUserResponse(String actionId, String userInput, String threadId, String channelId) {
        logger.info("Received user response for actionId: {} with input: {}", actionId, userInput);

        ActionNode action = currentGoal.getNodeById(actionId);
//        if (action == null || action.getActionStatus() != ActionStatus.WAITING_FOR_INPUT) {
//            logger.warn("Unexpected user response for actionId: {} - not waiting for input. Current status: {}",
//                    actionId, actionStatuses.get(actionId));
//            return;
//        }

        // Process the user input
        processUserInput(actionId, userInput, action, threadId, channelId);
    }

    private void processUserInput(String actionId, String userInput, ActionNode action, String threadId, String channelId) {
        logger.info("Processing user input for action: {}", actionId);

        try {
            // Update status to processing
            updateActionStatusLocal(actionId, ActionStatus.PROCESSING);
            action.setActionResponse(userInput);

            String convHistory = String.join(", ", action.getActionResponses());
            Promise<ActionStatus> determineTypePromise = Async.function(llmActivity::determineActionType, currentGoal, action, convHistory);

            // Wait for the promise to complete and get the result
            ActionStatus actionStatus = determineTypePromise.get(); // This will yield the workflow thread

            processBasedOnActionStatus(actionId, action, actionStatus, convHistory);

//            ActionEvaluationResult actionEvaluationResult = new ObjectMapper().readValue(
//                    llmActivity.evaluateAndProcessUserInput(currentGoal, action, userInput),
//                    ActionEvaluationResult.class);

//            if (actionEvaluationResult.getStatus() == ActionEvaluationResult.Status.COMPLETED) {
//                logger.info("User input status: {} successfully for actionId: {}",
//                        actionEvaluationResult.getStatus(), actionId);
//                updateActionOutput(actionId, userInput);
//                updateActionStatusLocal(actionId, ActionStatus.valueOf(actionEvaluationResult.getStatus().name()));
//                waitingForUserInputMap.remove(actionId);
//
//                // Async update to external system
//                Async.procedure(this::updateActiveTaskAsync, actionId, ActionStatus.COMPLETED);
//
//            } else if (actionEvaluationResult.getStatus() == ActionEvaluationResult.Status.WAITING_FOR_INPUT) {
//                // Ask user for more info - handled by re-prompting logic
//                handleInvalidInput(actionId, action);
//            } else {
//                logger.warn("User input failed for actionId: {}", actionId);
//                updateActionStatusLocal(actionId, ActionStatus.valueOf(actionEvaluationResult.getStatus().name()));
//
//                // Async update to external system
//                Async.procedure(this::updateActiveTaskAsync, actionId, ActionStatus.valueOf(actionEvaluationResult.getStatus().name()));
//            }

        } catch (Exception e) {
            logger.error("Error processing user input for action {}: {}", actionId, e.getMessage(), e);
            updateActionStatusLocal(actionId, ActionStatus.FAILED);
            waitingForUserInputMap.remove(actionId);

            // Async update to external system
//            Async.procedure(this::updateActiveTaskAsync, actionId, ActionStatus.FAILED);
        }
    }

    private void processBasedOnActionStatus(String actionId, ActionNode action, ActionStatus actionStatus, String convHistory) {
        if(actionStatus == ActionStatus.WAITING_FOR_INPUT) {
            // Asynchronously ask user
            Async.procedure(llmActivity::ask_user, currentGoal, action, convHistory, currentThreadId, currentChannelId, currentUserId); // Assuming generatePrompt is deterministic
            updateActionStatusLocal(actionId, ActionStatus.WAITING_FOR_INPUT);
//            waitingForUserInputMap.put(action.getActionId(), generatePrompt(action)); // Assuming generatePrompt is deterministic
//            Async.procedure(this::updateActiveTaskAsync, actionId, ActionStatus.WAITING_FOR_INPUT);

//                Async.procedure(this::updateActiveTaskAsync, action.getActionId(), ActionStatus.WAITING_FOR_INPUT);
        } else if(actionStatus == ActionStatus.AUTOMATED) {
            // Asynchronously try LLM completion
            Promise<LLMProcessingResult> llmCompletionPromise = Async.function(this::tryLLMCompletionViaActivity, action, convHistory);
            LLMProcessingResult llmProcessingResult = llmCompletionPromise.get();// This will yield the workflow thread
            StringBuffer updatedConvHistory = new StringBuffer(convHistory);
            updatedConvHistory.append(System.lineSeparator()).append(llmProcessingResult.getActionResult());
            processBasedOnActionStatus(actionId, action, llmProcessingResult.getActionStatus(), updatedConvHistory.toString());
        } else if(actionStatus == ActionStatus.COMPLETED) {
            updateActionStatusLocal(actionId, ActionStatus.COMPLETED);
//            Async.procedure(this::updateActiveTaskAsync, actionId, ActionStatus.COMPLETED);
        } else {
            logger.warn("Unexpected action status for actionId: {}", actionId);
        }
    }

    private void handleInvalidInput(String actionId, ActionNode action) {
        try {
            String originalPrompt = waitingForUserInputMap.get(actionId);
            String refinedPrompt = "Your previous input was not sufficient. " + originalPrompt;

            logger.info("Re-prompting for action {} with refined prompt", actionId);

            // Reset to waiting state and re-prompt
            updateActionStatusLocal(actionId, ActionStatus.WAITING_FOR_INPUT);
            askUserActivity.ask(actionId, refinedPrompt);

            // Async update to external system
            Async.procedure(this::updateActiveTaskAsync, actionId, ActionStatus.WAITING_FOR_INPUT);

        } catch (Exception e) {
            logger.error("Error re-prompting for action {}: {}", actionId, e.getMessage(), e);
            updateActionStatusLocal(actionId, ActionStatus.FAILED);
            waitingForUserInputMap.remove(actionId);
        }
    }

    // Local status update (non-blocking)
    private void updateActionStatusLocal(String actionId, ActionStatus status) {
        actionStatuses.put(actionId, status);
        ActionNode node = currentGoal.getNodeById(actionId);
        if (node != null) {
            node.setActionStatus(status);
        }
        logger.info("Updated action {} status to {}", actionId, status);
    }

    // Async procedure for external updates
    private void updateActiveTaskAsync(String actionId, ActionStatus status) {
        try {
            activeTaskActivity.updateActiveTask(actionId, status, currentGoal.getWorkflowId(), currentChannelId, currentUserId);
        } catch (Exception e) {
            logger.error("Error updating active task for action {}: {}", actionId, e.getMessage(), e);
        }
    }

    private void updateActionOutput(String actionId, String output) {
        actionOutputs.put(actionId, output);
        currentGoal.getActionOutputs().put(actionId, output);
        logger.info("Updated action {} output: {}", actionId, output);
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
                }).map(action -> {
                    action.setWorkflowId(currentGoal.getWorkflowId());
                    return action;
                })
                .collect(Collectors.toList());
    }

    private boolean hasWaitingActions() {
        return actionStatuses.values().stream()
                .anyMatch(status -> status == ActionStatus.WAITING_FOR_INPUT ||
                        status == ActionStatus.PENDING ||
                        status == ActionStatus.PROCESSING);
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

                logger.warn("Marking action {} as SKIPPED due to failed dependencies", actionId);
                updateActionStatusLocal(actionId, ActionStatus.SKIPPED);

                // Async update to external system
                Async.procedure(this::updateActiveTaskAsync, actionId, ActionStatus.SKIPPED);
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