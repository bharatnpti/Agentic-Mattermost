package com.example.mattermost;

import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import io.temporal.activity.Activity;
import java.util.Map;
import java.util.List; // Added missing import

public class ValidateInputActivityImpl implements ValidateInputActivity {

    private static final Logger logger = LoggerFactory.getLogger(ValidateInputActivityImpl.class);

    @Override
    public boolean validate(String actionId, Map<String, Object> userInput, Map<String, Object> actionParams) {
        String activityId = Activity.getExecutionContext().getInfo().getActivityId();
        String workflowId = Activity.getExecutionContext().getInfo().getWorkflowId();

        logger.info(
            "[Activity: ValidateInput, WorkflowID: {}, ActivityID: {}] Validating input for actionId: '{}'. Input: {}, Params: {}",
            workflowId,
            activityId,
            actionId,
            userInput,
            actionParams
        );

        // Mock validation logic:
        // For this PoC, we'll consider input valid if it's not null and not empty.
        // A real LLM validation would be more complex.
        if (userInput == null || userInput.isEmpty()) {
            logger.warn("Validation FAILED for actionId '{}': User input is null or empty.", actionId);
            return false;
        }

        // Example: Check if a required field from actionParams is present in userInput
        // This is a very basic check.
        if (actionParams != null && actionParams.containsKey("required_fields")) {
            try {
                @SuppressWarnings("unchecked")
                List<String> requiredFields = (List<String>) actionParams.get("required_fields");
                for (String field : requiredFields) {
                    if (!userInput.containsKey(field) || userInput.get(field) == null || userInput.get(field).toString().trim().isEmpty()) {
                        logger.warn("Validation FAILED for actionId '{}': Missing required field '{}'.", actionId, field);
                        return false;
                    }
                }
            } catch (ClassCastException e) {
                logger.error("Validation FAILED for actionId '{}': Could not cast 'required_fields' to List<String>. Params: {}", actionId, actionParams, e);
                return false; // Or handle more gracefully
            }
        }

        // Simulate a specific check for the 'get_requester_preferred_details_001' action from the example
        if ("get_requester_preferred_details_001".equals(actionId)) {
            boolean topicPresent = userInput.containsKey("topic") && userInput.get("topic") != null && !userInput.get("topic").toString().trim().isEmpty();
            boolean dateTimePresent = userInput.containsKey("datetime") && userInput.get("datetime") != null && !userInput.get("datetime").toString().trim().isEmpty();
            boolean durationPresent = userInput.containsKey("duration") && userInput.get("duration") != null && !userInput.get("duration").toString().trim().isEmpty();

            if (topicPresent && dateTimePresent && durationPresent) {
                 logger.info("Validation SUCCEEDED for actionId '{}': All required details (topic, datetime, duration) are present.", actionId);
                 return true;
            } else {
                logger.warn("Validation FAILED for actionId '{}': Missing some details. Topic: {}, DateTime: {}, Duration: {}.", actionId, topicPresent, dateTimePresent, durationPresent);
                return false;
            }
        }


        logger.info("Validation SUCCEEDED for actionId '{}' (default mock validation).", actionId);
        return true; // Default to true if no specific checks fail
    }
}
