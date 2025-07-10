package com.example.mattermost.workflow.activity;

import com.example.mattermost.domain.CurrentContext;
import com.example.mattermost.domain.MessageList;
import com.example.mattermost.domain.MessageRequest;
import com.example.mattermost.domain.model.*;
import io.temporal.activity.ActivityInterface;
import io.temporal.activity.ActivityMethod;
import io.temporal.workflow.Functions;

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
    LLMProcessingResult processActionWithLLM(LLMProcessingRequest request, String currentThreadId, String currentUserId, String currentChannelId);

    String evaluateAndProcessUserInput(Goal currentGoal, ActionNode action, String userInput);

    ActionStatus determineActionType(Goal currentGoal, ActionNode action, String string);

    MessageList formulate_user_message(Goal currentGoal, ActionNode action, String convHistory, String currentThreadId, String channelId, String currentUserId);

    String checkAndAskUser(MessageRequest messageRequest, String convHistory, CurrentContext currentContext);
}