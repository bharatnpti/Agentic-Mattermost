package com.example.mattermost.refactor.workflow;

import com.example.mattermost.domain.CurrentContext;
import com.example.mattermost.domain.model.ActionNode;
import com.example.mattermost.domain.model.ActionStatus;
import com.example.mattermost.domain.model.Goal;
import com.example.mattermost.domain.model.Relationship;
import com.example.mattermost.integration.mattermost.model.User;
import com.example.mattermost.service.GoalExtractionActivity;
import io.temporal.activity.ActivityOptions;
import io.temporal.common.RetryOptions;
import io.temporal.workflow.Async;
import io.temporal.workflow.ChildWorkflowOptions;
import io.temporal.workflow.Promise;
import io.temporal.workflow.Workflow;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Service;

import java.time.Duration;
import java.util.*;
import io.temporal.workflow.SignalMethod;

@Slf4j
@Service
public class DagExecutorWorkflowImpl implements DagExecutorWorkflow {

    private static final String TASK_QUEUE_CHILD = "child-workflow-queue";

    private final Map<String, ChildWorkflowInterface> childWorkflows = new HashMap<>();
    private final Map<String, ActionStatus> childStatuses = new HashMap<>();
    private final Map<String, List<String>> childStatusMessages = new HashMap<>();
    private final Map<String, String> childWorkflowIds = new HashMap<>();

    // Add instance-level tracking sets for DAG execution state
    private final Set<String> completed = new HashSet<>();
    private final Set<String> inProgress = new HashSet<>();
    private final Set<String> failed = new HashSet<>();

    private Map<String, Set<String>> dependencyGraph;
    private CurrentContext currentContext;

    @Override
    public String executeGoal(String workflowId, String task, String channelId, String threadId, User user) {

        // Create activity stub with proper timeout and retry options
        GoalExtractionActivity goalExtractionActivity = Workflow.newActivityStub(
                GoalExtractionActivity.class,
                ActivityOptions.newBuilder()
                        .setStartToCloseTimeout(Duration.ofMinutes(5))
                        .setScheduleToCloseTimeout(Duration.ofMinutes(10))
                        .setRetryOptions(RetryOptions.newBuilder()
                                .setInitialInterval(Duration.ofSeconds(1))
                                .setMaximumInterval(Duration.ofSeconds(10))
                                .setBackoffCoefficient(2.0)
                                .setMaximumAttempts(3)
                                .build())
                        .build()
        );

        log.info("About to call goal extraction activity for task: {}", task);
        Goal goal = Async.function(() -> goalExtractionActivity.extractGoalFromMessage(task)).get();
        goal.setWorkflowId(workflowId);

        CurrentContext context = new CurrentContext();
        context.setGoal(goal);
        context.setCurrentChannelId(channelId);
        context.setCurrentThreadId(threadId);
        context.setUser(user);
        log.info("Starting parent workflow for goal: {}", goal.getGoal());

        // Rest of the method remains the same...
        dependencyGraph = buildDependencyGraph(goal.getRelationships());
        logDependencyGraph(dependencyGraph, goal.getNodes());
        this.currentContext = context;
        initializeChildWorkflows(context);
        executeDAG(context, dependencyGraph);
        waitForAllChildWorkflows();

        return "Goal completed: " + goal.getGoal();
    }

    private void logDependencyGraph(Map<String, Set<String>> dependencies, List<ActionNode> nodes) {
        log.info("=== Dependency Graph ===");
        for (ActionNode node : nodes) {
            Set<String> deps = dependencies.getOrDefault(node.getActionId(), Collections.emptySet());
            log.info("Action {}: depends on {}", node.getActionId(), deps);
        }
        log.info("=== End Dependency Graph ===");
    }

