"""Socket Mode entrypoint for local development.

No public URL needed — connects outbound to Slack via WebSocket.
Run with: python socket_mode.py

For production, use main.py (FastAPI + HTTP endpoints) instead.
"""

import asyncio
import json
import logging

from slack_bolt.async_app import AsyncApp
from slack_bolt.adapter.socket_mode.async_handler import AsyncSocketModeHandler
from slack_sdk.web.async_client import AsyncWebClient

from agent.agent import TaskAgent
from agent.conversations import ConversationStore
from config import settings
from events.listener import EventListener
from interactions.handler import InteractionHandler
from messages.blocks import build_create_task_modal, build_edit_modal, replace_actions_with_status
from resolver.identity import IdentityResolver
from taskboard.client import TaskboardClient

logging.basicConfig(level=logging.INFO, format="%(asctime)s %(name)s %(levelname)s %(message)s")
log = logging.getLogger(__name__)

# --- Shared state ---

taskboard = TaskboardClient(settings.taskboard_url, settings.taskboard_api_key)
resolver = IdentityResolver(taskboard)
task_agent = TaskAgent(settings.anthropic_api_key, taskboard)
conversations = ConversationStore()

bolt_app = AsyncApp(token=settings.slack_bot_token)


# --- /task slash command ---


@bolt_app.command("/task")
async def handle_task_command(ack, command, respond):
    text = (command.get("text") or "").strip()

    if not text:
        await ack(
            ":clipboard: *Usage:* `/task <what you want to do>`\n\n"
            "Examples:\n"
            "- `/task what are my priorities today?`\n"
            "- `/task mark T-2026-00042 as done`\n"
            "- `/task create: follow up with ACME by Friday`\n"
            "- `/task I'm blocked on the vendor review`\n\n"
            "You can also DM me directly for a conversation."
        )
        return

    if text.lower() == "clear":
        conversations.clear(command["user_id"])
        await ack(":wastebasket: Conversation cleared.")
        return

    agent = await resolver.slack_to_agent(command["user_id"])
    if not agent:
        await ack(":x: You're not registered in Taskboard. Ask an admin to set your `slack_id`.")
        return

    await ack(":hourglass_flowing_sand: Thinking...")

    history = conversations.get(command["user_id"])
    try:
        result = await task_agent.run(
            agent_id=agent["id"],
            agent_name=agent.get("name", agent["id"]),
            user_input=text,
            history=history,
        )
        conversations.add_turn(command["user_id"], text, result)
        await respond(result)
    except Exception:
        log.exception("Agent failed for %s", agent["id"])
        await respond(":x: Something went wrong. Please try again.")


# --- Approval buttons ---


@bolt_app.action("approve_task")
async def handle_approve(ack, action, body, client):
    await ack()
    task_id = action["value"]
    channel = body["channel"]["id"]
    log.info("Approving task %s", task_id)

    try:
        await taskboard.update_task(task_id, {"status": "pending", "visibility": "public"})
        status_label = "approved"
    except Exception as e:
        log.warning("Could not approve %s: %s", task_id, e)
        await client.chat_postMessage(channel=channel, text=f":warning: Failed to approve `{task_id}`: {e}")
        status_label = "approved"  # Still update buttons — task was likely already approved

    message = body.get("message", {})
    blocks = replace_actions_with_status(message.get("blocks", []), task_id, status_label)
    await client.chat_update(channel=channel, ts=message["ts"], blocks=blocks)


@bolt_app.action("reject_task")
async def handle_reject(ack, action, body, client):
    await ack()
    task_id = action["value"]
    channel = body["channel"]["id"]
    log.info("Rejecting task %s", task_id)

    try:
        await taskboard.update_task(task_id, {"status": "cancelled"})
    except Exception as e:
        log.warning("Could not reject %s: %s", task_id, e)
        await client.chat_postMessage(channel=channel, text=f":warning: Failed to reject `{task_id}`: {e}")

    message = body.get("message", {})
    blocks = replace_actions_with_status(message.get("blocks", []), task_id, "rejected")
    await client.chat_update(channel=channel, ts=message["ts"], blocks=blocks)


