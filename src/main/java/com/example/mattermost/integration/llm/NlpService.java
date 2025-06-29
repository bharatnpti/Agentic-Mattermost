package com.example.mattermost.integration.llm;

import com.example.mattermost.domain.model.ActionNode;
import com.example.mattermost.domain.model.ActionStatus;
import com.example.mattermost.domain.model.Goal;
import com.example.mattermost.integration.mattermost.MattermostService;
import com.example.mattermost.util.PromptHolder;
import com.example.mattermost.workflow.MeetingSchedulerWorkflowImpl;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.ai.chat.client.ChatClient;
import org.springframework.ai.chat.client.advisor.SimpleLoggerAdvisor;
import org.springframework.ai.chat.model.ChatResponse;
import org.springframework.ai.chat.prompt.Prompt;
import org.springframework.ai.chat.prompt.PromptTemplate;
import org.springframework.ai.converter.BeanOutputConverter;
import org.springframework.ai.openai.OpenAiChatModel;
import org.springframework.ai.openai.OpenAiChatOptions;
import org.springframework.ai.openai.api.OpenAiApi;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Service;

import java.util.List;
import java.util.Map;
import java.util.stream.Collectors;

// Assuming RetrievedContextItem is accessible or defined similarly
// For simplicity, let's define a local record if not directly importing from MemoryService's scope
// For now, assume CoreOrchestrationService will pass a formatted string or a list of simple message strings.
// Let's refine this to accept a list of simple objects representing message history.

record ChatMessageHistoryItem(String role, String content) {}

// Pojo for BeanOutputConverter (ensure it's accessible or defined here)
class IntentExtractionPojo {
    private String intent;
    private Map<String, Object> entities;
    public String getIntent() { return intent; }
    public void setIntent(String intent) { this.intent = intent; }
    public Map<String, Object> getEntities() { return entities; }
    public void setEntities(Map<String, Object> entities) { this.entities = entities; }
}

@Service
public class NlpService {

    private static final Logger logger = LoggerFactory.getLogger(NlpService.class);

    private final Map<String, ChatClient> chatClient;
    private final BeanOutputConverter<IntentExtractionPojo> intentExtractionParser;
    private static final String openai4_1 = "openai-4.1-mini";

//    @Autowired
//    private SyncMcpToolCallbackProvider toolCallbackProvider;

    @Autowired
    private MattermostService mattermostService;

    public NlpService() {
        OpenAiChatModel baseOpenAiChatModel = OpenAiChatModel.builder()
                .openAiApi(OpenAiApi.builder()
                        .baseUrl("https://api.openai.com")
                        .build())
                .build();

        ChatClient openAi4_1Client = ChatClient.builder(baseOpenAiChatModel.mutate().defaultOptions(OpenAiChatOptions.builder()
                .model("o4-mini-2025-04-16").build()).build())
//                .defaultAdvisors(new SimpleLoggerAdvisor())
                .build();


        chatClient = Map.of(
                openai4_1, openAi4_1Client
        );

        this.intentExtractionParser = new BeanOutputConverter<>(IntentExtractionPojo.class);
    }

    /**
     * Formats a list of ChatMessageHistoryItem into a string for the prompt.
     */
    private String formatConversationHistory(List<ChatMessageHistoryItem> history) {
        if (history == null || history.isEmpty()) {
            return "No prior conversation history available for this interaction.";
        }
        return history.stream()
                .map(item -> item.role() + ": " + item.content())
                .collect(Collectors.joining("\n"));
    }

    /**
     * Generates text based on a given prompt, considering conversation history.
     *
     * @param userPromptText The current prompt/text from the user.
     * @param conversationHistory A list of previous messages (role and content).
     * @return The generated text.
     */
    public String generateText(String userPromptText, List<ChatMessageHistoryItem> conversationHistory) {
        logger.debug("Generating text for prompt with history: '{}'", userPromptText);

        String formattedHistory = formatConversationHistory(conversationHistory);

        // Construct a prompt that includes history and the new user message
        String fullPromptString = """
            Conversation History:
            {history}

            Current User Query:
            User: {currentUserPrompt}

            Agent Response:
            """; // The LLM will complete this.

        PromptTemplate promptTemplate = new PromptTemplate(fullPromptString);
        Prompt prompt = promptTemplate.create(Map.of(
            "history", formattedHistory,
            "currentUserPrompt", userPromptText
        ));

        logger.debug("Constructed prompt for LLM (generateText with history): {}", prompt.getContents());

        try {
            ChatResponse response = chatClient.get(openai4_1).prompt(prompt).call().chatResponse();
            return response.getResult().getOutput().getText();
        } catch (Exception e) {
            logger.error("Error calling LLM for text generation with history: {}", e.getMessage(), e);
            return "Error: Could not generate text due to: " + e.getMessage();
        }
    }

