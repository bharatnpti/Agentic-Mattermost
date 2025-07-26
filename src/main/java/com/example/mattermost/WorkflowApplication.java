//package com.example.mattermost;
//
//import com.example.mattermost.domain.CurrentContext;
//import com.example.mattermost.domain.model.ActionNode;
//import com.example.mattermost.domain.model.ActionStatus;
//import com.example.mattermost.domain.model.Goal;
//import com.example.mattermost.domain.model.Relationship;
//import com.example.mattermost.refactor.workflow.ChildWorkflowImpl;
//import com.example.mattermost.refactor.workflow.DagExecutorWorkflow;
//import com.example.mattermost.refactor.workflow.DagExecutorWorkflowImpl;
//import com.example.mattermost.refactor.workflow.WorkflowQueryActivityImpl;
//import com.example.mattermost.workflow.activity.impl.LLMActivityImpl;
//import lombok.RequiredArgsConstructor;
//import lombok.extern.slf4j.Slf4j;
//import org.springframework.boot.CommandLineRunner;
//import org.springframework.boot.SpringApplication;
//import org.springframework.boot.autoconfigure.SpringBootApplication;
//import io.temporal.client.WorkflowClient;
//import io.temporal.client.WorkflowOptions;
//import io.temporal.serviceclient.WorkflowServiceStubs;
//import io.temporal.worker.Worker;
//import io.temporal.worker.WorkerFactory;
//
//import java.time.Duration;
//import java.util.*;
//
//@Slf4j
//@RequiredArgsConstructor
//@SpringBootApplication
//public class WorkflowApplication implements CommandLineRunner {
//
//    private static final String TASK_QUEUE_PARENT = "parent-workflow-queue";
//    private static final String TASK_QUEUE_CHILD = "child-workflow-queue";
//
//    private final LLMActivityImpl llmActivityImpl;
//
//    public static void main(String[] args) {
//        SpringApplication.run(WorkflowApplication.class, args);
//    }
//
//    @Override
//    public void run(String... args) {
//        log.info("=== Starting Workflow Application ===");
//
//        Goal exampleGoal = createExampleGoal();
//        String workflowId = exampleGoal.getWorkflowId() + new Random().nextInt();
//        log.info("Generated workflow ID: {}", workflowId);
//
//        executeWorkflow(workflowId, exampleGoal);
//        log.info("Shutting down...");
//    }
//
//    private void executeWorkflow(String workflowId, Goal exampleGoal) {
//        WorkflowServiceStubs service = WorkflowServiceStubs.newLocalServiceStubs();
//        WorkflowClient client = WorkflowClient.newInstance(service);
//        WorkerFactory factory = WorkerFactory.newInstance(client);
//
//        Worker parentWorker = factory.newWorker(TASK_QUEUE_PARENT);
//        parentWorker.registerWorkflowImplementationTypes(DagExecutorWorkflowImpl.class);
//        parentWorker.registerActivitiesImplementations(new WorkflowQueryActivityImpl(client));
//
//        Worker childWorker = factory.newWorker(TASK_QUEUE_CHILD);
//        childWorker.registerWorkflowImplementationTypes(ChildWorkflowImpl.class);
//        childWorker.registerActivitiesImplementations(llmActivityImpl); // Autowired!
//
//        factory.start();
//        log.info("✅ All workers started successfully");
//
//
//        WorkflowOptions parentOptions = WorkflowOptions.newBuilder()
//                .setWorkflowId(workflowId)
//                .setTaskQueue(TASK_QUEUE_PARENT)
//                .setWorkflowExecutionTimeout(Duration.ofMinutes(30))
//                .setWorkflowRunTimeout(Duration.ofMinutes(15))
//                .build();
//
//        DagExecutorWorkflow workflow = client.newWorkflowStub(DagExecutorWorkflow.class, parentOptions);
//
//        log.info("Starting workflow execution...");
//        WorkflowClient.start(workflow::executeGoal,
//                new CurrentContext(exampleGoal, null, "currentThreadId", "currentChannelId", "currentUserId"));
//
//        monitorWorkflowWithClient(client, workflowId);
//    }
//
//    private Goal createExampleGoal() {
//        List<ActionNode> nodes = Arrays.asList(
//                createActionNode("get_requester_meeting_details_1", "Get Requester's Meeting Details",
//                        "Ask the user who initiated the request for the meeting topic...",
//                        Map.of("required_users", List.of("requester"), "prompt_message", "...")),
//                createActionNode("ask_arun_availability_1", "Ask Arun for Availability", "...", Map.of()),
//                createActionNode("ask_jasbir_availability_1", "Ask Jasbir for Availability", "...", Map.of()),
//                createActionNode("consolidate_availabilities_1", "Consolidate All Availabilities", "...", Map.of()),
//                createActionNode("send_meeting_invite_1", "Send Final Meeting Invite", "...", Map.of())
//        );
//
//        List<Relationship> relationships = List.of(
//                new Relationship("get_requester_meeting_details_1", "ask_arun_availability_1", "DEPENDS_ON"),
//                new Relationship("get_requester_meeting_details_1", "ask_jasbir_availability_1", "DEPENDS_ON"),
//                new Relationship("get_requester_meeting_details_1", "consolidate_availabilities_1", "DEPENDS_ON"),
//                new Relationship("ask_arun_availability_1", "consolidate_availabilities_1", "DEPENDS_ON"),
//                new Relationship("ask_jasbir_availability_1", "consolidate_availabilities_1", "DEPENDS_ON"),
//                new Relationship("consolidate_availabilities_1", "send_meeting_invite_1", "DEPENDS_ON")
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
//    private ActionNode createActionNode(String id, String name, String description, Map<String, Object> params) {
//        ActionNode node = new ActionNode();
//        node.setActionId(id);
//        node.setActionName(name);
//        node.setActionDescription(description);
//        node.setActionParams(params);
//        return node;
//    }
//
//    private void monitorWorkflowWithClient(WorkflowClient client, String workflowId) {
//        log.info("Starting workflow monitoring for: {}", workflowId);
//        DagExecutorWorkflow queryStub = client.newWorkflowStub(DagExecutorWorkflow.class, workflowId);
//
//        int queryCount = 0;
//        try {
//            while (true) {
//                queryCount++;
//                try {
//                    Map<String, ActionStatus> statuses = queryStub.getChildWorkflowStatuses();
//                    Map<String, List<String>> messages = queryStub.getChildWorkflowStatusMessages();
//                    String overallStatus = queryStub.getOverallStatus();
//
//                    System.out.println("=== Workflow Status (Query #" + queryCount + ") ===");
//                    System.out.println(overallStatus);
//
//                    for (Map.Entry<String, ActionStatus> entry : statuses.entrySet()) {
//                        String actionId = entry.getKey();
//                        System.out.println("Action " + actionId + ": " + entry.getValue());
//
//                        List<String> actionMessages = messages.get(actionId);
//                        if (actionMessages != null && !actionMessages.isEmpty()) {
//                            actionMessages.stream().skip(Math.max(0, actionMessages.size() - 3))
//                                    .forEach(msg -> System.out.println("  - " + msg));
//                        } else {
//                            System.out.println("  - No messages available");
//                        }
//                    }
//
//                    if (statuses.values().stream().allMatch(status -> status == ActionStatus.COMPLETED || status == ActionStatus.FAILED)) {
//                        System.out.println("✅ All child workflows completed!");
//                        break;
//                    }
//                } catch (Exception e) {
//                    log.warn("Query #{} failed: {}", queryCount, e.getMessage());
//                    if (queryCount > 10) break;
//                }
//                Thread.sleep(2000);
//            }
//        } catch (InterruptedException e) {
//            Thread.currentThread().interrupt();
//            log.info("Monitoring interrupted");
//        }
//    }
//}
