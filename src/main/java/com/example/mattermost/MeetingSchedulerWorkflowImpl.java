package com.example.mattermost;

import io.temporal.activity.ActivityOptions;
import io.temporal.common.RetryOptions;
import io.temporal.workflow.Workflow;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.time.Duration;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.Set;
import java.util.HashSet;
import java.util.stream.Collectors;

public class MeetingSchedulerWorkflowImpl implements MeetingSchedulerWorkflow {

    private static final Logger logger = LoggerFactory.getLogger(MeetingSchedulerWorkflowImpl.class);

    private Goal currentGoal;
    private final Map<String, ActionStatus> actionStatuses = new HashMap<>();
    private final Map<String, Map<String, Object>> actionOutputs = new HashMap<>();
    private final Map<String, String> waitingForUserInputMap = new HashMap<>(); // actionId -> "prompt"

    // Activities
    private final AskUserActivity askUserActivity;
    private final ValidateInputActivity validateInputActivity;
    private final LLMActivity llmActivity;

    public MeetingSchedulerWorkflowImpl() {
        // Define retry options for activities
        RetryOptions retryOptions = RetryOptions.newBuilder()
            .setInitialInterval(Duration.ofSeconds(1))
            .setMaximumInterval(Duration.ofSeconds(10))
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
        // Initialize action statuses from the goal object
        goal.getNodes().forEach(node -> {
            actionStatuses.put(node.getActionId(), node.getActionStatus());
            // Initialize outputs map for each action
            actionOutputs.put(node.getActionId(), new HashMap<>());
        });
        logger.info("Initialized actionStatuses: {}", actionStatuses); // Log initial statuses
        // Initialize transient actionOutputs in Goal object
        if (this.currentGoal.getActionOutputs() == null) {
            this.currentGoal.setActionOutputs(new HashMap<>());
        }

        // The main loop has been removed.
        // The logic for processing actions will be handled by evaluateAndRunProcessableActions.
        evaluateAndRunProcessableActions();

        logger.info("Workflow logic after initial action processing for goal: {}. Final outputs (will be updated by signals): {}", currentGoal.getGoal(), actionOutputs);
        // Ensure final outputs are set in the Goal object
        this.currentGoal.setActionOutputs(this.actionOutputs);
    }

    @Override
    public void onUserResponse(String actionId, Map<String, Object> userInput) {
        logger.info("Signal: Received onUserResponse for actionId: {} with input: {}. Current status: {}, waitingForUserInputMap containsKey: {}",
            actionId, userInput, actionStatuses.get(actionId), waitingForUserInputMap.containsKey(actionId));

        if (waitingForUserInputMap.containsKey(actionId) && actionStatuses.get(actionId) == ActionStatus.WAITING_FOR_INPUT) {
            ActionNode action = currentGoal.getNodeById(actionId);
            if (action == null) {
                logger.warn("Signal: Received response for unknown or already processed actionId: {}", actionId);
                return;
            }

            logger.info("Signal: Processing valid signal for action {}.", actionId);
            actionStatuses.put(actionId, ActionStatus.PROCESSING);
            action.setActionStatus(ActionStatus.PROCESSING);

            boolean isValid = false;
            try {
                logger.info("Signal: Calling validateInputActivity for action {}.", actionId);
                // The second parameter to validateInput could be specific criteria from actionParams
                isValid = validateInputActivity.validate(actionId, userInput, action.getActionParams());
                logger.info("Signal: validateInputActivity returned {} for action {}.", isValid, actionId);
            } catch (Exception e) {
                logger.error("Signal: Error calling validateInputActivity for action {}: {}", actionId, e.getMessage());
                actionStatuses.put(actionId, ActionStatus.FAILED);
                action.setActionStatus(ActionStatus.FAILED);
                waitingForUserInputMap.remove(actionId);
                return;
            }

            if (isValid) {
                logger.info("Input for action {} is valid. Marking as COMPLETED.", actionId);
                actionStatuses.put(actionId, ActionStatus.COMPLETED);
                action.setActionStatus(ActionStatus.COMPLETED);
                actionOutputs.put(actionId, userInput); // Store the validated user input as output

                // Update the shared Goal object's outputs
                this.currentGoal.getActionOutputs().put(actionId, userInput);

                waitingForUserInputMap.remove(actionId);
                // Since an action was completed by user input, evaluate if new actions are processable
                evaluateAndRunProcessableActions();
            } else {
                logger.info("Input for action {} is invalid. Re-prompting.", actionId);
                // Re-prompt by calling askUserActivity again
                try {
                    String originalPrompt = waitingForUserInputMap.get(actionId);
                    String refinedPrompt = "Your previous input was not sufficient. " + originalPrompt;
                    String resolvedReprompt = resolvePlaceholders(refinedPrompt, actionOutputs);
                    logger.info("Signal: Re-prompting for action {} with: '{}'", action.getActionId(), resolvedReprompt);
                    askUserActivity.ask(action.getActionId(), resolvedReprompt);
                    actionStatuses.put(action.getActionId(), ActionStatus.WAITING_FOR_INPUT); // Back to waiting
                    action.setActionStatus(ActionStatus.WAITING_FOR_INPUT);
                } catch (Exception e) {
                    logger.error("Signal: Error re-calling askUserActivity for action {}: {}", action.getActionId(), e.getMessage());
                    actionStatuses.put(actionId, ActionStatus.FAILED);
                    action.setActionStatus(ActionStatus.FAILED);
                    waitingForUserInputMap.remove(actionId);
                }
            }
        } else {
            logger.warn("Received unexpected user response for actionId: {} or action not in WAITING_FOR_INPUT state. Current status: {}", actionId, actionStatuses.get(actionId));
        }
    }

