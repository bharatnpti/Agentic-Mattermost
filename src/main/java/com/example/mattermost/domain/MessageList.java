package com.example.mattermost.domain;

import java.util.List;

// If you need a class to hold a list of these MessageRequest objects
public class MessageList {
    private List<MessageRequest> messages;

    public List<MessageRequest> getMessages() {
        return messages;
    }

    public void setMessages(List<MessageRequest> messages) {
        this.messages = messages;
    }

    @Override
    public String toString() {
        return "MessageList{" +
               "messages=" + messages +
               '}';
    }
}
