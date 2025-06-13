package com.example.mattermost;

import io.temporal.activity.ActivityInterface;
import io.temporal.activity.ActivityMethod;
import java.util.Map;

@ActivityInterface
public interface ValidateInputActivity {
    @ActivityMethod
    boolean validate(String actionId, Map<String, Object> userInput, Map<String, Object> actionParams);
}