    /**
     * Summarizes the given text. History might be less relevant here, or could be used to tailor summary style.
     * For now, not adding history to summarizeText, but it's an option.
     * @param textToSummarize The text to be summarized.
     * @return The summarized text.
     */
    public String summarizeText(String textToSummarize) {
        logger.debug("Summarizing text of length: {}", textToSummarize.length());
        String promptString = "Please summarize the following text concisely: \n\n{text}"; // Escaped \n
        PromptTemplate promptTemplate = new PromptTemplate(promptString);
        Prompt prompt = promptTemplate.create(Map.of("text", textToSummarize));

        try {
            ChatResponse response = chatClient.get(openai4_1).prompt(prompt).call().chatResponse();
            return response.getResult().getOutput().getText();
        } catch (Exception e) {
            logger.error("Error calling LLM for summarization: {}", e.getMessage(), e);
            return "Error: Could not summarize text due to: " + e.getMessage();
        }
    }

//    public String testTools(String query) {
//        PromptTemplate promptTemplate = new PromptTemplate("""
//                answer user query using the available tools {query}
//
//                #Note:
//                To send a message to someone first create a channel with them and then send message to that channel
//                """);
//        Prompt prompt = promptTemplate.create(Map.of("query", query));
//        ToolCallback[] toolCallbacks = toolCallbackProvider.getToolCallbacks();
//        ChatResponse chatResponse = chatClient.get(openai4_1).prompt(prompt).toolCallbacks(toolCallbacks).call().chatResponse();
//        System.out.println("Chat Response: " + chatResponse.getResult().getOutput().getText());
//        return chatResponse.getResult().getOutput().getText();
//    }

    public String executeAction(String goal, String previousActionsResponseString, ActionNode action, String currentThreadId, String currentUserId, String currentChannelId) {
        if(MeetingSchedulerWorkflowImpl.debug) {
            System.out.println("Processing LLM Activity DEBUG");
        }

        PromptTemplate promptTemplate = new PromptTemplate(PromptHolder.EXECUTE_ACTION);
        Prompt prompt = promptTemplate.create(
                Map.of("goal", goal,
                        "convHistory", previousActionsResponseString,
                        "action", action
                )
        );

        Map<String, Object> toolContext = Map.of(
                "workflowId", action.getWorkflowId(),
                "actionId", action.getActionId(),
                "rootId", currentThreadId,
                "channelId", currentChannelId,
                "currentUserId", currentUserId
        );

        ChatClient chatClient1 = chatClient.get(openai4_1);
        chatClient1 = chatClient1.mutate()
//                .defaultToolCallbacks(toolCallbackProvider.getToolCallbacks())
                .defaultAdvisors(new SimpleLoggerAdvisor())
//                .defaultTools(mattermostService)
                .defaultToolContext(toolContext)
                .build();
        ChatResponse chatResponse = chatClient1.prompt(prompt).call().chatResponse();
//        System.out.println("Executing action result: " + chatResponse);
        String text = chatResponse.getResult().getOutput().getText();
        logger.info("executeAction Result: {}", text);
        return text;
    }

    public ActionStatus determineActionResult(String goal, ActionNode action, String actionResult) {
        if(MeetingSchedulerWorkflowImpl.debug) {
            System.out.println("Processing LLM Activity DEBUG");
        }
        PromptTemplate promptTemplate = new PromptTemplate(PromptHolder.ACTION_STATUS);
        Prompt prompt = promptTemplate.create(
                Map.of("goal", goal,
                        "result", actionResult,
                        "action", action
                )
        );

        ChatClient chatClient1 = chatClient.get(openai4_1);
        chatClient1 = chatClient1.mutate()
//                .defaultToolCallbacks(toolCallbackProvider.getToolCallbacks())
//                .defaultTools(internalTools)
                .build();
        ChatResponse chatResponse = chatClient1.prompt(prompt).call().chatResponse();
//        System.out.println("Executing action result: " + chatResponse);
        logger.info("determineActionResult Result: {}", chatResponse.getResult().getOutput().getText());
        return ActionStatus.valueOf(chatResponse.getResult().getOutput().getText());
    }

