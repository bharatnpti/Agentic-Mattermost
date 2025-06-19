package com.example.mattermost;

import io.temporal.activity.ActivityInterface;
import io.temporal.activity.ActivityMethod;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Component;

import java.util.Map;


@Component
public class LLMActivityImpl implements LLMActivity {

    private static final Logger logger = LoggerFactory.getLogger(LLMActivityImpl.class);

    @Autowired
    private NlpService nlpService;

    @Override
    public boolean isActionComplete(String actionId, Map<String, Object> actionParams, Map<String, String> actionOutputs) {
        // Your existing logic here
        return false;
    }

    @Override
    public LLMProcessingResult processActionWithLLM(LLMProcessingRequest request) {

        if(MeetingSchedulerWorkflowImpl.debug) {
            System.out.println("Processing LLM Activity DEBUG");
        }
            // This is where the NLP service call happens - in the activity, not the workflow
            String actionResult = nlpService.executeAction(
                    request.getGoal(),
                    request.getConvHistory(),
                    request.getAction()
            );

            ActionStatus actionStatus = nlpService.determineActionResult(
                    request.getGoal(),
                    request.getAction(),
                    actionResult
            );

        if(MeetingSchedulerWorkflowImpl.debug) {
            System.out.println("Processing LLM Activity DEBUG");
        }

            return new LLMProcessingResult(true, actionResult, actionStatus);

    }

    @Override
    public String evaluateAndProcessUserInput(Goal currentGoal, ActionNode action, String userInput) {
        return nlpService.evaluateAndProcessUserInput(currentGoal, action, userInput);
    }
}