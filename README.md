# Workflow Engine

## Project Purpose

The project implements a workflow engine designed to orchestrate a series of actions, primarily focused on automating processes that require gathering input from users. A key example demonstrated in the codebase is scheduling a meeting, which involves coordinating with multiple participants, gathering their availability, and confirming a suitable time. The engine is generic enough to potentially handle other multi-step, input-driven tasks. It processes a predefined "Goal" (typically loaded from a JSON file) that outlines the sequence of actions, their parameters, and dependencies.

## Key Functionalities Summary

This project, a Temporal.io-based workflow engine, offers the following key functionalities:

*   **Dynamic Task Orchestration**: Executes a sequence of tasks (actions) based on a predefined "Goal" definition, typically loaded from a JSON file.
*   **Dependency Management**: Respects dependencies between actions, ensuring that an action only runs after its prerequisite actions have been successfully completed.
*   **User Interaction & Data Gathering**: Facilitates interaction with users to gather necessary information or decisions at various points in a workflow using activities and Temporal signals.
*   **Input Validation**: Supports validation of user-provided input against predefined criteria before proceeding with the workflow.
*   **Data Propagation**: Allows outputs from completed actions to be used as inputs or parameters for subsequent actions (through placeholder resolution in prompts and parameters).
*   **Stateful and Resilient Workflows**: Leverages Temporal.io to maintain workflow state, allowing workflows to be long-running, survive process restarts, and handle activity retries.
*   **Asynchronous Operations**: Efficiently manages asynchronous tasks, particularly waiting for user input via signals, without blocking worker threads.
*   **Extensible Action & Validation Logic**: The use of activities for prompting users (`AskUserActivity`) and validating input (`ValidateInputActivity`) means the specific logic for these operations can be extended or modified independently of the core workflow orchestration.
*   **Example Use Case**: Demonstrates a practical meeting scheduling scenario, showcasing how these functionalities can be combined to automate a multi-step, multi-participant process.

## Technology Stack

*   **Programming Language:** Java (version 1.8, as specified in `pom.xml`).
*   **Workflow Engine:** Temporal.io (SDK version 1.22.3). Temporal is used to define, manage, and execute the workflows, providing features like persistence, retries, and handling asynchronous operations (like waiting for user input).
*   **Build Tool:** Apache Maven. Maven is used for project build management, dependency resolution, and packaging.
*   **JSON Processing:** Jackson library (version 2.15.2). Jackson is used for serializing and deserializing Java objects to/from JSON, which is the format used for defining "Goal" objects.
*   **Logging:** SLF4J (Simple Logging Facade for Java). Used for application logging to monitor workflow execution and diagnose issues.
*   **Testing:** JUnit 5 (Jupiter). Used for unit testing the application components.

## Core Concepts
The workflow engine operates on a few key concepts that define the structure and execution of a process:

*   **`Goal`**:
    *   Represents the overall objective or the high-level task that the workflow aims to achieve. For instance, "Schedule a meeting with Arun and Jasbir."
    *   It is typically defined in a JSON file (e.g., `example-goal.json`) and loaded at the beginning of a workflow.
    *   A `Goal` object encapsulates:
        *   `goal`: A string describing the goal.
        *   `nodes`: A list of `ActionNode` objects, which are the individual steps or tasks required to achieve the goal.
        *   `relationships`: A list of `Relationship` objects that define the dependencies between these `ActionNode`s.
    *   It also transiently stores `actionOutputs`, a map where the results of completed actions are stored for potential use by subsequent actions.

