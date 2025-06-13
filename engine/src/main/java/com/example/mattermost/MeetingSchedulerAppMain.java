package com.example.mattermost;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.datatype.jsr310.JavaTimeModule;
import io.temporal.client.WorkflowClient;
import io.temporal.client.WorkflowOptions;
import io.temporal.client.WorkflowStub;
import io.temporal.serviceclient.WorkflowServiceStubs;
import io.temporal.serviceclient.WorkflowServiceStubsOptions;
import io.temporal.worker.Worker;
import io.temporal.worker.WorkerFactory;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.io.InputStream;
import java.util.Map;
import java.util.UUID;

public class MeetingSchedulerAppMain {

    private static final Logger logger = LoggerFactory.getLogger(MeetingSchedulerAppMain.class);
    public static final String TASK_QUEUE = "MeetingSchedulingTaskQueue";
    // In a real app, use a proper Temporal service endpoint
    public static final String TEMPORAL_SERVICE_ADDRESS = "127.0.0.1:7233";

    public static void main(String[] args) throws Exception {

        // 0. Setup Jackson Mapper for JSON
        ObjectMapper objectMapper = new ObjectMapper();
        objectMapper.registerModule(new JavaTimeModule());

        // 1. Configure Temporal Service Stub
        WorkflowServiceStubsOptions wfOptions = WorkflowServiceStubsOptions.newBuilder()
                                                .setTarget(TEMPORAL_SERVICE_ADDRESS)
                                                .build();
        WorkflowServiceStubs service = WorkflowServiceStubs.newInstance(wfOptions);
        WorkflowClient client = WorkflowClient.newInstance(service);

        // 2. Configure Worker Factory and Worker
        WorkerFactory factory = WorkerFactory.newInstance(client);
        Worker worker = factory.newWorker(TASK_QUEUE);

        // 3. Register Workflow and Activity Implementations
        worker.registerWorkflowImplementationTypes(MeetingSchedulerWorkflowImpl.class);
        worker.registerActivitiesImplementations(new AskUserActivityImpl(), new ValidateInputActivityImpl());

        // 4. Start the Worker Factory (starts all registered workers)
        factory.start();
        logger.info("Worker started for task queue: {}", TASK_QUEUE);

        // 5. Load the Goal JSON from the example
        String goalJsonString = loadGoalJsonFromResource("example-goal.json");
        if (goalJsonString == null || goalJsonString.isEmpty()) {
            logger.error("Could not load example-goal.json. Exiting.");
            System.exit(1);
        }

        Goal goal = objectMapper.readValue(goalJsonString, Goal.class);
        logger.info("Successfully parsed Goal JSON: {}", goal.getGoal());


        // 6. Start a Workflow Instance
        String workflowId = "MeetingSchedulerWorkflow_" + UUID.randomUUID().toString();
        MeetingSchedulerWorkflow workflow = client.newWorkflowStub(
                MeetingSchedulerWorkflow.class,
                WorkflowOptions.newBuilder()
                        .setWorkflowId(workflowId)
                        .setTaskQueue(TASK_QUEUE)
                        .build()
        );

        logger.info("Starting workflow with ID: {}", workflowId);
        // Start workflow asynchronously
        WorkflowClient.start(workflow::scheduleMeeting, goal);
        logger.info("Workflow {} initiated. It will now process actions and wait for user inputs via signals.", workflowId);
        logger.info("You can use tctl to send signals, for example:");
        logger.info("tctl workflow signal -w {} -n onUserResponse -i '{}'", workflowId, "{\"actionId_goes_here\": {\"key\": \"value\"}}");

        // The main thread can exit, the worker threads will keep the process alive.
        // Or you can add a Thread.sleep or a condition to keep main alive for observation if needed.
        // For this example, letting main exit is fine as worker runs in daemon threads.
    }

    private static String loadGoalJsonFromResource(String resourceName) {
        try (InputStream inputStream = MeetingSchedulerAppMain.class.getClassLoader().getResourceAsStream(resourceName)) {
            if (inputStream == null) {
                logger.error("Resource not found: {}", resourceName);
                return null;
            }
            return new String(inputStream.readAllBytes(), java.nio.charset.StandardCharsets.UTF_8);
        } catch (Exception e) {
            logger.error("Error loading resource {}: {}", resourceName, e.getMessage(), e);
            return null;
        }
    }
}
