//package com.example.mattermost;
//
//import org.slf4j.Logger;
//import org.slf4j.LoggerFactory;
//import org.springframework.ai.tool.annotation.Tool;
//import org.springframework.beans.factory.annotation.Autowired;
//import org.springframework.stereotype.Service;
//
//@Service
//public class InternalTools {
//
//
//    private static final Logger log = LoggerFactory.getLogger(InternalTools.class);
//
//    @Autowired
//    private ActiveTaskRepository  activeTaskRepository;
//
//    @Tool
//    public void updateAction(String workflowId, String actionId, String channelId) {
//        log.info("Updating action id {} for workflowId {}, {}, {}, {}", actionId, workflowId,  channelId);
//        ActiveTask activeTask = new ActiveTask();
//        activeTask.setWorkflowId(workflowId);
//        activeTask.setCurrentActionId(actionId);
//        activeTask.setChannelId(channelId);
//        activeTask.setStatus(ActionStatus.WAITING_FOR_INPUT);
//        activeTaskRepository.save(activeTask);
//    }
//
//}
