#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

dotenv_value() {
  local key="$1"
  local file="${REPO_ROOT}/.env"
  [[ -f "${file}" ]] || return 1

  python3 - "$file" "$key" <<'PY'
import sys

path, key = sys.argv[1], sys.argv[2]
with open(path, "r", encoding="utf-8") as fh:
    for line in fh:
        line = line.rstrip("\n")
        if not line or line.lstrip().startswith("#"):
            continue
        if "=" not in line:
            continue
        current_key, value = line.split("=", 1)
        if current_key == key:
            print(value)
            sys.exit(0)
sys.exit(1)
PY
}

if [[ -z "${TASKBOARD_ADMIN_API_KEY:-}" ]]; then
  TASKBOARD_ADMIN_API_KEY="$(dotenv_value TASKBOARD_ADMIN_API_KEY || true)"
fi
if [[ -z "${TASKBOARD_ENSURE_AGENT_1:-}" ]]; then
  TASKBOARD_ENSURE_AGENT_1="$(dotenv_value TASKBOARD_ENSURE_AGENT_1 || true)"
fi
if [[ -z "${TASKBOARD_ENSURE_AGENT_2:-}" ]]; then
  TASKBOARD_ENSURE_AGENT_2="$(dotenv_value TASKBOARD_ENSURE_AGENT_2 || true)"
fi

API_BASE="${TASKBOARD_API_URL:-http://localhost:14000/api/v1}"
ADMIN_KEY="${TASKBOARD_ADMIN_API_KEY:-hive_sk_hive-admin_replace-me}"
WORKSPACE_ID="${TASKBOARD_DEBUG_WORKSPACE_ID:-debug-assistants}"

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}"' EXIT

RESPONSE_STATUS=""
RESPONSE_BODY_FILE=""

lookup_ensured_api_key() {
  local agent_id="$1"
  local idx var_name raw ensured_key ensured_id

  for idx in 1 2 3 4 5; do
    var_name="TASKBOARD_ENSURE_AGENT_${idx}"
    raw="${!var_name:-}"
    [[ -n "${raw}" ]] || continue

    ensured_key="${raw%%|*}"
    ensured_id="${ensured_key#hive_sk_}"
    ensured_id="${ensured_id%%_*}"
    if [[ "${ensured_id}" == "${agent_id}" ]]; then
      printf '%s\n' "${ensured_key}"
      return 0
    fi
  done

  return 1
}

log() {
  printf '%s\n' "$*" >&2
}

fail() {
  printf 'Error: %s\n' "$*" >&2
  exit 1
}

json_field() {
  local file="$1"
  local field="$2"
  python3 - "$file" "$field" <<'PY'
import json
import sys

path = sys.argv[2].split(".")
with open(sys.argv[1], "r", encoding="utf-8") as fh:
    data = json.load(fh)

value = data
for part in path:
    if isinstance(value, dict):
        value = value.get(part)
    else:
        value = None
        break

if value is None:
    sys.exit(1)

if isinstance(value, (dict, list)):
    print(json.dumps(value))
else:
    print(value)
PY
}

request_json() {
  local method="$1"
  local path="$2"
  local data="${3:-}"
  local body_file
  body_file="$(mktemp "${TMP_DIR}/response.XXXXXX")"

  if [[ -n "${data}" ]]; then
    RESPONSE_STATUS="$(
      curl -sS -o "${body_file}" -w "%{http_code}" \
        -X "${method}" \
        -H "Authorization: Bearer ${ADMIN_KEY}" \
        -H "Content-Type: application/json" \
        "${API_BASE}${path}" \
        --data "${data}"
    )"
  else
    RESPONSE_STATUS="$(
      curl -sS -o "${body_file}" -w "%{http_code}" \
        -X "${method}" \
        -H "Authorization: Bearer ${ADMIN_KEY}" \
        "${API_BASE}${path}"
    )"
  fi

  RESPONSE_BODY_FILE="${body_file}"
}

request_file_upload() {
  local task_id="$1"
  local file_path="$2"
  local label="${3:-}"
  local body_file
  body_file="$(mktemp "${TMP_DIR}/upload.XXXXXX")"

  RESPONSE_STATUS="$(
    curl -sS -o "${body_file}" -w "%{http_code}" \
      -X POST \
      -H "Authorization: Bearer ${ADMIN_KEY}" \
      -F "file=@${file_path}" \
      -F "label=${label}" \
      "${API_BASE}/tasks/${task_id}/attachments"
  )"
  RESPONSE_BODY_FILE="${body_file}"
}

