import json
import time
import uuid
from typing import List, Optional, Dict, Any, Union
import asyncio
import logging

from fastapi import FastAPI
from fastapi.responses import StreamingResponse
from pydantic import BaseModel, Field

from langchain_core.messages import (
    AIMessage,
    BaseMessage,
    HumanMessage,
    ToolMessage,
    SystemMessage,
    AIMessageChunk,
)
from langchain_core.tools import ToolCall 
from mcp_agent import AgentState, mcp_graph

# --- FastAPI App Initialization ---
app = FastAPI(
    title="Mattermost Copilot Agent Server",
    version="0.1.0",
    description="A FastAPI server emulating the OpenAI API for chat completions, "
                "backed by a LangGraph agent with Mattermost Copilot Program tools.",
)

# Configure basic logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

# --- Pydantic Models for OpenAI Request Structure ---

class OpenAIFunctionParameters(BaseModel):
    type: str = "object"
    properties: Dict[str, Any] = Field(default_factory=dict)

class OpenAIToolFunction(BaseModel):
    name: str
    description: Optional[str] = None
    parameters: OpenAIFunctionParameters

class OpenAITool(BaseModel):
    type: str = "function"
    function: OpenAIToolFunction

class OpenAIMessageToolCallFunction(BaseModel):
    name: Optional[str] = None
    arguments: Optional[str] = None # JSON string for arguments

class OpenAIMessageToolCall(BaseModel):
    index: Optional[int] = None # OpenAI uses 'index' for streaming tool call chunks
    id: str = Field(default_factory=lambda: f"call_{uuid.uuid4().hex}")
    type: str = "function"
    function: OpenAIMessageToolCallFunction

class OpenAIChatMessageInput(BaseModel):
    role: str
    content: Optional[str] = None
    name: Optional[str] = None 
    tool_calls: Optional[List[OpenAIMessageToolCall]] = None
    tool_call_id: Optional[str] = None

class OpenAIChatCompletionRequest(BaseModel):
    model: str
    messages: List[OpenAIChatMessageInput]
    tools: Optional[List[OpenAITool]] = None
    tool_choice: Optional[Any] = None 
    stream: Optional[bool] = False
    temperature: Optional[float] = None
    max_tokens: Optional[int] = None
    top_p: Optional[float] = None
    frequency_penalty: Optional[float] = None
    presence_penalty: Optional[float] = None
    stop: Optional[Union[str, List[str]]] = None
    user: Optional[str] = None

# --- Pydantic Models for OpenAI Response Structure (Non-Streaming) ---

class OpenAIMessageOutput(BaseModel):
    role: str 
    content: Optional[str] = None
    tool_calls: Optional[List[OpenAIMessageToolCall]] = None

class OpenAIChoiceOutput(BaseModel):
    index: int
    message: OpenAIMessageOutput
    finish_reason: Optional[str] = None 

class OpenAIUsageOutput(BaseModel):
    prompt_tokens: int = 0
    completion_tokens: int = 0
    total_tokens: int = 0

class OpenAIChatCompletionResponse(BaseModel):
    id: str = Field(default_factory=lambda: f"chatcmpl-{uuid.uuid4().hex}")
    object: str = "chat.completion"
    created: int = Field(default_factory=lambda: int(time.time()))
    model: str
    choices: List[OpenAIChoiceOutput]
    usage: OpenAIUsageOutput
    system_fingerprint: Optional[str] = None


# --- Pydantic Models for OpenAI Response Structure (Streaming) ---

class OpenAIChatCompletionChunkDelta(BaseModel):
    role: Optional[str] = None
    content: Optional[str] = None
    tool_calls: Optional[List[OpenAIMessageToolCall]] = None 

class OpenAIChoiceChunk(BaseModel):
    index: int
    delta: OpenAIChatCompletionChunkDelta
    finish_reason: Optional[str] = None

class OpenAIChatCompletionChunk(BaseModel):
    id: str = Field(default_factory=lambda: f"chatcmpl-{uuid.uuid4().hex}")
    object: str = "chat.completion.chunk"
    created: int = Field(default_factory=lambda: int(time.time()))
    model: str
    choices: List[OpenAIChoiceChunk]
    usage: Optional[OpenAIUsageOutput] = None 
    system_fingerprint: Optional[str] = None