*   **`ActionNode`**:
    *   Represents a single, discrete action or step within the overall `Goal`. Examples from the meeting scheduling scenario include "Get Requester's Meeting Details," "Ask Arun for Availability," or "Send Final Meeting Invite."
    *   Each `ActionNode` has the following key attributes:
        *   `actionId`: A unique identifier for the action (e.g., "get_requester_preferred_details_001").
        *   `actionName`: A human-readable name for the action.
        *   `actionDescription`: A more detailed description of what the action entails.
        *   `actionParams`: A map of parameters required for the action's execution. These can include things like prompt messages for users, recipient details, or criteria for validation (e.g., `prompt_message`, `required_fields`).
        *   `actionStatus`: An enum indicating the current state of the action. Key statuses include:
            *   `PENDING`: The action is waiting to be processed.
            *   `PROCESSING`: The action is currently being executed.
            *   `WAITING_FOR_INPUT`: The action has prompted a user and is awaiting their response.
            *   `COMPLETED`: The action has finished successfully.
            *   `FAILED`: The action encountered an error and could not complete.
            *   `SKIPPED`: The action was not executed, often because one of its dependencies failed.

*   **`Relationship`**:
    *   Defines the connections and dependencies between `ActionNode`s within a `Goal`.
    *   The primary type of relationship observed in the project is "DEPENDS_ON."
    *   A "DEPENDS_ON" relationship specifies that a `targetActionId` cannot begin processing until a `sourceActionId` has reached the `COMPLETED` status.
    *   These relationships dictate the order of execution for actions, ensuring that necessary prerequisite steps are finished before dependent actions start. For example, one cannot "Consolidate All Availabilities" until availability has been requested from all participants.

## Main Workflow Logic (`MeetingSchedulerWorkflowImpl`)

The `MeetingSchedulerWorkflowImpl.java` class is the heart of the workflow engine. It implements the `MeetingSchedulerWorkflow` interface defined by Temporal.io and orchestrates the execution of actions defined in a `Goal` object.

The primary logic resides within the `scheduleMeeting(Goal goal)` method:

1.  **Initialization**:
    *   Upon starting, the workflow receives a `Goal` object.
    *   It initializes internal state, primarily by creating a map (`actionStatuses`) to track the status of each `ActionNode` in the `Goal`. Initially, these are set based on the statuses in the input `Goal` object (typically `PENDING`).
    *   An `actionOutputs` map is also initialized to store the results of completed actions.

2.  **Main Execution Loop**:
    *   The workflow enters a `do...while` loop that continues as long as the workflow is not considered complete (i.e., `isWorkflowComplete()` returns `false`).
    *   **Identifying Processable Actions**:
        *   Inside the loop, it first calls `getProcessableActions()`. This method filters the list of all `ActionNode`s to find those that are currently in `PENDING` status AND whose dependencies (as defined by `Relationship` objects of type "DEPENDS_ON") have all reached the `COMPLETED` status.
    *   **Waiting for Conditions (If Needed)**:
        *   If `getProcessableActions()` returns an empty list (meaning no actions can be immediately processed) AND the workflow is not yet complete, the workflow pauses its execution using `Workflow.await(...)`.
        *   This special Temporal construct effectively puts the workflow to sleep until a specified condition is met. The condition here is typically that either a new action becomes processable (e.g., a user responds to a prompt, triggering a signal that completes a `WAITING_FOR_INPUT` action, thereby unblocking dependent actions) or the entire workflow is marked as complete.
    *   **Processing Actions**:
        *   If there are processable actions:
            *   The workflow iterates through each such `ActionNode`.
            *   The action's status is updated to `PROCESSING`.
            *   A user-facing prompt message is generated. This message is often dynamically constructed using `actionParams` from the `ActionNode` and may include data from previously completed actions using a placeholder mechanism (e.g., `{some_previous_action.output_key}`). The `resolvePlaceholders()` method handles substituting these placeholders with actual values from the `actionOutputs` map.
            *   The `askUserActivity.ask(actionId, prompt)` method is invoked. This is a Temporal activity call, meaning its execution is managed by a Temporal worker and can involve external interactions (like sending a message via Mattermost).
            *   After successfully initiating the `askUserActivity`, the action's status is set to `WAITING_FOR_INPUT`, and the workflow typically waits for an external signal (user response) for this action.
    *   The loop continues, re-evaluating processable actions, waiting if necessary, and processing actions until `isWorkflowComplete()` indicates all actions are done.

