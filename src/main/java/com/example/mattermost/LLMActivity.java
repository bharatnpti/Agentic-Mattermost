package com.example.mattermost;

import io.temporal.activity.ActivityInterface;
import java.util.Map;

@ActivityInterface
public interface LLMActivity {

    /**
     * Determines if an action can be considered complete by an LLM.
     * @param actionId The ID of the action being checked.
     * @param actionParams The parameters of the action, which might guide the LLM.
     * @param currentActionOutputs Outputs from other actions that might serve as context.
     * @return true if the action is considered complete by the LLM, false otherwise.
     */
    boolean isActionComplete(String actionId, Map<String, Object> actionParams, Map<String, Map<String, Object>> currentActionOutputs);
}
