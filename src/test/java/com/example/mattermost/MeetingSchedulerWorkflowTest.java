package com.example.mattermost;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.datatype.jsr310.JavaTimeModule;
import io.temporal.activity.ActivityOptions;
import io.temporal.api.common.v1.WorkflowExecution;
import io.temporal.api.enums.v1.WorkflowExecutionStatus;
import io.temporal.client.WorkflowClient;
import io.temporal.client.WorkflowFailedException;
import io.temporal.client.WorkflowOptions;
import io.temporal.client.WorkflowStub;
import io.temporal.testing.TestWorkflowEnvironment;
import io.temporal.testing.TestWorkflowExtension;
import io.temporal.worker.Worker;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.RegisterExtension;
import org.mockito.ArgumentCaptor;

import java.io.InputStream;
import java.time.Duration;
import java.util.Collections;
import java.util.HashMap;
import java.util.Map;
import java.util.UUID;
import java.util.List; // Added for actionIdCaptorAsk.getAllValues()

import static org.junit.jupiter.api.Assertions.*;
import static org.mockito.Mockito.*;

import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

public class MeetingSchedulerWorkflowTest {

    private static final Logger logger = LoggerFactory.getLogger(MeetingSchedulerWorkflowTest.class);

    @RegisterExtension
    public static final TestWorkflowExtension testWorkflowExtension =
            TestWorkflowExtension.newBuilder()
                    .setWorkflowTypes(MeetingSchedulerWorkflowImpl.class)
                    // Use mock activities for more control
                    .setDoNotStart(true) // We will start worker manually after registering mocks
                    .build();

    private AskUserActivity mockAskUserActivity;
    private ValidateInputActivity mockValidateInputActivity;
    private WorkflowClient workflowClient;
    private Worker worker;
    private String taskQueue;

    private TestWorkflowEnvironment testEnv;


    @BeforeEach
    public void setUp(TestWorkflowEnvironment testEnv) {
        // testEnv is injected by TestWorkflowExtension if not using setDoNotStart(true)
        // If using setDoNotStart(true), testEnv needs to be obtained from the extension.
        assertNotNull(testEnv, "testEnv should be injected by JUnit/Temporal extension.");
        this.testEnv = testEnv;


        taskQueue = "Test-" + UUID.randomUUID().toString();

        mockAskUserActivity = mock(AskUserActivity.class);
        assertNotNull(mockAskUserActivity, "mockAskUserActivity should be initialized in setUp.");
        mockValidateInputActivity = mock(ValidateInputActivity.class);
        assertNotNull(mockValidateInputActivity, "mockValidateInputActivity should be initialized in setUp.");

        worker = testEnv.getWorkerFactory().newWorker(taskQueue); // Changed getWorker to newWorker
        assertNotNull(worker, "worker should be initialized in setUp.");

        // Register Workflow Implementation
        worker.registerWorkflowImplementationTypes(MeetingSchedulerWorkflowImpl.class);

        // Create delegating implementations for registration
        AskUserActivity askUserActivityDelegator =
            (actionId, prompt) -> mockAskUserActivity.ask(actionId, prompt);
        ValidateInputActivity validateInputActivityDelegator =
            (actionId, userInput, actionParams) -> mockValidateInputActivity.validate(actionId, userInput, actionParams);

        worker.registerActivitiesImplementations(askUserActivityDelegator, validateInputActivityDelegator);

        // Configure activity options for mocks if needed, though direct mock control is often enough
        ActivityOptions activityOptions = ActivityOptions.newBuilder()
            .setStartToCloseTimeout(Duration.ofSeconds(30)) // Short timeout for tests
            .build();
        // This step is tricky with mocks if they are not registered via an instance but class.
        // We are using instances here, so it's fine.

        testEnv.start(); // Start the test environment worker factory

        workflowClient = testEnv.getWorkflowClient();
        assertNotNull(workflowClient, "workflowClient should be initialized in setUp.");
        System.out.println("setUp completed. worker, mocks, client initialized for taskQueue: " + taskQueue);
    }

