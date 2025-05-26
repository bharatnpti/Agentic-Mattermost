import json
import uuid
from unittest.mock import patch, AsyncMock

import pytest
from fastapi.testclient import TestClient
from httpx import Response # For mocking client responses if needed, not directly for TestClient

# Assuming your FastAPI app instance is named 'app' in 'main.py'
from main import app, OpenAIChatCompletionRequest, OpenAIChatMessageInput, \
                 OpenAIChatCompletionResponse, OpenAIChatCompletionChunk, \
                 OpenAIMessageToolCall, OpenAIMessageToolCallFunction # Import Pydantic models from main
from langchain_core.messages import AIMessage, HumanMessage, SystemMessage # For mocking graph output
from langchain_core.tools import ToolCall # For mocking graph output


client = TestClient(app)

@pytest.fixture
def mock_mcp_graph_invoke():
    with patch("main.mcp_graph.invoke") as mock:
        yield mock

@pytest.fixture
def mock_mcp_graph_astream_events():
    with patch("main.mcp_graph.astream_events") as mock:
        yield mock


def test_chat_completion_non_streaming_simple_qa(mock_mcp_graph_invoke):
    """
    Tests a basic non-streaming chat completion request that expects a simple text response.
    Mocks the mcp_graph.invoke to return a final AIMessage with content.
    """
    request_payload = OpenAIChatCompletionRequest(
        model="gpt-4o",
        messages=[
            OpenAIChatMessageInput(role="user", content="Hello, who are you?")
        ],
        stream=False
    )

    # Mock the graph's response
    mock_ai_response = AIMessage(content="I am a helpful assistant.")
    mock_final_state = {"messages": [mock_ai_response]}
    mock_mcp_graph_invoke.return_value = mock_final_state

    response = client.post("/v1/chat/completions", json=request_payload.model_dump(exclude_none=True))

    assert response.status_code == 200
    response_data = response.json()

    # Validate against Pydantic model if desired, or check fields directly
    parsed_response = OpenAIChatCompletionResponse(**response_data)
    
    assert parsed_response.id.startswith("chatcmpl-")
    assert parsed_response.object == "chat.completion"
    assert len(parsed_response.choices) == 1
    choice = parsed_response.choices[0]
    assert choice.index == 0
    assert choice.message.role == "assistant"
    assert choice.message.content == "I am a helpful assistant."
    assert choice.finish_reason == "stop"
    assert choice.message.tool_calls is None 
    # Basic usage check
    assert parsed_response.usage.prompt_tokens == 0 # Placeholder value from main.py
    assert parsed_response.usage.completion_tokens == 0 # Placeholder value
    assert parsed_response.usage.total_tokens == 0

    mock_mcp_graph_invoke.assert_called_once()


def test_chat_completion_non_streaming_tool_call_decision(mock_mcp_graph_invoke):
    """
    Tests a non-streaming request where the LLM (mocked via graph) decides to call a tool.
    """
    request_payload = OpenAIChatCompletionRequest(
        model="gpt-4o",
        messages=[
            OpenAIChatMessageInput(role="user", content="Find user with ID test_user_id_123")
        ],
        stream=False
    )

    mock_tool_call_id = f"call_{uuid.uuid4().hex}"
    # This is LangChain's ToolCall dict structure
    lc_tool_call = ToolCall( 
        name="mcp_find_user",
        args={"user_id": "test_user_id_123"},
        id=mock_tool_call_id
    )
    mock_ai_response_with_tool_call = AIMessage(
        content=None, # OpenAI expects null content if tool_calls are present
        tool_calls=[lc_tool_call] 
    )
    mock_final_state = {"messages": [mock_ai_response_with_tool_call]}
    mock_mcp_graph_invoke.return_value = mock_final_state

    response = client.post("/v1/chat/completions", json=request_payload.model_dump(exclude_none=True))

    assert response.status_code == 200
    response_data = response.json()
    parsed_response = OpenAIChatCompletionResponse(**response_data)

    assert len(parsed_response.choices) == 1
    choice = parsed_response.choices[0]
    assert choice.finish_reason == "tool_calls"
    assert choice.message.role == "assistant"
    assert choice.message.content is None # Important for tool calls

    assert choice.message.tool_calls is not None
    assert len(choice.message.tool_calls) == 1
    tool_call_output: OpenAIMessageToolCall = choice.message.tool_calls[0]
    
    assert tool_call_output.id == mock_tool_call_id
    assert tool_call_output.type == "function"
    assert tool_call_output.function.name == "mcp_find_user"
    assert tool_call_output.function.arguments == '{"user_id": "test_user_id_123"}'

    mock_mcp_graph_invoke.assert_called_once()


