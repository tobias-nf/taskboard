// Taskboard API client — connects dashboard to the Go REST API

const API_BASE = process.env.NEXT_PUBLIC_TASKBOARD_API_URL || "http://localhost:4000/api/v1";
export const AUTH_BASE = process.env.NEXT_PUBLIC_TASKBOARD_AUTH_URL || "http://localhost:4000/auth";

// --- Session token (JWT from Google OAuth) ---

function getSessionToken(): string {
  if (typeof window === "undefined") return "";
  return localStorage.getItem("taskboard_session") || "";
}

export function setSessionToken(token: string) {
  localStorage.setItem("taskboard_session", token);
}

export function clearSession() {
  localStorage.removeItem("taskboard_session");
}

export function hasSession(): boolean {
  if (typeof window === "undefined") return false;
  return !!localStorage.getItem("taskboard_session");
}

async function apiFetch<T>(path: string, options: RequestInit = {}): Promise<T> {
  const token = getSessionToken();
  const res = await fetch(`${API_BASE}${path}`, {
    ...options,
    headers: {
      "Content-Type": "application/json",
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...options.headers,
    },
  });
  if (!res.ok) {
    const body = await res.json().catch(() => ({ error: "unknown", message: res.statusText }));
    throw new ApiError(res.status, body.error, body.message);
  }
  return res.json();
}

export class ApiError extends Error {
  constructor(public status: number, public code: string, message: string) {
    super(message);
  }
}

// --- Types matching the API responses ---

export interface Agent {
  id: string;
  type: "user" | "admin" | "service";
  email?: string;
  slack_id?: string;
  preferred_tool?: string;
  active: boolean;
  approved_by?: string;
  last_seen_at?: string;
  created_at: string;
  updated_at: string;
}

export interface AgentCreateInput {
  id: string;
  type: Agent["type"];
  email?: string;
}

export interface AgentUpdateInput {
  type?: Agent["type"];
  email?: string | null;
  slack_id?: string | null;
  preferred_tool?: string | null;
}

export interface AssignableAgent extends Agent {}

/** Display label for an agent: email if available, otherwise ID. */
export function agentLabel(agent: Agent | { id: string; email?: string }): string {
  return agent.email ?? agent.id;
}

/** Initials for avatar: first letter of each part of the email local part, or first 2 chars of ID. */
export function agentInitials(agent: Agent | { id: string; email?: string }): string {
  if (agent.email) {
    const local = agent.email.split("@")[0];
    const parts = local.split(".");
    return parts.map(p => p[0]).join("").toUpperCase().slice(0, 2);
  }
  return agent.id.slice(0, 2).toUpperCase();
}

export interface Task {
  id: string;
  title: string;
  description?: string;
  created_by: string;
  assigned_to?: string;
  visibility: "public" | "private";
  status: string;
  priority: string;
  deadline?: string;
  parent_id?: string;
  created_at: string;
  started_at?: string;
  completed_at?: string;
  updated_at: string;
}

export interface Tag {
  id: string;
  name: string;
  created_by: string;
  created_at: string;
}

export interface TaskOwedTo {
  task_id: string;
  agent_id: string;
  created_at: string;
}

export interface TaskMention {
  task_id: string;
  agent_id: string;
  created_at: string;
}

export interface TaskActivity {
  id: number;
  task_id: string;
  type: string;
  actor: string;
  actor_type: string;
  summary?: string;
  data?: unknown;
  old_value?: string;
  new_value?: string;
  created_at: string;
}

export interface TaskReference {
  id: number;
  task_id: string;
  type: string;
  source: string;
  external_id?: string;
  url?: string;
  title: string;
  metadata?: unknown;
  created_by: string;
  created_at: string;
}

export interface AuditEntry {
  id: number;
  action: string;
  actor: string;
  target_type?: string;
  target_id?: string;
  details?: unknown;
  created_at: string;
}

// --- API Calls ---

// Auth
export async function getMe(): Promise<Agent> {
  return apiFetch<Agent>("/agents/me");
}

export async function updateMe(fields: AgentUpdateInput): Promise<Agent> {
  return apiFetch<Agent>("/agents/me", { method: "PATCH", body: JSON.stringify(fields) });
}

export async function rotateMyKey(): Promise<{ api_key: string }> {
  return apiFetch<{ api_key: string }>("/agents/me/rotate-key", { method: "POST" });
}

