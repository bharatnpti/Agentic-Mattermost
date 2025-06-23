package com.example.mattermost;

import org.springframework.stereotype.Repository;

import java.util.List;
import java.util.Optional;

@Repository
public interface ActiveTaskRepository extends org.springframework.data.jpa.repository.JpaRepository<ActiveTask, Long> {
    Optional<ActiveTask> findByChannelIdAndStatus(String channelId, ActionStatus status);
    List<ActiveTask> findByChannelId(String channelId);
    List<ActiveTask> findByUserId(String userId);
    List<ActiveTask> findByStatus(ActionStatus status);

    Optional<ActiveTask> findByWorkflowIdAndCurrentActionId(String channelId, String currentActionId);

    List<ActiveTask> findByChannelIdAndUserId(String channelId, String userId);

    Optional<ActiveTask> findByChannelIdAndUserIdAndCurrentActionIdAndWorkflowId(String channelId, String userId, String actionId, String workflowId);
}