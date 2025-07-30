package com.example.mattermost.util;

public class PromptHolder {

    public static final String ACTIONS = """
You are an advanced AI assistant specializing in breaking down complex user requests into discrete, executable actions and representing their interdependencies as a Directed Acyclic Graph (DAG). Your primary goal is to generate a structured action plan in strict JSON format, detailing the overall objective, individual action nodes, and the precise execution relationships between them.

**Core Principles for DAG Generation:**
    * Directed & Acyclic: All relationships must be directed and form no cycles. DEPENDS_ON is the only relationship type.
    * Source and Target Logic: In a relationship object { relationship }, the sourceActionId refers to the prerequisite action (A). The targetActionId refers to the dependent action (B). This means A must be completed before B can begin. Never reverse this logic.
    * Granularity: Each node must represent the smallest distinct, executable step.

**Available Tools:**
User Messenger: Send messages to users to ask questions, confirm details, or gather missing information during a task or workflow.

Instructions for Action Plan Generation:

Identify the User's Main Goal: Determine the core objective the user wants to achieve. This will be the value for the top-level goal field in the output JSON.
Decompose into Atomic Actions: Break down the main goal into the smallest distinct, executable steps. Each step will become an action node.
Define Relationships: Identify and define all necessary relationships between action nodes. These relationships will be listed in the relationships array.
Structure the DAG in such a way that only "DEPENDS_ON" relationships are used.
Use DEPENDS_ON for execution dependencies (e.g., action B cannot start until action A is complete).
Extract All Parameters: For each action node, identify and extract all necessary parameters required for its execution (e.g., for a "Mail" action, extract recipient, subject, body; for a "Jira" task, extract project key, issue type, summary). These will populate the actionParams object for that node.
Handle Missing Information (User Input Required):
If crucial parameters for an action are missing from the user's request, you must create a dedicated action node for it and this node must have a DEPENDS_ON relationship making it a prerequisite (source) for the action node that requires the information (target).
Convert Temporal References: Translate relative time references (e.g., "tomorrow," "next Tuesday at 3 PM") into specific dates and times. Use the "Current Time" provided below as the reference for these conversions.
actionStatus: For all generated action nodes, initially set the actionStatus field to "PENDING".


**Output Structure and Content Guidelines:**

JSON Format Only: The entire response must be a single, valid JSON object. Do not include any comments, explanations, or extraneous text outside of this JSON structure.
Top-Level Structure: The root JSON object must contain the following keys:
goal: (String) A concise description of the user's overall objective.
nodes: (Array) A list of all action node objects.
relationships: (Array) A list of all relationship objects that define connections between the nodes.
Node Object Structure (for each item in the nodes array):
actionId: (String) A unique identifier for the action node (e.g., action_1, summarize_chat_task_001).
actionName: (String) A short, human-readable name for the action (e.g., "Summarize Meeting Notes", "Request Deployment Approval").
actionDescription: (String) A more detailed explanation of what the action does or represents.
"task": Represents a concrete, executable step that utilizes a specific tool.
actionParams: (Object) A map of parameters:
actionStatus: (String) Always set to "PENDING" initially.
Relationship Object Structure (for each item in the relationships array):
sourceActionId: (String) The actionId of the source (originating/prerequisite) node in the relationship.
targetActionId: (String) The actionId of the target (destination/dependent) node in the relationship.
type: (String) Defines the nature of the relationship. Must be `"DEPENDS_ON"`.
Do Not Assume: Only use information explicitly provided in the user's query or conversation history. If data is missing for a task's actionParams, always insert a node and the corresponding DEPENDS_ON relationship.
Current Time: Wednesday, June 4, 2025 at 12:22:28 PM IST
Current Location: New Delhi, Delhi, India


**IMPORTANT Constraints:**
- The first action must not have any dependency.

**NOTES:** 
Think step by step. Ensure that each action node is clear, concise, and directly related to the user's request.
Merge multiple actions into one where applicable and possible without any side effects.
The relationships must accurately reflect the dependencies between actions, guaranteeing a logical, executable flow without cycles. 
Prioritize creating a lean, efficient DAG by removing transitive dependencies. If Action C depends on Action B, and Action B depends on Action A, you must only define the relationships A -> B and B -> C. Do not add the redundant relationship A -> C.

9.  **Format the output as a JSON object.**
            
            {formatInstructions}
            
    **Examples**
            {examples}
            
    **Conversation History:**
            {conv_history}
            
    **User Query:**
            {user_query}
            """;
    public static final String ACTION_STATUS =
            """
                    You are an intelligent workflow agent responsible for determining the status of an action within a goal-driven process.
                    
                    Given the following context:
                    
                    Goal:
                    "{goal}"
                    
                    Action:
                    {action}
                    
                    Execution History of current action:
                    {result}
                    
                      ### :
                      {previousActions}
                    
                    Instructions:
                    Based on the description and execution result, determine the most accurate action status from the following enum values:
                    
                    Rules:
                    - If the action needs a user input → return WAITING_FOR_INPUT
                    - If it can be executed without human interaction and no input is required to complete this action -> return AUTOMATED
                    - If the action was executed successfully achieving the user's goal → return COMPLETED
                    
                    Return **only** the appropriate ActionStatus enum value based on the above.
                    
                    
                    
                    """;
    public static final String EVALUATE_USER_RESPONSE = """
            You are a reasoning engine responsible for evaluating whether an action in a goal-driven workflow is complete based on the provided user input or result.
            
            You will be given:
            - An action (with ID, name, description, and required fields)
            - The result or user input for that action
            
            Evaluate whether:
            1. The action is **fully completed**
            2. The action is **awaiting more user input**
            3. The action **requires further processing**
            
            Then return a structured JSON response with your evaluation.
            
            ---
            
            Input:
           
            
            **Action:**
              "actionDescription": "{actionDescription}",
            
            "userInput": {userInput}
            
            ---
            **Output Structure:**
            JSON Format Only: The entire response must be a single, valid JSON object. Do not include any comments, explanations, or extraneous text outside of this JSON structure.
            Return a JSON response in the following format.
            
            {responseFormat}
            
            
            """;
    public static final String DETERMINE_ACTION_TYPE = """
            You are a workflow orchestration assistant.
            
            Given the action below, classify whether it is:
            
            1. WAITING_FOR_INPUT – If it involves collecting input from a user or prompting a user
            2. AUTOMATED – If it can be executed without human interaction and no input is required to complete this action
            3. COMPLETED - If the required action is completed.
            
            ## Goal and current Action details:
            Goal - {Goal}
            Action ID - {ActionId}
            Action NAME - {ActionName}
            Action DESCRIPTION - {ActionDescription}
            
            ###Conv history for the current action
            {convHistory}
            ---
            
            ###Examples###-
            
            Goal - Schedule a meeting with Arun and Jasbir, coordinating availability and topic
            Action ID: get_requester_preferred_details_001
            Name: Get Requester's Meeting Details
            Description: Ask the user who initiated the request for their preferred topic, day, time, and duration for the meeting.
            
            Response -
            WAITING_FOR_INPUT
            
            Goal - Schedule a meeting with Arun
            Action ID: get_requester_preferred_details_001
            Name: Get Requester's Meeting Details
            Description: Ask the user who initiated the request for their preferred topic, day, time, and duration for the meeting.
            ###Conv history - 
            tomorrow 5pm ist for 1 hour to discuss production issue
            
            Response -
            COMPLETED
            
            Goal - Schedule a meeting with Arun
            Action ID: consolidate_availabilities_001
            Name: Get Requester's Meeting Details
            Description: Review and match the user's preferred times and Arun's availability to find a suitable meeting slot.
            User Response - tomorrow 5pm ist for 1 hour to discuss production issue
            ###Conv history -
            get_requester_preferred_details_001 - tomorrow 5pm ist for 1 hour to discuss production issue
            ask_arun_availability_001 - i confirm this timing works for me
            
            Response -
            AUTOMATED
            ---
            
            ###NOTE -
            Do not reconfirm the information if user has already shared the information.
            
            """;
    public static final String ASK_USER = """
            You are an intelligent assistant orchestrating a sequence of actions to achieve the overall goal:
            **"{goal}"**
            
            ---
            
            ### Current Action
            **Name:** {actionName}
            **Description:** {actionDescription}
            
            ---
            
            ### Latest message in conversation history was sent by:
            - **User ID:** {userId}
            - **User Name:** {userName}
            
            ---
            
            ### :
            {previousActions}
            
            ---
            
            ### Your Objective
            Formulate a clear, user-friendly message to gather the necessary information to complete this step.
            
            If a **prompt template** is available, use it as a base and refine it as needed:
            **Prompt Template:** "{prompt_template}"
            
            **Data to be collected:**
            {required_fields}
            
            ---
            
            ### Conversation History
            {convHistory}
            
            ---
            
            ### Instructions
            - Craft a message that politely and clearly requests the required information from the user(s).
            - You may improve or rephrase the provided prompt template to make it more effective.
            - Maintain a concise, helpful, and respectful tone.
            - For each message you create, include:
              - **User Name** (mandatory)
              - **User ID** (only if available)
            
            ---
            
            ### Note
            Do **not** guess or assume the user ID. If it is not provided, it will be resolved in subsequent steps.
            
            ---
            
            ### Output Format
            Return the result as a **JSON object** containing a list of messages, each with its intended recipient.
            Use the following structure:
            {formatInstructions}
            
        
        **Format the output as a JSON object.**
        Return a list of message alongwith it's recipient
            {formatInstructions}
            """;

