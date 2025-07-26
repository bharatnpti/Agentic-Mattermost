package com.example.mattermost.refactor.workflow;

import com.example.mattermost.domain.CurrentContext;
import com.example.mattermost.domain.model.ActionNode;
import com.example.mattermost.domain.model.ActionStatus;
import com.example.mattermost.domain.model.Goal;
import com.example.mattermost.domain.model.Relationship;
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

@Slf4j
@Service
public class DagExecutorWorkflowImpl implements DagExecutorWorkflow {

    private static final String TASK_QUEUE_CHILD = "child-workflow-queue";

    private final Map<String, ChildWorkflowInterface> childWorkflows = new HashMap<>();
    private final Map<String, ActionStatus> childStatuses = new HashMap<>();
    private final Map<String, List<String>> childStatusMessages = new HashMap<>();
    private final Map<String, String> childWorkflowIds = new HashMap<>();

    @Override
    public String executeGoal(String workflowId, String task, String channelId, String threadId, String userId) {

        // Create activity stub with proper timeout and retry options
        GoalExtractionActivity goalExtractionActivity = Workflow.newActivityStub(
                GoalExtractionActivity.class,
                ActivityOptions.newBuilder()
                        .setStartToCloseTimeout(Duration.ofSeconds(30))
                        .setScheduleToCloseTimeout(Duration.ofSeconds(60))
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
        context.setCurrentUserId(userId);
        log.info("Starting parent workflow for goal: {}", goal.getGoal());

        // Rest of the method remains the same...
        Map<String, Set<String>> dependencies = buildDependencyGraph(goal.getRelationships());
        logDependencyGraph(dependencies, goal.getNodes());
        initializeChildWorkflows(context);
        executeDAG(context, dependencies);
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
                    .setWorkflowExecutionTimeout(Duration.ofMinutes(10))
                    .setWorkflowRunTimeout(Duration.ofMinutes(5))
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
        Set<String> completed = new HashSet<>();
        Set<String> inProgress = new HashSet<>();
        Set<String> failed = new HashSet<>();
        Goal goal = context.getGoal();
        List<ActionNode> nodes = goal.getNodes();

        // Collect all promises to wait for them properly
        List<Promise<Void>> allPromises = new ArrayList<>();

        int maxIterations = 100; // Prevent infinite loops
        int iteration = 0;

        while (completed.size() + failed.size() < nodes.size() && iteration < maxIterations) {
            iteration++;

            log.info("DAG execution loop iteration {}: completed={}, inProgress={}, failed={}, total={}",
                    iteration, completed.size(), inProgress.size(), failed.size(), nodes.size());

            boolean progressMade = false;

            for (ActionNode node : nodes) {
                String actionId = node.getActionId();

                // Skip if already processed or in progress
                if (completed.contains(actionId) || inProgress.contains(actionId) || failed.contains(actionId)) {
                    continue;
                }

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
                } else if (allDepsCompleted) {
                    // Start child workflow asynchronously
                    inProgress.add(actionId);
                    progressMade = true;

                    log.info("Starting action {} (dependencies satisfied: {})", actionId, deps);

                    // Create a copy of context for this specific action
                    CurrentContext nodeContext = new CurrentContext(
                            goal, node, context.getCurrentThreadId(),
                            context.getCurrentChannelId(), context.getCurrentUserId()
                    );

                    Promise<Void> promise = startChildWorkflowAsync(nodeContext, completed, inProgress, failed);
                    allPromises.add(promise);
                } else {
                    log.debug("Action {} waiting for dependencies: {} (completed: {})",
                            actionId, deps, completed);
                }
            }

            // If no progress was made and we're not done, we might have a circular dependency
            if (!progressMade && completed.size() + failed.size() < nodes.size()) {
                log.error("No progress made in iteration {}. Possible circular dependency or all remaining actions are blocked.", iteration);

                // Log remaining actions and their dependencies
                for (ActionNode node : nodes) {
                    String actionId = node.getActionId();
                    if (!completed.contains(actionId) && !inProgress.contains(actionId) && !failed.contains(actionId)) {
                        Set<String> deps = dependencies.getOrDefault(actionId, Collections.emptySet());
                        Set<String> unsatisfiedDeps = new HashSet<>(deps);
                        unsatisfiedDeps.removeAll(completed);
                        log.error("Blocked action {}: waiting for dependencies {}", actionId, unsatisfiedDeps);
                    }
                }

                // Mark remaining actions as failed to prevent infinite loop
                for (ActionNode node : nodes) {
                    String actionId = node.getActionId();
                    if (!completed.contains(actionId) && !inProgress.contains(actionId) && !failed.contains(actionId)) {
                        failed.add(actionId);
                        childStatuses.put(actionId, ActionStatus.FAILED);
                        childStatusMessages.put(actionId, Arrays.asList("Failed due to circular dependency or blocked dependencies"));
                    }
                }
                break;
            }

            // Small delay to prevent busy waiting, but only if we haven't completed everything
            if (completed.size() + failed.size() < nodes.size()) {
                Workflow.sleep(Duration.ofMillis(500)); // Increased delay slightly
            }
        }

        if (iteration >= maxIterations) {
            log.error("Maximum iterations ({}) reached, stopping DAG execution", maxIterations);
        }

        // Wait for all promises to complete
        if (!allPromises.isEmpty()) {
            Promise.allOf(allPromises).get();
        }

        log.info("DAG execution completed. Final status: completed={}, failed={}, total={}",
                completed.size(), failed.size(), nodes.size());
    }

