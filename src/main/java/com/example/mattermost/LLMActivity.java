package com.example.mattermost;

import io.temporal.activity.ActivityInterface;
import io.temporal.activity.ActivityMethod;

import java.util.Map;


@ActivityInterface
public interface LLMActivity {

    @ActivityMethod
    boolean isActionComplete(String actionId, Map<String, Object> actionParams, Map<String, String> actionOutputs);

    /**
     * Process an action using LLM - this moves the NLP service call out of the workflow
     * and into an activity where it belongs
     */
    @ActivityMethod
    LLMProcessingResult processActionWithLLM(LLMProcessingRequest request);

    String evaluateAndProcessUserInput(Goal currentGoal, ActionNode action, String userInput);
}