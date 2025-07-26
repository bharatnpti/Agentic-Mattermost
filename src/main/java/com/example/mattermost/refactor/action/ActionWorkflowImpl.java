//package com.example.mattermost.refactor.action;
//
//import com.example.mattermost.domain.model.ActionNode;
//import io.temporal.activity.ActivityOptions;
//import io.temporal.common.RetryOptions;
//import io.temporal.workflow.Workflow;
//import com.example.mattermost.refactor.model.ActionResult;
//
//import java.time.Duration;
//
//public class ActionWorkflowImpl implements ActionWorkflow {
//
//    private final ActionNode taskActivity = Workflow.newActivityStub(
//        ActionNode.class,
//        ActivityOptions.newBuilder()
//            .setStartToCloseTimeout(Duration.ofMinutes(2))
//            .setRetryOptions(RetryOptions.newBuilder()
//                .setMaximumAttempts(3)
//                .setBackoffCoefficient(2.0)
//                .setInitialInterval(Duration.ofSeconds(10))
//                .build())
//            .build());
//
//    @Override
//    public ActionResult run(ActionNode node) {
//        String actionId = node.getActionId();
//        String actionName = node.getActionName();
//
//        Workflow.getLogger(this.getClass()).info("Executing action: " + actionName);
//
////        switch (actionName) {
////            case "Get Requester's Meeting Details":
////                return handlePromptResponse(node, "requester");
////
////            case "Ask Arun for Availability":
////                return handleMessageSendAndWait(node, "Arun");
////
////            case "Ask Jasbir for Availability":
////                return handleMessageSendAndWait(node, "Jasbir");
////
////            case "Consolidate All Availabilities":
////                String finalTime = taskActivity.consolidateAvailabilities();
////                return new ActionResult(actionId, "COMPLETED", finalTime);
////
////            case "Send Final Meeting Invite":
////                boolean success = taskActivity.sendCalendarInvite(node.getActionParams());
////                return new ActionResult(actionId, success ? "COMPLETED" : "FAILED", null);
////
////            default:
////                return new ActionResult(actionId, "FAILED", "Unknown action");
////        }
//
//        return null;
//    }
//
//    @Override
//    public void submitResponse(ActionNode actionNode, String userInput) {
//
//    }
//
//    private ActionResult handlePromptResponse(ActionNode node, String userId) {
//        String prompt = (String) node.getActionParams().get("prompt_message");
//        taskActivity.sendPromptToUser(userId, prompt);  // e.g. sends via chat or email
//
//        Workflow.await(() -> response != null && !response.trim().isEmpty());
//
//        boolean isValid = taskActivity.validateResponseWithLLM(response);
//        if (!isValid) {
//            return new ActionResult(node.getActionId(), "RETRY", "Response was invalid");
//        }
//
//        return new ActionResult(node.getActionId(), "COMPLETED", response);
//    }
//
//    private ActionResult handleMessageSendAndWait(ActionNode node, String userId) {
//        String message = (String) node.getActionParams().get("body");
//        boolean sent = taskActivity.sendPromptToUser(userId, message);
//
//        if (!sent) {
//            return new ActionResult(node.getActionId(), "FAILED", "Message send failed");
//        }
//
//        Workflow.await(Duration.ofMinutes(60), () -> response != null && !response.trim().isEmpty());
//
//        if (response == null) {
//            return new ActionResult(node.getActionId(), "RETRY", "No response received in time");
//        }
//
//        return new ActionResult(node.getActionId(), "COMPLETED", response);
//    }
//}
