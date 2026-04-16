"""Handle /task slash command — natural language task interaction."""

import asyncio
import logging

import httpx
from slack_sdk.web.async_client import AsyncWebClient

from agent.agent import TaskAgent
from resolver.identity import IdentityResolver

log = logging.getLogger(__name__)


class TaskCommandHandler:
    def __init__(self, agent: TaskAgent, resolver: IdentityResolver, slack: AsyncWebClient):
        self._agent = agent
        self._resolver = resolver
        self._slack = slack

    async def handle(self, form: dict):
        """Process a /task slash command.

        Slack requires a response within 3 seconds, so we acknowledge immediately
        and process the command asynchronously via the response_url.
        """
        slack_user_id = form.get("user_id", "")
        text = form.get("text", "").strip()
        response_url = form.get("response_url", "")

        if not text:
            return {
                "response_type": "ephemeral",
                "text": "Usage: `/task <what you want to do>`\n\nExamples:\n"
                        "- `/task what are my priorities today?`\n"
                        "- `/task mark T-2026-00042 as done`\n"
                        "- `/task create: follow up with ACME by Friday`\n"
                        "- `/task I'm blocked on the vendor review`",
            }

        # Resolve identity
        agent = await self._resolver.slack_to_agent(slack_user_id)
        if not agent:
            return {
                "response_type": "ephemeral",
                "text": ":x: You're not registered in Taskboard. Ask an admin to set your `slack_id`.",
            }

        # Acknowledge and process async
        asyncio.create_task(self._process(agent, text, response_url))

        return {"response_type": "ephemeral", "text": ":hourglass_flowing_sand: Thinking..."}

    async def _process(self, agent: dict, text: str, response_url: str):
        """Run the Claude agent and post the result back via response_url."""
        try:
            result = await self._agent.run(
                agent_id=agent["id"],
                agent_name=agent.get("name", agent["id"]),
                user_input=text,
            )

            payload = {
                "response_type": "ephemeral",
                "text": result,
            }
        except Exception:
            log.exception("Agent execution failed for %s", agent["id"])
            payload = {
                "response_type": "ephemeral",
                "text": ":x: Something went wrong processing your request. Please try again.",
            }

        # Post back to Slack via response_url
        async with httpx.AsyncClient() as client:
            try:
                await client.post(response_url, json=payload)
            except Exception:
                log.exception("Failed to post response to Slack")
