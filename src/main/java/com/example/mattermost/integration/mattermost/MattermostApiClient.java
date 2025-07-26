package com.example.mattermost.integration.mattermost;

import com.example.mattermost.integration.mattermost.model.*;
import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.boot.web.client.RestTemplateBuilder;
import org.springframework.core.ParameterizedTypeReference;
import org.springframework.http.*;
import org.springframework.stereotype.Service;
import org.springframework.util.StringUtils;
import org.springframework.web.client.RestTemplate;

import java.util.Collections;
import java.util.List;

@Service
public class MattermostApiClient {
    private static final Logger log = LoggerFactory.getLogger(MattermostApiClient.class);
    private final RestTemplate restTemplate;
    private String baseUrl = "http://localhost:8065/api/v4";
    private String bearerToken = "tfad7xwue7dqbqtw1imn3q8m9a"; // Replace with your actual token

    public MattermostApiClient(RestTemplateBuilder restTemplateBuilder) {
        this.restTemplate = restTemplateBuilder.build();
//        String mattermostHost = System.getenv("MATTERMOST_HOST");
//        if(mattermostHost != null || !StringUtils.isEmpty(mattermostHost)) {
//            baseUrl = mattermostHost;
//        }
//        bearerToken = System.getenv("MATTERMOST_TOKEN");
        log.info("MattermostApiClient baseUrl: {}, bearerToken: {}", baseUrl, bearerToken);
    }

    private HttpHeaders createHeaders() {
        HttpHeaders headers = new HttpHeaders();
        headers.setBearerAuth(bearerToken);
        headers.set(HttpHeaders.ACCEPT, MediaType.APPLICATION_JSON_VALUE);
        return headers;
    }

    /**
     * 1. Get Users
     * curl --request GET
     * --url http://localhost:8065/api/v4/users
     * --header 'Accept: application/json'
     * --header 'Authorization: Bearer 123'
     */
    public List<User> getUsers() {
        String url = baseUrl + "/users";
        HttpHeaders headers = createHeaders();
        HttpEntity<Void> entity = new HttpEntity<>(headers);

        ResponseEntity<List<User>> response = restTemplate.exchange(
                url,
                HttpMethod.GET,
                entity,
                new ParameterizedTypeReference<List<User>>() {}
        );
        return response.getBody();
    }

    /**
     * 2. Send Post
     * curl --request POST
     * --url http://localhost:8065/api/v4/posts
     * --header 'Accept: application/json'
     * --header 'Authorization: Bearer 123'
     * --header 'Content-Type: application/json'
     * --data '{ "channel_id": "string", "message": "string", "root_id": "string", "file_ids": [ "string" ], "props": {}, "metadata": { "priority": { "priority": "string", "requested_ack": true } } }'
     */
    public Post sendPost(SendPostRequest request) {
        String url = baseUrl + "/posts";
        HttpHeaders headers = createHeaders();
        headers.setContentType(MediaType.APPLICATION_JSON);
        HttpEntity<SendPostRequest> entity = new HttpEntity<>(request, headers);

        ResponseEntity<Post> response = restTemplate.exchange(
                url,
                HttpMethod.POST,
                entity,
                Post.class
        );

//        log.info("Send post request: {}", response);

        return response.getBody();
    }

    /**
     * 3. Send Ephemeral Post
     * curl --request POST
     * --url http://localhost:8065/api/v4/posts/ephemeral
     * --header 'Accept: application/json'
     * --header 'Authorization: Bearer 123'
     * --header 'Content-Type: application/json'
     * --data '{ "user_id": "string", "post": { "channel_id": "string", "message": "string" } }'
     */
    public Post sendEphemeralPost(SendEphemeralPostRequest request) {
        return null;
    }

    public MattermostChannel createDirectChannel(String selfUserId, String otherUserId) {
        String url = baseUrl + "/channels/direct";
        HttpHeaders headers = createHeaders();
        headers.setContentType(MediaType.APPLICATION_JSON);
        HttpEntity<List<String>> entity = new HttpEntity<>(List.of(selfUserId, otherUserId), headers);

        ResponseEntity<MattermostChannel> response = restTemplate.exchange(
                url,
                HttpMethod.POST,
                entity,
                MattermostChannel.class
        );
        return response.getBody();
    }

    public User getUserById(String id) {
        String url = baseUrl + "/users/" + id;
        HttpHeaders headers = createHeaders();
        HttpEntity<Void> entity = new HttpEntity<>(headers);


        ResponseEntity<User> response = restTemplate.exchange(
                url,
                HttpMethod.GET,
                entity,
                User.class
        );

        log.info("User: {}", response.getBody());

//        User user = null;
//        try {
//            user = new ObjectMapper().readValue(response.getBody(), User.class);
//        } catch (JsonProcessingException e) {
//            throw new RuntimeException(e);
//        }

        return response.getBody();
    }
}

//curl --request GET \
//        --url http://localhost:8065/api/v4/users \
//        --header 'Accept: application/json' \
//        --header 'Authorization: Bearer 6da8ysb4zjbi3d74g3aygnqyty'