@bolt_app.action("edit_task")
async def handle_edit(ack, action, body, client):
    await ack()
    task_id = action["value"]

    # Store original message reference so modal submit can update buttons
    message = body.get("message", {})
    channel_id = body["channel"]["id"]
    message_ts = message.get("ts", "")
    metadata = json.dumps({"task_id": task_id, "channel": channel_id, "ts": message_ts})

    task = await taskboard.get_task(task_id)
    agents = await taskboard.list_agents()
    modal = build_edit_modal(task, agents)
    modal["private_metadata"] = metadata
    await client.views_open(trigger_id=body["trigger_id"], view=modal)


@bolt_app.action("approve_all")
async def handle_approve_all(ack, action, body, client):
    await ack()
    task_ids = json.loads(action["value"])

    for tid in task_ids:
        await taskboard.update_task(tid, {"status": "pending", "visibility": "public"})
        log.info("Approved task %s", tid)

    # Update message: replace all action buttons with status, remove approve-all
    message = body.get("message", {})
    blocks = message.get("blocks", [])
    for tid in task_ids:
        blocks = replace_actions_with_status(blocks, tid, "approved")
    blocks = [b for b in blocks if b.get("block_id") != "actions_all"]

    await client.chat_update(
        channel=body["channel"]["id"],
        ts=message["ts"],
        blocks=blocks,
    )


# --- Edit modal submission ---


@bolt_app.view("edit_task_modal")
async def handle_edit_submit(ack, view, body, client):
    await ack()
    metadata = json.loads(view["private_metadata"])
    task_id = metadata["task_id"]
    channel = metadata.get("channel", "")
    message_ts = metadata.get("ts", "")
    values = view["state"]["values"]

    title = values["title_block"]["title_input"]["value"]
    visibility = values["visibility_block"]["visibility_input"]["selected_option"]["value"]
    assignee_field = values["assignee_block"]["assignee_input"]
    assignee = (assignee_field.get("selected_option") or {}).get("value") or assignee_field.get("value") or ""
    updates = {
        "title": title,
        "assigned_to": assignee,
        "priority": values["priority_block"]["priority_input"]["selected_option"]["value"],
        "visibility": visibility,
        "description": values["description_block"]["description_input"]["value"],
    }

    # First update the fields, then transition status separately
    log.info("Approving task %s with edits", task_id)
    user_id = body["user"]["id"]
    try:
        await taskboard.update_task(task_id, updates)
        # Now transition draft → pending
        await taskboard.update_task(task_id, {"status": "pending"})
    except Exception as e:
        log.exception("Could not fully update %s", task_id)
        await client.chat_postMessage(
            channel=user_id,
            text=f":x: Failed to approve `{task_id}` with edits: {e}",
        )
        return

    # Update the original approval message to show edited status
    if channel and message_ts:
        try:
            msg = await client.conversations_history(channel=channel, latest=message_ts, limit=1, inclusive=True)
            if msg["messages"]:
                blocks = replace_actions_with_status(msg["messages"][0].get("blocks", []), task_id, "edited")
                await client.chat_update(channel=channel, ts=message_ts, blocks=blocks)
        except Exception:
            log.warning("Could not update original message for %s", task_id)

    # Send confirmation DM
    await client.chat_postMessage(
        channel=user_id,
        text=f":pencil: Task `{task_id}` approved with edits.\n*{title}* — {visibility}, {updates['priority']} priority, assigned to {updates['assigned_to']}",
    )


# --- Message shortcut: Create Task ---


