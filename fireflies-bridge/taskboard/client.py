import httpx
from pydantic import BaseModel


class TaskCreate(BaseModel):
    title: str
    description: str | None = None
    assigned_to: str | None = None
    visibility: str = "private"
    status: str = "draft"
    priority: str = "standard"
    deadline: str | None = None
    parent_id: str | None = None
    owed_to: list[str] = []
    mentions: list[str] = []
    tags: list[str] = []


class TaskboardClient:
    def __init__(self, base_url: str, api_key: str):
        self._http = httpx.AsyncClient(
            base_url=base_url,
            headers={
                "Authorization": f"Bearer {api_key}",
                "Content-Type": "application/json",
            },
            timeout=30,
        )

    async def create_task(self, task: TaskCreate) -> dict:
        resp = await self._http.post("/tasks", json=task.model_dump(exclude_none=True))
        resp.raise_for_status()
        return resp.json()

    async def update_task(self, task_id: str, updates: dict) -> dict:
        resp = await self._http.patch(f"/tasks/{task_id}", json=updates)
        resp.raise_for_status()
        return resp.json()

    async def add_comment(self, task_id: str, body: str) -> dict:
        resp = await self._http.post(f"/tasks/{task_id}/activity", json={"type": "commented", "summary": body})
        resp.raise_for_status()
        return resp.json()

    async def add_reference(self, task_id: str, ref: dict) -> dict:
        resp = await self._http.post(f"/tasks/{task_id}/references", json=ref)
        resp.raise_for_status()
        return resp.json()

    async def get_open_tasks(
        self,
        assigned_to: str | None = None,
        tag: str | None = None,
    ) -> list[dict]:
        """Fetch open tasks visible to the bridge, optionally filtered."""
        params: dict = {"status": "draft,pending,in_progress,blocked", "limit": "100"}
        if tag:
            params["tag"] = tag
        resp = await self._http.get("/tasks/visible", params=params)
        resp.raise_for_status()
        data = resp.json()
        tasks = data if isinstance(data, list) else (data.get("tasks") or [])
        if assigned_to:
            tasks = [t for t in tasks if t.get("assigned_to") == assigned_to]
        return tasks

    async def list_agents(self) -> list[dict]:
        resp = await self._http.get("/agents", params={"limit": "200"})
        resp.raise_for_status()
        data = resp.json()
        return data if isinstance(data, list) else data.get("agents", [])

    async def close(self):
        await self._http.aclose()
