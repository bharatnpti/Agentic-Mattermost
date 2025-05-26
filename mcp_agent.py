import json
from typing import List
import os

from langchain_core.messages import (
    AIMessage,
    BaseMessage,
    HumanMessage,
    ToolMessage,
)
from langchain_openai import ChatOpenAI # Using OpenAI for this example
# from langchain_anthropic import ChatAnthropic # Alternative LLM
from langgraph.graph import StateGraph, END, START # Import START

# Assuming mcp_tools.py is in the same directory or accessible in PYTHONPATH
from mcp_tools import AgentState, all_mcp_tools

# --- LLM Configuration ---

# Ensure OPENAI_API_KEY is set in your environment for ChatOpenAI
# Or ANTHROPIC_API_KEY for ChatAnthropic, etc.
# You can specify the model and other parameters as needed.
# For example, if you have access to GPT-4o:
llm = ChatOpenAI(model=os.getenv("OPENAI_MODEL_NAME", "gpt-4o"))
# Or for Claude (ensure langchain-anthropic is installed):
# llm = ChatAnthropic(model=os.getenv("ANTHROPIC_MODEL_NAME", "claude-3-sonnet-20240229"))


# Bind the tools to the LLM.
# This allows the LLM to be aware of the tools and their schemas,
# and to generate tool_calls when it deems appropriate.
llm_with_mcp_tools = llm.bind_tools(all_mcp_tools)


# --- Chatbot Node (Executor for LLM with Tools) ---
def call_llm(state: AgentState) -> dict:
    """
    Invokes the LLM with the current state of messages.
    The LLM is expected to have tools bound to it.
    """
    messages = state["messages"]
    # The LLM invocation might result in an AIMessage with content,
    # or an AIMessage with tool_calls if it decides to use a tool.
    response_from_llm = llm_with_mcp_tools.invoke(messages)
    return {"messages": [response_from_llm]}


# --- Tool Execution Node ---
def run_mcp_tools(state: AgentState) -> dict:
    """
    Executes tools based on the tool_calls in the last AIMessage.
    """
    messages = state["messages"]
    if not messages:
        return {"messages": []}

    last_message = messages[-1]

    # Check if the last message is an AIMessage and has tool_calls
    if not isinstance(last_message, AIMessage) or not last_message.tool_calls:
        # No tool calls, so nothing to execute for this node.
        # Return an empty list for messages to signify no new messages were added by tool execution.
        return {"messages": []}

    tool_messages: List[ToolMessage] = []
    for tool_call in last_message.tool_calls:
        tool_name = tool_call.get("name") # Langchain v0.2.0+ uses .get("name")
        if not tool_name: # Compatibility with older Langchain
            tool_name = tool_call.get("name")

        tool_to_use = None
        for tool_def in all_mcp_tools:
            if tool_def.name == tool_name:
                tool_to_use = tool_def
                break
        
        tool_output_content = ""
        if tool_to_use:
            try:
                # Invoke the tool with its arguments
                tool_output = tool_to_use.invoke(tool_call["args"])
                # Ensure content is a string
                tool_output_content = str(tool_output)
            except Exception as e:
                # Capture any exception during tool execution as content
                tool_output_content = f"Error executing tool {tool_name}: {e}"
        else:
            tool_output_content = f"Error: Tool '{tool_name}' not found in all_mcp_tools."

        tool_messages.append(
            ToolMessage(
                content=tool_output_content,
                tool_call_id=tool_call["id"], # Langchain v0.2.0+ uses .get("id")
                name=tool_name, # Optional, but good for clarity
            )
        )

    return {"messages": tool_messages}


# --- Example of how to potentially build and run the graph (not part of this file's direct execution) ---
# --- Graph Construction ---

graph_builder = StateGraph(AgentState)

# Add nodes
graph_builder.add_node("llm", call_llm)
graph_builder.add_node("tools", run_mcp_tools)

# Define entry point
graph_builder.add_edge(START, "llm")

# Define conditional edge from LLM node
def should_continue(state: AgentState) -> str:
    """
    Determines the next step after the LLM has run.
    If the LLM made tool calls, route to the 'tools' node.
    Otherwise, end the conversation.
    """
    messages = state["messages"]
    if not messages:
        return "end_conversation" # Should ideally not happen if entry is LLM
    
    last_message = messages[-1]
    if isinstance(last_message, AIMessage) and last_message.tool_calls:
        return "tools"
    return "end_conversation"

graph_builder.add_conditional_edges(
    "llm",
    should_continue,
    {
        "tools": "tools",
        "end_conversation": END,
    },
)

# Add edge from Tools node back to LLM node
graph_builder.add_edge("tools", "llm")

# Compile the graph
mcp_graph = graph_builder.compile()


# Example of how to potentially run the graph (not part of this file's direct execution)
if __name__ == "__main__":
    print("MCP Agent Graph Compiled. Ready for interaction (conceptual).")
    
    # Conceptual example of streaming and printing messages:
    # config = {"recursion_limit": 100}
    # inputs = {"messages": [HumanMessage(content="Hello! Can you find user 'testuser' and then message them 'Hi from the agent!'?")]}
    # 
    # async def _main():
    #     async for event in mcp_graph.astream_events(inputs, config=config, version="v1"):
    #         kind = event["event"]
    #         if kind == "on_chat_model_stream":
    #             content = event["data"]["chunk"].content
    #             if content:
    #                 print(content, end="|")
    #         elif kind == "on_tool_start":
    #             print("--")
    #             print(f"Starting tool: {event['name']} with inputs: {event['data'].get('input')}")
    #         elif kind == "on_tool_end":
    #             print(f"Tool {event['name']} finished. Output: {event['data'].get('output')}")
    #             print("--")
    #         # else:
    #         #     # Print other event types for debugging if needed
    #         #     print(event)


    # For synchronous example (less detailed output than stream_events):
    # final_state = mcp_graph.invoke(inputs, config=config)
    # print("\nFinal state messages:")
    # for msg in final_state.get("messages", []):
    #     if isinstance(msg, AIMessage):
    #         print(f"AI: {msg.content}")
    #         if msg.tool_calls:
    #             print(f"AI Tool Calls: {msg.tool_calls}")
    #     elif isinstance(msg, ToolMessage):
    #         print(f"Tool ({msg.name} ID: {msg.tool_call_id}): {msg.content}")
    #     elif isinstance(msg, HumanMessage):
    #         print(f"Human: {msg.content}")
    #     else:
    #         print(msg)

    pass
```
