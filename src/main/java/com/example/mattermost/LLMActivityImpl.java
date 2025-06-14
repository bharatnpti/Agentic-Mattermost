package com.example.mattermost;

import io.temporal.activity.ActivityMethod; // This was missing in the thought process, but needed for @ActivityMethod
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import java.util.Map;

// @ActivityImpl annotation is usually not required if the class name ends with Impl
// and it's registered with the worker correctly.
// However, it's good practice to ensure clarity, though Temporal SDK might not strictly need it.
// For now, let's proceed without @ActivityImpl as worker registration handles it.
public class LLMActivityImpl implements LLMActivity {

    private static final Logger logger = LoggerFactory.getLogger(LLMActivityImpl.class);

    @Override
    @ActivityMethod // Explicitly marking the activity method
    public boolean isActionComplete(String actionId, Map<String, Object> actionParams, Map<String, Map<String, Object>> currentActionOutputs) {
        logger.info("LLMActivity: Checking if action '{}' can be completed by LLM.", actionId);
        logger.info("LLMActivity: Action params for {}: {}", actionId, actionParams);
        logger.info("LLMActivity: Current overall action outputs for context for {}: {}", actionId, currentActionOutputs);

        if (actionParams != null && Boolean.TRUE.equals(actionParams.get("llm_can_complete"))) {
            logger.info("LLMActivity: Action '{}' has 'llm_can_complete: true'. Marking as complete by LLM.", actionId);
            // Here you could add more sophisticated mock logic,
            // e.g., checking action type or specific parameters to decide.
            // For instance, an action to "summarize text" where text is an input from another action.
            // String textToSummarize = (String) currentActionOutputs.getOrDefault("previousActionId", Map.of()).get("text_to_process");
            // if (textToSummarize != null && !textToSummarize.isEmpty()) { return true; }
            return true;
        }

        if (actionParams != null && "llm_auto_complete_test".equals(actionParams.get("action_type"))) {
            logger.info("LLMActivity: Action '{}' has 'action_type: llm_auto_complete_test'. Marking as complete by LLM.", actionId);
            return true;
        }

        logger.info("LLMActivity: Action '{}' not marked for LLM completion or conditions not met. Returning false.", actionId);
        return false;
    }
}
