import json
import logging

import anthropic
from pydantic import BaseModel

from enrichment.extractor import ActionItem

log = logging.getLogger(__name__)

MATCHING_PROMPT = """\
You are checking whether a new action item from a meeting is a duplicate of an existing task.

New action item:
  Title: {new_title}
  Assignee: {new_assignee}
  Description: {new_description}

Existing open tasks for this assignee:
{existing_tasks}

---

For each existing task, assess whether the new action item refers to the SAME piece of work.
Same work means: the same deliverable, the same goal, not just the same topic area.

Return JSON:
{{
  "match_task_id": "T-2026-XXXXX" or null if no match,
  "confidence": 0.0 to 1.0,
  "reasoning": "one sentence explaining why",
  "update": {{
    "comment": "what to add as a meeting update comment (or null if no match)",
    "new_deadline": "ISO 8601 date or null",
    "new_priority": "low/standard/urgent or null",
    "status_hint": "e.g. 'completed' if the meeting says it's done, or null"
  }}
}}

Return ONLY the JSON object, no markdown fencing."""


class MatchResult(BaseModel):
    match_task_id: str | None = None
    confidence: float = 0.0
    reasoning: str = ""
    update: dict | None = {}


class Matcher:
    def __init__(self, api_key: str):
        self._client = anthropic.AsyncAnthropic(api_key=api_key)

    async def find_match(
        self,
        item: ActionItem,
        existing_tasks: list[dict],
    ) -> MatchResult:
        if not existing_tasks:
            return MatchResult()

        tasks_str = "\n".join(
            f"  [{t['id']}] \"{t.get('title', '')}\" "
            f"(status: {t.get('status', '?')}, created: {t.get('created_at', '?')})\n"
            f"    Description: {(t.get('description') or '')[:300]}"
            for t in existing_tasks
        )

        prompt = MATCHING_PROMPT.format(
            new_title=item.title,
            new_assignee=item.suggested_assignee_name or item.suggested_assignee_email or "unassigned",
            new_description=item.description,
            existing_tasks=tasks_str,
        )

        message = await self._client.messages.create(
            model="claude-sonnet-4-6",
            max_tokens=1024,
            messages=[{"role": "user", "content": prompt}],
        )

        text = message.content[0].text.strip()
        if text.startswith("```"):
            text = text.split("\n", 1)[1]
            if text.endswith("```"):
                text = text[: text.rfind("```")]

        try:
            data = json.loads(text)
        except json.JSONDecodeError:
            log.error("Failed to parse match result: %s", text[:500])
            return MatchResult()

        result = MatchResult.model_validate(data)
        if result.match_task_id:
            log.info(
                "Matched '%s' to %s (confidence: %.2f): %s",
                item.title,
                result.match_task_id,
                result.confidence,
                result.reasoning,
            )
        return result
