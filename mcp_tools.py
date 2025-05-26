from typing import Annotated, List, TypedDict, Union, Optional
from langchain_core.messages import BaseMessage
from langgraph.graph.message import add_messages
from langchain_core.tools import tool
import httpx
import json
import os

# --- Agent State Definition ---
class AgentState(TypedDict):
    messages: Annotated[List[BaseMessage], add_messages]

# --- MCP HTTP Client Setup ---

# Placeholder constants - these should be configured securely in a real environment
MCP_SERVER_BASE_URL: str = os.getenv("MCP_SERVER_BASE_URL", "http://localhost:8080/plugins/com.mattermost.mcp-plugin/api/v1")
MCP_AGENT_SECRET: str = os.getenv("MCP_AGENT_SECRET", "this_is_a_secret_mcp_agent_secret_string")

# Global httpx client instance
# Timeouts can be adjusted as needed.
# Default timeout is 5 seconds for all operations.
# Consider making timeout configurable if longer operations are expected.
mcp_client = httpx.Client(
    base_url=MCP_SERVER_BASE_URL,
    headers={
        "Content-Type": "application/json",
        "X-MCP-Agent-Secret": MCP_AGENT_SECRET,
    },
    timeout=10.0, # Default timeout for all operations
)

# --- MCP Tool Definitions ---

@tool
def mcp_find_user(user_id: str) -> str:
    """
    Finds a user by their user_id using the MCP server.
    Returns a JSON string with user details if found, or an error message.
    """
    if not user_id or not isinstance(user_id, str):
        return "Error: Invalid user_id provided. Must be a non-empty string."
    
    try:
        response = mcp_client.post("/find_user", json={"user_id": user_id})
        response.raise_for_status()  # Raise an exception for 4XX/5XX errors
        
        # Assuming successful response returns user details as JSON
        user_data = response.json()
        return json.dumps(user_data, indent=2)
        
    except httpx.HTTPStatusError as e:
        if e.response.status_code == 404:
            return f"Error: User not found. (Details: {e.response.text})"
        return f"Error: MCP server returned status {e.response.status_code}. Response: {e.response.text}"
    except httpx.RequestError as e:
        return f"Error: Request to MCP server failed: {e}"
    except json.JSONDecodeError:
        return "Error: Failed to decode JSON response from MCP server."

@tool
def mcp_message_user(user_id: str, message: str) -> str:
    """
    Sends a direct message to a user_id via the MCP server.
    Returns a success message or an error message.
    """
    if not user_id or not isinstance(user_id, str):
        return "Error: Invalid user_id provided. Must be a non-empty string."
    if not message or not isinstance(message, str):
        return "Error: Invalid message provided. Must be a non-empty string."

    try:
        payload = {"user_id": user_id, "message": message}
        response = mcp_client.post("/message_user", json=payload)
        response.raise_for_status() # Raise an exception for 4XX/5XX errors
        
        # Assuming successful response is JSON e.g., {"status": "ok", "message": "Message sent successfully"}
        # Or potentially just a 200 OK with a simple body.
        # The design doc for MCP API handler returns:
        # `{"status": "ok", "message": "Message sent successfully"}`
        try:
            json_response = response.json()
            if isinstance(json_response, dict) and json_response.get("status") == "ok":
                return json_response.get("message", "Message sent successfully.")
            # If the response is not as expected, return its string representation
            return f"MCP server response: {response.text}"
        except json.JSONDecodeError:
            # If response is not JSON but status was 2xx, consider it success or log/return text
            if 200 <= response.status_code < 300:
                 return f"Message sent (status {response.status_code}), but response was not JSON: {response.text}"
            return f"Error: Failed to decode JSON response from MCP server, status {response.status_code}: {response.text}"

    except httpx.HTTPStatusError as e:
        if e.response.status_code == 404: # User not found, or channel creation issue
            return f"Error: User not found or DM channel could not be created/retrieved. (Details: {e.response.text})"
        elif e.response.status_code == 403: # Permissions issue
             return f"Error: Permission denied by MCP server. (Details: {e.response.text})"
        return f"Error: MCP server returned status {e.response.status_code}. Response: {e.response.text}"
    except httpx.RequestError as e:
        return f"Error: Request to MCP server failed: {e}"

