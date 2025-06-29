package com.example.mattermost.service;

import com.example.mattermost.domain.model.Goal;
import org.springframework.stereotype.Service;

public interface GoalExtractionService {
    Goal extractGoalFromMessage(String message);
}


