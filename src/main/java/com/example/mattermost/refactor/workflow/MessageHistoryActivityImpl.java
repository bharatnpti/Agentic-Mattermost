package com.example.mattermost.refactor.workflow;

import com.example.mattermost.domain.model.MessageHistory;
import com.example.mattermost.domain.repository.MessageHistoryRepository;
import io.temporal.activity.ActivityInterface;
import io.temporal.activity.ActivityMethod;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Service;

import java.util.ArrayList;
import java.util.List;

@Service
@ActivityInterface
public class MessageHistoryActivityImpl implements MessageHistoryActivity {

    @Autowired
    private MessageHistoryRepository messageHistoryRepository;

    public List<MessageHistory> getMessageHistory(String workflowId) {
        return messageHistoryRepository.findByChildWorkFlowIdOrderByCreatedAtAsc(workflowId)
                .orElseGet(ArrayList::new);
    }

    public MessageHistory save(MessageHistory messageHistory) {
        return messageHistoryRepository.save(messageHistory);
    }
}