# --- Helper Functions ---
def convert_openai_req_to_lc_msgs(openai_msgs: List[OpenAIChatMessageInput]) -> List[BaseMessage]:
    lc_msgs = []
    for msg in openai_msgs:
        if msg.role == "system":
            lc_msgs.append(SystemMessage(content=msg.content or ""))
        elif msg.role == "user":
            lc_msgs.append(HumanMessage(content=msg.content or ""))
        elif msg.role == "assistant":
            lc_tool_calls = []
            if msg.tool_calls:
                for tc in msg.tool_calls:
                    try:
                        args_dict = json.loads(tc.function.arguments or "{}")
                    except json.JSONDecodeError:
                        logger.warning(f"Failed to parse tool call arguments JSON: {tc.function.arguments}")
                        args_dict = {"error": "failed to parse arguments JSON", "raw_arguments": tc.function.arguments}
                    
                    lc_tool_calls.append(ToolCall( 
                        name=tc.function.name or f"unknown_tool_name_for_id_{tc.id}",
                        args=args_dict,
                        id=tc.id
                    ))
            
            content_val = msg.content
            if not lc_tool_calls and content_val is None:
                content_val = "" 
            
            lc_msgs.append(AIMessage(content=content_val or "", tool_calls=lc_tool_calls if lc_tool_calls else []))

        elif msg.role == "tool":
            if msg.tool_call_id and msg.content is not None:
                lc_msgs.append(ToolMessage(
                    content=msg.content, 
                    tool_call_id=msg.tool_call_id, 
                    name=msg.name or msg.tool_call_id 
                ))
            else:
                logger.warning(f"Skipping invalid tool message: {msg.model_dump_json(exclude_none=True)}")
        else:
            logger.warning(f"Unknown role in input message: {msg.role}")
    return lc_msgs

def lc_tool_call_to_openai_message_tool_call(lc_tc: Union[ToolCall, dict]) -> OpenAIMessageToolCall:
    call_id = lc_tc.get("id") if isinstance(lc_tc, dict) else lc_tc.id
    name = lc_tc.get("name") if isinstance(lc_tc, dict) else lc_tc.name
    args = lc_tc.get("args") if isinstance(lc_tc, dict) else lc_tc.args

    return OpenAIMessageToolCall(
        id=call_id or f"call_{uuid.uuid4().hex}",
        type="function",
        function=OpenAIMessageToolCallFunction(
            name=name,
            arguments=json.dumps(args or {}) 
        )
    )

def convert_lc_msg_to_openai_output(lc_msg: BaseMessage) -> OpenAIMessageOutput:
    role = "assistant" 
    content: Optional[str] = None
    tool_calls_openai: Optional[List[OpenAIMessageToolCall]] = None

    if isinstance(lc_msg, AIMessage):
        role = "assistant"
        if lc_msg.tool_calls and len(lc_msg.tool_calls) > 0:
            tool_calls_openai = [lc_tool_call_to_openai_message_tool_call(tc) for tc in lc_msg.tool_calls]
            if not lc_msg.content: 
                content = None
            else:
                content = lc_msg.content
        else: 
            content = lc_msg.content if lc_msg.content is not None else "" 
    else:
        logger.warning(f"Unexpected LangChain message type for final output: {type(lc_msg)}. Content: {lc_msg.content}")
        content = str(lc_msg.content) if hasattr(lc_msg, 'content') else str(lc_msg)

    return OpenAIMessageOutput(role=role, content=content, tool_calls=tool_calls_openai)


