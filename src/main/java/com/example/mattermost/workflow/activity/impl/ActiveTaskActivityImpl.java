package com.example.mattermost.workflow.activity.impl;

import com.example.mattermost.domain.model.ActionStatus;
import com.example.mattermost.domain.repository.ActiveTaskRepository;
import com.example.mattermost.workflow.activity.ActiveTaskActivity;
import io.temporal.activity.ActivityInterface;
import io.temporal.activity.ActivityMethod;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Component;

import java.util.Map;
import java.util.Optional;


@Component
public class ActiveTaskActivityImpl implements ActiveTaskActivity {

    private static final Logger logger = LoggerFactory.getLogger(ActiveTaskActivityImpl.class);

    @Autowired
    private ActiveTaskRepository activeTaskRepository;

    @Override
    public void updateActiveTask(String actionId, ActionStatus status, String workflowId, String channelId, String userId) {
        logger.info("Empty Updating active task for action {}: {}: {}: {}:", actionId, status, channelId, userId);
//        Optional<ActiveTask> byWorkflowIdAndCurrentActionId = activeTaskRepository.findByWorkflowIdAndCurrentActionId(workflowId, actionId);
//        ActiveTask activeTask;
//        if (byWorkflowIdAndCurrentActionId.isPresent()) {
//            activeTask = byWorkflowIdAndCurrentActionId.get();
//            activeTask.setStatus(status);
//        } else {
//            activeTask = new ActiveTask();
//            activeTask.setWorkflowId(workflowId);
//            activeTask.setCurrentActionId(actionId);
//            activeTask.setStatus(status);
//            activeTask.setChannelId(channelId);
//            activeTask.setUserId(userId);
//        }
//        activeTaskRepository.save(activeTask);
    }
}