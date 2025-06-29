package com.example.mattermost.integration.mattermost.model;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;
import lombok.Data;

import java.util.List;
import java.util.Map;

@Data
@JsonIgnoreProperties(ignoreUnknown = true)
public class Post {
    private String id;
    private long create_at;
    private long update_at;
    private long delete_at;
    private long edit_at;
    private String user_id;
    private String channel_id;
    private String root_id;
    private String original_id;
    private String message;
    private String type;
    private Map<String, Object> props;
    private String hashtag;
    private List<String> file_ids;
    private String pending_post_id;
    private PostMetadata metadata;

    @Data
    @JsonIgnoreProperties(ignoreUnknown = true)
    public static class PostMetadata {
        private List<Embed> embeds;
        private List<Emoji> emojis;
        private List<File> files;
        private Map<String, Object> images; // Could be a more specific DTO if structure is known
        private List<Reaction> reactions;
        private Priority priority;
        private List<Acknowledgement> acknowledgements;
    }

    @Data
    @JsonIgnoreProperties(ignoreUnknown = true)
    public static class Embed {
        private String type;
        private String url;
        private Map<String, Object> data;
    }

    @Data
    @JsonIgnoreProperties(ignoreUnknown = true)
    public static class Emoji {
        private String id;
        private String creator_id;
        private String name;
        private long create_at;
        private long update_at;
        private long delete_at;
    }

    @Data
    @JsonIgnoreProperties(ignoreUnknown = true)
    public static class File {
        private String id;
        private String user_id;
        private String post_id;
        private long create_at;
        private long update_at;
        private long delete_at;
        private String name;
        private String extension;
        private long size;
        private String mime_type;
        private int width;
        private int height;
        private boolean has_preview_image;
    }

    @Data
    @JsonIgnoreProperties(ignoreUnknown = true)
    public static class Reaction {
        private String user_id;
        private String post_id;
        private String emoji_name;
        private long create_at;
    }

    @Data
    @JsonIgnoreProperties(ignoreUnknown = true)
    public static class Priority {
        private String priority;
        private boolean requested_ack;
    }

    @Data
    @JsonIgnoreProperties(ignoreUnknown = true)
    public static class Acknowledgement {
        private String user_id;
        private String post_id;
        private long acknowledged_at;
    }
}
