package com.example.mattermost.service;

import com.example.mattermost.integration.mattermost.MattermostService;
import io.swagger.v3.oas.annotations.media.Schema;
import org.springframework.ai.chat.model.ToolContext;
import org.springframework.ai.tool.annotation.Tool;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Service;

@Service
public class MeetingInviteTool {

    @Autowired
    private MattermostService mattermostService;

    public record InviteRequest(
        @Schema(description = "Title of the meeting") String title,
        @Schema(description = "List of participants") String participants,
        @Schema(description = "Scheduled time of the meeting") String time
    ) {}

    @Tool(name = "meeting_invite", description = "Generate a meeting invite from title, participants, and time")
    public String meeting_invite(InviteRequest input, ToolContext toolContext) {
        String message = String.format(
            """
            ****I do not have the capability to send meetings invite yet, all participants agreed to below - please send meeting invite manually****
            
            Meeting Invite
            Title: %s\n
            Time: %s\n
            Participants: %s\n
            Please add it to your calendar.""",
            input.title(), input.time(), input.participants()
        );

        mattermostService.askRequestor(message, toolContext);
        return "Invite Sent";
    }
}
