package com.example.workflowcontroller;

import com.example.mattermost.Goal;
import com.example.mattermost.MeetingSchedulerAppMain;
import com.example.mattermost.workflow.MeetingSchedulerWorkflow;
import com.fasterxml.jackson.databind.ObjectMapper;
import io.temporal.client.WorkflowClient;
import io.temporal.client.WorkflowOptions;
import io.temporal.client.WorkflowStub;
import org.junit.jupiter.api.Test;
import org.mockito.ArgumentCaptor;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.autoconfigure.web.servlet.WebMvcTest;
import org.springframework.boot.test.mock.mockito.MockBean;
import org.springframework.http.MediaType;
import org.springframework.test.web.servlet.MockMvc;

import java.time.LocalDateTime;
import java.util.Collections;
import java.util.UUID;

import static org.hamcrest.Matchers.containsString;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.*;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.content;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

@WebMvcTest(WorkflowController.class)
public class WorkflowControllerTests {

    @Autowired
    private MockMvc mockMvc;

    @MockBean
    private WorkflowClient workflowClient;

    @Autowired
    private ObjectMapper objectMapper; // Spring Boot provides one, or it can be from TemporalConfig if that's on the classpath for the test

    @Test
    public void testStartWorkflow() throws Exception {
        // Create a sample Goal object
        Goal sampleGoal = new Goal(
                "Test Meeting",
                LocalDateTime.now().plusDays(1),
                30,
                Collections.singletonList("user1@example.com"),
                "Test channel"
        );

        // Mock WorkflowStub
        MeetingSchedulerWorkflow mockedWorkflow = mock(MeetingSchedulerWorkflow.class);
        WorkflowStub mockedWorkflowStub = mock(WorkflowStub.class); // To handle WorkflowClient.start

        // Mock the workflowClient.newWorkflowStub(...) method
        when(workflowClient.newWorkflowStub(
                eq(MeetingSchedulerWorkflow.class),
                any(WorkflowOptions.class)))
                .thenReturn(mockedWorkflow);

        // Perform a POST request and verify
        String response = mockMvc.perform(post("/workflow/start")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content(objectMapper.writeValueAsString(sampleGoal)))
                .andExpect(status().isOk())
                .andExpect(content().string(containsString("Workflow started with ID: MeetingSchedulerWorkflow_")))
                .andReturn().getResponse().getContentAsString();

        // Extract workflowId from response to verify start call (optional, but good for thoroughness)
        String workflowId = response.substring(response.indexOf("MeetingSchedulerWorkflow_"));

        // Verify that WorkflowClient.start was called with the correct arguments
        // We need to capture the Runnable that is passed to WorkflowClient.start
        ArgumentCaptor<Runnable> runnableCaptor = ArgumentCaptor.forClass(Runnable.class);
        verify(workflowClient).start(runnableCaptor.capture());

        // Execute the captured runnable to trigger the actual workflow method call
        runnableCaptor.getValue().run();

        // Now verify that scheduleMeeting was called on the mockedWorkflow with the sampleGoal
        ArgumentCaptor<Goal> goalCaptor = ArgumentCaptor.forClass(Goal.class);
        verify(mockedWorkflow).scheduleMeeting(goalCaptor.capture());

        // Assert that the captured goal is the same as the sampleGoal
        // Note: Goal needs a proper .equals() method for this to work reliably if not the same instance.
        // For now, we'll assume it's okay or check a key field.
        assert sampleGoal.getSubject().equals(goalCaptor.getValue().getSubject());
    }
}
