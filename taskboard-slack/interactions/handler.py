"""Handle Slack interactive payloads — button clicks and modal submissions for task approvals."""

import json
import logging

from slack_sdk.web.async_client import AsyncWebClient

from messages.blocks import build_edit_modal, replace_actions_with_status
from taskboard.client import TaskboardClient

log = logging.getLogger(__name__)


class InteractionHandler:
    def __init__(self, slack: AsyncWebClient, taskboard: TaskboardClient):
        self._slack = slack
        self._taskboard = taskboard

    async def handle(self, payload: dict):
        payload_type = payload.get("type")

        if payload_type == "block_actions":
            for action in payload.get("actions", []):
                await self._handle_action(action, payload)

        elif payload_type == "view_submission":
            await self._handle_modal_submit(payload)

    async def _handle_action(self, action: dict, payload: dict):
        action_id = action["action_id"]
        value = action["value"]

        if action_id == "approve_task":
            await self._approve(value)
            await self._update_message(payload, value, "approved")

        elif action_id == "reject_task":
            await self._reject(value)
            await self._update_message(payload, value, "rejected")

        elif action_id == "edit_task":
            await self._open_edit_modal(value, payload["trigger_id"])

        elif action_id == "approve_all":
            task_ids = json.loads(value)
            for task_id in task_ids:
                await self._approve(task_id)
            await self._update_message_all(payload, task_ids, "approved")

    async def _approve(self, task_id: str):
        log.info("Approving draft task %s", task_id)
        await self._taskboard.update_task(task_id, {"status": "pending", "visibility": "public"})

    async def _reject(self, task_id: str):
        log.info("Rejecting draft task %s", task_id)
        await self._taskboard.update_task(task_id, {"status": "cancelled"})

    async def _open_edit_modal(self, task_id: str, trigger_id: str):
        task = await self._taskboard.get_task(task_id)
        agents = await self._taskboard.list_agents()
        modal = build_edit_modal(task, agents)
        await self._slack.views_open(trigger_id=trigger_id, view=modal)

    async def _handle_modal_submit(self, payload: dict):
        view = payload["view"]
        task_id = view["private_metadata"]
        values = view["state"]["values"]

        updates = {
            "title": values["title_block"]["title_input"]["value"],
            "assigned_to": values["assignee_block"]["assignee_input"]["value"],
            "priority": values["priority_block"]["priority_input"]["selected_option"]["value"],
            "description": values["description_block"]["description_input"]["value"],
            "status": "pending",
            "visibility": "public",
        }

        log.info("Approving draft task %s with edits", task_id)
        await self._taskboard.update_task(task_id, updates)

    async def _update_message(self, payload: dict, task_id: str, status: str):
        message = payload.get("message", {})
        blocks = message.get("blocks", [])
        updated = replace_actions_with_status(blocks, task_id, status)

        channel = payload["channel"]["id"]
        ts = message["ts"]
        await self._slack.chat_update(channel=channel, ts=ts, blocks=updated)

    async def _update_message_all(self, payload: dict, task_ids: list[str], status: str):
        message = payload.get("message", {})
        blocks = message.get("blocks", [])
        for task_id in task_ids:
            blocks = replace_actions_with_status(blocks, task_id, status)
        blocks = [b for b in blocks if b.get("block_id") != "actions_all"]

        channel = payload["channel"]["id"]
        ts = message["ts"]
        await self._slack.chat_update(channel=channel, ts=ts, blocks=blocks)
