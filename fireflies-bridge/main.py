"""Fireflies Bridge — Meeting action items to Taskboard draft tasks.

Receives Fireflies webhooks, extracts action items via AI, creates draft tasks
in the Taskboard API. The Taskboard Slack app handles approval and user interaction.
"""

import hashlib
import hmac
import logging
from contextlib import asynccontextmanager

from fastapi import FastAPI, Request, Response

from config import settings
from dedup.matcher import Matcher
from enrichment.extractor import Extractor
from fireflies.client import FirefliesClient
from fireflies.models import Transcript, WebhookPayload
from pipeline import Pipeline
from resolver.people import PeopleResolver
from store.processed import ProcessedStore
from taskboard.client import TaskboardClient

logging.basicConfig(level=logging.INFO, format="%(asctime)s %(name)s %(levelname)s %(message)s")
log = logging.getLogger(__name__)

pipeline: Pipeline


@asynccontextmanager
async def lifespan(app: FastAPI):
    global pipeline

    fireflies = FirefliesClient(settings.fireflies_api_key)
    taskboard = TaskboardClient(settings.taskboard_url, settings.taskboard_api_key)
    extractor = Extractor(settings.anthropic_api_key)
    matcher = Matcher(settings.anthropic_api_key)
    resolver = PeopleResolver(taskboard)
    store = ProcessedStore()

    pipeline = Pipeline(
        fireflies=fireflies,
        taskboard=taskboard,
        extractor=extractor,
        matcher=matcher,
        resolver=resolver,
        store=store,
    )

    log.info("Fireflies Bridge started")
    yield

    await fireflies.close()
    await taskboard.close()
    log.info("Fireflies Bridge stopped")


app = FastAPI(title="Fireflies Bridge", lifespan=lifespan)


@app.get("/health")
async def health():
    return {"status": "ok"}


def verify_fireflies_signature(body: bytes, signature: str) -> bool:
    if not settings.fireflies_webhook_secret:
        return True
    expected = hmac.new(
        settings.fireflies_webhook_secret.encode(),
        body,
        hashlib.sha256,
    ).hexdigest()
    return hmac.compare_digest(expected, signature)


@app.post("/webhooks/fireflies")
async def fireflies_webhook(request: Request):
    body = await request.body()

    signature = request.headers.get("x-hub-signature", "")
    if not verify_fireflies_signature(body, signature):
        log.warning("Invalid Fireflies webhook signature")
        return Response(status_code=401)

    payload = WebhookPayload.model_validate_json(body)
    event_type = payload.event_type_resolved
    meeting_id = payload.meeting_id_resolved
    log.info("Received Fireflies webhook: %s (meeting: %s)", event_type, meeting_id)

    # Accept both old ("Transcription completed") and new ("meeting.transcribed") event names
    if event_type not in ("Transcription completed", "meeting.transcribed"):
        return {"status": "ignored", "reason": f"unhandled event: {event_type}"}

    if not meeting_id:
        return {"status": "ignored", "reason": "no meeting ID"}

    try:
        await pipeline.process_meeting(meeting_id)
    except Exception:
        log.exception("Failed to process meeting %s", meeting_id)

    return {"status": "ok"}


@app.post("/test/process")
async def test_process(request: Request):
    """DEV ONLY: inject a mock transcript directly into the pipeline,
    bypassing the Fireflies API. Accepts a Transcript JSON body."""
    body = await request.json()
    transcript = Transcript.model_validate(body)
    log.info("Test endpoint: processing mock transcript %s", transcript.id)

    items = await pipeline.extractor.extract(transcript)
    if not items:
        return {"status": "ok", "action_items": 0, "tasks_created": 0}

    created = 0
    for item in items:
        was_created = await pipeline._process_item(item, transcript)
        if was_created:
            created += 1

    return {"status": "ok", "action_items": len(items), "tasks_created": created}
