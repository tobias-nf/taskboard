"""Taskboard REST client with impersonation support.

Uses X-Act-As header so actions appear as the real user in the audit trail,
while authenticating as the Slack app agent.
"""

import httpx


class TaskboardClient:
    def __init__(self, base_url: str, api_key: str):
        self._base_url = base_url
        self._api_key = api_key
        self._http = httpx.AsyncClient(
            base_url=base_url,
            headers={
                "Authorization": f"Bearer {api_key}",
                "Content-Type": "application/json",
            },
            timeout=30,
        )

    def _as_agent(self, agent_id: str | None) -> dict:
        """Return extra headers for impersonation."""
        if agent_id:
            return {"X-Act-As": agent_id}
        return {}

    # --- Tasks ---

    async def get_task(self, task_id: str) -> dict:
        resp = await self._http.get(f"/tasks/{task_id}")
        resp.raise_for_status()
        return resp.json()

    async def list_my_tasks(self, agent_id: str, status: str | None = None, priority: str | None = None, tag: str | None = None) -> list[dict]:
        params: dict = {}
        if status:
            params["status"] = status
        if priority:
            params["priority"] = priority
        if tag:
            params["tag"] = tag
        resp = await self._http.get("/tasks/me", params=params, headers=self._as_agent(agent_id))
        resp.raise_for_status()
        data = resp.json()
        return data if isinstance(data, list) else data.get("tasks", [])

    async def list_tasks_created(self, agent_id: str, status: str | None = None) -> list[dict]:
        params: dict = {}
        if status:
            params["status"] = status
        resp = await self._http.get("/tasks/me/created", params=params, headers=self._as_agent(agent_id))
        resp.raise_for_status()
        data = resp.json()
        return data if isinstance(data, list) else data.get("tasks", [])

    async def list_tasks_owed(self, agent_id: str, status: str | None = None) -> list[dict]:
        params: dict = {}
        if status:
            params["status"] = status
        resp = await self._http.get("/tasks/me/owed", params=params, headers=self._as_agent(agent_id))
        resp.raise_for_status()
        data = resp.json()
        return data if isinstance(data, list) else data.get("tasks", [])

    async def create_task(self, body: dict, agent_id: str | None = None) -> dict:
        resp = await self._http.post("/tasks", json=body, headers=self._as_agent(agent_id))
        resp.raise_for_status()
        return resp.json()

    async def update_task(self, task_id: str, updates: dict, agent_id: str | None = None) -> dict:
        resp = await self._http.patch(f"/tasks/{task_id}", json=updates, headers=self._as_agent(agent_id))
        resp.raise_for_status()
        return resp.json()

    async def add_comment(self, task_id: str, body: str, agent_id: str | None = None) -> dict:
        resp = await self._http.post(f"/tasks/{task_id}/activity", json={"body": body}, headers=self._as_agent(agent_id))
        resp.raise_for_status()
        return resp.json()

    async def get_owed_to(self, task_id: str) -> list[dict]:
        resp = await self._http.get(f"/tasks/{task_id}/owed-to")
        resp.raise_for_status()
        data = resp.json()
        return data if isinstance(data, list) else data.get("owed_to", [])

    async def list_visible_tasks(self, status: str | None = None) -> list[dict]:
        params: dict = {"limit": "200"}
        if status:
            params["status"] = status
        resp = await self._http.get("/tasks/visible", params=params)
        resp.raise_for_status()
        data = resp.json()
        return data if isinstance(data, list) else data.get("tasks", [])

    # --- Agents ---

    async def list_agents(self) -> list[dict]:
        resp = await self._http.get("/agents", params={"limit": "200"})
        resp.raise_for_status()
        data = resp.json()
        return data if isinstance(data, list) else data.get("agents", [])

    async def get_agent(self, agent_id: str) -> dict:
        resp = await self._http.get(f"/agents/{agent_id}")
        resp.raise_for_status()
        return resp.json()

    # --- SSE ---

    def events_url(self) -> str:
        return f"{self._base_url}/events"

    def auth_headers(self) -> dict:
        return {"Authorization": f"Bearer {self._api_key}"}

    async def close(self):
        await self._http.aclose()
