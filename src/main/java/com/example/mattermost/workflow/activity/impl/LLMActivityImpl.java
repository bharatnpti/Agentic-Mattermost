package com.example.mattermost.workflow.activity.impl;

import com.example.mattermost.domain.model.*;
import com.example.mattermost.integration.llm.NlpService;
import com.example.mattermost.workflow.MeetingSchedulerWorkflowImpl;
import com.example.mattermost.workflow.activity.LLMActivity;
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
    public LLMProcessingResult processActionWithLLM(LLMProcessingRequest request, String currentThreadId, String currentUserId, String currentChannelId) {

        if(MeetingSchedulerWorkflowImpl.debug) {
            System.out.println("Processing LLM Activity DEBUG");
        }
            // This is where the NLP service call happens - in the activity, not the workflow
            String actionResult = nlpService.executeAction(
                    request.getGoal(),
                    request.getConvHistory(),
                    request.getAction(),
                    currentThreadId,
                    currentUserId,
                    currentChannelId
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

    @Override
    public ActionStatus determineActionType(Goal currentGoal, ActionNode action, String convHistory) {

        ActionStatus actionStatus = nlpService.determineActionType(
                currentGoal.getGoal(),
                action,
                convHistory
        );
        logger.info("Determined action type: {}", actionStatus);
        return actionStatus;
    }


    @Override
    public String ask_user(Goal currentGoal, ActionNode action, String convHistory, String currentThreadId, String channelId, String currentUserId) {

        String askUser = nlpService.ask_user(
                currentGoal.getGoal(),
                action,
                convHistory,
                currentThreadId,
                channelId,
                currentUserId
        );
        logger.info("Asked user: {}", askUser);
        return askUser;
    }
}