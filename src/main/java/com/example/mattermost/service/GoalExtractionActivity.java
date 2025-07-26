package com.example.mattermost.service;

import com.example.mattermost.domain.model.Goal;
import io.temporal.activity.ActivityInterface;
import io.temporal.activity.ActivityMethod;

@ActivityInterface
public interface GoalExtractionActivity {

    @ActivityMethod
    Goal extractGoalFromMessage(String message);
}