3.  **Workflow Completion (`isWorkflowComplete()` method)**:
    *   This method determines if the entire goal has been achieved.
    *   It checks if all `ActionNode`s in the goal have reached either `COMPLETED` or `SKIPPED` status.
    *   It also includes logic to handle potential deadlocks or issues: if an action is `PENDING` but its dependencies have `FAILED`, it is marked as `SKIPPED` to allow the workflow to potentially complete.

This structured approach allows the workflow to manage complex sequences of tasks, handle dependencies, wait for external inputs, and react to events in a resilient manner.

## User Interaction and Signal Handling (`onUserResponse`)

A key feature of this workflow engine is its ability to interact with users to gather information. This is primarily managed through Temporal signals and the `onUserResponse` method within `MeetingSchedulerWorkflowImpl.java`.

1.  **Initiating User Interaction**:
    *   As described in the "Main Workflow Logic," when an `ActionNode` requires user input, the workflow calls the `askUserActivity.ask(actionId, prompt)` activity.
    *   This activity is responsible for delivering the `prompt` to the user (e.g., sending a message in a chat application like Mattermost, displaying a form, etc.). The actual mechanism of how `AskUserActivityImpl` does this is external to the workflow logic itself.
    *   After invoking `askUserActivity`, the `ActionNode`'s status is set to `WAITING_FOR_INPUT`. The workflow instance then typically waits.

2.  **Receiving User Input via Signals**:
    *   When the user provides a response through the external system (e.g., clicks a button in Mattermost, submits a form), that system is expected to send a **Temporal signal** back to the waiting workflow instance.
    *   The `MeetingSchedulerWorkflow` interface defines a signal method: `onUserResponse(String actionId, Map<String, Object> userInput)`.
    *   The external system would use a Temporal client to send a signal named "onUserResponse" to the correct workflow ID, providing the `actionId` of the action being responded to and a `Map` containing the `userInput`.

3.  **Processing User Input (`onUserResponse` method logic)**:
    *   When the signal is received, the `onUserResponse` method in `MeetingSchedulerWorkflowImpl` is invoked.
    *   **Validation Check**: It first verifies that the `actionId` received corresponds to an action that is actually in the `WAITING_FOR_INPUT` state. This prevents processing unexpected or out-of-order signals.
    *   **Input Validation Activity**:
        *   The workflow then calls `validateInputActivity.validate(actionId, userInput, action.getActionParams())`. This is another Temporal activity responsible for checking if the `userInput` is valid according to the criteria defined for that specific action (potentially passed via `actionParams`).
        *   The `ValidateInputActivityImpl` contains the actual validation rules.
    *   **Handling Valid Input**:
        *   If `validateInputActivity` returns `true` (input is valid):
            *   The `ActionNode`'s status is updated to `COMPLETED`.
            *   The validated `userInput` is stored in the `actionOutputs` map, associated with the `actionId`. This makes the input available for use by subsequent actions (e.g., via placeholder resolution in prompts).
            *   The `actionId` is removed from the `waitingForUserInputMap`.
    *   **Handling Invalid Input**:
        *   If `validateInputActivity` returns `false` (input is invalid):
            *   The user is typically re-prompted. The workflow calls `askUserActivity.ask()` again, often with a modified prompt indicating that the previous input was not sufficient (e.g., "Your previous input was not sufficient. Please provide input for...").
            *   The `ActionNode`'s status remains (or is set back to) `WAITING_FOR_INPUT`.
    *   **Handling Activity Errors**:
        *   If calling `validateInputActivity` or `askUserActivity` (during a re-prompt) results in an exception, the `ActionNode`'s status is set to `FAILED`.

This signal-based mechanism allows the workflow to be truly asynchronous, pausing for potentially long periods while waiting for user responses without consuming active compute resources, and then efficiently resuming when input is provided.

## Role of Activities (`AskUserActivity`, `ValidateInputActivity`)

