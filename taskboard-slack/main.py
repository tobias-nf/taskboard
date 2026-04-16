"""Taskboard Slack App — task approvals, /task commands, and event-driven notifications."""

import asyncio
import hashlib
import hmac
import json
import logging
import time
from contextlib import asynccontextmanager
from urllib.parse import parse_qs

from fastapi import FastAPI, Request, Response
from slack_sdk.web.async_client import AsyncWebClient

from agent.agent import TaskAgent
from commands.task_command import TaskCommandHandler
from config import settings
from events.listener import EventListener
from interactions.handler import InteractionHandler
from resolver.identity import IdentityResolver
from taskboard.client import TaskboardClient

logging.basicConfig(level=logging.INFO, format="%(asctime)s %(name)s %(levelname)s %(message)s")
log = logging.getLogger(__name__)

command_handler: TaskCommandHandler
interaction_handler: InteractionHandler


@asynccontextmanager
async def lifespan(app: FastAPI):
    global command_handler, interaction_handler

    taskboard = TaskboardClient(settings.taskboard_url, settings.taskboard_api_key)
    slack = AsyncWebClient(token=settings.slack_bot_token)
    resolver = IdentityResolver(taskboard)
    agent = TaskAgent(settings.anthropic_api_key, taskboard)

    command_handler = TaskCommandHandler(agent=agent, resolver=resolver, slack=slack)
    interaction_handler = InteractionHandler(slack=slack, taskboard=taskboard)

    # Start SSE event listener in background
    listener = EventListener(
        taskboard=taskboard,
        slack=slack,
        resolver=resolver,
        admin_slack_id=settings.admin_slack_id,
    )
    listener_task = asyncio.create_task(listener.listen_forever())

    log.info("Taskboard Slack App started")
    yield

    listener_task.cancel()
    await taskboard.close()
    log.info("Taskboard Slack App stopped")


app = FastAPI(title="Taskboard Slack App", lifespan=lifespan)


# --- Health ---


@app.get("/health")
async def health():
    return {"status": "ok"}


# --- Slack signature verification ---


def verify_slack(body: bytes, timestamp: str, signature: str) -> bool:
    if abs(time.time() - int(timestamp)) > 300:
        return False
    base = f"v0:{timestamp}:{body.decode()}"
    expected = "v0=" + hmac.new(
        settings.slack_signing_secret.encode(),
        base.encode(),
        hashlib.sha256,
    ).hexdigest()
    return hmac.compare_digest(expected, signature)


# --- Slash command: /task ---


@app.post("/slack/commands")
async def slash_command(request: Request):
    body = await request.body()
    timestamp = request.headers.get("X-Slack-Request-Timestamp", "0")
    signature = request.headers.get("X-Slack-Signature", "")

    if not verify_slack(body, timestamp, signature):
        return Response(status_code=401)

    form = dict(parse_qs(body.decode()))
    # parse_qs returns lists; flatten single values
    flat = {k: v[0] if len(v) == 1 else v for k, v in form.items()}

    result = await command_handler.handle(flat)
    return result


# --- Interactive callbacks (buttons, modals) ---


@app.post("/slack/interactions")
async def interactions(request: Request):
    body = await request.body()
    timestamp = request.headers.get("X-Slack-Request-Timestamp", "0")
    signature = request.headers.get("X-Slack-Signature", "")

    if not verify_slack(body, timestamp, signature):
        return Response(status_code=401)

    form = parse_qs(body.decode())
    payload = json.loads(form["payload"][0])

    try:
        await interaction_handler.handle(payload)
    except Exception:
        log.exception("Failed to handle Slack interaction")

    return Response(status_code=200)


# --- Slack Events API ---


@app.post("/slack/events")
async def slack_events(request: Request):
    body = await request.body()
    data = json.loads(body)

    # URL verification challenge
    if data.get("type") == "url_verification":
        return {"challenge": data["challenge"]}

    return {"status": "ok"}
