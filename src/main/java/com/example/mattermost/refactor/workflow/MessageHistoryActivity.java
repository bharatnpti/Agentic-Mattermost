package com.example.mattermost.refactor.workflow;

import com.example.mattermost.domain.model.MessageHistory;
import io.temporal.activity.ActivityInterface;
import io.temporal.activity.ActivityMethod;

import java.util.List;

@ActivityInterface
public interface MessageHistoryActivity {

    @ActivityMethod
    public List<MessageHistory> getMessageHistory(String workflowId);

    @ActivityMethod
    public MessageHistory save(MessageHistory messageHistory);

    }