@bolt_app.shortcut("create_task_from_message")
async def handle_create_task_shortcut(ack, shortcut, client):
    await ack()

    message = shortcut.get("message", {})
    message_text = message.get("text", "")
    message_user = message.get("user", "")
    channel = shortcut.get("channel", {})
    channel_name = channel.get("name", "")

    # Resolve message author name
    author_name = message_user
    if message_user:
        try:
            user_info = await client.users_info(user=message_user)
            author_name = user_info["user"].get("real_name", user_info["user"].get("name", message_user))
        except Exception:
            pass

    # Store channel info for the confirmation message
    metadata = json.dumps({
        "channel": channel.get("id", ""),
        "message_ts": message.get("ts", ""),
    })

    agents = await taskboard.list_agents()
    modal = build_create_task_modal(
        message_text=message_text,
        message_author=author_name,
        channel_name=channel_name,
        metadata=metadata,
        agents=agents,
    )
    await client.views_open(trigger_id=shortcut["trigger_id"], view=modal)


@bolt_app.view("create_task_modal")
async def handle_create_task_submit(ack, body, view, client):
    await ack()
    metadata = json.loads(view.get("private_metadata", "{}"))
    values = view["state"]["values"]
    user_id = body["user"]["id"]

    # Resolve the Slack user to an agent
    agent = await resolver.slack_to_agent(user_id)
    if not agent:
        await client.chat_postMessage(channel=user_id, text=":x: You're not registered in Taskboard.")
        return

    title = values["title_block"]["title_input"]["value"]
    assignee_field = values["assignee_block"]["assignee_input"]
    assignee = ((assignee_field.get("selected_option") or {}).get("value") or assignee_field.get("value") or "").strip()
    priority = values["priority_block"]["priority_input"]["selected_option"]["value"]
    visibility = values["visibility_block"]["visibility_input"]["selected_option"]["value"]
    description = values["description_block"]["description_input"]["value"]

    # Deadline (optional datepicker)
    deadline_data = values["deadline_block"]["deadline_input"]
    deadline = deadline_data.get("selected_date")

    body = {
        "title": title,
        "priority": priority,
        "visibility": visibility,
        "description": description,
    }
    if assignee:
        body["assigned_to"] = assignee
    if deadline:
        body["deadline"] = f"{deadline}T23:59:59Z"

    try:
        result = await taskboard.create_task(body, agent_id=agent["id"])
        task_id = result.get("id", "?")
        msg = f":white_check_mark: Task created: `{task_id}` — *{title}*"
        if assignee:
            msg += f"\nAssigned to: {assignee}"
        if deadline:
            msg += f"\nDeadline: {deadline}"
        await client.chat_postMessage(channel=user_id, text=msg)
    except Exception as e:
        log.exception("Failed to create task from message shortcut")
        await client.chat_postMessage(channel=user_id, text=f":x: Failed to create task: {e}")


# --- @Jarvis mentions (threads + channels) ---


