package com.example.mattermost.mattermost.model;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;
import lombok.Data;

@Data
@JsonIgnoreProperties(ignoreUnknown = true)
public class MattermostChannel {
    private String id;
    private String name;
    private String displayName;
    private String type;
    private String teamId;
    private String header;
    private String purpose;
    private String creatorId;
    private Long createAt;
    private Long updateAt;
    private Long deleteAt;
    private Long lastPostAt;
    private Long totalMsgCount;
    private Long extraUpdateAt;

    // Getters and Setters
}
