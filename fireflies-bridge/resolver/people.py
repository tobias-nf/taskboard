import logging

from taskboard.client import TaskboardClient

log = logging.getLogger(__name__)


class PeopleResolver:
    """Maps email addresses to Taskboard agent IDs. Caches the agent list."""

    def __init__(self, taskboard: TaskboardClient):
        self._taskboard = taskboard
        self._by_email: dict[str, dict] = {}
        self._loaded = False

    async def _load(self):
        agents = await self._taskboard.list_agents()
        self._by_email = {}
        for agent in agents:
            email = agent.get("email")
            if email:
                self._by_email[email.lower()] = agent
        self._loaded = True
        log.info("Loaded %d agents with email addresses", len(self._by_email))

    async def resolve_by_email(self, email: str | None) -> dict | None:
        """Return the agent dict for an email, or None if not found."""
        if not email:
            return None
        if not self._loaded:
            await self._load()
        return self._by_email.get(email.lower())

    async def resolve_agent_id(self, email: str | None) -> str | None:
        agent = await self.resolve_by_email(email)
        return agent["id"] if agent else None

    async def resolve_slack_id(self, email: str | None) -> str | None:
        agent = await self.resolve_by_email(email)
        if agent and agent.get("slack_id"):
            return agent["slack_id"]
        return None

    def invalidate(self):
        """Force re-fetch on next resolve."""
        self._loaded = False
