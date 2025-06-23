package com.example.mattermost.mattermost;

import com.example.mattermost.mattermost.model.*;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.boot.web.client.RestTemplateBuilder;
import org.springframework.core.ParameterizedTypeReference;
import org.springframework.http.*;
import org.springframework.stereotype.Service;
import org.springframework.web.client.RestTemplate;

import java.util.List;

@Service
public class MattermostApiClient {


    private static final Logger log = LoggerFactory.getLogger(MattermostApiClient.class);
    private final RestTemplate restTemplate;
    private final String baseUrl = "http://localhost:8065/api/v4";
    private final String bearerToken = "ihjaj3gxmjfetri8so8k396usa"; // Replace with your actual token

    public MattermostApiClient(RestTemplateBuilder restTemplateBuilder) {
        this.restTemplate = restTemplateBuilder.build();
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
}