Temporal activities are used to perform individual pieces of work that are part of a workflow. They represent tasks that can involve I/O operations, calls to external services, or any other non-deterministic code that shouldn't run directly within the workflow logic (which must be deterministic). In this project, `AskUserActivity` and `ValidateInputActivity` (along with their implementations `AskUserActivityImpl` and `ValidateInputActivityImpl`) play crucial roles:

*   **`AskUserActivity`** (implemented by `AskUserActivityImpl`):
    *   **Purpose**: This activity is responsible for taking a prompt message (generated by the workflow) and presenting it to a user. Its primary job is to bridge the workflow with the user interaction layer.
    *   **Functionality**:
        *   It receives an `actionId` and a `prompt` string from the workflow.
        *   The implementation (`AskUserActivityImpl`) would contain the specific logic to communicate with an external system. For example, it might:
            *   Send a message to a Mattermost channel or user.
            *   Make an API call to a service that generates a UI form.
            *   Log the prompt to the console (as is likely the case in the current basic implementation if no external integration is built out).
        *   **Important Note**: The `AskUserActivity` itself does *not* wait for the user's response directly. It only initiates the request for input. The workflow then enters a `WAITING_FOR_INPUT` state, and the actual response is delivered back to the workflow via the `onUserResponse` signal, which is triggered by an external system component that processes the user's reply.
    *   **Retryable**: As a Temporal activity, calls to `askUserActivity` can be configured with retry options, timeouts, etc., making the process of requesting user input more robust.

*   **`ValidateInputActivity`** (implemented by `ValidateInputActivityImpl`):
    *   **Purpose**: This activity is responsible for validating the input provided by the user (received via the `onUserResponse` signal) against the requirements of the specific action.
    *   **Functionality**:
        *   It receives the `actionId`, the `userInput` (a `Map<String, Object>`), and potentially `actionParams` (which might contain validation rules like `required_fields` or specific format expectations) from the workflow.
        *   The implementation (`ValidateInputActivityImpl`) contains the actual validation logic. This could range from simple checks (e.g., ensuring certain fields are present) to more complex business rule validations.
        *   It returns a `boolean` value: `true` if the input is valid, `false` otherwise.
    *   **Decoupling Validation Logic**: Using an activity for validation keeps the core workflow logic cleaner and allows validation rules to be complex and potentially involve external lookups without violating workflow determinism rules.
    *   **Retryable**: Similar to other activities, validation attempts can also be retried if they fail due to transient issues.

By delegating these tasks to activities, the main workflow logic in `MeetingSchedulerWorkflowImpl` remains focused on orchestration, state management, and decision-making based on activity results, while the messy details of external communication and specific validation rules are encapsulated within the activity implementations.

## Code Flow Overview

The following outlines the typical execution flow of the Mattermost Temporal workflow application:

1.  **Application Startup (`MeetingSchedulerAppMain.main`)**:
    *   The application is initiated via the `main` method in `MeetingSchedulerAppMain.java`.
    *   **Temporal Setup**:
        *   It configures and establishes a connection to the Temporal.io service (`WorkflowServiceStubs`).
        *   A `WorkflowClient` is created to interact with the Temporal service (e.g., to start workflows).
        *   A `WorkerFactory` and a `Worker` are instantiated. The worker listens to a specific task queue (defined by `MeetingSchedulerAppMain.TASK_QUEUE`).
    *   **Registration**:
        *   The workflow implementation (`MeetingSchedulerWorkflowImpl.class`) is registered with the worker. This tells the worker what code to execute when a workflow of this type is started.
        *   Activity implementations (instances of `AskUserActivityImpl` and `ValidateInputActivityImpl`) are registered with the worker. This tells the worker what code to execute when these activities are invoked by a workflow.
    *   **Worker Start**: The `factory.start()` method is called, which starts all registered workers. The worker begins polling its task queue for tasks.
    *   **Loading Goal**:
        *   The `example-goal.json` (or another specified JSON file) is loaded from resources.
        *   The JSON content is deserialized into a `Goal` object using Jackson.
    *   **Starting a Workflow Instance**:
        *   A unique `workflowId` is generated.
        *   A workflow stub (`MeetingSchedulerWorkflow`) is created using the `WorkflowClient`. This stub is a typed proxy that can be used to start the workflow and send signals to it.
        *   The `workflow.scheduleMeeting(goal)` method is called asynchronously using `WorkflowClient.start(...)`. This sends a command to the Temporal service to initiate a new workflow instance, passing the loaded `Goal` object as an argument.

