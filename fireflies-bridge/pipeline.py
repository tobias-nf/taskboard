"""Main pipeline: Fireflies transcript -> enriched draft tasks in Taskboard.

No Slack knowledge — the Taskboard Slack app picks up draft tasks via SSE events
and handles the approval flow independently.
"""

import logging
from datetime import datetime, timezone

from config import settings
from dedup.matcher import Matcher
from enrichment.extractor import ActionItem, Extractor
from fireflies.client import FirefliesClient
from fireflies.models import Transcript
from resolver.people import PeopleResolver
from store.processed import ProcessedStore
from taskboard.client import TaskboardClient, TaskCreate

log = logging.getLogger(__name__)


class Pipeline:
    def __init__(
        self,
        fireflies: FirefliesClient,
        taskboard: TaskboardClient,
        extractor: Extractor,
        matcher: Matcher,
        resolver: PeopleResolver,
        store: ProcessedStore,
    ):
        self.fireflies = fireflies
        self.taskboard = taskboard
        self.extractor = extractor
        self.matcher = matcher
        self.resolver = resolver
        self.store = store

    async def process_meeting(self, meeting_id: str):
        """Full pipeline for one meeting."""
        if self.store.already_processed(meeting_id):
            log.info("Meeting %s already processed, skipping", meeting_id)
            return

        log.info("Processing meeting %s", meeting_id)

        # 1. Fetch transcript
        transcript = await self.fireflies.get_transcript(meeting_id)
        log.info("Fetched transcript: %s (%d attendees)", transcript.title, len(transcript.meeting_attendees))

        # 2. Extract action items via AI
        items = await self.extractor.extract(transcript)
        if not items:
            log.info("No action items found in meeting %s", meeting_id)
            self.store.mark_processed(meeting_id)
            return

        # 3. For each item: resolve assignee, dedup, create draft task
        created = 0
        for item in items:
            was_created = await self._process_item(item, transcript)
            if was_created:
                created += 1

        log.info("Created %d draft tasks from %d action items (meeting %s)", created, len(items), meeting_id)
        self.store.mark_processed(meeting_id)

    async def _process_item(self, item: ActionItem, transcript: Transcript) -> bool:
        """Resolve assignee, check for duplicates, create draft task. Returns True if a new task was created."""
        # Resolve assignee
        agent_id = await self.resolver.resolve_agent_id(item.suggested_assignee_email)
        assignee_name = item.suggested_assignee_name or item.suggested_assignee_email or "unassigned"

        if not agent_id:
            log.info("Could not resolve assignee for '%s' (%s), leaving unassigned", item.title, item.suggested_assignee_email)

        # Check for duplicates against existing open tasks
        if agent_id:
            existing = await self.taskboard.get_open_tasks(assigned_to=agent_id, tag="meeting-action-item")
            if existing:
                match = await self.matcher.find_match(item, existing)
                if match.match_task_id and match.confidence >= 0.8:
                    # High-confidence duplicate: update existing task
                    upd = match.update or {}
                    comment = upd.get("comment") or self._build_update_comment(item, transcript)
                    await self.taskboard.add_comment(match.match_task_id, comment)

                    updates = {}
                    if upd.get("new_deadline"):
                        updates["deadline"] = upd["new_deadline"]
                    if upd.get("new_priority"):
                        updates["priority"] = upd["new_priority"]
                    if updates:
                        await self.taskboard.update_task(match.match_task_id, updates)

                    log.info("Updated existing task %s instead of creating new", match.match_task_id)
                    return False

        # Build description with meeting context
        meeting_label = transcript.title or "Untitled Meeting"
        meeting_date = self._format_date(transcript.date)
        description = (
            f"**From meeting:** {meeting_label} ({meeting_date})\n\n"
            f"{item.description}\n\n"
            f"**Original assignee suggestion:** {assignee_name}"
        )

        # Organizer becomes stakeholder (owed_to) — they'll get the approval via Slack app
        owed_to = []
        organizer_agent_id = await self.resolver.resolve_agent_id(transcript.organizer_email)
        if organizer_agent_id and organizer_agent_id != agent_id:
            owed_to.append(organizer_agent_id)

        # Mention the Slack app agent so it receives SSE events for this draft
        mentions = [settings.slack_app_agent_id]

        # Create draft task
        task = TaskCreate(
            title=item.title,
            description=description,
            assigned_to=agent_id,
            status="draft",
            visibility="private",
            priority=item.priority,
            tags=["meeting-action-item"] + item.tags,
            owed_to=owed_to,
            mentions=mentions,
        )
        result = await self.taskboard.create_task(task)
        task_id = result.get("id", "")
        log.info("Created draft task %s: %s", task_id, item.title)

        # Add reference to Fireflies transcript
        await self.taskboard.add_reference(task_id, {
            "type": "origin",
            "source": "fireflies",
            "external_id": transcript.id,
            "url": transcript.transcript_url or f"https://app.fireflies.ai/view/{transcript.id}",
            "title": f"{meeting_label} -- {meeting_date}",
        })

        return True

    def _build_update_comment(self, item: ActionItem, transcript: Transcript) -> str:
        meeting_label = transcript.title or "Untitled Meeting"
        meeting_date = self._format_date(transcript.date)
        return (
            f"**Meeting update:** {meeting_label} ({meeting_date})\n\n"
            f"{item.description}\n\n"
            f"*Source: [Fireflies transcript](https://app.fireflies.ai/view/{transcript.id})*"
        )

    @staticmethod
    def _format_date(date_val: str | int | None) -> str:
        if not date_val:
            return "unknown date"
        try:
            ts = int(date_val) / 1000  # Fireflies uses millisecond timestamps
            return datetime.fromtimestamp(ts, tz=timezone.utc).strftime("%Y-%m-%d")
        except (ValueError, TypeError, OSError):
            return str(date_val)
