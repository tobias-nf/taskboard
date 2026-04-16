"use client";

import { useEffect, useState, useMemo } from "react";
import Link from "next/link";
import { useTaskboard } from "@/lib/taskboard-context";
import * as api from "@/lib/api";
import type { Task, Tag } from "@/lib/api";

const statusLabel: Record<string, { icon: string; color: string }> = {
  pending:     { icon: "○", color: "#9CA3AF" },
  in_progress: { icon: "◑", color: "#F59E0B" },
  blocked:     { icon: "⊘", color: "#EF4444" },
  review:      { icon: "◉", color: "#8B5CF6" },
  completed:   { icon: "✓", color: "#00EC97" },
  failed:      { icon: "✗", color: "#EF4444" },
  cancelled:   { icon: "—", color: "#9CA3AF" },
};

const priorityWeight: Record<string, number> = {
  emergency: 0, urgent: 1, standard: 2, low: 3,
};

function sortTasks(tasks: Task[]): Task[] {
  return [...tasks].sort((a, b) => {
    const pa = priorityWeight[a.priority] ?? 99;
    const pb = priorityWeight[b.priority] ?? 99;
    if (pa !== pb) return pa - pb;
    if (a.deadline && b.deadline) return new Date(a.deadline).getTime() - new Date(b.deadline).getTime();
    if (a.deadline) return -1;
    if (b.deadline) return 1;
    return new Date(b.created_at).getTime() - new Date(a.created_at).getTime();
  });
}

type TabKey = "assigned" | "owed";