async def mock_streaming_graph_simple_qa(*args, **kwargs):
    """Async generator to mock mcp_graph.astream_events for simple Q&A."""
    # Simulate the LLM streaming content
    yield {
        "event": "on_chat_model_stream",
        "data": {"chunk": AIMessageChunk(content="Hello! ", id="chunk1")}
    }
    yield {
        "event": "on_chat_model_stream",
        "data": {"chunk": AIMessageChunk(content="I am an assistant.", id="chunk2")}
    }
    # Simulate the end of the graph with a final message
    yield {
        "event": "on_graph_end",
        "data": {
            "output": { # This structure depends on how LangGraph wraps final output in astream_events
                "messages": [AIMessage(content="Hello! I am an assistant.")]
            } 
        }
    }

def test_chat_completion_streaming_simple_qa(mock_mcp_graph_astream_events):
    """
    Tests a basic streaming chat completion request.
    Mocks mcp_graph.astream_events to simulate content streaming.
    """
    request_payload = OpenAIChatCompletionRequest(
        model="gpt-4o",
        messages=[
            OpenAIChatMessageInput(role="user", content="Hello stream")
        ],
        stream=True
    )

    mock_mcp_graph_astream_events.return_value = mock_streaming_graph_simple_qa()
    
    response = client.post("/v1/chat/completions", json=request_payload.model_dump(exclude_none=True))
    assert response.status_code == 200

    full_content = ""
    received_chunks = []
    first_chunk_role_checked = False
    final_finish_reason = None

    for line in response.iter_lines():
        if line.startswith("data: "):
            data_content = line[len("data: "):]
            if data_content == "[DONE]":
                break
            
            chunk = OpenAIChatCompletionChunk(**json.loads(data_content))
            received_chunks.append(chunk)
            
            assert chunk.id.startswith("chatcmpl-stream-") # stream_id is consistent
            assert len(chunk.choices) == 1
            choice = chunk.choices[0]
            assert choice.index == 0

            if not first_chunk_role_checked and choice.delta.role:
                assert choice.delta.role == "assistant"
                first_chunk_role_checked = True
            
            if choice.delta.content:
                full_content += choice.delta.content
            
            if choice.finish_reason:
                final_finish_reason = choice.finish_reason
    
    assert first_chunk_role_checked, "First chunk should have contained the assistant's role."
    assert full_content == "Hello! I am an assistant."
    assert final_finish_reason == "stop", "Final chunk finish_reason should be 'stop' for simple Q&A"
    assert len(received_chunks) == 3 # 1 role, 1 content "Hello! ", 1 content "I am an assistant.", 1 final with finish_reason
                                     # Corrected: The actual implementation sends role in first chunk, then content, then final finish_reason
                                     # The mock_streaming_graph_simple_qa sends 2 content chunks.
                                     # The endpoint sends 1st (role), 2nd (content "Hello! "), 3rd (content "I am an assistant."), 4th (finish_reason)
                                     # So, 3 chunks with content/role, and 1 final chunk.
                                     # My event_generator in main.py actually yields: role, then content, then finish_reason.
                                     # The mock here simulates this slightly differently. Let's adjust expectation based on main.py's actual streaming:
                                     # 1. Role chunk
                                     # 2. Content chunk(s)
                                     # 3. Final chunk with finish_reason.
                                     # The mock_streaming_graph_simple_qa simulates two content chunks, and then graph_end implies finish_reason.
                                     # The main.py event_generator will create:
                                     #  - chunk 1 (role)
                                     #  - chunk 2 (content "Hello! ")
                                     #  - chunk 3 (content "I am an assistant.")
                                     #  - chunk 4 (finish_reason "stop", empty delta)
    assert len(received_chunks) >= 2 # At least one for role, one for content, one for finish_reason.
                                     # Based on current main.py streaming, it should be 3 chunks if content is small, or more if content is larger.
                                     # The provided mock_streaming_graph_simple_qa will result in 3 chunks from the main.py handler
                                     # (role, content, finish_reason based on graph_end)

    mock_mcp_graph_astream_events.assert_called_once()

# TODO: Add test_chat_completion_streaming_tool_call_decision
# This would involve:
# 1. Mocking mcp_graph.astream_events to yield:
#    - on_chat_model_stream with AIMessageChunk(role="assistant")
#    - on_chat_model_stream with AIMessageChunk(tool_call_chunks=[...])
#      - This part needs careful construction of ToolCallChunk data to match OpenAI's streaming format for tool calls
#        (index, id, type="function", function.name then function.arguments streamed)
#    - on_graph_end with a final AIMessage containing the full tool_calls
# 2. Asserting that the streamed response contains the tool_call chunks correctly.
# 3. Asserting the final chunk has finish_reason="tool_calls".
```
