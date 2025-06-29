// User.java
package com.example.mattermost.integration.mattermost.model;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;
import lombok.Data;

import java.util.Map;

@Data
@JsonIgnoreProperties(ignoreUnknown = true) // Ignore fields not defined in this DTO
public class User {
    private String id;
    private long create_at;
    private long update_at;
    private long delete_at;
    private String username;
    private String first_name;
    private String last_name;
    private String nickname;
    private String email;
    private boolean email_verified;
    private String auth_service;
    private String roles;
    private String locale;
    private NotifyProps notify_props;
    private Map<String, Object> props; // Use Map for generic props
    private long last_password_update;
    private long last_picture_update;
    private int failed_attempts;
    private boolean mfa_active;
    private Timezone timezone;
    private String terms_of_service_id;
    private long terms_of_service_create_at;

    @Data
    @JsonIgnoreProperties(ignoreUnknown = true)
    public static class NotifyProps {
        private String email;
        private String push;
        private String desktop;
        private String desktop_sound;
        private String mention_keys;
        private String channel;
        private String first_name;
    }

    @Data
    @JsonIgnoreProperties(ignoreUnknown = true)
    public static class Timezone {
        private boolean useAutomaticTimezone;
        private String manualTimezone;
        private String automaticTimezone;
    }

    @Override
    public String toString() {
        return "User{" +
                "id='" + id + '\'' +
                ", first_name='" + first_name + '\'' +
                ", last_name='" + last_name + '\'' +
                ", nickname='" + nickname + '\'' +
                ", email='" + email + '\'' +
                '}';
    }
}

