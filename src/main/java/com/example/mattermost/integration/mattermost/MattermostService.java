package com.example.mattermost.integration.mattermost;

import com.example.mattermost.domain.CurrentContext;
import com.example.mattermost.domain.model.*;
import com.example.mattermost.domain.repository.ActiveTaskRepository;
import com.example.mattermost.domain.repository.ChannelMappingRepository;
import com.example.mattermost.domain.repository.MessageHistoryRepository;
import com.example.mattermost.integration.mattermost.model.MattermostChannel;
import com.example.mattermost.integration.mattermost.model.Post;
import com.example.mattermost.integration.mattermost.model.SendPostRequest;
import com.example.mattermost.integration.mattermost.model.User;
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

    private static String ownerUserId = "4bozi1ch8pdi9fowf8j8isbjxh";

    private static final Logger log = LoggerFactory.getLogger(MattermostService.class);
    @Autowired
    private MattermostApiClient mattermostApiClient;

    @Autowired
    private ActiveTaskRepository activeTaskRepository;

    @Autowired
    private ChannelMappingRepository channelMappingRepository;

    @Autowired
    private MessageHistoryRepository messageHistoryRepository;

    public MattermostService() {
        ownerUserId = System.getenv("MATTERMOST_BOT_ID");
        log.debug("Mattermost service initializing with id: {}", ownerUserId);
    }

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

        log.info("Send Message to a channel: {}, user: {}, message: {}", channelId, userId, message);
        Post post = mattermostApiClient.sendPost(SendPostRequest.builder()
                .channel_id(channelId)
                .message(message)
                .build());
        try {
            extracted(channelId, userId, toolContext, post);
            MessageHistory messageHistory = new MessageHistory();
            messageHistory.setMessage("Assistant: " + System.lineSeparator() + message);
            messageHistory.setChildWorkFlowId(toolContext.getContext().get("workflowId").toString());
            messageHistory.setUserName("Assistant");
            messageHistoryRepository.save(messageHistory);
        } catch (Exception e) {
            log.error("Error while sending personal message to channelId: {}, message: {}", channelId, message, e);
        }
        return post.toString();
    }

    private void extracted(String channelId, String userId, ToolContext toolContext, Post post) {
        CurrentContext context = (CurrentContext) toolContext.getContext().get("context");
        String actionId = toolContext.getContext().get("actionId").toString();
        String workflowId = toolContext.getContext().get("workflowId").toString();
        Optional<ActiveTask> byChannelIdAndUserIdAndCurrentActionIdAndWorkflowId = activeTaskRepository.findByChannelIdAndUserIdAndCurrentActionIdAndWorkflowId(channelId, userId, actionId, workflowId);
        log.info("Saving active tasks for channelId: {}, userId: {}", channelId, userId);
        ActiveTask activeTask = byChannelIdAndUserIdAndCurrentActionIdAndWorkflowId.orElseGet(ActiveTask::new);
        log.info("Retrieved active tasks for channelId: {}, userId: {}, is: {}", channelId, userId, activeTask);
        log.info("Post Details:  {}", post);
        activeTask.setChannelId(channelId);
        activeTask.setUserId(userId);
        activeTask.setCurrentActionId(actionId);
        activeTask.setWorkflowId(workflowId);
        activeTask.setStatus(ActionStatus.WAITING_FOR_INPUT);
        String rootId = post.getRoot_id();
        if(rootId == null || rootId.isEmpty()) {
            rootId = post.getId();
        }
        context.setCurrentThreadId(rootId);
        activeTask.setThreadRootId(rootId);
        ActiveTask save = activeTaskRepository.save(activeTask);
        log.info("Saved action : {}", save);
    }

    private void extractedRequestor(String channelId, ToolContext toolContext, Post post) {
        CurrentContext context = (CurrentContext) toolContext.getContext().get("context");
        String rootId = post.getRoot_id();
        if(rootId == null || rootId.isEmpty()) {
            rootId = post.getId();
        }
        context.setCurrentThreadId(rootId);
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
        activeTask.setThreadRootId(rootId);
        activeTaskRepository.save(activeTask);
    }

    @Tool(description = "Reply or ask requestor")
    public String askRequestor(String message, ToolContext toolContext) {
        String channelId = toolContext.getContext().get("channelId").toString();
        String rootId = toolContext.getContext().get("rootId").toString();
        log.info("Send Message to a Requestor, channelId: {}, rootId: {}, {}", channelId, rootId, message);
        Post post = mattermostApiClient.sendPost(SendPostRequest.builder()
                .channel_id(channelId)
                .message(message)
                .root_id(rootId)
                .build());

        extractedRequestor(channelId, toolContext, post);
        MessageHistory messageHistory = new MessageHistory();
        messageHistory.setMessage("Assistant: " + System.lineSeparator() + message);
        messageHistory.setChildWorkFlowId(toolContext.getContext().get("workflowId").toString());
        messageHistory.setUserName("Assistant");
        messageHistoryRepository.save(messageHistory);

        String mattermostPostResponse = post.toString();
        log.info("mattermost response from requestor: {}", mattermostPostResponse);
        return mattermostPostResponse;
    }

    @Tool(description = "Create a direct channel with user")
    public MattermostChannel createDirectChannel(String otherUserId) {
        log.info("Create a direct channel with user: {}", otherUserId);
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

    public User getUserById(String id) {
        return mattermostApiClient.getUserById(id);
    }

}