    public String evaluateAndProcessUserInput(Goal currentGoal, ActionNode action, String userInput) {
        PromptTemplate promptTemplate = new PromptTemplate(PromptHolder.EVALUATE_USER_RESPONSE);
        Prompt prompt = promptTemplate.create(
                Map.of("goal", currentGoal,
                        "userInput", userInput,
                        "actionDescription", action.getActionDescription(),
                        "responseFormat", PromptHolder.EVALUATE_RESPONSE_FORMAT
                )
        );

        ChatClient chatClient1 = chatClient.get(openai4_1);
        chatClient1 = chatClient1.mutate()
//                .defaultToolCallbacks(toolCallbackProvider.getToolCallbacks())
//                .defaultTools(internalTools)
                .build();
        ChatResponse chatResponse = chatClient1.prompt(prompt).call().chatResponse();
        String text = chatResponse.getResult().getOutput().getText();
        String s = text.replaceAll("```json", "").replaceAll("```", "");
        logger.info("evaluateAndProcessUserInput Result: {}", s);
        return s;
    }

    public Goal createActions(String textInput, List<ChatMessageHistoryItem> chatHistory) {
        PromptTemplate promptTemplate = new PromptTemplate(PromptHolder.ACTIONS);
        BeanOutputConverter<Goal> actionsConverter = new BeanOutputConverter<>(Goal.class);
        Prompt prompt = promptTemplate.create(Map.of("conv_history", formatConversationHistory(chatHistory),
                        "user_query", textInput,
                        "formatInstructions", actionsConverter.getFormat(),
                        "relationship", """
                        {"sourceActionId": "A", "targetActionId": "B", "type": "DEPENDS_ON"}
                        """,
                        "examples", PromptHolder.ACTIONS_EXAMPLES
                )
        );

        try {
            ChatClient chatClient1 = chatClient.get(openai4_1);
            ChatResponse response = chatClient1.prompt(prompt).call().chatResponse();
            String text = response.getResult().getOutput().getText();
            logger.info("Raw LLM response for action creation: {}", text);
            return actionsConverter.convert(text);
        } catch (Exception e) {
            logger.error("Error calling LLM for action creation: {}", e.getMessage(), e);
            throw new RuntimeException("Error: Could not create actions due to: " + e.getMessage(), e);
        }
    }

    public ActionStatus determineActionType(String goal, ActionNode action, String convHistory) {
        PromptTemplate promptTemplate = new PromptTemplate(PromptHolder.DETERMINE_ACTION_TYPE);
        BeanOutputConverter<Goal> actionsConverter = new BeanOutputConverter<>(Goal.class);
        Prompt prompt = promptTemplate.create(Map.of("Goal", goal,
                        "ActionId", action.getActionId(),
                        "ActionName", action.getActionName(),
                        "ActionDescription", action.getActionDescription(),
                "convHistory", convHistory
                )
        );

        try {
            ChatClient chatClient1 = chatClient.get(openai4_1);
            ChatResponse response = chatClient1.prompt(prompt).call().chatResponse();
            String text = response.getResult().getOutput().getText();
            logger.info("determineActionType Result: {}", text);
            return ActionStatus.valueOf(text);
        } catch (Exception e) {
            logger.error("Error calling LLM for action creation: {}", e.getMessage(), e);
            throw new RuntimeException("Error: Could not create actions due to: " + e.getMessage(), e);
        }
    }

    public String ask_user(String goal, ActionNode action, String convHistory, String currentThreadId, String channelId, String currentUserId) {

        PromptTemplate promptTemplate = new PromptTemplate(PromptHolder.ASK_USER);
        String promptTemplate1 = action.getActionParams().get("prompt_template") == null ? "" : action.getActionParams().get("prompt_template").toString();
        Object requiredFields = action.getActionParams().get("required_fields")  == null ? "" : action.getActionParams().get("required_fields").toString();
        Prompt prompt = promptTemplate.create(Map.of("goal", goal,
                        "actionName", action.getActionName(),
                        "actionDescription", action.getActionDescription(),
                "prompt_template", promptTemplate1,
                "required_fields", requiredFields,
                "convHistory", convHistory
                )
        );

        Map<String, Object> toolContext = Map.of(
                "workflowId", action.getWorkflowId(),
                "actionId", action.getActionId(),
                "rootId", currentThreadId,
                "channelId", channelId,
                "currentUserId", currentUserId
        );

        try {
            ChatClient chatClient1 = chatClient.get(openai4_1);
            ChatResponse response = chatClient1.prompt(prompt)
//                    .toolCallbacks(toolCallbackProvider.getToolCallbacks())
                    .tools(mattermostService)
                    .toolContext(toolContext)
                    .call().chatResponse();
            logger.info("ask_user Result: {}", response.getResult().getOutput().getText());
            return response.getResult().getOutput().getText();
        } catch (Exception e) {
            logger.error("Error calling LLM for action creation: {}", e.getMessage(), e);
            throw new RuntimeException("Error: Could not create actions due to: " + e.getMessage(), e);
        }
    }


}