# --- Chat Completions Endpoint ---
@app.post("/v1/chat/completions")
async def chat_completions(request: OpenAIChatCompletionRequest):
    request_id = f"chatcmpl-{uuid.uuid4().hex}"
    created_time = int(time.time())

    langchain_messages = convert_openai_req_to_lc_msgs(request.messages)
    agent_input: AgentState = {"messages": langchain_messages}
    
    # TODO: Add handling for request.tools and request.tool_choice 

    if request.stream:
        async def event_generator():
            stream_id = request_id # OpenAI uses same ID for all chunks in a stream
            first_chunk_sent = False
            
            final_graph_output_messages: Optional[List[BaseMessage]] = None
            config = {"recursion_limit": 150}

            async for event in mcp_graph.astream_events(agent_input, config=config, version="v2"):
                event_name = event["event"]
                data = event["data"]

                if event_name == "on_chat_model_stream":
                    chunk_payload: AIMessageChunk = data.get("chunk")
                    if not chunk_payload: continue

                    choice_idx = 0 
                    delta = OpenAIChatCompletionChunkDelta()

                    if not first_chunk_sent:
                        delta.role = "assistant"
                        first_chunk_sent = True
                    
                    if chunk_payload.content:
                        delta.content = chunk_payload.content
                    
                    if chunk_payload.tool_call_chunks:
                        openai_tool_call_deltas: List[OpenAIMessageToolCall] = []
                        for lc_tc_chunk in chunk_payload.tool_call_chunks:
                            # LangChain ToolCallChunk: name, args (str), id, index
                            # OpenAI delta.tool_calls: List[OpenAIMessageToolCall], each needs index, id, type, function {name, arguments}
                            openai_tc_chunk_index = lc_tc_chunk.get("index")
                            if openai_tc_chunk_index is None: continue

                            openai_tool_call_deltas.append(OpenAIMessageToolCall(
                                index=openai_tc_chunk_index,
                                id=lc_tc_chunk.get("id"),
                                type="function",
                                function=OpenAIMessageToolCallFunction(
                                    name=lc_tc_chunk.get("name"),
                                    arguments=lc_tc_chunk.get("args") 
                                )
                            ))
                        
                        if openai_tool_call_deltas:
                            delta.tool_calls = openai_tool_call_deltas
                    
                    if delta.role or delta.content or delta.tool_calls:
                        choice_chunk = OpenAIChoiceChunk(index=choice_idx, delta=delta)
                        api_chunk = OpenAIChatCompletionChunk(
                            id=stream_id, model=request.model, created=created_time, choices=[choice_chunk]
                        )
                        yield f"data: {api_chunk.model_dump_json(exclude_none=True)}\n\n"
                
                elif event_name == "on_graph_end": 
                    if isinstance(data.get("output"), dict): 
                        final_graph_output_messages = data["output"].get("messages", [])

            finish_reason_str = "stop" 
            if final_graph_output_messages:
                last_msg_from_graph = final_graph_output_messages[-1]
                if isinstance(last_msg_from_graph, AIMessage) and \
                   last_msg_from_graph.tool_calls and \
                   len(last_msg_from_graph.tool_calls) > 0:
                    finish_reason_str = "tool_calls"
            
            final_delta = OpenAIChatCompletionChunkDelta() 
            final_choice = OpenAIChoiceChunk(index=0, delta=final_delta, finish_reason=finish_reason_str)
            final_api_chunk = OpenAIChatCompletionChunk(
                id=stream_id, model=request.model, created=created_time, choices=[final_choice]
            )
            yield f"data: {final_api_chunk.model_dump_json(exclude_none=True)}\n\n"
            yield "data: [DONE]\n\n"
        
        return StreamingResponse(event_generator(), media_type="text/event-stream")
    else:
        # --- Non-Streaming Logic ---
        try:
            graph_output: AgentState = mcp_graph.invoke(
                agent_input, 
                config={"recursion_limit": 150} 
            )
        except Exception as e:
            logger.error(f"Error invoking mcp_graph: {e}", exc_info=True)
            return OpenAIChatCompletionResponse(
                id=request_id, created=created_time, model=request.model,
                choices=[OpenAIChoiceOutput(
                    index=0,
                    message=OpenAIMessageOutput(role="assistant", content=f"Agent execution error: {str(e)}"),
                    finish_reason="error" 
                )],
                usage=OpenAIUsageOutput()
            )

        final_lc_messages = graph_output.get("messages", [])
        if not final_lc_messages:
            logger.error("No messages found in mcp_graph output.")
            return OpenAIChatCompletionResponse(
                id=request_id, created=created_time, model=request.model,
                choices=[OpenAIChoiceOutput(
                    index=0,
                    message=OpenAIMessageOutput(role="assistant", content="Agent produced no output."),
                    finish_reason="error" 
                )],
                usage=OpenAIUsageOutput()
            )

        last_lc_message = final_lc_messages[-1]
        openai_message_output = convert_lc_msg_to_openai_output(last_lc_message)
        
        finish_reason = "stop"
        if isinstance(last_lc_message, AIMessage) and last_lc_message.tool_calls and len(last_lc_message.tool_calls) > 0:
            finish_reason = "tool_calls"
        
        usage_data = OpenAIUsageOutput(prompt_tokens=0, completion_tokens=0, total_tokens=0) # Placeholder
        
        return OpenAIChatCompletionResponse(
            id=request_id,
            created=created_time,
            model=request.model,
            choices=[
                OpenAIChoiceOutput(
                    index=0,
                    message=openai_message_output,
                    finish_reason=finish_reason
                )
            ],
            usage=usage_data
        )

if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=8000)
```
