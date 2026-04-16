"""Bidirectional mapping between Slack user IDs and Taskboard agent IDs."""

import logging

from taskboard.client import TaskboardClient

log = logging.getLogger(__name__)


class IdentityResolver:
    def __init__(self, taskboard: TaskboardClient):
        self._taskboard = taskboard
        self._by_slack_id: dict[str, dict] = {}
        self._by_agent_id: dict[str, dict] = {}
        self._loaded = False

    async def _load(self):
        agents = await self._taskboard.list_agents()
        self._by_slack_id = {}
        self._by_agent_id = {}
        for agent in agents:
            self._by_agent_id[agent["id"]] = agent
            slack_id = agent.get("slack_id")
            if slack_id:
                self._by_slack_id[slack_id] = agent
        self._loaded = True
        log.info("Loaded %d agents (%d with Slack IDs)", len(self._by_agent_id), len(self._by_slack_id))

    async def slack_to_agent(self, slack_user_id: str) -> dict | None:
        """Resolve Slack user ID to agent dict."""
        if not self._loaded:
            await self._load()
        return self._by_slack_id.get(slack_user_id)

    async def agent_to_slack(self, agent_id: str) -> str | None:
        """Resolve agent ID to Slack user ID."""
        if not self._loaded:
            await self._load()
        agent = self._by_agent_id.get(agent_id)
        return agent.get("slack_id") if agent else None

    def invalidate(self):
        self._loaded = False