    private void initializeChildWorkflows(CurrentContext context) {
        Goal goal = context.getGoal();
        for (ActionNode node : goal.getNodes()) {
            String childWorkflowId = goal.getWorkflowId() + "_" + node.getActionId();
            node.setWorkflowId(childWorkflowId);

            ChildWorkflowOptions childOptions = ChildWorkflowOptions.newBuilder()
                    .setWorkflowId(childWorkflowId)
                    .setTaskQueue(TASK_QUEUE_CHILD)
                    .setWorkflowExecutionTimeout(Duration.ofMinutes(120))
                    .setWorkflowRunTimeout(Duration.ofMinutes(60))
                    .setRetryOptions(RetryOptions.newBuilder()
                            .setMaximumAttempts(3)
                            .setInitialInterval(Duration.ofMinutes(10))
                            .setMaximumInterval(Duration.ofMinutes(100))
                            .setBackoffCoefficient(2.0)
                            .build())
                    .build();

            ChildWorkflowInterface childWorkflow = Workflow.newChildWorkflowStub(
                    ChildWorkflowInterface.class, childOptions);

            childWorkflows.put(node.getActionId(), childWorkflow);
            childWorkflowIds.put(node.getActionId(), childWorkflowId);
            childStatuses.put(node.getActionId(), ActionStatus.PENDING);
            childStatusMessages.put(node.getActionId(), Arrays.asList("Initialized"));
        }
    }

    private Map<String, Set<String>> buildDependencyGraph(List<Relationship> relationships) {
        Map<String, Set<String>> dependencies = new HashMap<>();

        // Handle null or empty relationships
        if (relationships == null || relationships.isEmpty()) {
            log.info("No relationships defined, all actions can run independently");
            return dependencies;
        }

        for (Relationship rel : relationships) {
            if (Objects.equals(rel.getType(), "DEPENDS_ON")) {
                dependencies.computeIfAbsent(rel.getTargetActionId(), k -> new HashSet<>())
                        .add(rel.getSourceActionId());
            }
        }

        return dependencies;
    }

    private void executeDAG(CurrentContext context, Map<String, Set<String>> dependencies) {

        WorkflowQueryActivity workflowQueryActivity = Workflow.newActivityStub(
                WorkflowQueryActivity.class,
                ActivityOptions.newBuilder()
                        .setStartToCloseTimeout(Duration.ofMinutes(30))
                        .setScheduleToCloseTimeout(Duration.ofSeconds(60))
                        .setRetryOptions(RetryOptions.newBuilder()
                                .setInitialInterval(Duration.ofSeconds(1))
                                .setMaximumInterval(Duration.ofSeconds(10))
                                .setBackoffCoefficient(2.0)
                                .setMaximumAttempts(3)
                                .build())
                        .build()
        );

        Goal goal = context.getGoal();
        List<ActionNode> nodes = goal.getNodes();
        boolean progressMade = false;
        List<String> newlyStartedActions = new ArrayList<>();

        log.info("executeDAG called - Current state: Completed={}, Failed={}, InProgress={}",
                completed.size(), failed.size(), inProgress.size());

        for (ActionNode node : nodes) {
            String actionId = node.getActionId();

            // Skip if already processed
            if (completed.contains(actionId) || failed.contains(actionId) || inProgress.contains(actionId)) {
                continue;
            }

//            ActionStatus actionStatus = workflowQueryActivity.queryChildWorkflowActionStatus(node.getWorkflowId());
//
//            // Update status tracking based on current status
//            if (ActionStatus.COMPLETED == actionStatus) {
//                if (!completed.contains(actionId)) {
//                    completed.add(actionId);
//                    inProgress.remove(actionId);
//                    childStatuses.put(actionId, ActionStatus.COMPLETED);
//                    log.info("Action {} marked as completed via query", actionId);
//                    progressMade = true;
//                }
//                continue;
//            } else if (ActionStatus.FAILED == actionStatus) {
//                if (!failed.contains(actionId)) {
//                    failed.add(actionId);
//                    inProgress.remove(actionId);
//                    childStatuses.put(actionId, ActionStatus.FAILED);
//                    log.info("Action {} marked as failed via query", actionId);
//                    progressMade = true;
//                }
//                continue;
//            } else if (ActionStatus.PROCESSING == actionStatus) {
//                if (!inProgress.contains(actionId)) {
//                    inProgress.add(actionId);
//                    childStatuses.put(actionId, ActionStatus.PROCESSING);
//                    log.info("Action {} is processing", actionId);
//                }
//                continue;
//            }

            // Check if all dependencies are satisfied
            Set<String> deps = dependencies.getOrDefault(actionId, Collections.emptySet());
            boolean allDepsCompleted = completed.containsAll(deps);

            // Check if any dependency failed
            boolean anyDepFailed = deps.stream().anyMatch(failed::contains);

            if (anyDepFailed) {
                // Mark this action as failed due to dependency failure
                failed.add(actionId);
                childStatuses.put(actionId, ActionStatus.FAILED);
                childStatusMessages.put(actionId, Arrays.asList("Failed due to dependency failure"));
                log.warn("Action {} failed due to dependency failure", actionId);
                progressMade = true;
            } else if (allDepsCompleted && !inProgress.contains(actionId)) {
                inProgress.add(actionId);
                newlyStartedActions.add(actionId);
                progressMade = true;

                log.info("Starting action {} (dependencies satisfied: {})", actionId, deps);

                // Create a copy of context for this specific action
                CurrentContext nodeContext = new CurrentContext(
                        goal, node, context.getCurrentThreadId(),
                        context.getCurrentChannelId(), context.getUser()
                );

                Map<String, String> completedResponses = new HashMap<>();
                log.info("completed tasks: {}", completed);
                for (String completedNodeId : completed) {
                    String completedWorkflowId = context.getGoal().getNodeById(completedNodeId).getWorkflowId();
//                    ChildWorkflowInterface completedChild = childWorkflows.get(completedWorkflowId);
                    try {
                        ActionStatus actionStatus = workflowQueryActivity.queryChildWorkflowActionStatus(completedWorkflowId);
                        log.info("Action with status: {} {}", completedWorkflowId, actionStatus);
                        String response = workflowQueryActivity.getActionResponse(completedWorkflowId);
                        completedResponses.put(completedNodeId, response);
                    } catch (Exception e) {
                        log.warn("Failed to fetch response from child {} e: {}, {}", completedWorkflowId, e, e.getMessage());
                    }
                }

                nodeContext.setPreviousActionResponses(completedResponses);

                log.info("Completed action Response: {} ", completedResponses);

                startChildWorkflowAsync(nodeContext);
            } else {
                log.debug("Action {} waiting for dependencies: {} (completed: {})",
                        actionId, deps, completed);
            }
        }

        // Log progress for parallel execution debugging
        if (progressMade) {
            log.info("Progress made in DAG execution iteration. Newly started actions: {}", newlyStartedActions);
        }

        log.info("executeDAG completed - Final state: Completed={}, Failed={}, InProgress={}",
                completed.size(), failed.size(), inProgress.size());
    }

