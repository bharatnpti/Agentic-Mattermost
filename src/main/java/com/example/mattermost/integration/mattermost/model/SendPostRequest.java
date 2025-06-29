package com.example.mattermost.integration.mattermost.model;

import lombok.Builder;
import lombok.Data;

import java.util.List;
import java.util.Map;

@Data
@Builder
public class SendPostRequest {
    private String channel_id;
    private String message;
    private String root_id;
    private List<String> file_ids;
    private Map<String, Object> props;
    private PostMetadata metadata;

    @Data
    @Builder
    public static class PostMetadata {
        private Priority priority;
    }

    @Data
    @Builder
    public static class Priority {
        private String priority;
        private boolean requested_ack;
    }
}