assert_ready() {
  request_json GET "/agents/me"
  if [[ "${RESPONSE_STATUS}" != "200" ]]; then
    cat "${RESPONSE_BODY_FILE}" >&2 || true
    fail "Taskboard API is not ready at ${API_BASE} or the admin key is invalid."
  fi
}

ensure_agent() {
  local agent_id="$1"
  local name="$2"
  local email="$3"
  local title="$4"
  local description="$5"
  local domains_json="$6"

  request_json GET "/agents/${agent_id}"
  if [[ "${RESPONSE_STATUS}" == "200" ]]; then
    log "Agent ${agent_id} already exists; rotating key for repeatable local access."
    request_json POST "/agents/${agent_id}/rotate-key"
    [[ "${RESPONSE_STATUS}" == "200" ]] || fail "Failed to rotate key for ${agent_id}: $(cat "${RESPONSE_BODY_FILE}")"
    json_field "${RESPONSE_BODY_FILE}" "api_key"
    return
  fi

  local payload
  payload="$(cat <<JSON
{"id":"${agent_id}","name":"${name}","type":"personal","description":"${description}","email":"${email}","title":"${title}","domains":${domains_json}}
JSON
)"
  request_json POST "/agents" "${payload}"
  [[ "${RESPONSE_STATUS}" == "201" ]] || fail "Failed to create agent ${agent_id}: $(cat "${RESPONSE_BODY_FILE}")"
  json_field "${RESPONSE_BODY_FILE}" "api_key"
}

ensure_workspace() {
  request_json GET "/workspaces/${WORKSPACE_ID}"
  if [[ "${RESPONSE_STATUS}" == "200" ]]; then
    log "Workspace ${WORKSPACE_ID} already exists."
    return
  fi

  local payload
  payload="$(cat <<JSON
{"id":"${WORKSPACE_ID}","name":"Debug Assistants","description":"Shared workspace for frontend debugging tasks and attachments"}
JSON
)"
  request_json POST "/workspaces" "${payload}"
  [[ "${RESPONSE_STATUS}" == "201" ]] || fail "Failed to create workspace ${WORKSPACE_ID}: $(cat "${RESPONSE_BODY_FILE}")"
  log "Created workspace ${WORKSPACE_ID}."
}

workspace_has_member() {
  local agent_id="$1"
  request_json GET "/workspaces/${WORKSPACE_ID}/members"
  [[ "${RESPONSE_STATUS}" == "200" ]] || fail "Failed to list members for ${WORKSPACE_ID}: $(cat "${RESPONSE_BODY_FILE}")"

  python3 - "${RESPONSE_BODY_FILE}" "${agent_id}" <<'PY'
import json
import sys

with open(sys.argv[1], "r", encoding="utf-8") as fh:
    payload = json.load(fh)

agent_id = sys.argv[2]
found = any(member.get("agent_id") == agent_id for member in (payload.get("members") or []))
sys.exit(0 if found else 1)
PY
}

ensure_workspace_member() {
  local agent_id="$1"
  local role="${2:-member}"

  if workspace_has_member "${agent_id}"; then
    log "Workspace member ${agent_id} already present."
    return
  fi

  local payload
  payload="$(cat <<JSON
{"agent_id":"${agent_id}","role":"${role}"}
JSON
)"
  request_json POST "/workspaces/${WORKSPACE_ID}/members" "${payload}"
  [[ "${RESPONSE_STATUS}" == "201" ]] || fail "Failed to add ${agent_id} to ${WORKSPACE_ID}: $(cat "${RESPONSE_BODY_FILE}")"
  log "Added ${agent_id} to workspace ${WORKSPACE_ID}."
}

find_task_id_by_title() {
  local title="$1"
  request_json GET "/workspaces/${WORKSPACE_ID}/tasks?limit=200"
  [[ "${RESPONSE_STATUS}" == "200" ]] || fail "Failed to list tasks for ${WORKSPACE_ID}: $(cat "${RESPONSE_BODY_FILE}")"

  python3 - "${RESPONSE_BODY_FILE}" "${title}" <<'PY'
import json
import sys

with open(sys.argv[1], "r", encoding="utf-8") as fh:
    payload = json.load(fh)

title = sys.argv[2]
for task in (payload.get("tasks") or []):
    if task.get("title") == title:
        print(task["id"])
        break
PY
}