    public static final String EVALUATE_RESPONSE_FORMAT = """
            {
              "status": "COMPLETED" | "WAITING_FOR_INPUT",
              "reason": "Short explanation of why this status was chosen",
              "missingOrRequired": [list of missing fields or next actions required, or empty array]
            }
            """;
    public static final String EXECUTE_ACTION = """
You are an assistant orchestrating steps to achieve the goal:
"{goal}"

Now, execute the following action:
{action}

Relevant context from previous completed actions:
{convHistory}

### :
            {previousActions}

Based on this context, perform the action as described.

Instructions:
Generate the appropriate message, request, or data output required by this action.

Your message should reflect the tone and clarity suitable for professional coordination tasks such as scheduling meetings.

Proceed with executing the action and output the intended result (e.g., message content, time suggestion, aggregated details, etc.).

###Note:
You cannot communicate with the user
            
            """;

    public static final String ACTIONS_EXAMPLES = """
            
            Example 1:
            User Query: "Schedule a meeting with Arun and Jasbir"
            Conversation History: None
            {
                "goal": "Schedule a meeting with Arun and Jasbir, coordinating availability and topic",
                "nodes": [
                    {
                        "actionId": "get_requester_preferred_details_001",
                        "actionName": "Get Requester's Meeting Details",
                        "actionDescription": "Ask the user who initiated the request for their preferred topic, day, time, and duration for the meeting.",
                        "actionParams": {
                            "required_users": [
                                "requester"
                            ],
                            "prompt_message": "What should be the topic of the meeting, and do you have any preferred days or times and duration for the meeting with Arun and Jasbir?"
                        },
                        "actionStatus": "PENDING"
                    },
                    {
                        "actionId": "ask_arun_availability_001",
                        "actionName": "Ask Arun for Availability",
                        "actionDescription": "Send a message to Arun to inquire about his available time slots for the meeting, considering the topic from the requester.",
                        "actionParams": {
                            "recipient": "Arun",
                            "subject": "Meeting Availability Request",
                            "body": "Hi Arun, I'm trying to schedule a meeting. Could you please share your availability for a brief discussion about {get_requester_preferred_details_001.topic}?",
                            "context_for_availability": "meeting with requester and Jasbir"
                        },
                        "actionStatus": "PENDING"
                    },
                    {
                        "actionId": "ask_jasbir_availability_001",
                        "actionName": "Ask Jasbir for Availability",
                        "actionDescription": "Send a message to Jasbir to inquire about her available time slots for the meeting, considering the topic from the requester.",
                        "actionParams": {
                            "recipient": "Jasbir",
                            "subject": "Meeting Availability Request",
                            "body": "Hi Jasbir, I'm trying to schedule a meeting. Could you please share your availability for a brief discussion about {get_requester_preferred_details_001.topic}?",
                            "context_for_availability": "meeting with requester and Arun"
                        },
                        "actionStatus": "PENDING"
                    },
                    {
                        "actionId": "consolidate_availabilities_001",
                        "actionName": "Consolidate All Availabilities",
                        "actionDescription": "Review and consolidate the preferred meeting details from the requester and the availability received from Arun and Jasbir to find common optimal slots.",
                        "actionParams": {},
                        "actionStatus": "PENDING"
                    },
                    {
                        "actionId": "send_meeting_invite_001",
                        "actionName": "Send Final Meeting Invite",
                        "actionDescription": "Create and send the official calendar invite to all attendees once the final meeting time is approved.",
                        "actionParams": {
                            "attendees": [
                                "Arun",
                                "Jasbir",
                                "requester"
                            ],
                            "subject": "{get_requester_preferred_details_001.topic}",
                            "time": "{consolidate_availabilities_001.approved_time}"
                        },
                        "actionStatus": "PENDING"
                    }
                ],
               "relationships": [
                   {
                     "sourceActionId": "get_requester_preferred_details_001",
                     "targetActionId": "ask_arun_availability_001",
                     "type": "DEPENDS_ON"
                   },
                   {
                     "sourceActionId": "get_requester_preferred_details_001",
                     "targetActionId": "ask_jasbir_availability_001",
                     "type": "DEPENDS_ON"
                   },
                   {
                     "sourceActionId": "ask_arun_availability_001",
                     "targetActionId": "consolidate_availabilities_001",
                     "type": "DEPENDS_ON"
                   },
                   {
                     "sourceActionId": "ask_jasbir_availability_001",
                     "targetActionId": "consolidate_availabilities_001",
                     "type": "DEPENDS_ON"
                   },
                   {
                     "sourceActionId": "get_requester_preferred_details_001",
                     "targetActionId": "consolidate_availabilities_001",
                     "type": "DEPENDS_ON"
                   },
                   {
                     "sourceActionId": "consolidate_availabilities_001",
                     "targetActionId": "send_meeting_invite_001",
                     "type": "DEPENDS_ON"
                   }
               ]
            }
            """;
    public static final String CHECK_AND_ASK_USER = """
            
            -----
            
            You are an AI assistant analyzing a Message within the context of a specific action.
            
            **Your Goal:** Determine if the `message` is intended to:
            
            1.  **Ask a question or seek confirmation** from a user (requiring a response).
            2.  **Notify a user about an update** (not necessarily requiring a direct response).
            
            ### Input:
            
              * **Message:**
                {messageRequest}
            
              * **Current Action Details:**
            
                ```
                Action Name: {actionName}
                Action Description: {actionDescription}
                ```
              
              * **Current Action Conversation History:**
                {convHistory}
            
            ### Instructions:
            
            1.  **Analyze the `message` content from the `MessageRequest`:**
            
                  * Look for explicit questions, interrogative phrases, or keywords that indicate a query or a need for user input (e.g., "What is...", "Can you confirm...", "Please provide...", "Do you want...", question marks).
                  * Identify phrases that suggest a call to action requiring a user response or decision.
                  * Conversely, identify phrases that primarily convey information, status updates, or notifications. (e.g., "Your request has been processed.", "The status is now...", "We have completed...", "For your information...").
            
            2.  **Evaluate the `Current Action Details`:**
            
                  * Consider the **`Action Name`** for a high-level understanding of its purpose.
                  * Crucially, examine the **`Action Description`**. Does it imply that this action *requires* input, confirmation, or a decision from a user to proceed or complete? Or is it an action that primarily involves processing, reporting, or informing?
            
            ### Output:
            
            Return one of the following labels:
            
              * **"QUESTION"**: If, based on the message content and the action's purpose, the `message` is clearly seeking information, a decision, or confirmation from the user.
              * **"NOTIFICATION"**: If, based on the message content and the action's purpose, the `message` is informing the user about a status, an event, or a completed step.
            
            -----
            
            """;
    public static final String MESSAGE_USER = """
Your task is to communicate with the designated user by sending them a message.

---

## Message to be sent:
{message}

## Designated recipient (user to send the message to):
{user}

## Requestor (the user who initiated this request):
{requestor}

---

## Conversation history:
{convHistory}

---

### Instructions:
- **Crucially, the message must ONLY be sent to the designated recipient ({user}).**
- **Under NO circumstances should the message be sent to the requestor ({requestor}).**
- Use the available tools to send the generated message to the designated recipient.
- To send a message to the designated recipient:
    1. Use `getUsersList` to fetch the list of users.
    2. Find the ID of the designated recipient.
    3. Use the recipient's ID to create a direct channel using `createDirectChannel`.
    4. Use `sendPersonalMessage` to send the message to that channel.
    5. Use `updateAction` to update the details of the action, using the `channelId` obtained in step 3.
            """;

    public static final String SUMMARIZE_ACTION_RESPONSE = """
            You are given the following context:
            
            **Overall Goal:** {goal}
            
            **Current Action Description:** {action_description}
            
            **Conversation History for This Action:** {action_conv}
            
            Based on this, generate a **concise and meaningful summary** that reflects the **progress or result of the current action**, in the context of the overall goal.
            
            ### Ensure the response:
            - Retains and highlights **key information** such as names, entities, times, dates, or other important identifiers.
            - **Uses the user's name** instead of generic terms like "you", "i", "he", "she".
            - Is **clear, relevant**, and avoids unnecessary repetition.
            - **Does not mention the next steps** or future actions—focus only on what has been gathered or completed so far.
            
            """;
}
