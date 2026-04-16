# Taskboard IronClaw Skills

Two IronClaw skills for task management with local-first storage and cloud sync to the Taskboard API.

## Skills

| File | Skill | Description |
|------|-------|-------------|
| `SKILL.md` | `taskboard` | Core task management — signal detection, task CRUD, digest, cloud sync |
| `MEETINGS.md` | `taskboard-meetings` | Meeting transcript → action items → deduplicated tasks with approval |

## Relationship to IronClaw Skills

This skill is designed for [IronClaw](https://github.com/nearai/ironclaw), an AI agent framework. It replaces the three existing commitment tracking skills:

- [`commitment-setup`](https://github.com/nearai/ironclaw/tree/staging/skills/commitment-setup) — workspace initialization and mission creation
- [`commitment-triage`](https://github.com/nearai/ironclaw/tree/staging/skills/commitment-triage) — passive signal detection and explicit task capture
- [`commitment-digest`](https://github.com/nearai/ironclaw/tree/staging/skills/commitment-digest) — daily summaries and status reports

The taskboard skill merges all three into a single skill backed by a cloud API, while keeping local-first performance for signal detection. Remove the commitment-* skills after installing this one.

See all available IronClaw skills: https://github.com/nearai/ironclaw/tree/staging/skills

## Install

Copy both skills to your IronClaw skills directory:
```bash
cp SKILL.md /path/to/ironclaw/skills/taskboard/SKILL.md
cp MEETINGS.md /path/to/ironclaw/skills/taskboard-meetings/SKILL.md
```

Then say "setup taskboard" to initialize. The setup flow will:
1. Ask for your Taskboard instance URL (prod/dev/self-hosted)
2. Create the local workspace structure (`tasks/`)
3. Install triage and digest missions
4. Pull existing tasks from the cloud

## How it works

- **Local files** in `tasks/` for instant response (zero API latency on signal detection)
- **Background sync** pushes to / pulls from Taskboard REST API after task mutations
- **Direct HTTP calls** — full API reference embedded in the skill file
- **Cloud wins** on conflicts — Taskboard has the multi-user truth
- **Missions** run automatically: triage (twice daily), digest (weekday mornings)

## API Key

Get your key from the Taskboard dashboard (Settings) or ask an admin.
Format: `hive_sk_<agent-id>_<secret>`

The key is managed by IronClaw's credential system — the skill never sees it directly.