2.  **Workflow Execution (`MeetingSchedulerWorkflowImpl.scheduleMeeting`)**:
    *   The Temporal service receives the command to start the workflow and assigns a task to a worker listening on the correct task queue.
    *   The worker picks up the task and begins executing the `scheduleMeeting(Goal goal)` method in a new instance of `MeetingSchedulerWorkflowImpl`.
    *   The logic described in the "Main Workflow Logic" section takes over:
        *   Actions are processed based on their status and dependencies.
        *   When user input is needed, `askUserActivity` is called. The workflow state is persisted by Temporal, and it effectively waits.

3.  **User Interaction Loop**:
    *   **`AskUserActivityImpl.ask`**:
        *   This activity implementation executes. It sends the prompt to the user (e.g., logs to console, or in a full implementation, interacts with Mattermost).
    *   **External System & User Response**:
        *   The user sees the prompt in their interface (e.g., Mattermost).
        *   The user provides a response.
        *   An external component (e.g., a Mattermost bot or integration that listens to user actions) captures this response.
        *   This external component then uses a Temporal client to send a signal (e.g., `onUserResponse`) to the specific workflow instance ID, including the `actionId` and the user's input.
    *   **`MeetingSchedulerWorkflowImpl.onUserResponse`**:
        *   The signal is delivered to the waiting workflow instance, and the `onUserResponse` method is invoked.
        *   The input is processed:
            *   `ValidateInputActivityImpl.validate` is called to validate the user's data.
            *   Based on validation, the action is either completed, or the user is re-prompted (looping back to calling `AskUserActivityImpl.ask`).
    *   The workflow continues processing actions, potentially waiting for more signals, until the entire goal is complete.

4.  **Workflow Completion**:
    *   Once `isWorkflowComplete()` returns `true`, the `scheduleMeeting` method finishes.
    *   The workflow instance is marked as completed in the Temporal service.
    *   Final outputs might be logged or available in the `currentGoal.actionOutputs` map within the workflow's final state (viewable via Temporal UI/tctl).

This flow demonstrates the interplay between the application's main entry point, the Temporal worker, the workflow definition, activities, and external user interactions facilitated by signals.

