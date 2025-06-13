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


        boolean progressMade;
        do {
            progressMade = false;
            logger.info("Main loop: Checking processable actions.");
            List<ActionNode> pendingActions = getProcessableActions();
            logger.info("Main loop: Found {} processable actions: {}", pendingActions.size(), pendingActions.stream().map(ActionNode::getActionId).collect(Collectors.toList()));

            if (pendingActions.isEmpty() && !isWorkflowComplete()) {
                logger.info("Main loop: No actions currently processable. Waiting for signals or workflow completion. Current statuses: {}", actionStatuses);
                Workflow.await(() -> {
                    boolean newProcessableActions = !getProcessableActions().isEmpty();
                    boolean workflowNowComplete = isWorkflowComplete();
                    logger.info("Main loop: Workflow.await condition check: newProcessableActions={}, workflowNowComplete={}", newProcessableActions, workflowNowComplete);
                    return newProcessableActions || workflowNowComplete;
                });
                logger.info("Main loop: Awakened from Workflow.await().");
                // Re-check processable actions after await condition is met
                pendingActions = getProcessableActions();
                logger.info("Main loop: Re-checked processable actions after await: {}. Count: {}", pendingActions.stream().map(ActionNode::getActionId).collect(Collectors.toList()), pendingActions.size());
            }

            if (isWorkflowComplete()) {
                logger.info("Main loop: All actions completed. Workflow finishing.");
                break;
            }

            for (ActionNode action : pendingActions) {
                logger.info("Main loop: Evaluating action {} with status {}", action.getActionId(), actionStatuses.get(action.getActionId()));
                if (actionStatuses.get(action.getActionId()) == ActionStatus.PENDING) {
                    logger.info("Main loop: Processing PENDING action: {}", action.getActionId());
                    actionStatuses.put(action.getActionId(), ActionStatus.PROCESSING);
                    currentGoal.getNodeById(action.getActionId()).setActionStatus(ActionStatus.PROCESSING);
                    progressMade = true;

                    // For simplicity, this mock POC will assume all actions might need user input.
                    // A real implementation would check actionParams or type.
                    String prompt = generatePrompt(action);
                    String resolvedPrompt = resolvePlaceholders(prompt, actionOutputs);
                    logger.info("Main loop: Generated prompt for action {}: '{}'", action.getActionId(), resolvedPrompt);

                    try {
                        logger.info("Main loop: About to call askUserActivity.ask for action {}", action.getActionId());
                        askUserActivity.ask(action.getActionId(), resolvedPrompt);
                        logger.info("Main loop: askUserActivity.ask called for action {}. Setting status to WAITING_FOR_INPUT.", action.getActionId());
                        actionStatuses.put(action.getActionId(), ActionStatus.WAITING_FOR_INPUT);
                        currentGoal.getNodeById(action.getActionId()).setActionStatus(ActionStatus.WAITING_FOR_INPUT);
                        waitingForUserInputMap.put(action.getActionId(), prompt);
                        logger.info("Action {} is now WAITING_FOR_INPUT.", action.getActionId());
                    } catch (Exception e) {
                        logger.error("Error calling askUserActivity for action {}: {}", action.getActionId(), e.getMessage());
                        actionStatuses.put(action.getActionId(), ActionStatus.FAILED);
                        currentGoal.getNodeById(action.getActionId()).setActionStatus(ActionStatus.FAILED);
                    }
                }
            }
            // If no pending actions were found but workflow is not complete,
            // it means we are waiting for signals for actions that are in WAITING_FOR_INPUT state.
            // The Workflow.await() at the beginning of the loop handles this.
        } while (!isWorkflowComplete());

        logger.info("Workflow finished for goal: {}. Final outputs: {}", currentGoal.getGoal(), actionOutputs);
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
