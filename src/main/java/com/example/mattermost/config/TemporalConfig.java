package com.example.mattermost.config;

import com.example.mattermost.MeetingSchedulerAppMain;
import com.example.mattermost.refactor.workflow.DagExecutorWorkflowImpl;
import com.example.mattermost.refactor.workflow.ChildWorkflowImpl;
import com.example.mattermost.refactor.workflow.MessageHistoryActivityImpl;
import com.example.mattermost.refactor.workflow.WorkflowQueryActivity;
import com.example.mattermost.service.GoalExtractionActivity;
import com.example.mattermost.workflow.activity.ActiveTaskActivity;
import com.example.mattermost.workflow.activity.impl.AskUserActivityImpl;
import com.example.mattermost.workflow.activity.impl.LLMActivityImpl;
import com.example.mattermost.workflow.activity.impl.ValidateInputActivityImpl;
import io.temporal.client.WorkflowClient;
import io.temporal.serviceclient.WorkflowServiceStubs;
import io.temporal.serviceclient.WorkflowServiceStubsOptions;
import io.temporal.worker.Worker;
import io.temporal.worker.WorkerFactory;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;

@Configuration
public class TemporalConfig {

    private static final Logger logger = LoggerFactory.getLogger(TemporalConfig.class);

    // Using constants from MeetingSchedulerAppMain, consider moving them to application properties
    private static final String TASK_QUEUE = MeetingSchedulerAppMain.TASK_QUEUE;

    @Value("${temporal.service.address:#{null}}")
    private String temporalServiceAddressProperty;

    @Autowired
    private LLMActivityImpl llmActivity;

    @Autowired
    private ActiveTaskActivity activeTaskActivity;

    @Bean
    public WorkflowServiceStubs workflowServiceStubs() {
        // Priority: application property -> environment variable -> default
        String temporalServiceAddress = temporalServiceAddressProperty;

        if (temporalServiceAddress == null || temporalServiceAddress.isEmpty()) {
            temporalServiceAddress = System.getenv("TEMPORAL_SERVICE_ADDRESS");
        }

        if (temporalServiceAddress == null || temporalServiceAddress.isEmpty()) {
            temporalServiceAddress = MeetingSchedulerAppMain.TEMPORAL_SERVICE_ADDRESS;
        }

        logger.info("temporalServiceAddress is : {}", temporalServiceAddress);

        WorkflowServiceStubsOptions options = WorkflowServiceStubsOptions.newBuilder()
                .setTarget(temporalServiceAddress)
                .build();
        logger.info("Configuring WorkflowServiceStubs to target: {}", temporalServiceAddress);
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
    public Worker startWorkerFactory(WorkerFactory workerFactory,
                                     GoalExtractionActivity goalExtractionActivity,
                                     WorkflowQueryActivity workflowQueryActivity,
                                     MessageHistoryActivityImpl messageHistoryActivityImpl) {
        logger.info("Starting Temporal Worker Factory and registering components...");

        // Register the original worker for TASK_QUEUE
        Worker mainWorker = workerFactory.newWorker(TASK_QUEUE);
        mainWorker.registerWorkflowImplementationTypes(DagExecutorWorkflowImpl.class);
        mainWorker.registerActivitiesImplementations(
                new AskUserActivityImpl(),
                new ValidateInputActivityImpl(),
                llmActivity,
                activeTaskActivity
        );
        logger.info("Registered main worker for task queue: {}", TASK_QUEUE);

        // Register parent workflow worker
        Worker parentWorker = workerFactory.newWorker("parent-workflow-queue");
        parentWorker.registerWorkflowImplementationTypes(DagExecutorWorkflowImpl.class);
        parentWorker.registerActivitiesImplementations(
                goalExtractionActivity,
                workflowQueryActivity
        );
        logger.info("Registered parent worker for task queue: parent-workflow-queue");

        // Register child workflow worker
        Worker childWorker = workerFactory.newWorker("child-workflow-queue");
        childWorker.registerWorkflowImplementationTypes(ChildWorkflowImpl.class);
        childWorker.registerActivitiesImplementations(
                llmActivity,
                messageHistoryActivityImpl,
                activeTaskActivity
        );
        logger.info("Registered child worker for task queue: child-workflow-queue");

        // Start the worker factory. This effectively starts all configured workers.
        workerFactory.start();
        logger.info("Temporal WorkerFactory started with all workers");
        return mainWorker;
    }
}