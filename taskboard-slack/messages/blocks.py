"""Slack Block Kit message builders."""

import json


def build_draft_approval(task: dict, stakeholders: list[str] | None = None) -> list[dict]:
    """Build approval message for a single draft task.

    The Slack app groups drafts from the same meeting into one message
    by batching tasks that arrive in quick succession with the same origin.
    For simplicity, this builds per-task blocks that can be composed.
    """
    blocks: list[dict] = []
    task_id = task.get("id", "")
    title = task.get("title", "Untitled")
    priority = task.get("priority", "standard")
    assignee = task.get("assigned_to", "unassigned")
    description = task.get("description", "")

    # Task summary
    text = f"*{title}*\n-> {assignee} -- {priority} priority"
    if description:
        # Show first 200 chars of description as context
        preview = description[:200].replace("\n", " ")
        if len(description) > 200:
            preview += "..."
        text += f"\n>{preview}"

    blocks.append({"type": "section", "text": {"type": "mrkdwn", "text": text}})

    # Action buttons
    blocks.append({
        "type": "actions",
        "block_id": f"draft_{task_id}",
        "elements": [
            {
                "type": "button",
                "text": {"type": "plain_text", "text": "Approve"},
                "style": "primary",
                "action_id": "approve_task",
                "value": task_id,
            },
            {
                "type": "button",
                "text": {"type": "plain_text", "text": "Edit"},
                "action_id": "edit_task",
                "value": task_id,
            },
            {
                "type": "button",
                "text": {"type": "plain_text", "text": "Reject"},
                "style": "danger",
                "action_id": "reject_task",
                "value": task_id,
            },
        ],
    })

    return blocks


def build_draft_batch(meeting_title: str, meeting_date: str, tasks: list[dict], transcript_url: str | None = None) -> list[dict]:
    """Build a grouped approval message for multiple draft tasks from the same meeting."""
    blocks: list[dict] = []

    header = f"*Meeting Action Items* -- {meeting_title}"
    if meeting_date:
        header += f"\n{meeting_date}"
    header += f" -- {len(tasks)} item{'s' if len(tasks) != 1 else ''}"

    blocks.append({"type": "section", "text": {"type": "mrkdwn", "text": header}})

    if transcript_url:
        blocks.append({
            "type": "context",
            "elements": [{"type": "mrkdwn", "text": f"<{transcript_url}|View transcript>"}],
        })

    blocks.append({"type": "divider"})

    for i, task in enumerate(tasks, 1):
        task_id = task.get("id", "")
        title = task.get("title", "Untitled")
        assignee = task.get("assigned_to", "unassigned")
        priority = task.get("priority", "standard")

        blocks.append({
            "type": "section",
            "text": {"type": "mrkdwn", "text": f"*{i}. {title}*\n-> {assignee} -- {priority} priority"},
        })
        blocks.append({
            "type": "actions",
            "block_id": f"draft_{task_id}",
            "elements": [
                {"type": "button", "text": {"type": "plain_text", "text": "Approve"}, "style": "primary", "action_id": "approve_task", "value": task_id},
                {"type": "button", "text": {"type": "plain_text", "text": "Edit"}, "action_id": "edit_task", "value": task_id},
                {"type": "button", "text": {"type": "plain_text", "text": "Reject"}, "style": "danger", "action_id": "reject_task", "value": task_id},
            ],
        })

    # Approve all
    all_ids = json.dumps([t.get("id", "") for t in tasks])
    blocks.append({"type": "divider"})
    blocks.append({
        "type": "actions",
        "block_id": "actions_all",
        "elements": [{
            "type": "button",
            "text": {"type": "plain_text", "text": "Approve All"},
            "style": "primary",
            "action_id": "approve_all",
            "value": all_ids,
        }],
    })

    return blocks


def _agent_options(agents: list[dict], selected_id: str | None = None) -> tuple[list[dict], dict | None]:
    """Build Slack static_select options from agent list. Returns (options, initial_option)."""
    options = []
    initial = None
    for a in agents:
        if not a.get("active"):
            continue
        aid = a.get("id", "")
        label = a.get("email") or aid
        opt = {"text": {"type": "plain_text", "text": label[:75]}, "value": aid}
        options.append(opt)
        if aid == selected_id:
            initial = opt
    return options, initial