    private void evaluateAndRunProcessableActions() {
        logger.info("Evaluating processable actions.");
        List<ActionNode> processableActions = getProcessableActions();
        logger.info("Found {} processable actions: {}", processableActions.size(), processableActions.stream().map(ActionNode::getActionId).collect(Collectors.toList()));

        for (ActionNode action : processableActions) {
            logger.info("Processing action: {}", action.getActionId());
            processSingleAction(action);
        }
    }

    private void processSingleAction(ActionNode action) {
        logger.info("processSingleAction: Evaluating action {} with status {}", action.getActionId(), actionStatuses.get(action.getActionId()));
        // Ensure action is PENDING - this method should only be called for PENDING actions from getProcessableActions
        if (actionStatuses.get(action.getActionId()) != ActionStatus.PENDING) {
            logger.warn("processSingleAction: Action {} is not PENDING, current status: {}. Skipping.", action.getActionId(), actionStatuses.get(action.getActionId()));
            return;
        }

        logger.info("processSingleAction: Processing PENDING action: {}", action.getActionId());
        actionStatuses.put(action.getActionId(), ActionStatus.PROCESSING);
        currentGoal.getNodeById(action.getActionId()).setActionStatus(ActionStatus.PROCESSING);

        boolean llmAttempted = false;
        boolean llmCompleted = false;
        Map<String, Object> actionParams = action.getActionParams();

        if (actionParams != null && Boolean.TRUE.equals(actionParams.get("llm_can_complete"))) {
            llmAttempted = true;
            logger.info("processSingleAction: Action {} is configured for LLM completion. Attempting.", action.getActionId());
            try {
                if (llmActivity.isActionComplete(action.getActionId(), actionParams, actionOutputs)) {
                    logger.info("processSingleAction: LLM completed action {}.", action.getActionId());
                    actionStatuses.put(action.getActionId(), ActionStatus.COMPLETED);
                    currentGoal.getNodeById(action.getActionId()).setActionStatus(ActionStatus.COMPLETED);
                    actionOutputs.get(action.getActionId()).put("llm_output", "Completed by LLM mock"); // Optional: mock LLM output
                    llmCompleted = true;
                    // Since an action completed, re-evaluate to see if other actions become processable
                    evaluateAndRunProcessableActions();
                    return; // LLM completed, so we return
                } else {
                    logger.info("processSingleAction: LLM did not complete action {}.", action.getActionId());
                }
            } catch (Exception e) {
                logger.error("processSingleAction: Error calling LLMActivity for action {}: {}", action.getActionId(), e.getMessage(), e);
                actionStatuses.put(action.getActionId(), ActionStatus.FAILED);
                currentGoal.getNodeById(action.getActionId()).setActionStatus(ActionStatus.FAILED);
                // If LLM activity fails, we might not want to proceed to user input for this action.
                // Depending on policy, could return or let it fall through. For now, returning.
                return;
            }
        }

        // If not completed by LLM, try user input if a prompt is available
        if (!llmCompleted) {
            String prompt = generatePrompt(action); // Generates prompt (checks actionParams for "prompt_message")
            // Check if a specific prompt is set, similar to original logic
            boolean hasSpecificPrompt = (actionParams != null && actionParams.containsKey("prompt_message")) ||
                                        (prompt != null && !prompt.startsWith("Please provide input for action: "));

            if (hasSpecificPrompt) {
                String resolvedPrompt = resolvePlaceholders(prompt, actionOutputs);
                logger.info("processSingleAction: Action {} not completed by LLM or not configured for LLM. Proceeding with user input. Prompt: '{}'", action.getActionId(), resolvedPrompt);
                try {
                    logger.info("processSingleAction: About to call askUserActivity.ask for action {}", action.getActionId());
                    askUserActivity.ask(action.getActionId(), resolvedPrompt);
                    logger.info("processSingleAction: askUserActivity.ask called for action {}. Setting status to WAITING_FOR_INPUT.", action.getActionId());
                    actionStatuses.put(action.getActionId(), ActionStatus.WAITING_FOR_INPUT);
                    currentGoal.getNodeById(action.getActionId()).setActionStatus(ActionStatus.WAITING_FOR_INPUT);
                    waitingForUserInputMap.put(action.getActionId(), prompt); // Store original prompt for re-prompting
                    logger.info("processSingleAction: Action {} is now WAITING_FOR_INPUT.", action.getActionId());
                } catch (Exception e) {
                    logger.error("processSingleAction: Error calling askUserActivity for action {}: {}", action.getActionId(), e.getMessage(), e);
                    actionStatuses.put(action.getActionId(), ActionStatus.FAILED);
                    currentGoal.getNodeById(action.getActionId()).setActionStatus(ActionStatus.FAILED);
                }
            } else {
                // No LLM completion and no specific prompt configured.
                if (llmAttempted) {
                    logger.warn("processSingleAction: Action {} was LLM-attempted but not completed, and no fallback prompt. Marking FAILED.", action.getActionId());
                } else {
                    logger.info("processSingleAction: Action {} has no LLM configuration and no specific prompt. Marking FAILED.", action.getActionId());
                }
                actionStatuses.put(action.getActionId(), ActionStatus.FAILED);
                currentGoal.getNodeById(action.getActionId()).setActionStatus(ActionStatus.FAILED);
            }
        }
    }

