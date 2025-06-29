package com.example.mattermost.service.impl;

import com.example.mattermost.domain.model.Goal;
import com.example.mattermost.integration.llm.NlpService;
import com.example.mattermost.service.GoalExtractionService;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Service;

@Service
public class DummyGoalExtractionService implements GoalExtractionService {


    private static final Logger log = LoggerFactory.getLogger(DummyGoalExtractionService.class);

    @Autowired
    private NlpService nlpService;


    @Override
    public Goal extractGoalFromMessage(String message) throws RuntimeException {
        Goal goal = nlpService.createActions(message, null);
//        Goal goal = null;
//        try {
//            goal = new ObjectMapper().readValue(goalString, Goal.class);
//        } catch (JsonProcessingException e) {
//            throw new RuntimeException(e);
//        }
        log.debug("Extracted goal from NLP message: {}", goal);
        return goal;
    }


    private static final String goalString = """
            {
              "goal": "Schedule a meeting with the development team",
              "nodes": [
                {
                  "actionId": "get_meeting_details_001",
                  "actionName": "Get Meeting Details from User",
                  "actionDescription": "Ask the user for the preferred meeting date, time, duration, and topic to schedule a meeting with the development team.",
                  "actionParams": {
                    "required_users": [
                      "requester"
                    ],
                    "prompt_message": "Please provide the preferred date, time, duration, and topic for the meeting with the development team."
                  },
                  "actionStatus": "PENDING"
                },
                {
                  "actionId": "check_development_team_availability_001",
                  "actionName": "Check Development Team Availability",
                  "actionDescription": "Check the calendar availability of the development team members for the proposed meeting time provided by the requester.",
                  "actionParams": {
                    "attendees": [
                      "development team"
                    ],
                    "proposed_date": "{get_meeting_details_001.date}",
                    "proposed_time": "{get_meeting_details_001.time}",
                    "duration": "{get_meeting_details_001.duration}",
                    "timezone": "Asia/Kolkata"
                  },
                  "actionStatus": "PENDING"
                },
                {
                  "actionId": "propose_meeting_time_001",
                  "actionName": "Propose Meeting Time to Requester",
                  "actionDescription": "Propose an optimal meeting time to the requester based on the development team's availability and seek confirmation.",
                  "actionParams": {
                    "recipient": "requester",
                    "subject": "Proposed Meeting Time with Development Team",
                    "body": "Based on the development team's availability, the proposed meeting time is {check_development_team_availability_001.optimal_time}. Does this time work for you?"
                  },
                  "actionStatus": "PENDING"
                },
                {
                  "actionId": "send_meeting_invite_001",
                  "actionName": "Send Meeting Invite",
                  "actionDescription": "Create and send a calendar invite for the meeting to the development team and the requester after final approval of the meeting time.",
                  "actionParams": {
                    "attendees": [
                      "development team",
                      "requester"
                    ],
                    "subject": "{get_meeting_details_001.topic}",
                    "date": "{propose_meeting_time_001.approved_date}",
                    "time": "{propose_meeting_time_001.approved_time}",
                    "duration": "{get_meeting_details_001.duration}",
                    "timezone": "Asia/Kolkata"
                  },
                  "actionStatus": "PENDING"
                }
              ],
              "relationships": [
                {
                  "sourceActionId": "get_meeting_details_001",
                  "targetActionId": "check_development_team_availability_001",
                  "type": "DEPENDS_ON"
                },
                {
                  "sourceActionId": "check_development_team_availability_001",
                  "targetActionId": "propose_meeting_time_001",
                  "type": "DEPENDS_ON"
                },
                {
                  "sourceActionId": "propose_meeting_time_001",
                  "targetActionId": "send_meeting_invite_001",
                  "type": "DEPENDS_ON"
                }
              ],
              "workflowId": "workflow_schedule_meeting_dev_team_001",
              "actionOutputs": {}
            }
            """;
}
