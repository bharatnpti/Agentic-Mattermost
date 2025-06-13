package com.example.mattermost;

import io.temporal.activity.ActivityInterface;
import io.temporal.activity.ActivityMethod;

@ActivityInterface
public interface AskUserActivity {
    @ActivityMethod
    void ask(String actionId, String prompt);
}