    private Promise<Void> startChildWorkflowAsync(CurrentContext context, Set<String> completed,
                                                  Set<String> inProgress, Set<String> failed) {
        ActionNode node = context.getCurrentActionNode();
        String actionId = node.getActionId();

        ChildWorkflowInterface childWorkflow = childWorkflows.get(actionId);

        Promise<String> resultPromise = Async.function(() -> {
            try {
                return childWorkflow.executeAction(context);
            } catch (Exception e) {
                log.error("Child workflow {} execution failed: {}", actionId, e.getMessage());
                throw e;
            }
        });

        return resultPromise.handle((result, failure) -> {
            // Remove from in-progress first
            inProgress.remove(actionId);

            if (failure == null) {
                completed.add(actionId);
                childStatuses.put(actionId, ActionStatus.COMPLETED);
                log.info("Child workflow {} completed successfully", actionId);

                try {
                    List<String> childStatus = childWorkflow.getStatus();
                    childStatusMessages.put(actionId, childStatus);
                } catch (Exception e) {
                    // If we can't get status, use a default message
                    childStatusMessages.put(actionId, Arrays.asList("Completed successfully"));
                    log.warn("Could not retrieve status for completed action {}: {}", actionId, e.getMessage());
                }
            } else {
                failed.add(actionId);
                childStatuses.put(actionId, ActionStatus.FAILED);
                log.error("Child workflow {} failed: {}", actionId, failure.getMessage());

                try {
                    List<String> childStatus = childWorkflow.getStatus();
                    List<String> updatedStatus = new ArrayList<>(childStatus);
                    updatedStatus.add("Failed: " + failure.getMessage());
                    childStatusMessages.put(actionId, updatedStatus);
                } catch (Exception e) {
                    // If we can't get status, create a failure message
                    childStatusMessages.put(actionId, Arrays.asList("Failed: " + failure.getMessage()));
                }
            }

            return null;
        });
    }

    private void waitForAllChildWorkflows() {
        log.info("Waiting for all child workflows to complete...");

        Workflow.await(() -> {
            boolean allDone = childStatuses.values().stream()
                    .allMatch(status -> status == ActionStatus.COMPLETED || status == ActionStatus.FAILED);

            if (!allDone) {
                // Log current status for debugging
                long completed = childStatuses.values().stream()
                        .mapToLong(status -> status == ActionStatus.COMPLETED ? 1 : 0).sum();
                long failed = childStatuses.values().stream()
                        .mapToLong(status -> status == ActionStatus.FAILED ? 1 : 0).sum();
                long pending = childStatuses.values().stream()
                        .mapToLong(status -> status == ActionStatus.PENDING || status == ActionStatus.PROCESSING ? 1 : 0).sum();

                log.debug("Still waiting: completed={}, failed={}, pending/processing={}",
                        completed, failed, pending);
            }

            childStatuses.forEach((key, value) -> log.info("Child workflow {} status: {}", key, value));

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
                // Query the child workflow directly with timeout
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
                // Query the child workflow directly
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
        long completed = childStatuses.values().stream()
                .mapToLong(status -> status == ActionStatus.COMPLETED ? 1 : 0).sum();
        long failed = childStatuses.values().stream()
                .mapToLong(status -> status == ActionStatus.FAILED ? 1 : 0).sum();
        long processing = childStatuses.values().stream()
                .mapToLong(status -> status == ActionStatus.PROCESSING || status == ActionStatus.WAITING_FOR_INPUT ? 1 : 0).sum();
        long pending = childStatuses.values().stream()
                .mapToLong(status -> status == ActionStatus.PENDING ? 1 : 0).sum();
        long total = childStatuses.size();

        return String.format("Overall Status: %d/%d completed, %d failed, %d processing, %d pending",
                completed, total, failed, processing, pending);
    }
}
