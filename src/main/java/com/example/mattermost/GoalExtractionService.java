package com.example.mattermost;

import org.springframework.stereotype.Service;

interface GoalExtractionService {
    Goal extractGoalFromMessage(String message);
}


