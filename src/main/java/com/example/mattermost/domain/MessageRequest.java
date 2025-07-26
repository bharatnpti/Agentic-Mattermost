package com.example.mattermost.domain;

import com.example.mattermost.integration.mattermost.model.User;

import java.util.List;

public class MessageRequest {
    private MUser user;
    private String message;


    public MUser getUser() {
        return user;
    }

    public void setUser(MUser user) {
        this.user = user;
    }

    public String getMessage() {
        return message;
    }

    public void setMessage(String message) {
        this.message = message;
    }

    @Override
    public String toString() {
        return "MessageRequest{" +
                "user=" + user +
                ", message='" + message + '\'' +
                '}';
    }
}

