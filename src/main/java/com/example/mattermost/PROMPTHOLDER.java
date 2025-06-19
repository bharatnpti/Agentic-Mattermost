package com.example.mattermost;

public class PROMPTHOLDER {
    public static final String ACTION_STATUS =
            """
                    You are an intelligent workflow agent responsible for determining the status of an action within a goal-driven process.
                    
                    Given the following context:
                    
                    Goal:
                    "{goal}"
                    
                    Action:
                    {action}
                    
                    Execution Result:
                    {result}
                    
                    Instructions:
                    Based on the description and execution result, determine the most accurate action status from the following enum values:
                    
                    Rules:
                    - If the action has not started and required inputs are not available yet → return PENDING
                    - If the action is asking for user/system input and awaiting a response → return WAITING_FOR_INPUT
                    - If input has been received and action is actively being executed → return PROCESSING
                    - If the action was executed successfully with all required fields present → return COMPLETED
                    - If execution failed or required input was invalid → return FAILED
                    - If the action's dependencies failed or were skipped → return SKIPPED
                    
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

    static final String EVALUATE_RESPONSE_FORMAT = """
            {
              "status": "COMPLETED" | "WAITING_FOR_INPUT",
              "reason": "Short explanation of why this status was chosen",
              "missingOrRequired": [list of missing fields or next actions required, or empty array]
            }
            """;
    static final String EXECUTE_ACTION = """
            
            You are an assistant orchestrating steps to achieve the goal:
            **"{goal}"**
            
            Now, execute the following action:
            {action}
            
            Relevant context from previous completed actions:
            {convHistory}
            
            Based on this context, perform the action as described.
            
            Instructions:
            - Generate the appropriate message, request, or data output required by this action.
            - Maintain clarity and tone suitable for scheduling meetings.
            
            Your output should directly reflect the action's intent (e.g., message to user, proposed time, consolidated data, etc.).
            
            """;
}
