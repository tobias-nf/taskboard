"""Claude-powered agent for natural language task interactions via /task."""

import json
import logging

import anthropic

from taskboard.client import TaskboardClient

log = logging.getLogger(__name__)

SYSTEM_PROMPT = """\
You are the Taskboard assistant in Slack. You are acting on behalf of agent "{agent_id}" ({agent_name}).

You help the user manage their tasks using natural language. You can:
- List their tasks (assigned, created, owed to them)
- Show task details
- Update task status, priority, deadline
- Add comments to tasks
- Create new tasks
- Mark tasks as complete or blocked

Rules:
- You can ONLY access tasks that this agent can see.
- Keep responses concise — this is Slack. Use bullet points, short sentences.
- Format task IDs as `T-2026-XXXXX`.
- When listing tasks, show the most important ones first.
- If the user refers to a task by description rather than ID, search their tasks to find the match.
- Today's date is {today}.
- Never fabricate task IDs or data. If a tool call fails, say so."""

TOOLS = [
    {
        "name": "list_my_tasks",
        "description": "List tasks assigned to the user. Returns up to 50 tasks.",
        "input_schema": {
            "type": "object",
            "properties": {
                "status": {"type": "string", "description": "Comma-separated status filter (e.g. 'pending,in_progress,blocked')"},
                "priority": {"type": "string", "description": "Comma-separated priority filter (e.g. 'urgent,emergency')"},
                "tag": {"type": "string", "description": "Filter by tag name"},
            },
        },
    },
    {
        "name": "list_tasks_created",
        "description": "List tasks the user created (they may be assigned to others).",
        "input_schema": {
            "type": "object",
            "properties": {
                "status": {"type": "string", "description": "Comma-separated status filter"},
            },
        },
    },
    {
        "name": "list_tasks_owed",
        "description": "List tasks where the user is a stakeholder (owed to them).",
        "input_schema": {
            "type": "object",
            "properties": {
                "status": {"type": "string", "description": "Comma-separated status filter"},
            },
        },
    },
    {
        "name": "get_task",
        "description": "Get full details of a specific task by ID.",
        "input_schema": {
            "type": "object",
            "properties": {
                "task_id": {"type": "string", "description": "Task ID (e.g. T-2026-00042)"},
            },
            "required": ["task_id"],
        },
    },
    {
        "name": "update_task",
        "description": "Update a task's status, priority, deadline, description, or parent. Use parent_id to move a task under another task, or set it to empty string to detach.",
        "input_schema": {
            "type": "object",
            "properties": {
                "task_id": {"type": "string", "description": "Task ID"},
                "status": {"type": "string", "enum": ["in_progress", "completed", "blocked", "cancelled"]},
                "priority": {"type": "string", "enum": ["low", "standard", "urgent", "emergency"]},
                "deadline": {"type": "string", "description": "ISO 8601 deadline"},
                "parent_id": {"type": "string", "description": "Move under a parent task (empty string to detach)"},
            },
            "required": ["task_id"],
        },
    },
    {
        "name": "add_comment",
        "description": "Add a comment to a task's activity log.",
        "input_schema": {
            "type": "object",
            "properties": {
                "task_id": {"type": "string", "description": "Task ID"},
                "body": {"type": "string", "description": "Comment text (markdown)"},
            },
            "required": ["task_id", "body"],
        },
    },
    {
        "name": "create_task",
        "description": "Create a new task. Use parent_id to create a subtask under an existing task.",
        "input_schema": {
            "type": "object",
            "properties": {
                "title": {"type": "string", "description": "Task title"},
                "description": {"type": "string", "description": "Task description (markdown)"},
                "assigned_to": {"type": "string", "description": "Agent ID to assign to (defaults to self)"},
                "priority": {"type": "string", "enum": ["low", "standard", "urgent", "emergency"], "default": "standard"},
                "deadline": {"type": "string", "description": "ISO 8601 deadline"},
                "tags": {"type": "array", "items": {"type": "string"}, "description": "Tags to apply"},
                "parent_id": {"type": "string", "description": "Parent task ID to create this as a subtask"},
            },
            "required": ["title"],
        },
    },
]


class TaskAgent:
    def __init__(self, api_key: str, taskboard: TaskboardClient):
        self._client = anthropic.AsyncAnthropic(api_key=api_key)
        self._taskboard = taskboard

    async def run(self, agent_id: str, agent_name: str, user_input: str, history: list[dict] | None = None) -> str:
        """Process a natural language task command. Returns markdown text for Slack.

        If history is provided, it's prepended as prior conversation context.
        History should contain only rendered text turns (no tool calls).
        """
        from datetime import date

        system = SYSTEM_PROMPT.format(agent_id=agent_id, agent_name=agent_name, today=date.today().isoformat())
        messages = list(history or []) + [{"role": "user", "content": user_input}]

        # Agentic loop — run until Claude stops calling tools
        for _ in range(10):  # safety limit
            response = await self._client.messages.create(
                model="claude-sonnet-4-6",
                max_tokens=1024,
                system=system,
                tools=TOOLS,
                messages=messages,
            )

            # If no tool use, extract the text response
            if response.stop_reason == "end_of_turn":
                return self._extract_text(response)

            # Process tool calls
            tool_results = []
            for block in response.content:
                if block.type == "tool_use":
                    result = await self._execute_tool(block.name, block.input, agent_id)
                    serialized = json.dumps(result, default=str)
                    tool_results.append({
                        "type": "tool_result",
                        "tool_use_id": block.id,
                        "content": serialized if serialized and serialized != "null" else "No results.",
                    })

            if not tool_results:
                # No tool calls but not end_of_turn — extract whatever text is there
                return self._extract_text(response)

            # Convert response content blocks to serializable dicts
            assistant_content = []
            for block in response.content:
                if block.type == "text":
                    assistant_content.append({"type": "text", "text": block.text})
                elif block.type == "tool_use":
                    assistant_content.append({"type": "tool_use", "id": block.id, "name": block.name, "input": block.input})

            messages.append({"role": "assistant", "content": assistant_content})
            messages.append({"role": "user", "content": tool_results})

        return "I hit my processing limit. Please try a simpler request."

    async def _execute_tool(self, name: str, input: dict, agent_id: str) -> dict | list:
        """Execute a tool call against the Taskboard API, impersonating the user."""
        try:
            if name == "list_my_tasks":
                return await self._taskboard.list_my_tasks(agent_id, **input)
            elif name == "list_tasks_created":
                return await self._taskboard.list_tasks_created(agent_id, **input)
            elif name == "list_tasks_owed":
                return await self._taskboard.list_tasks_owed(agent_id, **input)
            elif name == "get_task":
                return await self._taskboard.get_task(input["task_id"])
            elif name == "update_task":
                task_id = input.pop("task_id")
                return await self._taskboard.update_task(task_id, input, agent_id)
            elif name == "add_comment":
                return await self._taskboard.add_comment(input["task_id"], input["body"], agent_id)
            elif name == "create_task":
                return await self._taskboard.create_task(input, agent_id)
            else:
                return {"error": f"Unknown tool: {name}"}
        except Exception as e:
            log.exception("Tool execution failed: %s", name)
            return {"error": str(e)}

    @staticmethod
    def _extract_text(response) -> str:
        parts = []
        for block in response.content:
            if hasattr(block, "text"):
                parts.append(block.text)
        return "\n".join(parts) or "Done."