    private Promise<Void> startChildWorkflowAsync(CurrentContext context) {
        ActionNode node = context.getCurrentActionNode();
        String actionId = node.getActionId();

        ChildWorkflowInterface childWorkflow = childWorkflows.get(actionId);

        Promise<Void> resultPromise = Async.procedure(() -> {
            try {
                childWorkflow.executeAction(context);
                // Note: Don't update status here as it will be handled by the signal
            } catch (Exception e) {
                log.error("startChildWorkflowAsync: Child workflow {} execution failed: {}", actionId, e.getMessage());

                // Update tracking sets on failure
                inProgress.remove(actionId);
                failed.add(actionId);
                childStatuses.put(actionId, ActionStatus.FAILED);
                childStatusMessages.put(actionId, Arrays.asList("Execution failed: " + e.getMessage()));

                throw e;
            }
        });

        return resultPromise;
    }

    private void waitForAllChildWorkflows() {
        log.info("Waiting for all child workflows to complete...");

        Workflow.await(() -> {
            boolean allDone = childStatuses.values().stream()
                    .allMatch(status -> status == ActionStatus.COMPLETED || status == ActionStatus.FAILED);

            if (!allDone) {
                // Log current status for debugging
                long completedCount = completed.size();
                long failedCount = failed.size();
                long pendingCount = inProgress.size();

                log.debug("Still waiting: completed={}, failed={}, in_progress={}",
                        completedCount, failedCount, pendingCount);
            }

//            childStatuses.forEach((key, value) -> log.info("waitForAllChildWorkflows: Child workflow {} status: {}", key, value));

            return allDone;
        });

        log.info("All child workflows completed");
    }

