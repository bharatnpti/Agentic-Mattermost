package com.example.mattermost.mattermost;

import com.example.mattermost.*;
import com.example.mattermost.mattermost.model.MattermostChannel;
import com.example.mattermost.mattermost.model.SendPostRequest;
import com.example.mattermost.mattermost.model.User;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.ai.chat.model.ToolContext;
import org.springframework.ai.tool.annotation.Tool;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Service;

import java.util.Collections;
import java.util.List;
import java.util.Optional;

@Service
public class MattermostService {

    private final static String ownerUserId = "nif1p7emd3yp5kq9tcid8eciay";

    private static final Logger log = LoggerFactory.getLogger(MattermostService.class);
    @Autowired
    private MattermostApiClient mattermostApiClient;

    @Autowired
    private ActiveTaskRepository activeTaskRepository;

    @Autowired
    private ChannelMappingRepository channelMappingRepository;

    @Tool(description = "Get Users List")
    public List<User> getUsersList() {

        List<User> users = mattermostApiClient.getUsers();

        log.info("Get Users List");
        users.forEach(user -> {
            log.info(user.toString());
        });

        return users;
    }

    @Tool(description = "Send Message to a channel")
    public String sendPersonalMessage(String channelId, String userId, String message, ToolContext toolContext) {
        try {
            extracted(channelId, userId, toolContext);
        } catch (Exception e) {
            log.error("Error while sending personal message to channelId: {}, message: {}", channelId, message, e);
        }
        log.info("Send Message to a channel: {}, {}", channelId, message);
        return mattermostApiClient.sendPost(SendPostRequest.builder()
                        .channel_id(channelId)
                        .message(message)
                .build()).toString();
    }

    private void extracted(String channelId, String userId, ToolContext toolContext) {
        String actionId = toolContext.getContext().get("actionId").toString();
        String workflowId = toolContext.getContext().get("workflowId").toString();
        Optional<ActiveTask> byChannelIdAndUserIdAndCurrentActionIdAndWorkflowId = activeTaskRepository.findByChannelIdAndUserIdAndCurrentActionIdAndWorkflowId(channelId, userId, actionId, workflowId);
        log.info("Saving active tasks for channelId: {}, userId: {}", channelId, userId);
        ActiveTask activeTask = byChannelIdAndUserIdAndCurrentActionIdAndWorkflowId.orElseGet(ActiveTask::new);
        activeTask.setChannelId(channelId);
        activeTask.setUserId(userId);
        activeTask.setCurrentActionId(actionId);
        activeTask.setWorkflowId(workflowId);
        activeTask.setStatus(ActionStatus.WAITING_FOR_INPUT);
        activeTaskRepository.save(activeTask);
    }

    private void extracted(String channelId, ToolContext toolContext) {
        String userId = toolContext.getContext().get("currentUserId").toString();
        String actionId = toolContext.getContext().get("actionId").toString();
        String workflowId = toolContext.getContext().get("workflowId").toString();
        Optional<ActiveTask> byChannelIdAndUserIdAndCurrentActionIdAndWorkflowId = activeTaskRepository.findByChannelIdAndUserIdAndCurrentActionIdAndWorkflowId(channelId, userId, actionId, workflowId);
        log.info("Saving active tasks for channelId: {}, userId: {}, present: {}", channelId, userId, byChannelIdAndUserIdAndCurrentActionIdAndWorkflowId.isPresent());
        ActiveTask activeTask = byChannelIdAndUserIdAndCurrentActionIdAndWorkflowId.orElseGet(ActiveTask::new);
        activeTask.setChannelId(channelId);
        activeTask.setUserId(userId);
        activeTask.setCurrentActionId(actionId);
        activeTask.setWorkflowId(workflowId);
        activeTask.setStatus(ActionStatus.WAITING_FOR_INPUT);
        activeTaskRepository.save(activeTask);
    }

    @Tool(description = "Reply or ask requestor")
    public String askRequestor(String message, ToolContext toolContext) {
        String channelId = toolContext.getContext().get("channelId").toString();
        String rootId = toolContext.getContext().get("rootId").toString();
        log.info("Send Message to a Requestor, channelId: {}, rootId: {}, {}", channelId, rootId, message);
        try {
            extracted(channelId, toolContext);
        } catch (Exception e) {
            log.error("Error while sending requestor", e);
        }
        String mattermostPostResponse = mattermostApiClient.sendPost(SendPostRequest.builder()
                .channel_id(channelId)
                .message(message)
                .root_id(rootId)
                .build()).toString();
        log.info("mattermost response from requestor: {}", mattermostPostResponse);
        return mattermostPostResponse;
    }

    @Tool(description = "Create a direct channel with user")
    public MattermostChannel createDirectChannel(String otherUserId) {


        Optional<ChannelMapping> byOwnerUserIdAndOtherUserId = channelMappingRepository.findByOwnerUserIdAndOtherUserId(ownerUserId, otherUserId);
        if (byOwnerUserIdAndOtherUserId.isPresent()) {
            MattermostChannel mattermostChannel = new MattermostChannel();
            mattermostChannel.setCreatorId(ownerUserId);
            mattermostChannel.setId(byOwnerUserIdAndOtherUserId.get().getChannelId());
            log.info("Using existing mattermost channel: {}", mattermostChannel.getId());
            return mattermostChannel;
        }

        MattermostChannel mattermostChannel = mattermostApiClient.createDirectChannel(ownerUserId, otherUserId);

        ChannelMapping channelMapping = new ChannelMapping();
        channelMapping.setOwnerUserId(ownerUserId);
        channelMapping.setOtherUserId(otherUserId);
        channelMapping.setChannelId(mattermostChannel.getId());
        channelMappingRepository.save(channelMapping);

        log.info("Create a direct channel with id: {}", mattermostChannel.getId());
        return mattermostChannel;
    }

    @Tool(description = "Sends meeting invite to a user")
    public String createInviteChannel(String userId, String subject, String timing, String body) {
        MattermostChannel directChannel = createDirectChannel(userId);
        sendPersonalMessage(directChannel.getId(), userId, subject + System.lineSeparator() + timing + System.lineSeparator() + body, new ToolContext(Collections.emptyMap()));
        log.info("Meeting invite sent to channel: {}, user: {}", directChannel.getId(), userId);
        return "Invite Sent";
    }

}


//2025-06-25T20:25:01.878+05:30  INFO 2279 --- [nio-8080-exec-1] c.example.mattermost.WorkflowController  : Received message from channelId: bjh9m1uny7nx9qrttgqfiqndgc, userId: nfe84kf3utrf8gwwcx8kcfnd3a, with content: 'want to schedule a meeting with arun'

