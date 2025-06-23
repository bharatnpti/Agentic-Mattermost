package com.example.mattermost;

import io.temporal.client.WorkflowClient;
import io.temporal.serviceclient.WorkflowServiceStubs;
import io.temporal.serviceclient.WorkflowServiceStubsOptions;
import io.temporal.worker.Worker;
import io.temporal.worker.WorkerFactory;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;

@Configuration
public class TemporalConfig {

    private static final Logger logger = LoggerFactory.getLogger(TemporalConfig.class);

    // Using constants from MeetingSchedulerAppMain, consider moving them to application properties
    private static final String TEMPORAL_SERVICE_ADDRESS = MeetingSchedulerAppMain.TEMPORAL_SERVICE_ADDRESS;
    private static final String TASK_QUEUE = MeetingSchedulerAppMain.TASK_QUEUE;

    @Autowired
    private LLMActivityImpl llmActivity;

    @Autowired
    private ActiveTaskActivity activeTaskActivity;

//    @Autowired
//    private WorkerFactory workerFactory;

    @Bean
    public WorkflowServiceStubs workflowServiceStubs() {
        WorkflowServiceStubsOptions options = WorkflowServiceStubsOptions.newBuilder()
                .setTarget(TEMPORAL_SERVICE_ADDRESS)
                .build();
        logger.info("Configuring WorkflowServiceStubs to target: {}", TEMPORAL_SERVICE_ADDRESS);
        return WorkflowServiceStubs.newInstance(options);
    }

    @Bean
    public WorkflowClient workflowClient(WorkflowServiceStubs serviceStubs) {
        logger.info("Configuring WorkflowClient");
        return WorkflowClient.newInstance(serviceStubs);
    }

    @Bean
    public WorkerFactory workerFactory(WorkflowClient workflowClient) {
        logger.info("Configuring WorkerFactory");
        return WorkerFactory.newInstance(workflowClient);
    }

    @Bean
    public Worker startWorkerFactory(WorkerFactory workerFactory) {
        logger.info("Starting Temporal Worker Factory and registering components...");
        Worker worker = workerFactory.newWorker(TASK_QUEUE);

        // Register Workflow Implementation
        worker.registerWorkflowImplementationTypes(MeetingSchedulerWorkflowImpl.class);
        logger.info("Registered workflow implementation: {}", MeetingSchedulerWorkflowImpl.class.getName());

        // Register Activity Implementations
        // Assuming AskUserActivityImpl and ValidateInputActivityImpl will be Spring beans
        // or instantiated directly if not. For now, direct instantiation.
        // If these were Spring beans, they could be @Autowired into this class.
        worker.registerActivitiesImplementations(new AskUserActivityImpl(), new ValidateInputActivityImpl(), llmActivity, activeTaskActivity);
        logger.info("Registered activity implementations: {}, {}", AskUserActivityImpl.class.getName(), ValidateInputActivityImpl.class.getName());

        // Start the worker factory. This effectively starts all configured workers.
        workerFactory.start();
        logger.info("Temporal WorkerFactory started for task queue: {}", TASK_QUEUE);
        return worker;
    }
}