@bolt_app.event("app_mention")
async def handle_mention(event, client):
    """Handle @Jarvis mentions — especially in threads to create tasks from context."""
    channel = event.get("channel", "")
    thread_ts = event.get("thread_ts") or event.get("ts")
    user_text = (event.get("text") or "").strip()
    slack_user_id = event.get("user", "")

    # Strip the @Jarvis mention from the text
    # Slack sends it as <@BOTID> — remove any <@...> prefix
    import re
    user_text = re.sub(r"<@[A-Z0-9]+>\s*", "", user_text).strip()

    if not user_text:
        await client.chat_postMessage(
            channel=channel,
            thread_ts=thread_ts,
            text="Tag me with what you need — e.g. `@Jarvis create a task from this, deadline Friday`",
        )
        return

    agent = await resolver.slack_to_agent(slack_user_id)
    if not agent:
        await client.chat_postMessage(
            channel=channel,
            thread_ts=thread_ts,
            text=":x: You're not registered in Taskboard. Ask an admin to set your `slack_id`.",
        )
        return

    # Post thinking indicator in the thread
    thinking = await client.chat_postMessage(
        channel=channel,
        thread_ts=thread_ts,
        text=":hourglass_flowing_sand: Thinking...",
    )

    # Fetch thread context if we're in a thread
    thread_context = ""
    if event.get("thread_ts"):
        try:
            thread = await client.conversations_replies(channel=channel, ts=event["thread_ts"], limit=20)
            messages = thread.get("messages", [])
            # Build context from thread messages (excluding the @Jarvis mention itself)
            context_parts = []
            for msg in messages:
                if msg.get("ts") == event.get("ts"):
                    continue  # skip the mention itself
                author = msg.get("user", "unknown")
                # Try to resolve author name
                try:
                    user_info = await client.users_info(user=author)
                    author = user_info["user"].get("real_name", author)
                except Exception:
                    pass
                context_parts.append(f"[{author}]: {msg.get('text', '')}")
            if context_parts:
                thread_context = "\n".join(context_parts)
        except Exception:
            log.warning("Could not fetch thread context for channel=%s ts=%s", channel, event.get("thread_ts"))

    # Build the prompt — include thread context if available
    prompt = user_text
    if thread_context:
        prompt = f"Thread context:\n---\n{thread_context}\n---\n\nUser request: {user_text}"

    history = conversations.get(slack_user_id)
    try:
        result = await task_agent.run(
            agent_id=agent["id"],
            agent_name=agent.get("name", agent["id"]),
            user_input=prompt,
            history=history,
        )
        conversations.add_turn(slack_user_id, user_text, result)
        await client.chat_update(channel=channel, ts=thinking["ts"], text=result)
    except Exception:
        log.exception("Agent failed for @mention by %s", agent["id"])
        await client.chat_update(channel=channel, ts=thinking["ts"], text=":x: Something went wrong. Please try again.")


# --- DM chat ---


@bolt_app.event("message")
async def handle_dm(event, client):
    """Handle direct messages to Jarvis — conversational task management."""
    # Ignore bot messages (including our own), edits, and non-im channels
    if event.get("bot_id") or event.get("subtype"):
        return

    text = (event.get("text") or "").strip()
    if not text:
        return

    slack_user_id = event.get("user", "")
    channel = event.get("channel", "")

    if text.lower() == "clear":
        conversations.clear(slack_user_id)
        await client.chat_postMessage(channel=channel, text=":wastebasket: Conversation cleared.")
        return

    agent = await resolver.slack_to_agent(slack_user_id)
    if not agent:
        await client.chat_postMessage(channel=channel, text=":x: You're not registered in Taskboard. Ask an admin to set your `slack_id`.")
        return

    # Post a thinking placeholder, then update it with the real response
    thinking = await client.chat_postMessage(channel=channel, text=":hourglass_flowing_sand: Thinking...")

    history = conversations.get(slack_user_id)
    try:
        result = await task_agent.run(
            agent_id=agent["id"],
            agent_name=agent.get("name", agent["id"]),
            user_input=text,
            history=history,
        )
        conversations.add_turn(slack_user_id, text, result)
        await client.chat_update(channel=channel, ts=thinking["ts"], text=result)
    except Exception:
        log.exception("Agent failed for %s in DM", agent["id"])
        await client.chat_update(channel=channel, ts=thinking["ts"], text=":x: Something went wrong. Please try again.")


# --- Main ---


async def main():
    if not settings.slack_app_token:
        log.error("SLACK_APP_TOKEN is required for Socket Mode. Generate one at api.slack.com/apps → Basic Information → App-Level Tokens.")
        return

    # Start SSE event listener in background
    slack_client = AsyncWebClient(token=settings.slack_bot_token)
    listener = EventListener(
        taskboard=taskboard,
        slack=slack_client,
        resolver=resolver,
        admin_slack_id=settings.admin_slack_id,
    )
    asyncio.create_task(listener.listen_forever())

    # Start Socket Mode
    handler = AsyncSocketModeHandler(bolt_app, settings.slack_app_token)
    log.info("Starting Taskboard Slack App in Socket Mode (local dev)")
    await handler.start_async()


if __name__ == "__main__":
    asyncio.run(main())