@tool
def mcp_find_team_and_members(team_id: str) -> str:
    """
    Finds a team and its members by team_id using the MCP server.
    Returns a JSON string with team details and member list if found, or an error message.
    """
    if not team_id or not isinstance(team_id, str):
        return "Error: Invalid team_id provided. Must be a non-empty string."

    try:
        response = mcp_client.post("/find_group_channels_teams", json={"team_id": team_id})
        response.raise_for_status()
        data = response.json()
        return json.dumps(data, indent=2)
    except httpx.HTTPStatusError as e:
        if e.response.status_code == 404:
            return f"Error: Team not found. (Details: {e.response.text})"
        return f"Error: MCP server returned status {e.response.status_code}. Response: {e.response.text}"
    except httpx.RequestError as e:
        return f"Error: Request to MCP server failed: {e}"
    except json.JSONDecodeError:
        return "Error: Failed to decode JSON response from MCP server."

@tool
def mcp_find_channel_and_members(channel_id: str) -> str:
    """
    Finds a channel and its members by channel_id using the MCP server.
    Returns a JSON string with channel details and member list if found, or an error message.
    """
    if not channel_id or not isinstance(channel_id, str):
        return "Error: Invalid channel_id provided. Must be a non-empty string."

    try:
        response = mcp_client.post("/find_group_channels_teams", json={"channel_id": channel_id})
        response.raise_for_status()
        data = response.json()
        return json.dumps(data, indent=2)
    except httpx.HTTPStatusError as e:
        if e.response.status_code == 404:
            return f"Error: Channel not found. (Details: {e.response.text})"
        return f"Error: MCP server returned status {e.response.status_code}. Response: {e.response.text}"
    except httpx.RequestError as e:
        return f"Error: Request to MCP server failed: {e}"
    except json.JSONDecodeError:
        return "Error: Failed to decode JSON response from MCP server."

@tool
def mcp_reply_to_message(post_id: str, message: str) -> str:
    """
    Posts a reply to a specific message (post_id) via the MCP server.
    Returns a JSON string of the created reply post or an error message.
    """
    if not post_id or not isinstance(post_id, str):
        return "Error: Invalid post_id provided. Must be a non-empty string."
    if not message or not isinstance(message, str):
        return "Error: Invalid message provided. Must be a non-empty string."

    try:
        payload = {"post_id": post_id, "message": message}
        response = mcp_client.post("/reply_to_message", json=payload)
        response.raise_for_status() 
        reply_post_data = response.json()
        return json.dumps(reply_post_data, indent=2)
    except httpx.HTTPStatusError as e:
        if e.response.status_code == 404:
            return f"Error: Original post not found or replying is not allowed. (Details: {e.response.text})"
        return f"Error: MCP server returned status {e.response.status_code} when replying. Response: {e.response.text}"
    except httpx.RequestError as e:
        return f"Error: Request to MCP server failed: {e}"
    except json.JSONDecodeError:
        return "Error: Failed to decode JSON response from MCP server for reply."

@tool
def mcp_create_team(team_name: str, team_display_name: str, user_ids: Optional[List[str]] = None) -> str:
    """
    Creates a new team and optionally adds users to it via the MCP server.
    Returns a JSON string of the created team or an error message.
    """
    if not team_name or not isinstance(team_name, str):
        return "Error: Invalid team_name provided. Must be a non-empty string."
    if not team_display_name or not isinstance(team_display_name, str):
        return "Error: Invalid team_display_name provided. Must be a non-empty string."
    if user_ids is None:
        user_ids = []
    if not isinstance(user_ids, list) or not all(isinstance(uid, str) for uid in user_ids):
        return "Error: Invalid user_ids provided. Must be a list of strings."

    try:
        payload = {
            "type": "team",
            "team_name": team_name,
            "team_display_name": team_display_name,
            "user_ids": user_ids,
        }
        response = mcp_client.post("/create_group_channels_teams", json=payload)
        response.raise_for_status() # Handles 4xx/5xx for actual errors like 400 bad request, 500 internal
        
        created_team_data = response.json()
        return json.dumps(created_team_data, indent=2)
    except httpx.HTTPStatusError as e:
        # Specific error for team name conflict (example, depends on actual API response)
        if e.response.status_code == 400 and "name exists" in e.response.text.lower(): # Or similar message
            return f"Error: Team creation failed, name '{team_name}' may already exist or is invalid. (Details: {e.response.text})"
        return f"Error: MCP server returned status {e.response.status_code} during team creation. Response: {e.response.text}"
    except httpx.RequestError as e:
        return f"Error: Request to MCP server failed: {e}"
    except json.JSONDecodeError:
        return "Error: Failed to decode JSON response from MCP server for team creation."

