package com.example.mattermost.domain.repository;

import com.example.mattermost.domain.model.MessageHistory;
import org.springframework.stereotype.Repository;

import java.util.List;
import java.util.Optional;

@Repository
public interface MessageHistoryRepository extends org.springframework.data.jpa.repository.JpaRepository<MessageHistory, Long> {

    Optional<List<MessageHistory>> findByChildWorkFlowIdOrderByCreatedAtAsc(String childWorkFlowId);

}