// Agents
export async function listAgents(): Promise<{ agents: Agent[]; total: number }> {
  return apiFetch("/agents");
}

export async function getAgent(id: string): Promise<Agent> {
  return apiFetch(`/agents/${id}`);
}

export async function updateAgent(id: string, fields: AgentUpdateInput): Promise<Agent> {
  return apiFetch(`/agents/${id}`, { method: "PATCH", body: JSON.stringify(fields) });
}

export async function approveAgent(id: string): Promise<{ id: string; active: boolean }> {
  return apiFetch(`/agents/${id}/approve`, { method: "POST" });
}

export async function suspendAgent(id: string): Promise<{ id: string; active: boolean }> {
  return apiFetch(`/agents/${id}/suspend`, { method: "POST" });
}

export async function reactivateAgent(id: string): Promise<{ id: string; active: boolean }> {
  return apiFetch(`/agents/${id}/reactivate`, { method: "POST" });
}

export async function createAgent(req: AgentCreateInput): Promise<{ id: string; api_key: string; active: boolean }> {
  return apiFetch("/agents", { method: "POST", body: JSON.stringify(req) });
}

export async function adminRotateKey(agentId: string): Promise<{ api_key: string }> {
  return apiFetch(`/agents/${agentId}/rotate-key`, { method: "POST" });
}

export async function getAssignableAgents(): Promise<{ agents: AssignableAgent[]; total: number }> {
  return apiFetch("/agents/me/assignable");
}

// Tasks
export interface TaskListParams {
  status?: string;   // comma-separated, e.g. "pending,in_progress"
  priority?: string; // comma-separated
  tag?: string;      // filter by tag
  limit?: number;
  offset?: number;
  sort?: string;     // "deadline" | "created_at" | "priority"
}

function buildTaskQuery(params?: TaskListParams): string {
  if (!params) return "";
  const qs = new URLSearchParams();
  if (params.status) qs.set("status", params.status);
  if (params.priority) qs.set("priority", params.priority);
  if (params.tag) qs.set("tag", params.tag);
  if (params.limit) qs.set("limit", String(params.limit));
  if (params.offset) qs.set("offset", String(params.offset));
  if (params.sort) qs.set("sort", params.sort);
  const s = qs.toString();
  return s ? `?${s}` : "";
}

export async function createTask(task: Partial<Task>): Promise<Task> {
  return apiFetch("/tasks", { method: "POST", body: JSON.stringify(task) });
}

export async function getMyTasks(params?: TaskListParams): Promise<{ tasks: Task[]; total: number; limit: number; offset: number }> {
  return apiFetch(`/tasks/me${buildTaskQuery(params)}`);
}

export async function getMyCreatedTasks(params?: TaskListParams): Promise<{ tasks: Task[]; total: number; limit: number; offset: number }> {
  return apiFetch(`/tasks/me/created${buildTaskQuery(params)}`);
}

export async function getTasksOwedToMe(params?: TaskListParams): Promise<{ tasks: Task[]; total: number; limit: number; offset: number }> {
  return apiFetch(`/tasks/me/owed${buildTaskQuery(params)}`);
}

export async function getVisibleTasks(params?: TaskListParams): Promise<{ tasks: Task[]; total: number; limit: number; offset: number }> {
  return apiFetch(`/tasks/visible${buildTaskQuery(params)}`);
}

export async function getTask(id: string): Promise<Task> {
  return apiFetch(`/tasks/${id}`);
}

export async function updateTask(id: string, fields: Partial<Task> & { status?: string }): Promise<Task> {
  return apiFetch(`/tasks/${id}`, { method: "PATCH", body: JSON.stringify(fields) });
}

export async function cancelTask(id: string): Promise<{ id: string; status: string }> {
  return apiFetch(`/tasks/${id}`, { method: "DELETE" });
}

// Tags
export async function listTags(): Promise<{ tags: Tag[]; total: number }> {
  return apiFetch("/tags");
}

export async function createTag(name: string): Promise<Tag> {
  return apiFetch("/tags", { method: "POST", body: JSON.stringify({ name }) });
}

export async function deleteTag(id: string): Promise<void> {
  await apiFetch(`/tags/${id}`, { method: "DELETE" });
}

export async function getTaskTags(taskId: string): Promise<{ tags: Tag[]; total: number }> {
  return apiFetch(`/tasks/${taskId}/tags`);
}