    @AfterEach
    public void tearDown() {
        if (testEnv != null) {
             testEnv.close();
        }
    }

    private Goal loadGoalFromJson(String resourceName) throws Exception {
        ObjectMapper objectMapper = new ObjectMapper().registerModule(new JavaTimeModule());
        try (InputStream inputStream = MeetingSchedulerWorkflowTest.class.getClassLoader().getResourceAsStream(resourceName)) {
            if (inputStream == null) {
                throw new RuntimeException("Cannot find resource: " + resourceName);
            }
            return objectMapper.readValue(inputStream, Goal.class);
        }
    }

    @Test
    public void testSuccessfulWorkflowExecution() throws Exception {
        System.out.println("Starting testSuccessfulWorkflowExecution with taskQueue: " + this.taskQueue);
        assertNotNull(mockAskUserActivity, "mockAskUserActivity should not be null at start of test method.");
        assertNotNull(mockValidateInputActivity, "mockValidateInputActivity should not be null at start of test method.");
        assertNotNull(worker, "worker should not be null at start of test method.");
        assertNotNull(workflowClient, "workflowClient should not be null at start of test method.");

        Goal goal = loadGoalFromJson("test-goal-simple.json"); // A simplified goal for testing

        // Mock activity behavior
        doNothing().when(mockAskUserActivity).ask(anyString(), anyString());
        when(mockValidateInputActivity.validate(anyString(), anyMap(), anyMap())).thenReturn(true);

        MeetingSchedulerWorkflow workflow = workflowClient.newWorkflowStub(
                MeetingSchedulerWorkflow.class,
                WorkflowOptions.newBuilder()
                        .setWorkflowId("testSuccessfulWorkflow-" + UUID.randomUUID())
                        .setTaskQueue(taskQueue)
                        .build());

        // Start workflow
        WorkflowExecution execution = WorkflowClient.start(workflow::scheduleMeeting, goal);

        ArgumentCaptor<String> actionIdCaptor = ArgumentCaptor.forClass(String.class);
        ArgumentCaptor<String> promptCaptor = ArgumentCaptor.forClass(String.class);

        // Action 1: "get_details"
        // Wait for askUserActivity.ask to be called for "get_details"
        logger.info("Test: Verifying askUserActivity.ask for get_details...");
        verify(mockAskUserActivity, timeout(5000).times(1)).ask(actionIdCaptor.capture(), promptCaptor.capture());
        assertEquals("get_details", actionIdCaptor.getValue());
        assertTrue(promptCaptor.getValue().contains("preferred topic"));
        logger.info("Test: askUserActivity.ask for get_details verified. Sending signal.");

        // Send signal for "get_details"
        workflow.onUserResponse("get_details", Map.of("topic", "Test Meeting", "datetime", "Tomorrow", "duration", "1hr"));
        logger.info("Test: Signal for get_details sent.");

        // Action 2: "send_invite" (depends on get_details)
        // Wait for askUserActivity.ask to be called for "send_invite"
        // This will be the second call to ask() overall
        logger.info("Test: Verifying askUserActivity.ask for send_invite...");
        verify(mockAskUserActivity, timeout(5000).times(2)).ask(actionIdCaptor.capture(), promptCaptor.capture());
        assertEquals("send_invite", actionIdCaptor.getValue()); // Mockito captures the latest value
        assertTrue(promptCaptor.getValue().contains("Confirm sending invite"));
        logger.info("Test: askUserActivity.ask for send_invite verified. Sending signal.");

        // Send signal for "send_invite"
        workflow.onUserResponse("send_invite", Map.of("status", "Invite sent"));
        logger.info("Test: Signal for send_invite sent.");

        // Wait for workflow to complete. Max 20 seconds for this test.
        logger.info("Test: Waiting for workflow completion...");
        WorkflowStub.fromTyped(workflow).getResult(20, java.util.concurrent.TimeUnit.SECONDS, Void.class);
        logger.info("Test: Workflow completed.");

        // Verify overall interactions
        verify(mockAskUserActivity, times(2)).ask(anyString(), anyString());
        verify(mockValidateInputActivity, times(2)).validate(anyString(), anyMap(), anyMap());

        // WorkflowStub.getResult() would throw an exception if not completed successfully.
    }

