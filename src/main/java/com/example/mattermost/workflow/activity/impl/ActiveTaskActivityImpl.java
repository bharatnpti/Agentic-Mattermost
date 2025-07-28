package com.example.mattermost.workflow.activity.impl;

import com.example.mattermost.domain.model.ActionStatus;
import com.example.mattermost.domain.model.ActiveTask;
import com.example.mattermost.domain.repository.ActiveTaskRepository;
import com.example.mattermost.workflow.activity.ActiveTaskActivity;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Component;

import java.util.List;


@Component
public class ActiveTaskActivityImpl implements ActiveTaskActivity {

    private static final Logger logger = LoggerFactory.getLogger(ActiveTaskActivityImpl.class);

    @Autowired
    private ActiveTaskRepository activeTaskRepository;

    @Override
    public void updateActiveTask(String actionId, ActionStatus status, String workflowId, String channelId, String userId, String currentThreadId) {
        logger.info("updateActiveTask with actionId: {}, status: {}, workflowId: {}, currentThreadId: {}", actionId, status, workflowId, currentThreadId);
        List<ActiveTask> byWorkflowIdAndCurrentActionId = activeTaskRepository.findByWorkflowIdAndCurrentActionIdAndThreadRootId(workflowId, actionId, currentThreadId);
        ActiveTask activeTask;
        if (!byWorkflowIdAndCurrentActionId.isEmpty()) {
            logger.info("Active task already exists: {}", byWorkflowIdAndCurrentActionId);
            byWorkflowIdAndCurrentActionId.forEach(task -> {
//                        task.setWorkflowId(workflowId);
                        task.setStatus(status);
//                        task.setThreadRootId(currentThreadId);
                    }
            );
            byWorkflowIdAndCurrentActionId.forEach(task -> logger.info("updated task list : {}", task.getStatus()));
            activeTaskRepository.saveAll(byWorkflowIdAndCurrentActionId);
        } else {
            logger.info("No active task exists: {}", byWorkflowIdAndCurrentActionId);
            activeTask = new ActiveTask();
            activeTask.setWorkflowId(workflowId);
            activeTask.setCurrentActionId(actionId);
            activeTask.setStatus(status);
            activeTask.setChannelId(channelId);
            activeTask.setUserId(userId);
            activeTask.setThreadRootId(currentThreadId);
            ActiveTask save = activeTaskRepository.save(activeTask);
            logger.info("saved active task : {}", save);
        }

    }
}