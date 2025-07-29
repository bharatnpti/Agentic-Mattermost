package com.example.mattermost.workflow.activity.impl;

import com.example.mattermost.domain.CurrentContext;
import com.example.mattermost.domain.MessageList;
import com.example.mattermost.domain.MessageRequest;
import com.example.mattermost.domain.model.*;
import com.example.mattermost.integration.llm.NlpService;
import com.example.mattermost.integration.mattermost.MattermostService;
import com.example.mattermost.workflow.activity.LLMActivity;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.ai.chat.model.ToolContext;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Component;

import java.util.Map;
import java.util.Objects;


@Component
public class LLMActivityImpl implements LLMActivity {

    private static final Logger logger = LoggerFactory.getLogger(LLMActivityImpl.class);

    @Autowired
    private NlpService nlpService;

    @Autowired
    private MattermostService mattermostService;

    @Override
    public boolean isActionComplete(String actionId, Map<String, Object> actionParams, Map<String, String> actionOutputs) {
        // Your existing logic here
        return false;
    }

    public LLMProcessingResult processActionWithLLM(LLMProcessingRequest request, String currentThreadId, String currentUserId, String currentChannelId) {

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


        return new LLMProcessingResult(true, actionResult, actionStatus);

    }

    @Override
    public String evaluateAndProcessUserInput(Goal currentGoal, ActionNode action, String userInput) {
        return nlpService.evaluateAndProcessUserInput(currentGoal, action, userInput);
    }

    public ActionStatus determineActionType(Goal currentGoal, ActionNode action,  String convHistory) {

        ActionStatus actionStatus = nlpService.determineActionType(
                currentGoal.getGoal(),
                action
        );
        logger.info("Determined action type: {}", actionStatus);
        return actionStatus;
    }

    @Override
    public ActionStatus determineActionType(CurrentContext context) {
        logger.info("Determine action type for action: {}", context.getCurrentActionNode().getActionId());
        ActionStatus actionStatus = nlpService.determineActionType(
                context.getGoal().getGoal(),
                context.getCurrentActionNode()
        );
        logger.info("Determined action type: {}", actionStatus);
        return actionStatus;
    }


    public MessageList formulate_user_message(Goal currentGoal, ActionNode action, String convHistory, String currentThreadId, String channelId, String currentUserId) {

        MessageList askUser = nlpService.formulate_user_message(
                currentGoal.getGoal(),
                action,
                convHistory,
                currentThreadId,
                channelId,
                currentUserId
        );
        logger.info("Ask user: {}", askUser);
        return askUser;
    }

    @Override
    public MessageList formulate_user_message(CurrentContext context) {

        MessageList askUser = nlpService.formulate_user_message(context);
        logger.info("Ask user: {}", askUser);
        return askUser;
    }

//    public String checkAndAskUser(MessageRequest messageRequest, String convHistory, CurrentContext currentContext) {
//        String checkAndAskUser = nlpService.checkAndAskUser(messageRequest, currentContext.getCurrentActionNode(), convHistory, currentContext.getCurrentThreadId(), currentContext.getCurrentChannelId(), currentContext.getCurrentUserId());
//        if("QUESTION".equalsIgnoreCase(checkAndAskUser) && messageRequest.getRecipient() == Recipient.REQUESTOR) {
//            ActionNode actionNode = currentContext.getCurrentActionNode();
//            Map<String, Object> toolContext = Map.of(
//                    "workflowId", actionNode.getWorkflowId(),
//                    "actionId", actionNode.getActionId(),
//                    "rootId", currentContext.getCurrentThreadId(),
//                    "channelId", currentContext.getCurrentChannelId(),
//                    "currentUserId", currentContext.getUser().getId(),
//                    "currentUser", currentContext.getUser().getFirst_name()
//            );
//            ToolContext toolContext1 = new ToolContext(toolContext);
//
//            mattermostService.askRequestor( messageRequest.getMessage(),
//                    toolContext1
//                    );
//        } else if("QUESTION".equalsIgnoreCase(checkAndAskUser)) {
//            nlpService.askUser( messageRequest,
//                    currentContext.getCurrentActionNode(),
//                    currentContext.getCurrentThreadId(),
//                    currentContext.getCurrentChannelId(),
//                    currentContext.getUser().getId());
//        }
//        return "";
//    }

    @Override
    public String checkAndAskUser(MessageRequest messageRequest, CurrentContext currentContext) {
        String checkAndAskUser = nlpService.checkAndAskUser(messageRequest, currentContext);
        ActionNode actionNode = currentContext.getCurrentActionNode();
        if("QUESTION".equalsIgnoreCase(checkAndAskUser) && Objects.equals(messageRequest.getUser().getId(), currentContext.getUser().getId())) {
            actionNode.setActionResponse("BOT: to " + messageRequest.getUser().getUsername() + ": " + messageRequest.getMessage());
            Map<String, Object> toolContext = Map.of(
                    "context", currentContext,
                    "workflowId", actionNode.getWorkflowId(),
                    "actionId", actionNode.getActionId(),
                    "rootId", currentContext.getCurrentThreadId(),
                    "channelId", currentContext.getCurrentChannelId(),
                    "currentUserId", currentContext.getUser().getId()
            );
            ToolContext toolContext1 = new ToolContext(toolContext);
            mattermostService.askRequestor( messageRequest.getMessage(),
                    toolContext1
            );
        } else if("QUESTION".equalsIgnoreCase(checkAndAskUser)) {
            nlpService.askUser(currentContext, messageRequest,
                    currentContext.getCurrentActionNode(),
                    currentContext.getCurrentThreadId(),
                    currentContext.getCurrentChannelId(),
                    currentContext.getUser().getId());
        }
        return "";
    }

    @Override
    public LLMProcessingResult processActionWithLLM(CurrentContext context) {

        // This is where the NLP service call happens - in the activity, not the workflow
        String actionResult = nlpService.executeAction(context);

        context.getCurrentActionNode().setActionResponse(actionResult);

        ActionStatus actionStatus = nlpService.determineActionResult(context);


        return new LLMProcessingResult(true, actionResult, actionStatus);

    }

    @Override
    public String summarize(CurrentContext context) {
        return nlpService.summarizeActionResponse(context);
    }
}