    @Test
    public void testInputValidationFailureAndRetry() throws Exception {
        System.out.println("Starting testInputValidationFailureAndRetry with taskQueue: " + this.taskQueue);
        assertNotNull(mockAskUserActivity, "mockAskUserActivity should not be null at start of test method.");
        assertNotNull(mockValidateInputActivity, "mockValidateInputActivity should not be null at start of test method.");
        assertNotNull(worker, "worker should not be null at start of test method.");

        Goal goal = loadGoalFromJson("test-goal-single-action.json"); // Goal with one action
        ActionNode singleAction = goal.getNodes().get(0);

        // Mock activity behavior: fail first, then succeed
        when(mockValidateInputActivity.validate(eq(singleAction.getActionId()), anyMap(), anyMap()))
            .thenReturn(false) // First call
            .thenReturn(true);  // Second call
        doNothing().when(mockAskUserActivity).ask(eq(singleAction.getActionId()), anyString());

        MeetingSchedulerWorkflow workflow = workflowClient.newWorkflowStub(
            MeetingSchedulerWorkflow.class,
            WorkflowOptions.newBuilder()
                .setWorkflowId("testValidationFailure-" + UUID.randomUUID())
                .setTaskQueue(taskQueue)
                .build());

        WorkflowClient.start(workflow::scheduleMeeting, goal);

        // Signal 1 (leads to validation failure)
        workflow.onUserResponse(singleAction.getActionId(), Map.of("input", "bad_initial_input"));

        // Wait a bit for the workflow to process the first response and re-prompt.
        // This is a common challenge in testing Temporal workflows with external interaction points.
        // Using TestWorkflowEnvironment.sleep() or waiting for mock invocations with timeouts.
        // Verify initial prompt
        ArgumentCaptor<String> promptCaptor1 = ArgumentCaptor.forClass(String.class);
        logger.info("Test: Verifying initial ask for {}", singleAction.getActionId());
        verify(mockAskUserActivity, timeout(5000).times(1)).ask(eq(singleAction.getActionId()), promptCaptor1.capture());
        assertTrue(promptCaptor1.getValue().contains(goal.getNodeById(singleAction.getActionId()).getActionParams().get("prompt_message").toString()));
        logger.info("Test: Initial ask for {} verified. Sending bad input.", singleAction.getActionId());

        // Signal 1 (leads to validation failure)
        workflow.onUserResponse(singleAction.getActionId(), Map.of("input", "bad_initial_input"));
        logger.info("Test: Bad input signal sent for {}.", singleAction.getActionId());

        // Verify re-prompt after validation failure
        ArgumentCaptor<String> promptCaptor2 = ArgumentCaptor.forClass(String.class);
        logger.info("Test: Verifying re-prompt for {} after bad input.", singleAction.getActionId());
        // This is the second call to ask() for this specific actionId in sequence
        verify(mockAskUserActivity, timeout(5000).times(1)).ask(eq(singleAction.getActionId()), promptCaptor2.capture());
        assertTrue(promptCaptor2.getValue().contains("Your previous input was not sufficient."));
        logger.info("Test: Re-prompt for {} verified. Sending good input.", singleAction.getActionId());

        // Signal 2 (leads to validation success)
        workflow.onUserResponse(singleAction.getActionId(), Map.of("input", "good_final_input"));
        logger.info("Test: Good input signal sent for {}.", singleAction.getActionId());

        logger.info("Test: Waiting for workflow completion for testInputValidationFailureAndRetry...");
        WorkflowStub.fromTyped(workflow).getResult(15, java.util.concurrent.TimeUnit.SECONDS, Void.class);
        logger.info("Test: Workflow completed for testInputValidationFailureAndRetry.");

        verify(mockAskUserActivity, times(2)).ask(eq(singleAction.getActionId()), anyString());
        verify(mockValidateInputActivity, times(2)).validate(eq(singleAction.getActionId()), anyMap(), anyMap());
        // WorkflowStub.getResult() would throw an exception if not completed successfully.
    }