## Request Flow Diagram
```plantuml
@startuml Request Flow Diagram

!theme vibrant
title Meeting Scheduler Workflow - Request Flow Example

actor User
participant "External System\n(e.g., Mattermost Bot)" as ExternalSystem
participant "MeetingSchedulerAppMain" as AppMain
participant "Temporal Service" as Temporal
participant "MeetingSchedulerWorkflowImpl" as Workflow
activity "AskUserActivity" as AskActivity
activity "ValidateInputActivity" as ValidateActivity

skinparam sequenceMessageAlign center
skinparam activity {
    BackgroundColor PaleGreen
    BorderColor Green
}

== Workflow Initiation ==
AppMain -> Temporal: StartWorkflow(scheduleMeeting, Goal)
Temporal -> Workflow: Execute scheduleMeeting(Goal)
activate Workflow

== Action Requiring User Input ==
Workflow -> Workflow: Identify processable action
Workflow -> Temporal: CallActivity(AskUserActivity.ask, actionId, prompt)
Temporal -> AskActivity: Execute ask(actionId, prompt)
activate AskActivity
AskActivity -> ExternalSystem: SendPromptToUser(prompt)
deactivate AskActivity
ExternalSystem -> User: DisplayPrompt(prompt)

== User Responds ==
User -> ExternalSystem: SubmitInput(userInput)
ExternalSystem -> Temporal: SignalWorkflow(onUserResponse, actionId, userInput)
Temporal -> Workflow: DeliverSignal(onUserResponse, actionId, userInput)
activate Workflow #LightBlue

== Input Validation ==
Workflow -> Temporal: CallActivity(ValidateInputActivity.validate, actionId, userInput, params)
Temporal -> ValidateActivity: Execute validate(actionId, userInput, params)
activate ValidateActivity
ValidateActivity --> Workflow: ValidationResult(isValid)
deactivate ValidateActivity

== Processing Validation Result ==
alt Input is Valid
    Workflow -> Workflow: MarkActionCompleted(actionId)
    Workflow -> Workflow: StoreOutput(actionId, userInput)
    Workflow -> Workflow: Check if workflow complete
else Input is Invalid
    Workflow -> Temporal: CallActivity(AskUserActivity.ask, actionId, rePrompt)
    Temporal -> AskActivity: Execute ask(actionId, rePrompt)
    activate AskActivity
    AskActivity -> ExternalSystem: SendPromptToUser(rePrompt)
    deactivate AskActivity
    ExternalSystem -> User: DisplayPrompt(rePrompt)
    Workflow -> Workflow: Set action to WAITING_FOR_INPUT
end
deactivate Workflow

== Workflow Continues or Completes ==
Workflow -> Workflow: Process next actions or complete
...
Workflow -> Temporal: WorkflowExecutionCompleted
deactivate Workflow

@enduml
```

## Areas for Further Exploration

To gain a deeper understanding of the entire application or to extend its capabilities, the following areas could be explored:

*   **`AskUserActivityImpl.java` and `ValidateInputActivityImpl.java`**:
    *   Reviewing the concrete implementations of these activities is crucial to understand how user prompts are actually delivered (e.g., console output, API call to Mattermost) and how input validation rules are specifically applied. The current descriptions are based on their interface and role within the workflow.

*   **`example-goal.json` (and other goal definitions)**:
    *   While the structure has been discussed, analyzing more complex or varied `Goal` JSON files would provide insights into the flexibility and limitations of the current action definitions and relationship types.
    *   Experimenting with creating new goal JSONs for different use cases would test the engine's adaptability.

*   **External System Integration (e.g., Mattermost)**:
    *   The workflow relies on an external system to:
        1.  Present prompts (initiated by `AskUserActivity`) to the end-user.
        2.  Capture the user's response.
        3.  Send a Temporal signal (e.g., `onUserResponse`) back to the correct workflow instance.
    *   Understanding or implementing this external component (e.g., a Mattermost bot, a web UI that communicates with a backend capable of sending Temporal signals) is key to seeing the workflow in a real-world operational context.

*   **Error Handling and Compensation Logic**:
    *   While Temporal provides activity retries, exploring more sophisticated error handling within the workflow (e.g., compensation logic for failed actions, alternative paths) could be beneficial for more critical processes. The current workflow primarily marks actions as `FAILED` or `SKIPPED`.

*   **`ActionStatus.java` and `Relationship.java`**:
    *   While their use is clear, one might explore if additional action statuses or relationship types could enhance the workflow's capabilities (e.g., conditional branching based on output, parallel execution blocks).

*   **Temporal Client Configuration and Deployment**:
    *   The `MeetingSchedulerAppMain.java` shows basic setup. For a production scenario, more advanced Temporal client/worker configuration, metrics, and deployment strategies would need consideration.

*   **Security and Authorization**:
    *   In a real application, especially one interacting with users and potentially sensitive data, aspects like authenticating users and authorizing actions would be important additions.

*   **Testing Strategy**:
    *   Examining `MeetingSchedulerWorkflowTest.java` would provide insights into how Temporal workflows and activities are unit/integration tested using `temporal-testing`.
