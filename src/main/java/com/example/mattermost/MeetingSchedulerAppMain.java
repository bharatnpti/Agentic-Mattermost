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
import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;

import java.io.InputStream;
import java.util.Map;
import java.util.UUID;

@SpringBootApplication
public class MeetingSchedulerAppMain {

    private static final Logger logger = LoggerFactory.getLogger(MeetingSchedulerAppMain.class);
    public static final String TASK_QUEUE = "MeetingSchedulingTaskQueue";
    // In a real app, use a proper Temporal service endpoint
    public static final String TEMPORAL_SERVICE_ADDRESS = "127.0.0.1:7233";

    public static void main(String[] args) throws Exception {
        SpringApplication.run(MeetingSchedulerAppMain.class, args);
    }
}