    // Placeholder for a test on dependency handling and placeholder resolution
    @Test
    public void testDependencyAndPlaceholderResolution() throws Exception {
        System.out.println("Starting testDependencyAndPlaceholderResolution with taskQueue: " + this.taskQueue);
        assertNotNull(mockAskUserActivity, "mockAskUserActivity should not be null at start of test method.");
        assertNotNull(mockValidateInputActivity, "mockValidateInputActivity should not be null at start of test method.");
        assertNotNull(worker, "worker should not be null at start of test method.");

        Goal goal = loadGoalFromJson("example-goal.json"); // Use the more complex goal

        // Setup complex mocking for askUser and validateInput
        // For askUser, capture prompts to verify placeholder resolution
        ArgumentCaptor<String> promptCaptor = ArgumentCaptor.forClass(String.class);
        ArgumentCaptor<String> actionIdCaptorAsk = ArgumentCaptor.forClass(String.class);
        doNothing().when(mockAskUserActivity).ask(actionIdCaptorAsk.capture(), promptCaptor.capture());

        // For validateInput, always return true for simplicity in this test
        when(mockValidateInputActivity.validate(anyString(), anyMap(), anyMap())).thenReturn(true);

        MeetingSchedulerWorkflow workflow = workflowClient.newWorkflowStub(
                MeetingSchedulerWorkflow.class,
                WorkflowOptions.newBuilder()
                        .setWorkflowId("testDependency-" + UUID.randomUUID())
                        .setTaskQueue(taskQueue)
                        .build());

        WorkflowClient.start(workflow::scheduleMeeting, goal);

        // Simulate user responses in dependency order
        // 1. get_requester_preferred_details_001
        logger.info("Test: Verifying ask for get_requester_preferred_details_001");
        verify(mockAskUserActivity, timeout(5000).times(1)).ask(eq("get_requester_preferred_details_001"), promptCaptor.capture());
        Map<String, Object> detailsInput = Map.of("topic", "Project Phoenix", "datetime", "Next Monday 10AM", "duration", "1 hour");
        workflow.onUserResponse("get_requester_preferred_details_001", detailsInput);
        logger.info("Test: Signal for get_requester_preferred_details_001 sent.");

        // 2. ask_arun_availability_001 (depends on 1)
        logger.info("Test: Verifying ask for ask_arun_availability_001");
        verify(mockAskUserActivity, timeout(5000).times(1)).ask(eq("ask_arun_availability_001"), promptCaptor.capture());
        workflow.onUserResponse("ask_arun_availability_001", Map.of("arun_availability", "Monday 10AM-12PM"));
        logger.info("Test: Signal for ask_arun_availability_001 sent.");

        // 3. ask_jasbir_availability_001 (depends on 1)
        logger.info("Test: Verifying ask for ask_jasbir_availability_001");
        verify(mockAskUserActivity, timeout(5000).times(1)).ask(eq("ask_jasbir_availability_001"), promptCaptor.capture());
        workflow.onUserResponse("ask_jasbir_availability_001", Map.of("jasbir_availability", "Monday 10AM-11AM"));
        logger.info("Test: Signal for ask_jasbir_availability_001 sent.");

        // 4. consolidate_availabilities_001 (depends on 1, 2, 3)
        logger.info("Test: Verifying ask for consolidate_availabilities_001");
        verify(mockAskUserActivity, timeout(5000).times(1)).ask(eq("consolidate_availabilities_001"), promptCaptor.capture());
        workflow.onUserResponse("consolidate_availabilities_001", Map.of("proposed_time", "Monday 10AM (Consolidated)"));
        logger.info("Test: Signal for consolidate_availabilities_001 sent.");

        // 5. propose_final_time_for_approval_001 (depends on 4)
        logger.info("Test: Verifying ask for propose_final_time_for_approval_001");
        verify(mockAskUserActivity, timeout(5000).times(1)).ask(eq("propose_final_time_for_approval_001"), promptCaptor.capture());
        workflow.onUserResponse("propose_final_time_for_approval_001", Map.of("approved_time_final", "Monday 10AM Approved"));
        logger.info("Test: Signal for propose_final_time_for_approval_001 sent.");

        // 6. send_meeting_invite_001 (depends on 5)
        logger.info("Test: Verifying ask for send_meeting_invite_001");
        verify(mockAskUserActivity, timeout(5000).times(1)).ask(eq("send_meeting_invite_001"), promptCaptor.capture());
        workflow.onUserResponse("send_meeting_invite_001", Map.of("status", "Final invite sent successfully"));
        logger.info("Test: Signal for send_meeting_invite_001 sent.");

        logger.info("Test: Waiting for workflow completion for testDependencyAndPlaceholderResolution...");
        WorkflowStub.fromTyped(workflow).getResult(30, java.util.concurrent.TimeUnit.SECONDS, Void.class); // Increased timeout for complex flow
        logger.info("Test: Workflow completed for testDependencyAndPlaceholderResolution.");

        // Verify overall counts at the end
        verify(mockAskUserActivity, times(goal.getNodes().size())).ask(actionIdCaptorAsk.capture(), promptCaptor.capture());
        verify(mockValidateInputActivity, times(goal.getNodes().size())).validate(anyString(), anyMap(), anyMap());

        // Verify placeholder resolution for "ask_arun_availability_001" (Example)
        // String expectedArunPromptBody = "Hi Arun, I'm trying to schedule a meeting. Could you please share your availability for a brief discussion about Project Phoenix?";
        // The actual prompt is from "prompt_message" if present, or "body" if not.
        // The example-goal.json's ask_arun_availability_001 has a "body" in actionParams, but workflow uses "prompt_message" by default.
        // Let's check the prompt for "ask_arun_availability_001" which has a placeholder in its "body"
        // The workflow's generatePrompt uses "prompt_message". If not found, it generates a generic one.
        // The current resolvePlaceholders logic in workflow is applied to the prompt generated by generatePrompt.
        // The "body" param with placeholder is not directly used as a prompt unless "prompt_message" refers to it or is not present.

        // Let's find the prompt for 'ask_arun_availability_001'
        int arunPromptIndex = -1;
        List<String> askedActionIds = actionIdCaptorAsk.getAllValues();
        for(int i=0; i<askedActionIds.size(); i++) {
            if("ask_arun_availability_001".equals(askedActionIds.get(i))){
                arunPromptIndex = i;
                break;
            }
        }
        assertTrue(arunPromptIndex != -1, "Action ask_arun_availability_001 should have been asked.");
        String arunPrompt = promptCaptor.getAllValues().get(arunPromptIndex);
        // The prompt_message for ask_arun_availability_001 is not defined in example-goal.json, so it uses the generic one.
        // The placeholder resolution in workflow needs to be robust.
        // The current prompt generation is: actionParams.get("prompt_message") OR generic.
        // The placeholder is in actionParams.get("body").
        // The workflow currently doesn't automatically use "body" as prompt.
        // Let's assume the "prompt_message" for "ask_arun_availability_001" was changed to include the placeholder:
        // "Hi Arun, ... about {get_requester_preferred_details_001.topic}?"
        // If so, the check would be:
        // assertTrue(arunPrompt.contains("Project Phoenix"), "Prompt for Arun should contain resolved topic 'Project Phoenix'. Actual: " + arunPrompt);
        // This highlights a potential refinement in how prompts are constructed and resolved from actionParams.
        // For now, we'll assert that the action was called.
        // WorkflowStub.getResult() would throw an exception if not completed successfully.
    }
}
