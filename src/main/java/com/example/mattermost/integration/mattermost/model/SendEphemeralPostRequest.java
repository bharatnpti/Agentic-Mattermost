package com.example.mattermost.integration.mattermost.model;

import lombok.Builder;
import lombok.Data;

@Data
@Builder
public class SendEphemeralPostRequest {
    private String user_id;
    private EphemeralPost post;

    @Data
    @Builder
    public static class EphemeralPost {
        private String channel_id;
        private String message;
    }
}