ensure_task() {
  local title="$1"
  local assigned_to="$2"
  local priority="$3"
  local status="$4"
  local description="$5"
  local notes="$6"

  local task_id
  task_id="$(find_task_id_by_title "${title}")"
  if [[ -z "${task_id}" ]]; then
    local payload
    payload="$(cat <<JSON
{"title":"${title}","description":"${description}","assigned_to":"${assigned_to}","workspace_id":"${WORKSPACE_ID}","priority":"${priority}","notes":"${notes}"}
JSON
)"
    request_json POST "/tasks" "${payload}"
    [[ "${RESPONSE_STATUS}" == "201" ]] || fail "Failed to create task '${title}': $(cat "${RESPONSE_BODY_FILE}")"
    task_id="$(json_field "${RESPONSE_BODY_FILE}" "id")"
    log "Created task ${task_id}: ${title}"
  else
    log "Task already exists as ${task_id}: ${title}"
  fi

  request_json GET "/tasks/${task_id}"
  [[ "${RESPONSE_STATUS}" == "200" ]] || fail "Failed to fetch task ${task_id}: $(cat "${RESPONSE_BODY_FILE}")"
  local current_status
  current_status="$(json_field "${RESPONSE_BODY_FILE}" "status")"

  while [[ "${current_status}" != "${status}" ]]; do
    local next_status=""
    if [[ "${current_status}" == "pending" && "${status}" == "accepted" ]]; then
      next_status="accepted"
    elif [[ "${current_status}" == "accepted" && "${status}" == "in_progress" ]]; then
      next_status="in_progress"
    elif [[ "${current_status}" == "review" && ( "${status}" == "completed" || "${status}" == "in_progress" ) ]]; then
      next_status="${status}"
    elif [[ "${current_status}" == "blocked" && "${status}" == "in_progress" ]]; then
      next_status="in_progress"
    elif [[ "${current_status}" == "in_progress" && ( "${status}" == "review" || "${status}" == "completed" || "${status}" == "blocked" ) ]]; then
      next_status="${status}"
    elif [[ "${current_status}" == "pending" && ( "${status}" == "in_progress" || "${status}" == "completed" || "${status}" == "blocked" ) ]]; then
      next_status="accepted"
    elif [[ "${current_status}" == "accepted" && ( "${status}" == "completed" || "${status}" == "blocked" ) ]]; then
      next_status="in_progress"
    elif [[ "${current_status}" == "blocked" && "${status}" == "completed" ]]; then
      next_status="in_progress"
    else
      fail "Unsupported status transition while seeding ${task_id}: ${current_status} -> ${status}"
    fi

    request_json PATCH "/tasks/${task_id}" "{\"status\":\"${next_status}\"}"
    [[ "${RESPONSE_STATUS}" == "200" ]] || fail "Failed to update status for ${task_id}: $(cat "${RESPONSE_BODY_FILE}")"
    current_status="${next_status}"
  done

  printf '%s\n' "${task_id}"
}

task_has_attachment() {
  local task_id="$1"
  local filename="$2"
  request_json GET "/tasks/${task_id}/attachments"
  [[ "${RESPONSE_STATUS}" == "200" ]] || fail "Failed to list attachments for ${task_id}: $(cat "${RESPONSE_BODY_FILE}")"

  python3 - "${RESPONSE_BODY_FILE}" "${filename}" <<'PY'
import json
import sys

with open(sys.argv[1], "r", encoding="utf-8") as fh:
    payload = json.load(fh)

filename = sys.argv[2]
found = any(att.get("filename") == filename for att in (payload.get("attachments") or []))
sys.exit(0 if found else 1)
PY
}

ensure_attachment() {
  local task_id="$1"
  local filename="$2"
  local label="$3"
  local content="$4"

  if task_has_attachment "${task_id}" "${filename}"; then
    log "Attachment already exists on ${task_id}: ${filename}"
    return
  fi

  local file_path="${TMP_DIR}/${filename}"
  printf '%s\n' "${content}" > "${file_path}"
  request_file_upload "${task_id}" "${file_path}" "${label}"
  [[ "${RESPONSE_STATUS}" == "201" ]] || fail "Failed to upload ${filename} to ${task_id}: $(cat "${RESPONSE_BODY_FILE}")"
  log "Uploaded attachment to ${task_id}: ${filename}"
}