    private List<ActionNode> getProcessableActions() {
        logger.info("getProcessableActions: Checking all nodes in currentGoal.");
        List<ActionNode> processableNodes = currentGoal.getNodes().stream()
            .filter(node -> {
                ActionStatus currentStatus = actionStatuses.get(node.getActionId());
                boolean isPending = currentStatus == ActionStatus.PENDING;
                boolean depsMet = false;
                if (isPending) { // Only check dependencies if it's pending
                    depsMet = dependenciesMet(node.getActionId());
                }
                logger.info("getProcessableActions: Node ID: {}, Status: {}, IsPending: {}, DependenciesMet: {}",
                    node.getActionId(), currentStatus, isPending, depsMet);
                return isPending && depsMet;
            })
            .collect(Collectors.toList());
        logger.info("getProcessableActions: Returning {} processable nodes.", processableNodes.size());
        return processableNodes;
    }

    private boolean dependenciesMet(String actionId) {
        logger.info("dependenciesMet: Checking dependencies for actionId: {}", actionId);
        if (currentGoal.getRelationships() == null) {
            logger.info("dependenciesMet: No relationships defined in goal for actionId: {}. Returning true.", actionId);
            return true; // No relationships means no dependencies
        }
        // Find all source actions that this actionId depends on
        Set<String> sourceDependencies = currentGoal.getRelationships().stream()
            .filter(r -> Objects.equals(r.getTargetActionId(), actionId) && "DEPENDS_ON".equalsIgnoreCase(r.getType()))
            .map(Relationship::getSourceActionId)
            .collect(Collectors.toSet());

        if (sourceDependencies.isEmpty()) {
            logger.info("dependenciesMet: No explicit dependencies found for actionId: {}. Returning true.", actionId);
            return true; // No dependencies for this action
        }
        logger.info("dependenciesMet: ActionId: {} has source dependencies: {}", actionId, sourceDependencies);

        // Check if all source dependencies are COMPLETED
        boolean allDepsMet = sourceDependencies.stream()
            .allMatch(sourceId -> {
                ActionStatus depStatus = actionStatuses.get(sourceId);
                boolean depCompleted = depStatus == ActionStatus.COMPLETED;
                logger.info("dependenciesMet: ActionId: {}, Dependency: {}, Status: {}, IsCompleted: {}",
                    actionId, sourceId, depStatus, depCompleted);
                return depCompleted;
            });
        logger.info("dependenciesMet: ActionId: {}, AllDependenciesMet: {}", actionId, allDepsMet);
        return allDepsMet;
    }