def build_edit_modal(task: dict, agents: list[dict] | None = None) -> dict:
    """Modal for editing a draft task before approval."""
    task_id = task.get("id", "")
    agent_list = agents or []
    agent_opts, agent_initial = _agent_options(agent_list, task.get("assigned_to"))

    assignee_element: dict
    if agent_opts:
        assignee_element = {
            "type": "static_select",
            "action_id": "assignee_input",
            "placeholder": {"type": "plain_text", "text": "Select an agent"},
            "options": agent_opts,
        }
        if agent_initial:
            assignee_element["initial_option"] = agent_initial
    else:
        assignee_element = {
            "type": "plain_text_input",
            "action_id": "assignee_input",
            "initial_value": task.get("assigned_to", ""),
            "placeholder": {"type": "plain_text", "text": "Agent ID"},
        }

    return {
        "type": "modal",
        "callback_id": "edit_task_modal",
        "private_metadata": task_id,
        "title": {"type": "plain_text", "text": "Edit Action Item"},
        "submit": {"type": "plain_text", "text": "Approve with Changes"},
        "blocks": [
            {
                "type": "input",
                "block_id": "title_block",
                "element": {"type": "plain_text_input", "action_id": "title_input", "initial_value": task.get("title", "")},
                "label": {"type": "plain_text", "text": "Title"},
            },
            {
                "type": "input",
                "block_id": "assignee_block",
                "element": assignee_element,
                "label": {"type": "plain_text", "text": "Assign to"},
            },
            {
                "type": "input",
                "block_id": "priority_block",
                "element": {
                    "type": "static_select",
                    "action_id": "priority_input",
                    "initial_option": {
                        "text": {"type": "plain_text", "text": task.get("priority", "standard")},
                        "value": task.get("priority", "standard"),
                    },
                    "options": [
                        {"text": {"type": "plain_text", "text": p}, "value": p}
                        for p in ["low", "standard", "urgent", "emergency"]
                    ],
                },
                "label": {"type": "plain_text", "text": "Priority"},
            },
            {
                "type": "input",
                "block_id": "visibility_block",
                "element": {
                    "type": "static_select",
                    "action_id": "visibility_input",
                    "initial_option": {
                        "text": {"type": "plain_text", "text": "public"},
                        "value": "public",
                    },
                    "options": [
                        {"text": {"type": "plain_text", "text": "public"}, "value": "public"},
                        {"text": {"type": "plain_text", "text": "private"}, "value": "private"},
                    ],
                },
                "label": {"type": "plain_text", "text": "Visibility"},
            },
            {
                "type": "input",
                "block_id": "description_block",
                "element": {
                    "type": "plain_text_input",
                    "action_id": "description_input",
                    "multiline": True,
                    "initial_value": (task.get("description") or "")[:3000],
                },
                "label": {"type": "plain_text", "text": "Description"},
            },
        ],
    }


