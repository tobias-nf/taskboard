"use client";

import { useEffect, useState, useRef } from "react";
import { useTaskboard } from "@/lib/taskboard-context";
import * as api from "@/lib/api";
import type { Task, TaskActivity, TaskReference, TaskAttachment, Tag, TaskOwedTo, TaskMention, Agent } from "@/lib/api";
import { TaskActions } from "@/components/task-actions";
import Link from "next/link";
import { useParams, useRouter } from "next/navigation";

const statusStyles: Record<string, { bg: string; color: string }> = {
  pending:     { bg: "rgba(156,163,175,0.15)", color: "#6B7280" },
  in_progress: { bg: "rgba(245,158,11,0.12)", color: "#D97706" },
  blocked:     { bg: "rgba(239,68,68,0.12)", color: "#EF4444" },
  review:      { bg: "rgba(139,92,246,0.12)", color: "#8B5CF6" },
  completed:   { bg: "rgba(0,236,151,0.15)", color: "#059669" },
  failed:      { bg: "rgba(239,68,68,0.12)", color: "#EF4444" },
  cancelled:   { bg: "rgba(156,163,175,0.10)", color: "#9CA3AF" },
};

const priorityStyles: Record<string, { bg: string; color: string }> = {
  emergency: { bg: "rgba(239,68,68,0.1)", color: "#EF4444" },
  urgent:    { bg: "rgba(245,158,11,0.1)", color: "#D97706" },
  standard:  { bg: "rgba(0,236,151,0.10)", color: "#059669" },
  low:       { bg: "var(--bg-tertiary)", color: "var(--text-faint)" },
};

function formatBytes(bytes: number): string {
  if (bytes < 1024) return bytes + " B";
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + " KB";
  return (bytes / (1024 * 1024)).toFixed(1) + " MB";
}