export async function addTaskTag(taskId: string, tagId: string): Promise<void> {
  await apiFetch(`/tasks/${taskId}/tags`, { method: "POST", body: JSON.stringify({ tag_id: tagId }) });
}

export async function removeTaskTag(taskId: string, tagId: string): Promise<void> {
  await apiFetch(`/tasks/${taskId}/tags/${tagId}`, { method: "DELETE" });
}

// Owed-to (stakeholders)
export async function getTaskOwedTo(taskId: string): Promise<{ owed_to: TaskOwedTo[]; total: number }> {
  return apiFetch(`/tasks/${taskId}/owed-to`);
}

export async function addTaskOwedTo(taskId: string, agentId: string): Promise<TaskOwedTo> {
  return apiFetch(`/tasks/${taskId}/owed-to`, { method: "POST", body: JSON.stringify({ agent_id: agentId }) });
}

export async function removeTaskOwedTo(taskId: string, agentId: string): Promise<void> {
  await apiFetch(`/tasks/${taskId}/owed-to/${agentId}`, { method: "DELETE" });
}

// Mentions (access grants)
export async function getTaskMentions(taskId: string): Promise<{ mentions: TaskMention[]; total: number }> {
  return apiFetch(`/tasks/${taskId}/mentions`);
}

export async function addTaskMention(taskId: string, agentId: string): Promise<TaskMention> {
  return apiFetch(`/tasks/${taskId}/mentions`, { method: "POST", body: JSON.stringify({ agent_id: agentId }) });
}

export async function removeTaskMention(taskId: string, agentId: string): Promise<void> {
  await apiFetch(`/tasks/${taskId}/mentions/${agentId}`, { method: "DELETE" });
}

// Activity
export async function getTaskActivity(taskId: string): Promise<{ activity: TaskActivity[]; total: number }> {
  return apiFetch(`/tasks/${taskId}/activity`);
}

export async function addComment(taskId: string, summary: string): Promise<TaskActivity> {
  return apiFetch(`/tasks/${taskId}/activity`, {
    method: "POST",
    body: JSON.stringify({ type: "commented", summary, actor_type: "human" }),
  });
}

// Attachments
export interface TaskAttachment {
  id: number;
  task_id: string;
  filename: string;
  mime_type?: string;
  size_bytes?: number;
  sha256?: string;
  storage_key: string;
  url?: string;
  label?: string;
  uploaded_by: string;
  created_at: string;
}

export async function getTaskAttachments(taskId: string): Promise<{ attachments: TaskAttachment[]; total: number }> {
  return apiFetch(`/tasks/${taskId}/attachments`);
}

export async function uploadAttachment(taskId: string, file: File, label?: string): Promise<TaskAttachment> {
  const token = getSessionToken();
  const form = new FormData();
  form.append("file", file);
  if (label) form.append("label", label);

  const res = await fetch(`${API_BASE}/tasks/${taskId}/attachments`, {
    method: "POST",
    headers: { ...(token ? { Authorization: `Bearer ${token}` } : {}) },
    body: form,
  });
  if (!res.ok) {
    const body = await res.json().catch(() => ({ error: "unknown", message: res.statusText }));
    throw new ApiError(res.status, body.error, body.message);
  }
  return res.json();
}

export function getAttachmentDownloadUrl(attachmentId: number): string {
  return `${API_BASE}/attachments/${attachmentId}/download`;
}

export async function deleteAttachment(attachmentId: number): Promise<void> {
  await apiFetch(`/attachments/${attachmentId}`, { method: "DELETE" });
}

// References
export async function getTaskReferences(taskId: string): Promise<{ references: TaskReference[]; total: number }> {
  return apiFetch(`/tasks/${taskId}/references`);
}

export async function addReference(taskId: string, ref: Partial<TaskReference>): Promise<TaskReference> {
  return apiFetch(`/tasks/${taskId}/references`, { method: "POST", body: JSON.stringify(ref) });
}

export async function removeReference(taskId: string, refId: number): Promise<void> {
  await apiFetch(`/tasks/${taskId}/references/${refId}`, { method: "DELETE" });
}

// Audit
export async function getAuditLog(limit?: number): Promise<{ entries: AuditEntry[]; total: number }> {
  const params = limit ? `?limit=${limit}` : "";
  return apiFetch(`/admin/audit${params}`);
}