def build_create_task_modal(message_text: str, message_author: str, channel_name: str, metadata: str, agents: list[dict] | None = None) -> dict:
    """Modal for creating a task from a Slack message."""
    description = f"From Slack message by {message_author}"
    if channel_name:
        description += f" in #{channel_name}"
    description += f":\n\n> {message_text[:2500]}"

    agent_opts, _ = _agent_options(agents or [])
    assignee_element: dict
    if agent_opts:
        assignee_element = {
            "type": "static_select",
            "action_id": "assignee_input",
            "placeholder": {"type": "plain_text", "text": "Select an agent (or leave empty for self)"},
            "options": agent_opts,
        }
    else:
        assignee_element = {
            "type": "plain_text_input",
            "action_id": "assignee_input",
            "initial_value": "",
            "placeholder": {"type": "plain_text", "text": "Agent ID — leave empty to assign to yourself"},
        }

    return {
        "type": "modal",
        "callback_id": "create_task_modal",
        "private_metadata": metadata,
        "title": {"type": "plain_text", "text": "Create Task"},
        "submit": {"type": "plain_text", "text": "Create"},
        "blocks": [
            {
                "type": "context",
                "elements": [{"type": "mrkdwn", "text": f"_From message by {message_author}_"}],
            },
            {
                "type": "input",
                "block_id": "title_block",
                "element": {
                    "type": "plain_text_input",
                    "action_id": "title_input",
                    "placeholder": {"type": "plain_text", "text": "Task title"},
                },
                "label": {"type": "plain_text", "text": "Title"},
            },
            {
                "type": "input",
                "block_id": "assignee_block",
                "element": assignee_element,
                "optional": True,
                "label": {"type": "plain_text", "text": "Assign to"},
            },
            {
                "type": "input",
                "block_id": "priority_block",
                "element": {
                    "type": "static_select",
                    "action_id": "priority_input",
                    "initial_option": {"text": {"type": "plain_text", "text": "standard"}, "value": "standard"},
                    "options": [
                        {"text": {"type": "plain_text", "text": p}, "value": p}
                        for p in ["low", "standard", "urgent", "emergency"]
                    ],
                },
                "label": {"type": "plain_text", "text": "Priority"},
            },
            {
                "type": "input",
                "block_id": "deadline_block",
                "element": {
                    "type": "datepicker",
                    "action_id": "deadline_input",
                    "placeholder": {"type": "plain_text", "text": "Pick a date"},
                },
                "optional": True,
                "label": {"type": "plain_text", "text": "Deadline"},
            },
            {
                "type": "input",
                "block_id": "visibility_block",
                "element": {
                    "type": "static_select",
                    "action_id": "visibility_input",
                    "initial_option": {"text": {"type": "plain_text", "text": "public"}, "value": "public"},
                    "options": [
                        {"text": {"type": "plain_text", "text": "public"}, "value": "public"},
                        {"text": {"type": "plain_text", "text": "private"}, "value": "private"},
                    ],
                },
                "label": {"type": "plain_text", "text": "Visibility"},
            },
            {
                "type": "input",
                "block_id": "description_block",
                "element": {
                    "type": "plain_text_input",
                    "action_id": "description_input",
                    "multiline": True,
                    "initial_value": description[:3000],
                },
                "label": {"type": "plain_text", "text": "Description"},
            },
        ],
    }


def replace_actions_with_status(blocks: list[dict], task_id: str, status: str) -> list[dict]:
    """Replace action buttons for a task with a status indicator."""
    emoji = {"approved": ":white_check_mark:", "rejected": ":x:", "edited": ":pencil:"}.get(status, ":grey_question:")
    updated = []
    for block in blocks:
        if block.get("block_id") == f"draft_{task_id}":
            updated.append({
                "type": "context",
                "block_id": f"draft_{task_id}",
                "elements": [{"type": "mrkdwn", "text": f"{emoji} *{status.capitalize()}*"}],
            })
        else:
            updated.append(block)
    return updated


def format_task_list(tasks: list[dict], heading: str = "Your tasks") -> list[dict]:
    """Format a task list for Slack display."""
    if not tasks:
        return [{"type": "section", "text": {"type": "mrkdwn", "text": f"*{heading}*\nNo tasks found."}}]

    blocks: list[dict] = [{"type": "section", "text": {"type": "mrkdwn", "text": f"*{heading}* ({len(tasks)})"}}]
    blocks.append({"type": "divider"})

    priority_emoji = {"emergency": ":rotating_light:", "urgent": ":fire:", "standard": "", "low": ":snail:"}

    for task in tasks[:20]:  # Slack block limit
        emoji = priority_emoji.get(task.get("priority", ""), "")
        status = task.get("status", "?")
        deadline = task.get("deadline", "")
        deadline_str = f" -- due {deadline[:10]}" if deadline else ""

        text = f"{emoji} `{task['id']}` *{task.get('title', 'Untitled')}*\n{status}{deadline_str}"
        blocks.append({"type": "section", "text": {"type": "mrkdwn", "text": text}})

    if len(tasks) > 20:
        blocks.append({"type": "context", "elements": [{"type": "mrkdwn", "text": f"_...and {len(tasks) - 20} more_"}]})

    return blocks