    @Override
    public Map<String, ActionStatus> getChildWorkflowStatuses() {
        Map<String, ActionStatus> currentStatuses = new HashMap<>();

        for (Map.Entry<String, ChildWorkflowInterface> entry : childWorkflows.entrySet()) {
            String actionId = entry.getKey();
            ChildWorkflowInterface childWorkflow = entry.getValue();

            try {
                ActionStatus status = childWorkflow.getActionStatus();
                currentStatuses.put(actionId, status);
                // Update cache with fresh data
                childStatuses.put(actionId, status);
            } catch (Exception e) {
                // If query fails, use the cached status
                ActionStatus cachedStatus = childStatuses.getOrDefault(actionId, ActionStatus.PENDING);
                currentStatuses.put(actionId, cachedStatus);
                // Only log as debug for expected initial failures
                if (cachedStatus != ActionStatus.PENDING) {
                    log.warn("Failed to query status for action {}, using cached status: {} - {}",
                            actionId, cachedStatus, e.getMessage());
                } else {
                    log.debug("Child workflow {} not ready for querying yet, using cached status: {}",
                            actionId, cachedStatus);
                }
            }
        }

        return currentStatuses;
    }

    @Override
    public Map<String, List<String>> getChildWorkflowStatusMessages() {
        Map<String, List<String>> result = new HashMap<>();

        for (Map.Entry<String, ChildWorkflowInterface> entry : childWorkflows.entrySet()) {
            String actionId = entry.getKey();
            ChildWorkflowInterface childWorkflow = entry.getValue();

            try {
                List<String> status = childWorkflow.getStatus();
                result.put(actionId, new ArrayList<>(status));
                // Update cache with fresh data
                childStatusMessages.put(actionId, new ArrayList<>(status));
            } catch (Exception e) {
                // If query fails, use cached messages or error message
                List<String> cachedMessages = childStatusMessages.get(actionId);
                if (cachedMessages != null) {
                    result.put(actionId, new ArrayList<>(cachedMessages));
                } else {
                    result.put(actionId, Arrays.asList("Initializing..."));
                }
                log.debug("Could not retrieve status messages for action {}: {}", actionId, e.getMessage());
            }
        }

        return result;
    }

    @Override
    public String getOverallStatus() {
        // Use the current cached statuses
        long completedCount = completed.size();
        long failedCount = failed.size();
        long processingCount = inProgress.size();
        long pendingCount = childStatuses.values().stream()
                .mapToLong(status -> status == ActionStatus.PENDING ? 1 : 0).sum();
        long total = childStatuses.size();

        return String.format("Overall Status: %d/%d completed, %d failed, %d processing, %d pending",
                completedCount, total, failedCount, processingCount, pendingCount);
    }

    @SignalMethod
    public void onChildCompleted(String actionId, ActionStatus actionStatus, String message) {
        log.info("Received signal: child {} completed with status {}, message={}", actionId, actionStatus, message);

        // Validate the action exists
        if (!childWorkflows.containsKey(actionId)) {
            log.warn("Received completion signal for unknown action: {}", actionId);
            return;
        }

        // Update all tracking mechanisms
        childStatuses.put(actionId, actionStatus);
        childStatusMessages.put(actionId, Arrays.asList(message));

        // Update DAG execution tracking sets
        inProgress.remove(actionId);

        if (actionStatus == ActionStatus.COMPLETED) {
            completed.add(actionId);
            log.info("Action {} moved to completed set. Total completed: {}", actionId, completed.size());
        } else if (actionStatus == ActionStatus.FAILED) {
            failed.add(actionId);
            log.info("Action {} moved to failed set. Total failed: {}", actionId, failed.size());
        }

        // Log current state for debugging parallel execution
        log.info("Current DAG state - Completed: {}, Failed: {}, InProgress: {}",
                completed, failed, inProgress);

        // Resume DAG execution to check for next executable actions
        if (currentContext != null && dependencyGraph != null) {
            log.info("Resuming DAG execution after child completion: {}", actionId);
            executeDAG(currentContext, dependencyGraph);
        } else {
            log.warn("Cannot resume DAG - context or dependency graph is null");
        }
    }

    @SignalMethod
    public void onUserResponse(String actionId, String userInput, String threadId, String channelId) {
        log.info("Received user response signal for action {}: {}", actionId, userInput);
        ChildWorkflowInterface childWorkflow = childWorkflows.get(actionId);
        if (childWorkflow != null) {
            childWorkflow.onUserResponse(userInput, threadId, channelId);
        }
    }
}