@tool
def mcp_create_channel(team_id: str, channel_name: str, channel_display_name: str, channel_type: str, user_ids: Optional[List[str]] = None) -> str:
    """
    Creates a new channel within a team and optionally adds users to it via the MCP server.
    channel_type must be 'O' (Open/Public) or 'P' (Private).
    Returns a JSON string of the created channel or an error message.
    """
    if not team_id or not isinstance(team_id, str):
        return "Error: Invalid team_id provided. Must be a non-empty string."
    if not channel_name or not isinstance(channel_name, str):
        return "Error: Invalid channel_name provided. Must be a non-empty string."
    if not channel_display_name or not isinstance(channel_display_name, str):
        return "Error: Invalid channel_display_name provided. Must be a non-empty string."
    if channel_type not in ["O", "P"]:
        return "Error: Invalid channel_type provided. Must be 'O' (Open) or 'P' (Private)."
    if user_ids is None:
        user_ids = []
    if not isinstance(user_ids, list) or not all(isinstance(uid, str) for uid in user_ids):
        return "Error: Invalid user_ids provided. Must be a list of strings."

    try:
        payload = {
            "type": "channel",
            "team_id": team_id,
            "channel_name": channel_name,
            "channel_display_name": channel_display_name,
            "channel_type": channel_type,
            "user_ids": user_ids,
        }
        response = mcp_client.post("/create_group_channels_teams", json=payload)
        response.raise_for_status()
        created_channel_data = response.json()
        return json.dumps(created_channel_data, indent=2)
    except httpx.HTTPStatusError as e:
        if e.response.status_code == 400: # General bad request (e.g. name exists, invalid team_id)
             return f"Error: Channel creation failed. Team may not exist or channel name may be invalid/taken. (Details: {e.response.text})"
        if e.response.status_code == 404: # Specifically if team_id not found
             return f"Error: Team with ID '{team_id}' not found for channel creation. (Details: {e.response.text})"
        return f"Error: MCP server returned status {e.response.status_code} during channel creation. Response: {e.response.text}"
    except httpx.RequestError as e:
        return f"Error: Request to MCP server failed: {e}"
    except json.JSONDecodeError:
        return "Error: Failed to decode JSON response from MCP server for channel creation."

@tool
def mcp_post_message_to_channel(channel_id: str, message: str) -> str:
    """
    Posts a message to a specific channel_id via the MCP server.
    Returns a JSON string of the created post or an error message.
    """
    if not channel_id or not isinstance(channel_id, str):
        return "Error: Invalid channel_id provided. Must be a non-empty string."
    if not message or not isinstance(message, str):
        return "Error: Invalid message provided. Must be a non-empty string."

    try:
        payload = {"channel_id": channel_id, "message": message}
        # Note: MCP API uses /post_message_to_group_channels_teams for this
        response = mcp_client.post("/post_message_to_group_channels_teams", json=payload)
        response.raise_for_status()
        created_post_data = response.json()
        return json.dumps(created_post_data, indent=2)
    except httpx.HTTPStatusError as e:
        if e.response.status_code == 404:
            return f"Error: Channel not found or posting is not allowed. (Details: {e.response.text})"
        return f"Error: MCP server returned status {e.response.status_code} when posting message. Response: {e.response.text}"
    except httpx.RequestError as e:
        return f"Error: Request to MCP server failed: {e}"
    except json.JSONDecodeError:
        return "Error: Failed to decode JSON response from MCP server for post creation."

all_mcp_tools = [
    mcp_find_user, 
    mcp_message_user,
    mcp_find_team_and_members,
    mcp_find_channel_and_members,
    mcp_reply_to_message,
    mcp_create_team,
    mcp_create_channel,
    mcp_post_message_to_channel,
]

# Example usage (optional, for testing)
if __name__ == "__main__":
    print("MCP Tools Module - Extended")
    print("---")
    # Add example calls for new tools here if needed for standalone testing,
    # ensuring the MCP server is running and configured.
    pass
```
