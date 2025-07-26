//package com.example.mattermost.refactor.workflow;
//
//import com.example.mattermost.domain.CurrentContext;
//import com.example.mattermost.domain.model.ActionNode;
//import com.example.mattermost.domain.model.ActionStatus;
//import com.example.mattermost.domain.model.Goal;
//import com.example.mattermost.domain.model.Relationship;
//import com.example.mattermost.workflow.activity.impl.LLMActivityImpl;
//import io.temporal.client.WorkflowClient;
//import io.temporal.client.WorkflowOptions;
//import io.temporal.serviceclient.WorkflowServiceStubs;
//import io.temporal.worker.Worker;
//import io.temporal.worker.WorkerFactory;
//import lombok.extern.slf4j.Slf4j;
//
//import java.time.Duration;
//import java.util.Arrays;
//import java.util.List;
//import java.util.Map;
//import java.util.Random;
//
//@Slf4j
//public class WorkflowStarter_old {
//    static final String TASK_QUEUE_PARENT = "parent-workflow-queue";
//    static final String TASK_QUEUE_CHILD = "child-workflow-queue";
//
//    public static void main(String[] args) {
//        log.info("=== Starting Workflow Application ===");
//
//        // Setup Temporal client
//        log.info("Setting up Temporal client...");
//        WorkflowServiceStubs service = WorkflowServiceStubs.newLocalServiceStubs();
//        WorkflowClient client = WorkflowClient.newInstance(service);
//        WorkerFactory factory = WorkerFactory.newInstance(client);
//
//        // Create workers
//        log.info("Creating parent worker for task queue: {}", TASK_QUEUE_PARENT);
//        Worker parentWorker = factory.newWorker(TASK_QUEUE_PARENT);
//        parentWorker.registerWorkflowImplementationTypes(DagExecutorWorkflowImpl.class);
//
//        // Register activities for parent worker
//        log.info("Registering WorkflowQueryActivity for parent worker");
//        parentWorker.registerActivitiesImplementations(new WorkflowQueryActivityImpl(client));
//
//        log.info("Creating child worker for task queue: {}", TASK_QUEUE_CHILD);
//        Worker childWorker = factory.newWorker(TASK_QUEUE_CHILD);
//        childWorker.registerWorkflowImplementationTypes(ChildWorkflowImpl.class);
//
//        // CRITICAL: Register activities for child worker too!
//        log.info("Registering LLMActivity for child worker");
//        childWorker.registerActivitiesImplementations(new LLMActivityImpl()); // Make sure this exists!
//
//        // Start workers
//        log.info("Starting worker factory...");
//        factory.start();
//        log.info("✅ All workers started successfully");
//
//        // Create and execute the example goal
//        log.info("Creating example goal...");
//        Goal exampleGoal = createExampleGoal();
//        log.info("Example goal created: {}", exampleGoal.getGoal());
//
//        String workflowId = exampleGoal.getWorkflowId() + new Random().nextInt();
//        log.info("Generated workflow ID: {}", workflowId);
//
//        WorkflowOptions parentOptions = WorkflowOptions.newBuilder()
//                .setWorkflowId(workflowId)
//                .setTaskQueue(TASK_QUEUE_PARENT)
//                .setWorkflowExecutionTimeout(Duration.ofMinutes(30))
//                .setWorkflowRunTimeout(Duration.ofMinutes(15))
//                .build();
//
//        log.info("Creating workflow stub...");
//        DagExecutorWorkflow workflow = client.newWorkflowStub(
//                DagExecutorWorkflow.class, parentOptions);
//
//        // Start workflow execution
//        log.info("Starting workflow execution...");
//        WorkflowClient.start(workflow::executeGoal,
//                new CurrentContext(exampleGoal, null, "currentThreadId", "currentChannelId", "currentUserId"));
//
//        log.info("✅ Workflow started successfully. Beginning monitoring...");
//
//        // Monitor workflow progress
//        monitorWorkflowWithClient(client, workflowId);
//
//        log.info("Shutting down...");
//        System.exit(0);
//    }
//
//    private static Goal createExampleGoal() {
//        List<ActionNode> nodes = Arrays.asList(
//            createActionNode("get_requester_meeting_details_1", "Get Requester's Meeting Details",
//                "Ask the user who initiated the request for the meeting topic, preferred days, times, and duration for the meeting with Arun and Jasbir.",
//                Map.of("required_users", Arrays.asList("requester"), "prompt_message", "What should be the topic of the meeting and do you have any preferred days, times, and duration for the meeting with Arun and Jasbir?")),
//
//            createActionNode("ask_arun_availability_1", "Ask Arun for Availability",
//                "Send a message to Arun to inquire about his available time slots for the meeting, considering the details provided by the requester.",
//                Map.of("recipient", "Arun", "subject", "Meeting Availability Request", "body", "Hi Arun, I'm trying to schedule a meeting with Jasbir and you. Could you please share your availability for a brief discussion about {get_requester_meeting_details_1.topic}?")),
//
//            createActionNode("ask_jasbir_availability_1", "Ask Jasbir for Availability",
//                "Send a message to Jasbir to inquire about her available time slots for the meeting, considering the details provided by the requester.",
//                Map.of("recipient", "Jasbir", "subject", "Meeting Availability Request", "body", "Hi Jasbir, I'm trying to schedule a meeting with Arun and you. Could you please share your availability for a brief discussion about {get_requester_meeting_details_1.topic}?")),
//
//            createActionNode("consolidate_availabilities_1", "Consolidate All Availabilities",
//                "Review and consolidate the meeting details from the requester and the availability received from Arun and Jasbir to find a common optimal time slot.",
//                Map.of()),
//
//            createActionNode("send_meeting_invite_1", "Send Final Meeting Invite",
//                "Create and send the official calendar invite to all attendees once the final meeting time is determined.",
//                Map.of("attendees", Arrays.asList("Arun", "Jasbir", "requester"), "subject", "{get_requester_meeting_details_1.topic}", "time", "{consolidate_availabilities_1.final_time}"))
//        );
//
//        List<Relationship> relationships = Arrays.asList(
//            new Relationship("get_requester_meeting_details_1", "ask_arun_availability_1", "DEPENDS_ON"),
//            new Relationship("get_requester_meeting_details_1", "ask_jasbir_availability_1", "DEPENDS_ON"),
//            new Relationship("get_requester_meeting_details_1", "consolidate_availabilities_1", "DEPENDS_ON"),
//            new Relationship("ask_arun_availability_1", "consolidate_availabilities_1", "DEPENDS_ON"),
//            new Relationship("ask_jasbir_availability_1", "consolidate_availabilities_1", "DEPENDS_ON"),
//            new Relationship("consolidate_availabilities_1", "send_meeting_invite_1", "DEPENDS_ON")
//        );
//
//        Goal goal = new Goal();
//        goal.setGoal("Schedule a meeting with Arun and Jasbir");
//        goal.setNodes(nodes);
//        goal.setRelationships(relationships);
//        goal.setWorkflowId("MeetingSchedulerWorkflow_156746");
//        return goal;
//    }
//
//    private static ActionNode createActionNode(String actionId, String actionName, String actionDescription, Map<String, Object> actionParams) {
//        ActionNode node = new ActionNode();
//        node.setActionId(actionId);
//        node.setActionName(actionName);
//        node.setActionDescription(actionDescription);
//        node.setActionParams(actionParams);
//        return node;
//    }
//
//    private static void monitorWorkflowWithClient(WorkflowClient client, String workflowId) {
//        log.info("Starting workflow monitoring for: {}", workflowId);
//
//        // Create a workflow stub for querying
//        DagExecutorWorkflow queryStub = client.newWorkflowStub(
//                DagExecutorWorkflow.class, workflowId);
//
//        int queryCount = 0;
//        try {
//            while (true) {
//                queryCount++;
//                try {
//                    log.debug("Querying workflow status (query #{})", queryCount);
//
//                    // Query the workflow status
//                    Map<String, ActionStatus> statuses = queryStub.getChildWorkflowStatuses();
//                    Map<String, List<String>> messages = queryStub.getChildWorkflowStatusMessages();
//                    String overallStatus = queryStub.getOverallStatus();
//
//                    System.out.println("=== Workflow Status (Query #" + queryCount + ") ===");
//                    System.out.println(overallStatus);
//
//                    // Show detailed information
//                    for (Map.Entry<String, ActionStatus> entry : statuses.entrySet()) {
//                        String actionId = entry.getKey();
//                        ActionStatus status = entry.getValue();
//
//                        System.out.println("Action " + actionId + ": " + status);
//
//                        List<String> actionMessages = messages.get(actionId);
//                        if (actionMessages != null && !actionMessages.isEmpty()) {
//                            // Show last few messages
//                            int messagesToShow = Math.min(3, actionMessages.size());
//                            for (int i = actionMessages.size() - messagesToShow; i < actionMessages.size(); i++) {
//                                System.out.println("  - " + actionMessages.get(i));
//                            }
//                        } else {
//                            System.out.println("  - No messages available");
//                        }
//                    }
//
//                    // Check if all workflows are completed or failed
//                    boolean allDone = statuses.values().stream()
//                            .allMatch(status -> status == ActionStatus.COMPLETED || status == ActionStatus.FAILED);
//
//                    if (allDone) {
//                        System.out.println("✅ All child workflows completed!");
//                        break;
//                    }
//
//                } catch (Exception e) {
//                    log.warn("Error querying workflow (query #{}): {}", queryCount, e.getMessage());
//                    System.out.println("Error querying workflow: " + e.getMessage());
//
//                    // If we can't query the workflow, it might have completed or failed
//                    if (queryCount > 10) { // Give it some tries
//                        System.out.println("Too many query failures, stopping monitoring");
//                        break;
//                    }
//                }
//
//                // Wait before next query
//                Thread.sleep(2000);
//            }
//        } catch (InterruptedException e) {
//            Thread.currentThread().interrupt();
//            log.info("Monitoring interrupted");
//            System.out.println("Monitoring interrupted");
//        }
//    }
//}