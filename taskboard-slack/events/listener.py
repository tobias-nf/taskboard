"""Subscribe to Taskboard SSE events and react to draft task creation."""

import asyncio
import json
import logging

import httpx
from slack_sdk.web.async_client import AsyncWebClient

from messages.blocks import build_draft_approval
from resolver.identity import IdentityResolver
from taskboard.client import TaskboardClient

log = logging.getLogger(__name__)


class EventListener:
    """Connects to the Taskboard SSE stream and sends Slack notifications for relevant events."""

    def __init__(self, taskboard: TaskboardClient, slack: AsyncWebClient, resolver: IdentityResolver, admin_slack_id: str):
        self._taskboard = taskboard
        self._slack = slack
        self._resolver = resolver
        self._admin_slack_id = admin_slack_id
        # Buffer drafts from the same source to batch approval messages
        self._draft_buffer: dict[str, list[dict]] = {}  # created_by -> [tasks]
        self._flush_task: asyncio.Task | None = None

    async def listen_forever(self):
        """Reconnecting SSE listener. Runs as a background task."""
        backoff = 5
        while True:
            try:
                # On every (re)connect, catch up on any draft tasks that were
                # created while we were disconnected
                await self._catch_up_drafts()
                await self._connect()
                backoff = 5  # Reset on successful connection
            except Exception:
                log.exception("SSE connection lost, reconnecting in %ds", backoff)
                await asyncio.sleep(backoff)
                backoff = min(backoff * 2, 60)

    async def _catch_up_drafts(self):
        """Poll for existing draft tasks and send approval messages for any that haven't been notified yet."""
        try:
            drafts = await self._taskboard.list_visible_tasks(status="draft")
            if drafts:
                log.info("Catching up on %d draft tasks", len(drafts))
                for task in drafts:
                    await self._handle_new_draft(task)
        except Exception:
            log.exception("Failed to catch up on draft tasks")

    async def _connect(self):
        async with httpx.AsyncClient(timeout=None) as client:
            async with client.stream(
                "GET",
                self._taskboard.events_url(),
                headers=self._taskboard.auth_headers(),
            ) as stream:
                if stream.status_code != 200:
                    body = await stream.aread()
                    raise RuntimeError(f"SSE connection failed: HTTP {stream.status_code} — {body[:200]}")
                log.info("Connected to Taskboard SSE event stream")
                event_type = ""
                data_lines: list[str] = []

                async for line in stream.aiter_lines():
                    if line.startswith("event: "):
                        event_type = line[7:].strip()
                    elif line.startswith("data: "):
                        data_lines.append(line[6:])
                    elif line == "" and event_type:
                        # End of event
                        raw = "\n".join(data_lines)
                        data_lines = []
                        try:
                            event = json.loads(raw)
                            await self._handle_event(event_type, event)
                        except json.JSONDecodeError:
                            log.warning("Failed to parse SSE data: %s", raw[:200])
                        event_type = ""
                    elif line.startswith(":"):
                        pass  # heartbeat

    async def _handle_event(self, event_type: str, event: dict):
        task_id = event.get("task_id", "")

        if event_type == "task.created":
            # Fetch full task to check if it's a draft
            try:
                task = await self._taskboard.get_task(task_id)
            except Exception:
                log.warning("Could not fetch task %s for event handling", task_id)
                return

            if task.get("status") == "draft":
                await self._handle_new_draft(task)

    async def _handle_new_draft(self, task: dict):
        """Send approval message to stakeholders for a new draft task."""
        task_id = task.get("id", "")

        # Find who should approve — stakeholders (owed_to)
        try:
            owed_to = await self._taskboard.get_owed_to(task_id)
        except Exception:
            owed_to = []

        stakeholder_ids = [e.get("agent_id", "") for e in (owed_to or []) if e.get("agent_id")]

        if not stakeholder_ids:
            log.warning("Draft task %s has no stakeholders, notifying admin", task_id)
            stakeholder_ids = []
            if self._admin_slack_id:
                await self._slack.chat_postMessage(
                    channel=self._admin_slack_id,
                    text=f":warning: Draft task `{task_id}` has no stakeholders for approval. "
                         f"Title: *{task.get('title', '')}*",
                )
            return

        # Send approval message to each stakeholder
        blocks = build_draft_approval(task)
        for agent_id in stakeholder_ids:
            slack_id = await self._resolver.agent_to_slack(agent_id)
            if slack_id:
                await self._slack.chat_postMessage(
                    channel=slack_id,
                    blocks=blocks,
                    text=f"Draft task for approval: {task.get('title', '')}",
                )
                log.info("Sent approval for %s to %s (Slack: %s)", task_id, agent_id, slack_id)
            else:
                log.warning("Agent %s has no Slack ID, cannot send approval for %s", agent_id, task_id)
                if self._admin_slack_id:
                    await self._slack.chat_postMessage(
                        channel=self._admin_slack_id,
                        text=f":warning: Agent `{agent_id}` has no Slack ID. "
                             f"Cannot send approval for draft `{task_id}`: *{task.get('title', '')}*",
                    )
