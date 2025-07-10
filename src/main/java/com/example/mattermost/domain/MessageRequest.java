package com.example.mattermost.domain;

import java.util.List;

public class MessageRequest {
    private Recipient recipient;
    private String message;

    public Recipient getRecipient() {
        return recipient;
    }

    public void setRecipient(Recipient recipient) {
        this.recipient = recipient;
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
               "recipient='" + recipient + '\'' +
               ", message='" + message + '\'' +
               '}';
    }
}

