#!/usr/bin/env bash
# Demo script: paste a meeting transcript and run it through the Fireflies bridge
# pipeline to create draft tasks in Taskboard.
#
# Usage:
#   ./scripts/demo-transcript.sh                    # interactive — paste transcript
#   ./scripts/demo-transcript.sh < transcript.txt   # pipe from file
#   echo "..." | ./scripts/demo-transcript.sh       # pipe from command

set -euo pipefail

BRIDGE_URL="${BRIDGE_URL:-http://localhost:14002}"
MEETING_TITLE="${MEETING_TITLE:-Demo Meeting}"

# Attendees — edit these for your demo
ATTENDEES='[
  {"displayName": "Tobias Holenstein", "email": "tobias.holenstein@near.foundation"},
  {"displayName": "Alice Johnson", "email": "alice@near.foundation"},
  {"displayName": "Bob Smith", "email": "bob@near.foundation"}
]'

if [ -t 0 ]; then
  echo "Paste your meeting transcript below, then press Ctrl-D when done:"
  echo "---"
fi

TRANSCRIPT_TEXT=$(cat)

if [ -z "$TRANSCRIPT_TEXT" ]; then
  echo "Error: no transcript provided" >&2
  exit 1
fi

# Build sentences array from the raw text (one sentence per line)
SENTENCES=$(echo "$TRANSCRIPT_TEXT" | python3 -c "
import sys, json
lines = [l.strip() for l in sys.stdin if l.strip()]
sentences = [{'text': line, 'ai_filters': {'task': False}} for line in lines]
print(json.dumps(sentences))
")

# Build the Transcript payload
PAYLOAD=$(python3 -c "
import json, time

payload = {
    'id': 'demo-' + str(int(time.time())),
    'title': $(echo "$MEETING_TITLE" | python3 -c "import sys,json; print(json.dumps(sys.stdin.read().strip()))"),
    'date': str(int(time.time() * 1000)),
    'organizer_email': 'tobias.holenstein@near.foundation',
    'participants': ['tobias.holenstein@near.foundation', 'alice@near.foundation', 'bob@near.foundation'],
    'meeting_attendees': $ATTENDEES,
    'speakers': [
        {'id': '1', 'name': 'Tobias Holenstein'},
        {'id': '2', 'name': 'Alice Johnson'},
        {'id': '3', 'name': 'Bob Smith'}
    ],
    'summary': {
        'action_items': None,
        'overview': $(echo "$TRANSCRIPT_TEXT" | head -5 | python3 -c "import sys,json; print(json.dumps(sys.stdin.read().strip()))"),
        'keywords': None,
        'gist': None
    },
    'sentences': $SENTENCES
}

print(json.dumps(payload))
")

echo ""
echo "Sending transcript to bridge ($BRIDGE_URL/test/process)..."
echo ""

RESPONSE=$(curl -s -X POST "$BRIDGE_URL/test/process" \
  -H "Content-Type: application/json" \
  -d "$PAYLOAD")

echo "$RESPONSE" | python3 -m json.tool 2>/dev/null || echo "$RESPONSE"
echo ""

ACTION_ITEMS=$(echo "$RESPONSE" | python3 -c "import sys,json; print(json.load(sys.stdin).get('action_items', 0))" 2>/dev/null || echo "?")
TASKS_CREATED=$(echo "$RESPONSE" | python3 -c "import sys,json; print(json.load(sys.stdin).get('tasks_created', 0))" 2>/dev/null || echo "?")

echo "Done: $ACTION_ITEMS action items found, $TASKS_CREATED draft tasks created."
echo "Check the Taskboard dashboard or Slack for approval requests."
