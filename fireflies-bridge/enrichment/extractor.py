import json
import logging

import anthropic
from pydantic import BaseModel

from fireflies.models import Transcript

log = logging.getLogger(__name__)

EXTRACTION_PROMPT = """\
You are extracting action items from a meeting transcript.

Meeting: {title} ({date})
Attendees (with emails):
{attendees}

Speakers: {speakers}

AI-generated action items summary:
{action_items}

Sentences flagged as tasks by AI:
{task_sentences}

Meeting overview:
{overview}

---

For each distinct action item, return a JSON array. Each element:
{{
  "title": "concise imperative task title, max 100 chars",
  "description": "2-3 sentences of context from the meeting. Who raised it, why, what was discussed.",
  "suggested_assignee_email": "email of the person who should do this, based on speaker/context, or null",
  "suggested_assignee_name": "name of the person, for display",
  "priority": "low | standard | urgent",
  "tags": ["relevant", "topic", "tags"]
}}

Rules:
- Deduplicate: if the same action appears in both the summary and task sentences, emit it once.
- Only include genuine action items (things someone needs to DO), not observations or decisions.
- If you cannot confidently identify an assignee, set suggested_assignee_email to null.
- Return an empty array [] if there are no action items.

Return ONLY the JSON array, no markdown fencing."""


class ActionItem(BaseModel):
    title: str
    description: str
    suggested_assignee_email: str | None = None
    suggested_assignee_name: str | None = None
    priority: str = "standard"
    tags: list[str] = []


class Extractor:
    def __init__(self, api_key: str):
        self._client = anthropic.AsyncAnthropic(api_key=api_key)

    async def extract(self, transcript: Transcript) -> list[ActionItem]:
        attendees_str = "\n".join(
            f"  - {a.displayName or a.name or 'Unknown'} <{a.email or 'no email'}>"
            for a in transcript.meeting_attendees
        ) or "(no attendee data)"

        speakers_str = ", ".join(s.name or s.id or "?" for s in transcript.speakers) or "(unknown)"

        task_sentences = [
            s for s in transcript.sentences if s.ai_filters and s.ai_filters.task
        ]
        task_str = "\n".join(
            f"  [{s.speaker_name}]: {s.text}" for s in task_sentences
        ) or "(none flagged)"

        prompt = EXTRACTION_PROMPT.format(
            title=transcript.title or "Untitled Meeting",
            date=transcript.date or "unknown date",
            attendees=attendees_str,
            speakers=speakers_str,
            action_items=transcript.summary.action_items if transcript.summary else "(no summary)",
            task_sentences=task_str,
            overview=transcript.summary.overview if transcript.summary else "(no overview)",
        )

        log.info("Extracting action items from meeting: %s", transcript.title)

        message = await self._client.messages.create(
            model="claude-sonnet-4-6",
            max_tokens=2048,
            messages=[{"role": "user", "content": prompt}],
        )

        text = message.content[0].text.strip()
        # Strip markdown fencing if present
        if text.startswith("```"):
            text = text.split("\n", 1)[1]
            if text.endswith("```"):
                text = text[: text.rfind("```")]

        try:
            items = json.loads(text)
        except json.JSONDecodeError:
            log.error("Failed to parse extraction result as JSON: %s", text[:500])
            return []

        result = [ActionItem.model_validate(item) for item in items]
        log.info("Extracted %d action items", len(result))
        return result