    private String generatePrompt(ActionNode action) {
        // Default prompt or extract from actionParams
        Object promptMessage = action.getActionParams().get("prompt_message");
        if (promptMessage instanceof String) {
            return (String) promptMessage;
        }
        // Fallback generic prompt
        return "Please provide input for action: " + action.getActionName();
    }

    private String resolvePlaceholders(String text, Map<String, Map<String, Object>> allActionOutputs) {
        String resolvedText = text;
        for (Map.Entry<String, Map<String, Object>> entry : allActionOutputs.entrySet()) {
            String sourceActionId = entry.getKey();
            Map<String, Object> outputs = entry.getValue();
            if (outputs != null) {
                for (Map.Entry<String, Object> outputEntry : outputs.entrySet()) {
                    String placeholder = "{" + sourceActionId + "." + outputEntry.getKey() + "}";
                    if (resolvedText.contains(placeholder)) {
                         Object value = outputEntry.getValue();
                         resolvedText = resolvedText.replace(placeholder, value != null ? String.valueOf(value) : "");
                    }
                }
            }
        }
        // Resolve direct action outputs if any (e.g. {actionId.output_key})
        // This part might need refinement based on how outputs are structured and named.
        // The current approach focuses on outputs from *other* actions.
        return resolvedText;
    }


    private boolean isWorkflowComplete() {
        if (currentGoal == null || currentGoal.getNodes() == null) {
            return true; // Or handle as an error state
        }
        long nonCompletedCount = currentGoal.getNodes().stream()
            .filter(node -> {
                ActionStatus status = actionStatuses.get(node.getActionId());
                return status != ActionStatus.COMPLETED && status != ActionStatus.SKIPPED;
            })
            .count();
        if (nonCompletedCount == 0) return true;

        // Check for deadlocks: if there are pending or waiting actions but none are processable
        // and some actions are not yet completed/skipped.
        List<ActionNode> processableActions = getProcessableActions();
        boolean hasPendingOrWaitingActions = currentGoal.getNodes().stream()
            .anyMatch(node -> {
                ActionStatus status = actionStatuses.get(node.getActionId());
                return status == ActionStatus.PENDING || status == ActionStatus.WAITING_FOR_INPUT;
            });

        if (processableActions.isEmpty() && hasPendingOrWaitingActions) {
             logger.warn("Potential deadlock or all remaining actions are waiting for external signals. Workflow might be stuck if no signals arrive.");
             // In a real scenario, you might have a timeout or a manual intervention path.
             // For now, if nothing is processable and we are waiting, we are not "complete" unless all are WAITING.
             // If some are PENDING but not processable (due to failed dependencies), they should eventually be marked SKIPPED or FAILED.

            // Let's try to mark actions with failed dependencies as SKIPPED
            boolean changedStatus = false;
            for (ActionNode node : currentGoal.getNodes()) {
                if (actionStatuses.get(node.getActionId()) == ActionStatus.PENDING && !dependenciesMet(node.getActionId())) {
                    // Check if any dependency is FAILED
                    if (hasFailedDependencies(node.getActionId())) {
                        actionStatuses.put(node.getActionId(), ActionStatus.SKIPPED);
                        node.setActionStatus(ActionStatus.SKIPPED);
                        logger.info("Action {} marked as SKIPPED due to failed dependencies.", node.getActionId());
                        changedStatus = true;
                    }
                }
            }
            // If we changed any status to SKIPPED, re-evaluate completion
            if (changedStatus) return isWorkflowComplete();
        }
        return false; // Default: not complete
    }

    private boolean hasFailedDependencies(String actionId) {
        if (currentGoal.getRelationships() == null) {
            return false;
        }
        Set<String> sourceDependencies = currentGoal.getRelationships().stream()
            .filter(r -> Objects.equals(r.getTargetActionId(), actionId) && "DEPENDS_ON".equalsIgnoreCase(r.getType()))
            .map(Relationship::getSourceActionId)
            .collect(Collectors.toSet());

        return sourceDependencies.stream()
            .anyMatch(sourceId -> actionStatuses.get(sourceId) == ActionStatus.FAILED);
    }
}
