package com.example.mattermost;

import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import io.temporal.activity.Activity;

public class AskUserActivityImpl implements AskUserActivity {

    private static final Logger logger = LoggerFactory.getLogger(AskUserActivityImpl.class);

    @Override
    public void ask(String actionId, String prompt) {
        // Get current activity context
        String activityId = Activity.getExecutionContext().getInfo().getActivityId();
        String workflowId = Activity.getExecutionContext().getInfo().getWorkflowId();

        logger.info(
            "[Activity: AskUser, WorkflowID: {}, ActivityID: {}] Asking user for actionId: '{}'. Prompt: '{}'",
            workflowId,
            activityId,
            actionId,
            prompt
        );
        // In a real application, this would involve sending a message via Mattermost API
        // and waiting for an external system to call the signal method on the workflow.
        // For this mock, we just log. The signal will be sent manually or by a test.
        System.out.println("**************************************************************************************");
        System.out.println("SIMULATED USER PROMPT (Action ID: " + actionId + ")");
        System.out.println("Prompt: " + prompt);
        System.out.println("To respond, signal the workflow with actionId '" + actionId + "' and your input.");
        System.out.println("e.g., using tctl: tctl workflow signal -w " + workflowId + " -n onUserResponse -i '{"" + actionId + "": {"key": "value"}}'");
        System.out.println("**************************************************************************************");
    }
}