function timeAgo(dateStr: string): string {
  const diff = Date.now() - new Date(dateStr).getTime();
  const mins = Math.floor(diff / 60000);
  if (mins < 1) return "just now";
  if (mins < 60) return `${mins}m ago`;
  const hours = Math.floor(mins / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  if (days < 30) return `${days}d ago`;
  return new Date(dateStr).toLocaleDateString();
}

export default function TaskDetail() {
  const params = useParams();
  const router = useRouter();
  const id = params.id as string;
  const { getAgentName, agents } = useTaskboard();
  const [task, setTask] = useState<Task | null>(null);
  const [activity, setActivity] = useState<TaskActivity[]>([]);
  const [references, setReferences] = useState<TaskReference[]>([]);
  const [attachments, setAttachments] = useState<TaskAttachment[]>([]);
  const [tags, setTags] = useState<Tag[]>([]);
  const [owedTo, setOwedTo] = useState<TaskOwedTo[]>([]);
  const [mentions, setMentions] = useState<TaskMention[]>([]);
  const [subtasks, setSubtasks] = useState<Task[]>([]);
  const [comment, setComment] = useState("");
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    Promise.all([
      api.getTask(id),
      api.getTaskActivity(id),
      api.getTaskReferences(id),
      api.getTaskAttachments(id),
      api.getTaskTags(id),
      api.getTaskOwedTo(id),
      api.getTaskMentions(id),
      api.getVisibleTasks({ limit: 50 }), // for subtasks
    ]).then(([t, act, refs, atts, tagRes, owedRes, mentionRes, visible]) => {
      setTask(t);
      setActivity(act.activity || []);
      setReferences(refs.references || []);
      setAttachments(atts.attachments || []);
      setTags(tagRes.tags || []);
      setOwedTo(owedRes.owed_to || []);
      setMentions(mentionRes.mentions || []);
      setSubtasks((visible.tasks || []).filter((st: Task) => st.parent_id === id));
      setLoading(false);
    }).catch(() => setLoading(false));
  }, [id]);

  async function handleComment() {
    if (!comment.trim()) return;
    try {
      const entry = await api.addComment(id, comment.trim());
      setActivity(prev => [...prev, entry]);
      setComment("");
    } catch (e) {
      console.error("Failed to add comment:", e);
    }
  }

  if (loading) return <div style={{ padding: 32, color: "var(--text-faint)", fontSize: 14 }}>Loading...</div>;
  if (!task) return <div style={{ padding: 32, color: "var(--text-faint)" }}>Task not found: {id}</div>;

  const creator = agents.find(a => a.id === task.created_by);
  const assignee = agents.find(a => a.id === task.assigned_to);
  const isOverdue = task.deadline && new Date(task.deadline) < new Date() && !["completed", "cancelled", "failed"].includes(task.status);
  const ss = statusStyles[task.status] || statusStyles.pending;
  const ps = priorityStyles[task.priority] || priorityStyles.standard;

  return (
    <div style={{ maxWidth: 1000, margin: "0 auto", padding: "24px 24px" }}>
      {/* Back */}
      <button
        onClick={() => router.back()}
        style={{ fontSize: 13, color: "var(--text-faint)", background: "none", border: "none", cursor: "pointer", marginBottom: 16, display: "flex", alignItems: "center", gap: 4 }}
      >
        ← Back
      </button>

      {/* Header */}
      <div style={{ marginBottom: 24 }}>
        <div style={{ display: "flex", alignItems: "center", gap: 8, marginBottom: 8, flexWrap: "wrap" }}>
          <span style={{ fontFamily: "monospace", fontSize: 12, color: "var(--text-faint)" }}>{task.id}</span>
          <Pill bg={ss.bg} color={ss.color}>{task.status.replace("_", " ")}</Pill>
          <Pill bg={ps.bg} color={ps.color}>{task.priority}</Pill>
          {task.visibility === "private"
            ? <Pill bg="#FFF7ED" color="#C2410C" border="rgba(253,186,116,0.25)">private</Pill>
            : <Pill bg="rgba(0,236,151,0.10)" color="#059669">public</Pill>
          }
          {isOverdue && <Pill bg="rgba(239,68,68,0.1)" color="#EF4444">overdue</Pill>}
        </div>
        <h1 style={{ fontSize: 22, fontWeight: 700, letterSpacing: "-0.025em", lineHeight: 1.3 }}>{task.title}</h1>
        {tags.length > 0 && (
          <div style={{ display: "flex", gap: 4, marginTop: 8 }}>
            {tags.map(t => (
              <span key={t.id} style={{
                fontSize: 11, fontWeight: 500, padding: "2px 8px", borderRadius: 9999,
                border: "1px solid var(--border)", color: "var(--text-muted)",
              }}>{t.name}</span>
            ))}
          </div>
        )}
      </div>

      <div style={{ display: "grid", gridTemplateColumns: "1fr 300px", gap: 24 }}>
        {/* Left column */}
        <div style={{ display: "flex", flexDirection: "column", gap: 16 }}>
          {/* Description */}
          <Card title="Description">
            {task.description ? (
              <pre style={{ whiteSpace: "pre-wrap", fontSize: 14, fontFamily: "inherit", lineHeight: 1.6, margin: 0 }}>{task.description}</pre>
            ) : (
              <Empty />
            )}
          </Card>

          {/* Subtasks */}
          {subtasks.length > 0 && (
            <Card title={`Subtasks (${subtasks.length})`}>
              <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
                {subtasks.map(st => {
                  const sts = statusStyles[st.status] || statusStyles.pending;
                  return (
                    <Link key={st.id} href={`/tasks/${st.id}`} style={{ textDecoration: "none", color: "inherit" }}>
                      <div style={{
                        display: "flex", alignItems: "center", gap: 8, padding: "6px 10px",
                        borderRadius: 8, border: "1px solid var(--border)", fontSize: 13,
                        transition: "all 0.15s",
                      }} className="hover:shadow-sm">
                        <span style={{ fontSize: 14, color: sts.color }}>
                          {st.status === "completed" ? "✓" : st.status === "in_progress" ? "◑" : "○"}
                        </span>
                        <span style={{ flex: 1, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{st.title}</span>
                        <span style={{ fontSize: 10, fontFamily: "monospace", color: "var(--text-faint)" }}>{st.id}</span>
                      </div>
                    </Link>
                  );
                })}
              </div>
            </Card>
          )}

          {/* Activity + Comments */}
          <Card title="Activity">
            <div style={{ display: "flex", flexDirection: "column", gap: 0 }}>
              {activity.map((entry, i) => (
                <div key={entry.id} style={{ display: "flex", gap: 10, paddingBottom: 12 }}>
                  <div style={{ display: "flex", flexDirection: "column", alignItems: "center" }}>
                    <div style={{
                      width: 24, height: 24, borderRadius: 12, display: "flex", alignItems: "center", justifyContent: "center",
                      fontSize: 11, background: entry.type === "commented" ? "rgba(0,236,151,0.15)" : "var(--bg-tertiary)",
                      border: "1px solid var(--border)", flexShrink: 0,
                    }}>
                      {entry.type === "commented" ? "💬" : entry.type === "status_changed" ? "→" : entry.type === "created" ? "+" : "·"}
                    </div>
                    {i < activity.length - 1 && <div style={{ width: 1, flex: 1, background: "var(--border)", marginTop: 4 }} />}
                  </div>
                  <div style={{ flex: 1, minWidth: 0 }}>
                    <p style={{ fontSize: 13, lineHeight: 1.5 }}>
                      <span style={{ fontWeight: 500 }}>{getAgentName(entry.actor)}</span>
                      <span style={{ color: "var(--text-muted)" }}> {entry.summary}</span>
                    </p>
                    <p style={{ fontSize: 11, color: "var(--text-faint)", marginTop: 1 }}>{timeAgo(entry.created_at)}</p>
                  </div>
                </div>
              ))}
              {activity.length === 0 && (
                <p style={{ fontSize: 13, color: "var(--text-faint)", fontStyle: "italic" }}>No activity yet</p>
              )}
            </div>

            {/* Comment box with @mention autocomplete */}
            <div style={{ marginTop: 12, paddingTop: 12, borderTop: "1px solid var(--border)" }}>
              <MentionTextarea
                value={comment}
                onChange={setComment}
                onSubmit={handleComment}
                agents={agents}
                placeholder="Add a comment... Use @ to mention"
              />
            </div>
          </Card>
        </div>

        {/* Right sidebar */}
        <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
          {/* People */}
          <Card title="People">
            <MetaRow label="Created by" value={creator?.email ?? task.created_by} />
            <MetaRow
              label="Assigned to"
              value={assignee?.email ?? task.assigned_to ?? "Unassigned"}
              faded={!task.assigned_to}
            />
            <div style={{ marginTop: 8 }}>
              <p style={{ fontSize: 10, color: "var(--text-faint)", textTransform: "uppercase", letterSpacing: "0.05em", marginBottom: 4 }}>Owed to</p>
              {owedTo.length > 0 ? owedTo.map(o => (
                <p key={o.agent_id} style={{ fontSize: 13 }}>{getAgentName(o.agent_id)}</p>
              )) : <Empty />}
            </div>
            <div style={{ marginTop: 8 }}>
              <p style={{ fontSize: 10, color: "var(--text-faint)", textTransform: "uppercase", letterSpacing: "0.05em", marginBottom: 4 }}>Mentioned</p>
              {mentions.length > 0 ? mentions.map(m => (
                <p key={m.agent_id} style={{ fontSize: 13 }}>{getAgentName(m.agent_id)}</p>
              )) : <Empty />}
            </div>
          </Card>

          {/* Details */}
          <Card title="Details">
            <MetaRow
              label="Deadline"
              value={task.deadline ? new Date(task.deadline).toLocaleDateString() : "—"}
              color={isOverdue ? "#EF4444" : undefined}
              faded={!task.deadline}
            />
            <div style={{ marginTop: 4 }}>
              <p style={{ fontSize: 10, color: "var(--text-faint)", textTransform: "uppercase", letterSpacing: "0.05em" }}>Parent</p>
              {task.parent_id ? (
                <Link href={`/tasks/${task.parent_id}`} style={{ fontSize: 13, color: "var(--accent-text, #059669)", textDecoration: "none" }}>
                  {task.parent_id}
                </Link>
              ) : <Empty />}
            </div>
            <div style={{ marginTop: 8, paddingTop: 8, borderTop: "1px solid var(--border)" }}>
              <TimelineRow label="Created" date={task.created_at} />
              {task.started_at && <TimelineRow label="Started" date={task.started_at} />}
              {task.completed_at && <TimelineRow label="Completed" date={task.completed_at} />}
              <TimelineRow label="Updated" date={task.updated_at} />
            </div>
          </Card>

          {/* References */}
          <Card title={`References${references.length > 0 ? ` (${references.length})` : ""}`}>
            {references.length > 0 ? references.map(ref => (
              <div key={ref.id} style={{ display: "flex", alignItems: "center", gap: 6, marginBottom: 4 }}>
                <span style={{ fontSize: 10, padding: "1px 5px", borderRadius: 4, background: "var(--bg-tertiary)", color: "var(--text-faint)" }}>{ref.type}</span>
                {ref.url ? (
                  <a href={ref.url} target="_blank" rel="noopener noreferrer" style={{ fontSize: 12, color: "var(--accent-text, #059669)" }}>{ref.title}</a>
                ) : (
                  <span style={{ fontSize: 12 }}>{ref.title}</span>
                )}
              </div>
            )) : <Empty />}
          </Card>

          {/* Attachments */}
          <Card title={`Attachments${attachments.length > 0 ? ` (${attachments.length})` : ""}`}>
            {attachments.length > 0 ? attachments.map(att => (
              <div key={att.id} style={{ display: "flex", alignItems: "center", gap: 6, marginBottom: 4, fontSize: 12 }}>
                <span>📄</span>
                <a href={api.getAttachmentDownloadUrl(att.id)} target="_blank" rel="noopener noreferrer"
                  style={{ color: "var(--accent-text, #059669)", flex: 1, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                  {att.filename}
                </a>
                {att.size_bytes && <span style={{ color: "var(--text-faint)", fontSize: 11 }}>{formatBytes(att.size_bytes)}</span>}
              </div>
            )) : <Empty />}
          </Card>

          <TaskActions status={task.status} />
        </div>
      </div>
    </div>
  );
}

// ── Sub-components ─────────────────────────────────────

function MentionTextarea({ value, onChange, onSubmit, agents, placeholder }: {
  value: string;
  onChange: (v: string) => void;
  onSubmit: () => void;
  agents: Agent[];
  placeholder: string;
}) {
  const [showSuggestions, setShowSuggestions] = useState(false);
  const [suggestions, setSuggestions] = useState<Agent[]>([]);
  const [selectedIdx, setSelectedIdx] = useState(0);
  const [mentionStart, setMentionStart] = useState(-1);
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  function handleChange(e: React.ChangeEvent<HTMLTextAreaElement>) {
    const text = e.target.value;
    const cursor = e.target.selectionStart;
    onChange(text);

    // Find if we're in an @mention context
    const before = text.slice(0, cursor);
    const atIdx = before.lastIndexOf("@");

    if (atIdx >= 0 && (atIdx === 0 || /\s/.test(before[atIdx - 1]))) {
      const query = before.slice(atIdx + 1).toLowerCase();
      // Only show if no space after the query (still typing the mention)
      if (!query.includes(" ")) {
        const matches = agents.filter(a =>
          a.active && (
            a.id.toLowerCase().includes(query) ||
            (a.email ?? "").toLowerCase().includes(query)
          )
        ).slice(0, 6);
        setSuggestions(matches);
        setShowSuggestions(matches.length > 0);
        setMentionStart(atIdx);
        setSelectedIdx(0);
        return;
      }
    }

    setShowSuggestions(false);
  }

  function insertMention(agent: Agent) {
    const before = value.slice(0, mentionStart);
    const afterCursor = textareaRef.current ? value.slice(textareaRef.current.selectionStart) : "";
    const newValue = `${before}@${agent.id} ${afterCursor}`;
    onChange(newValue);
    setShowSuggestions(false);
    textareaRef.current?.focus();
  }

  function handleKeyDown(e: React.KeyboardEvent) {
    if (!showSuggestions) {
      if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) {
        e.preventDefault();
        onSubmit();
      }
      return;
    }

    if (e.key === "ArrowDown") {
      e.preventDefault();
      setSelectedIdx(i => Math.min(i + 1, suggestions.length - 1));
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setSelectedIdx(i => Math.max(i - 1, 0));
    } else if (e.key === "Enter" || e.key === "Tab") {
      e.preventDefault();
      if (suggestions[selectedIdx]) insertMention(suggestions[selectedIdx]);
    } else if (e.key === "Escape") {
      setShowSuggestions(false);
    }
  }

  return (
    <div style={{ position: "relative" }}>
      <textarea
        ref={textareaRef}
        value={value}
        onChange={handleChange}
        onKeyDown={handleKeyDown}
        placeholder={placeholder}
        onBlur={() => setTimeout(() => setShowSuggestions(false), 150)}
        style={{
          width: "100%", background: "var(--bg)", border: "1px solid var(--border)",
          borderRadius: 8, padding: 10, fontSize: 13, resize: "none", height: 60,
          outline: "none", fontFamily: "inherit",
        }}
        onFocus={e => e.target.style.borderColor = "var(--accent)"}
      />

      {/* Suggestions dropdown */}
      {showSuggestions && (
        <div style={{
          position: "absolute", bottom: "calc(100% + 4px)", left: 0,
          width: "100%", maxWidth: 280, borderRadius: 10, padding: 4,
          border: "1px solid var(--border)", background: "var(--surface)",
          boxShadow: "0 4px 12px rgba(0,0,0,0.10)", zIndex: 50,
        }}>
          {suggestions.map((agent, i) => (
            <button
              key={agent.id}
              onMouseDown={(e) => { e.preventDefault(); insertMention(agent); }}
              style={{
                display: "flex", alignItems: "center", gap: 8, width: "100%",
                padding: "6px 10px", borderRadius: 6, border: "none", textAlign: "left",
                background: i === selectedIdx ? "var(--bg-secondary)" : "transparent",
                cursor: "pointer", fontSize: 13,
              }}
              onMouseEnter={() => setSelectedIdx(i)}
            >
              <div style={{
                width: 22, height: 22, borderRadius: 11, background: "var(--accent)",
                display: "flex", alignItems: "center", justifyContent: "center",
                fontSize: 9, fontWeight: 700, color: "#111827", flexShrink: 0,
              }}>
                {(agent.email ?? agent.id).slice(0, 2).toUpperCase()}
              </div>
              <div style={{ flex: 1, minWidth: 0 }}>
                <div style={{ fontWeight: 500, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{agent.email ?? agent.id}</div>
              </div>
              {agent.type === "admin" && (
                <span style={{ fontSize: 9, color: "var(--text-faint)", background: "var(--bg-tertiary)", padding: "1px 4px", borderRadius: 3 }}>admin</span>
              )}
            </button>
          ))}
        </div>
      )}

      <div style={{ display: "flex", alignItems: "center", gap: 8, marginTop: 6 }}>
        <button
          onClick={onSubmit}
          disabled={!value.trim()}
          style={{
            padding: "6px 14px", borderRadius: 8, border: "none",
            background: "var(--accent)", color: "#111827", fontSize: 12, fontWeight: 600,
            cursor: value.trim() ? "pointer" : "default", opacity: value.trim() ? 1 : 0.4,
          }}
        >
          Comment
        </button>
        <span style={{ fontSize: 11, color: "var(--text-faint)" }}>⌘ Enter to submit</span>
      </div>
    </div>
  );
}

function Empty() {
  return <p style={{ fontSize: 13, color: "var(--text-faint)", fontStyle: "italic" }}>—</p>;
}

function Pill({ children, bg, color, border }: { children: React.ReactNode; bg: string; color: string; border?: string }) {
  return (
    <span style={{
      fontSize: 11, fontWeight: 600, padding: "2px 8px", borderRadius: 9999,
      background: bg, color, border: border ? `1px solid ${border}` : "none",
      textTransform: "capitalize",
    }}>
      {children}
    </span>
  );
}

function Card({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section style={{
      background: "var(--surface)", border: "1px solid var(--border)",
      borderRadius: 10, padding: 16, boxShadow: "0 1px 2px rgba(0,0,0,0.04)",
    }}>
      <h3 style={{ fontSize: 12, fontWeight: 600, color: "var(--text-muted)", marginBottom: 10, textTransform: "uppercase", letterSpacing: "0.03em" }}>{title}</h3>
      {children}
    </section>
  );
}

function MetaRow({ label, value, sub, faded, color }: { label: string; value: string; sub?: string | null; faded?: boolean; color?: string }) {
  return (
    <div style={{ marginBottom: 6 }}>
      <p style={{ fontSize: 10, color: "var(--text-faint)", textTransform: "uppercase", letterSpacing: "0.05em" }}>{label}</p>
      <p style={{ fontSize: 13, fontWeight: 500, color: color ?? (faded ? "var(--text-faint)" : "var(--text)"), fontStyle: faded ? "italic" : "normal" }}>{value}</p>
      {sub && <p style={{ fontSize: 11, color: "var(--text-faint)" }}>{sub}</p>}
    </div>
  );
}

function TimelineRow({ label, date }: { label: string; date: string }) {
  return (
    <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 3 }}>
      <span style={{ fontSize: 11, color: "var(--text-faint)" }}>{label}</span>
      <span style={{ fontSize: 11, color: "var(--text-muted)" }}>{timeAgo(date)}</span>
    </div>
  );
}