main() {
  log "Checking Taskboard API at ${API_BASE}..."
  assert_ready

  local bob_key alice_key
  if bob_key="$(lookup_ensured_api_key "bob-personal")"; then
    log "Using env-managed agent bob-personal from TASKBOARD_ENSURE_AGENT_*."
  else
    bob_key="$(ensure_agent \
      "bob-personal" \
      "Bob Smith" \
      "bob@near.foundation" \
      "Personal Assistant" \
      "Local debugging assistant for Bob" \
      '["assistant","operations","debug"]'
    )"
  fi
  if alice_key="$(lookup_ensured_api_key "alice-personal")"; then
    log "Using env-managed agent alice-personal from TASKBOARD_ENSURE_AGENT_*."
  else
    alice_key="$(ensure_agent \
      "alice-personal" \
      "Alice Johnson" \
      "alice@near.foundation" \
      "Personal Assistant" \
      "Local debugging assistant for Alice" \
      '["assistant","calendar","debug"]'
    )"
  fi

  ensure_workspace
  ensure_workspace_member "bob-personal"
  ensure_workspace_member "alice-personal"

  local task_bob_briefing task_alice_inbox task_alice_packet task_bob_receipts
  task_bob_briefing="$(ensure_task \
    "Prepare Bob travel briefing for Zurich meetings" \
    "bob-personal" \
    "urgent" \
    "in_progress" \
    "Create a concise travel briefing covering agenda, contacts, and open prep items for the Bob Zurich trip." \
    "Used for frontend debugging: attachment-rich task with realistic assistant notes."
  )"
  task_alice_inbox="$(ensure_task \
    "Triage Alice inbox for overdue follow-ups" \
    "alice-personal" \
    "standard" \
    "accepted" \
    "Sort the inbox backlog into reply now, delegate, and archive buckets before the end of the day." \
    "Used for frontend debugging: a lighter task with one structured attachment."
  )"
  task_alice_packet="$(ensure_task \
    "Draft founder update packet for Monday sync" \
    "alice-personal" \
    "standard" \
    "pending" \
    "Assemble the Monday founder packet with product highlights, hiring notes, and open risks." \
    "Used for frontend debugging: pending state plus attachment download coverage."
  )"
  task_bob_receipts="$(ensure_task \
    "Collect Bob expense receipts for March close" \
    "bob-personal" \
    "low" \
    "blocked" \
    "Collect the missing hotel, rail, and meal receipts before finance closes the month." \
    "Blocked on vendor portals still emailing missing PDFs."
  )"

  ensure_attachment "${task_bob_briefing}" "zurich-briefing.md" "Travel Briefing" "# Zurich Briefing

- Monday 09:00 product sync with founders
- Tuesday 14:00 partner review near Bahnhofstrasse
- Open items: finalize dinner invite list, confirm rail transfer, print hotel confirmation"

  ensure_attachment "${task_bob_briefing}" "zurich-itinerary.csv" "Itinerary" "date,time,item,owner
2026-03-24,09:00,Founder sync,Bob
2026-03-24,12:30,Investor lunch,Bob
2026-03-25,14:00,Partner review,Bob"

  ensure_attachment "${task_alice_inbox}" "alice-inbox-triage.json" "Inbox Snapshot" '{
  "reply_now": 6,
  "delegate": 4,
  "archive": 11,
  "top_threads": [
    "Board deck comments",
    "Vendor renewal follow-up",
    "Calendar reshuffle for Monday sync"
  ]
}'

  ensure_attachment "${task_alice_packet}" "founder-update-outline.md" "Founder Packet Outline" "# Founder Update Outline

1. Product milestones
2. GTM highlights
3. Hiring updates
4. Risks and asks"

  log ""
  log "Debug seed complete."
  log ""
  log "Workspace:"
  log "  ${WORKSPACE_ID}"
  log ""
  log "Agent keys:"
  log "  bob-personal:   ${bob_key}"
  log "  alice-personal: ${alice_key}"
  log ""
  log "Task URLs:"
  log "  http://localhost:14001/app/tasks/${task_bob_briefing}"
  log "  http://localhost:14001/app/tasks/${task_alice_inbox}"
  log "  http://localhost:14001/app/tasks/${task_alice_packet}"
  log "  http://localhost:14001/app/tasks/${task_bob_receipts}"
}

main "$@"