export function AssignedTasksPage() {
  const { currentAgent, getAgentName } = useTaskboard();
  const [assignedTasks, setAssignedTasks] = useState<Task[]>([]);
  const [owedTasks, setOwedTasks] = useState<Task[]>([]);
  const [tags, setTags] = useState<Tag[]>([]);
  const [loading, setLoading] = useState(true);
  const [activeTab, setActiveTab] = useState<TabKey>("assigned");
  const [activeTag, setActiveTag] = useState<string | null>(null);
  const [showCompleted, setShowCompleted] = useState(false);
  const [expandedParents, setExpandedParents] = useState<Set<string>>(new Set());

  useEffect(() => {
    if (!currentAgent) return;
    setLoading(true);
    Promise.all([
      api.getMyTasks({ limit: 200 }),
      api.getTasksOwedToMe({ limit: 200 }),
      api.listTags(),
    ]).then(([assigned, owed, tagRes]) => {
      setAssignedTasks(assigned.tasks || []);
      setOwedTasks(owed.tasks || []);
      setTags(tagRes.tags || []);
      setLoading(false);
    }).catch(() => setLoading(false));
  }, [currentAgent]);

  const allTasks = activeTab === "assigned" ? assignedTasks : owedTasks;

  const { topLevel: filtered, childrenMap } = useMemo(() => {
    let tasks = allTasks;
    if (!showCompleted) {
      tasks = tasks.filter(t => !["completed", "failed", "cancelled"].includes(t.status));
    }

    const cMap = new Map<string, Task[]>();
    for (const t of tasks) {
      if (t.parent_id) {
        const list = cMap.get(t.parent_id) || [];
        list.push(t);
        cMap.set(t.parent_id, list);
      }
    }
    for (const [key, children] of cMap) {
      cMap.set(key, sortTasks(children));
    }

    return { topLevel: sortTasks(tasks.filter(t => !t.parent_id)), childrenMap: cMap };
  }, [allTasks, showCompleted]);

  function toggleParent(id: string) {
    setExpandedParents(prev => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }

  // Re-fetch with tag filter when activeTag changes
  useEffect(() => {
    if (!currentAgent) return;
    const params = activeTag ? { tag: activeTag, limit: 200 } : { limit: 200 };
    if (activeTab === "assigned") {
      api.getMyTasks(params).then(r => setAssignedTasks(r.tasks || []));
    } else {
      api.getTasksOwedToMe(params).then(r => setOwedTasks(r.tasks || []));
    }
  }, [activeTag, activeTab, currentAgent]);

  const assignedCount = assignedTasks.filter(t => !["completed", "failed", "cancelled"].includes(t.status)).length;
  const owedCount = owedTasks.filter(t => !["completed", "failed", "cancelled"].includes(t.status)).length;

  if (loading) {
    return (
      <div className="flex items-center justify-center py-20">
        <p style={{ color: "var(--text-tertiary, #9CA3AF)" }} className="text-sm">Loading tasks...</p>
      </div>
    );
  }

  return (
    <div style={{ maxWidth: 900, margin: "0 auto", padding: "32px 24px" }}>
      {/* Header */}
      <div style={{ marginBottom: 24 }}>
        <h1 style={{ fontSize: 24, fontWeight: 700, letterSpacing: "-0.025em", color: "var(--text, #111827)" }}>
          My Tasks
        </h1>
      </div>

      {/* Tab pills */}
      <div style={{ display: "flex", alignItems: "center", gap: 8, marginBottom: 20 }}>
        <TabPill
          label="Assigned to me"
          count={assignedCount}
          active={activeTab === "assigned"}
          onClick={() => setActiveTab("assigned")}
        />
        <TabPill
          label="Owed to me"
          count={owedCount}
          active={activeTab === "owed"}
          onClick={() => setActiveTab("owed")}
        />
        <div style={{ flex: 1 }} />
        <button
          onClick={() => setShowCompleted(!showCompleted)}
          style={{
            fontSize: 12,
            padding: "6px 12px",
            borderRadius: 9999,
            border: "1px solid var(--border, #E5E7EB)",
            background: showCompleted ? "var(--bg-tertiary, #F0F1F3)" : "transparent",
            color: "var(--text-muted, #6B7280)",
            cursor: "pointer",
          }}
        >
          {showCompleted ? "Hide completed" : "Show completed"}
        </button>
      </div>

      {/* Tag filter pills */}
      {tags.length > 0 && (
        <div style={{ display: "flex", flexWrap: "wrap", gap: 6, marginBottom: 20 }}>
          {tags.map(tag => (
            <button
              key={tag.id}
              onClick={() => setActiveTag(activeTag === tag.name ? null : tag.name)}
              style={{
                display: "flex",
                alignItems: "center",
                gap: 6,
                padding: "4px 10px",
                borderRadius: 9999,
                fontSize: 12,
                fontWeight: 500,
                border: `1px solid ${activeTag === tag.name ? "var(--accent, #00EC97)" : "var(--border, #E5E7EB)"}`,
                background: activeTag === tag.name ? "rgba(0, 236, 151, 0.10)" : "transparent",
                color: activeTag === tag.name ? "var(--accent-text, #059669)" : "var(--text-muted, #9CA3AF)",
                cursor: "pointer",
                transition: "all 0.15s",
              }}
            >
              {tag.name}
            </button>
          ))}
        </div>
      )}

      {/* Task list */}
      <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
        {filtered.length === 0 ? (
          <div style={{ textAlign: "center", padding: "48px 0", color: "var(--text-muted, #9CA3AF)", fontSize: 14 }}>
            {activeTab === "assigned" ? "No tasks assigned to you" : "No tasks owed to you"}
            {activeTag && " with this tag"}
          </div>
        ) : (
          filtered.map(task => {
            const children = childrenMap.get(task.id);
            const isExpanded = expandedParents.has(task.id);
            return (
              <div key={task.id}>
                <TaskRow
                  task={task}
                  getAgentName={getAgentName}
                  subtaskCount={children?.length}
                  expanded={isExpanded}
                  onToggle={children ? () => toggleParent(task.id) : undefined}
                />
                {isExpanded && children && (
                  <div style={{ paddingLeft: 28, marginTop: 2, display: "flex", flexDirection: "column", gap: 4 }}>
                    {children.map(st => (
                      <TaskRow key={st.id} task={st} getAgentName={getAgentName} isSubtask />
                    ))}
                  </div>
                )}
              </div>
            );
          })
        )}
      </div>
    </div>
  );
}

function TabPill({ label, count, active, onClick }: { label: string; count: number; active: boolean; onClick: () => void }) {
  return (
    <button
      onClick={onClick}
      style={{
        display: "flex",
        alignItems: "center",
        gap: 6,
        padding: "6px 14px",
        borderRadius: 9999,
        fontSize: 13,
        fontWeight: active ? 600 : 500,
        border: `1px solid ${active ? "var(--accent, #00EC97)" : "var(--border, #E5E7EB)"}`,
        background: active ? "rgba(0, 236, 151, 0.10)" : "transparent",
        color: active ? "var(--accent-text, #059669)" : "var(--text-muted, #6B7280)",
        cursor: "pointer",
        transition: "all 0.15s",
      }}
    >
      {label}
      <span style={{
        fontSize: 11,
        fontWeight: 600,
        background: active ? "var(--accent, #00EC97)" : "var(--border, #E5E7EB)",
        color: active ? "#111827" : "var(--text-muted, #6B7280)",
        borderRadius: 9999,
        padding: "1px 7px",
        minWidth: 20,
        textAlign: "center",
      }}>
        {count}
      </span>
    </button>
  );
}

function TaskRow({ task, getAgentName, subtaskCount, expanded, onToggle, isSubtask }: {
  task: Task;
  getAgentName: (id: string) => string;
  subtaskCount?: number;
  expanded?: boolean;
  onToggle?: () => void;
  isSubtask?: boolean;
}) {
  const s = statusLabel[task.status] || statusLabel.pending;
  const isOverdue = task.deadline && new Date(task.deadline) < new Date() && !["completed", "cancelled", "failed"].includes(task.status);
  const daysLeft = task.deadline ? Math.ceil((new Date(task.deadline).getTime() - Date.now()) / 86400000) : null;

  return (
    <div style={{ display: "flex", alignItems: "center", gap: 0 }}>
      {onToggle ? (
        <button
          onClick={e => { e.preventDefault(); onToggle(); }}
          style={{
            width: 24, height: 24, display: "flex", alignItems: "center", justifyContent: "center",
            background: "none", border: "none", cursor: "pointer", flexShrink: 0,
            color: "var(--text-faint)", fontSize: 12, borderRadius: 4,
            transition: "transform 0.15s",
            transform: expanded ? "rotate(90deg)" : "rotate(0deg)",
          }}
          title={expanded ? "Collapse subtasks" : "Expand subtasks"}
        >
          &#9656;
        </button>
      ) : (
        <div style={{ width: isSubtask ? 0 : 24, flexShrink: 0 }} />
      )}

      <Link href={`/tasks/${task.id}`} style={{ textDecoration: "none", color: "inherit", flex: 1, minWidth: 0 }}>
        <div
          style={{
            display: "flex", alignItems: "center", gap: 12,
            padding: isSubtask ? "7px 12px" : "10px 14px",
            borderRadius: isSubtask ? 8 : 10,
            border: "1px solid var(--border, #E5E7EB)",
            background: isSubtask ? "var(--bg)" : "var(--surface, #fff)",
            cursor: "pointer", transition: "all 0.15s",
          }}
          className="hover:shadow-sm"
          onMouseEnter={e => { (e.currentTarget as HTMLElement).style.borderColor = "var(--border-hover, #D1D5DB)"; }}
          onMouseLeave={e => { (e.currentTarget as HTMLElement).style.borderColor = "var(--border, #E5E7EB)"; }}
        >
          <span style={{ fontSize: isSubtask ? 14 : 16, color: s.color, flexShrink: 0, width: 20, textAlign: "center" }} title={task.status}>
            {s.icon}
          </span>

          <div style={{ flex: 1, minWidth: 0 }}>
            <div style={{ fontSize: isSubtask ? 13 : 14, fontWeight: 500, lineHeight: 1.4, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
              {task.title}
            </div>
            <div style={{ display: "flex", alignItems: "center", gap: 8, marginTop: 2, fontSize: 11, color: "var(--text-muted, #9CA3AF)" }}>
              <span style={{ fontFamily: "monospace" }}>{task.id}</span>
              {task.assigned_to && <span>{getAgentName(task.assigned_to)}</span>}
              {task.visibility === "private" && (
                <span style={{
                  fontSize: 9, fontWeight: 600, textTransform: "uppercase",
                  background: "#FFF7ED", color: "#C2410C",
                  border: "1px solid rgba(253,186,116,0.25)",
                  padding: "0 5px", borderRadius: 3,
                }}>private</span>
              )}
            </div>
          </div>

          {subtaskCount && subtaskCount > 0 && (
            <span style={{
              fontSize: 10, fontWeight: 600, padding: "2px 7px", borderRadius: 9999, flexShrink: 0,
              background: "var(--bg-tertiary, #F0F1F3)", color: "var(--text-muted, #6B7280)",
            }}>
              {subtaskCount} subtask{subtaskCount > 1 ? "s" : ""}
            </span>
          )}

          <span style={{
            fontSize: 11, fontWeight: 500, padding: "2px 8px", borderRadius: 9999, flexShrink: 0,
            background: task.priority === "emergency" ? "rgba(239,68,68,0.1)" :
                        task.priority === "urgent" ? "rgba(245,158,11,0.1)" :
                        task.priority === "standard" ? "rgba(0,236,151,0.10)" : "var(--bg-tertiary, #F0F1F3)",
            color: task.priority === "emergency" ? "#EF4444" :
                   task.priority === "urgent" ? "#D97706" :
                   task.priority === "standard" ? "#059669" : "var(--text-muted, #9CA3AF)",
          }}>
            {task.priority}
          </span>

          {daysLeft !== null && (
            <span style={{
              fontSize: 11, fontWeight: 500, flexShrink: 0,
              color: isOverdue ? "#EF4444" : daysLeft <= 2 ? "#D97706" : "var(--text-muted, #9CA3AF)",
            }}>
              {isOverdue ? `${Math.abs(daysLeft)}d overdue` : `${daysLeft}d`}
            </span>
          )}
        </div>
      </Link>
    </div>
  );
}
