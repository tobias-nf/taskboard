"use client";

import { useEffect, useState, useMemo, useCallback } from "react";
import Link from "next/link";
import { useTaskboard } from "@/lib/taskboard-context";
import * as api from "@/lib/api";
import type { Task, Tag, Agent } from "@/lib/api";
import { KanbanBoard } from "./kanban-board";
import { CalendarView } from "./calendar-view";
import { WorkloadView } from "./workload-view";
import { ActivityView } from "./activity-view";

// ── Constants ──────────────────────────────────────────

const statusLabel: Record<string, { icon: string; color: string }> = {
  pending:     { icon: "○", color: "#9CA3AF" },
  in_progress: { icon: "◑", color: "#F59E0B" },
  blocked:     { icon: "⊘", color: "#EF4444" },
  review:      { icon: "◉", color: "#8B5CF6" },
  completed:   { icon: "✓", color: "#00EC97" },
  failed:      { icon: "✗", color: "#EF4444" },
  cancelled:   { icon: "—", color: "#9CA3AF" },
};

const priorityLabel: Record<string, { color: string }> = {
  emergency: { color: "#EF4444" },
  urgent:    { color: "#D97706" },
  standard:  { color: "#059669" },
  low:       { color: "#9CA3AF" },
};

const priorityWeight: Record<string, number> = { emergency: 0, urgent: 1, standard: 2, low: 3 };

const activeStatuses = ["pending", "in_progress", "blocked", "review"];
const doneStatuses = ["completed", "failed", "cancelled"];
const allStatuses = [...activeStatuses, ...doneStatuses];
const allPriorities = ["emergency", "urgent", "standard", "low"];

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

// ── Types ──────────────────────────────────────────────

type ViewMode = "list" | "board" | "calendar" | "workload" | "activity";

type FetchFn = (params?: api.TaskListParams) => Promise<{ tasks: Task[]; total: number }>;

// ── Main Component ─────────────────────────────────────

export function TaskListPage({ title, emptyMessage, fetchTasks }: {
  title: string;
  emptyMessage: string;
  fetchTasks: FetchFn;
}) {
  const { currentAgent, agents, getAgentName } = useTaskboard();
  const [tasks, setTasks] = useState<Task[]>([]);
  const [tags, setTags] = useState<Tag[]>([]);
  const [loading, setLoading] = useState(true);
  const [viewMode, setViewMode] = useState<ViewMode>("list");
  const [expandedParents, setExpandedParents] = useState<Set<string>>(new Set());
  const [filtersVisible, setFiltersVisible] = useState(false);

  // Filter state — sets allow multi-select
  const [selectedStatuses, setSelectedStatuses] = useState<Set<string>>(new Set());
  const [selectedPriorities, setSelectedPriorities] = useState<Set<string>>(new Set());
  const [selectedTag, setSelectedTag] = useState<string | null>(null);
  const [selectedAssignee, setSelectedAssignee] = useState<string | null>(null);
  const [assigneeMenuOpen, setAssigneeMenuOpen] = useState(false);
  const [tagMenuOpen, setTagMenuOpen] = useState(false);

  // Initial load
  useEffect(() => {
    if (!currentAgent) return;
    setLoading(true);
    Promise.all([fetchTasks({ limit: 200 }), api.listTags()])
      .then(([res, tagRes]) => {
        setTasks(res.tasks || []);
        setTags(tagRes.tags || []);
        setLoading(false);
      })
      .catch(() => setLoading(false));
  }, [currentAgent, fetchTasks]);

  // Re-fetch when tag filter changes (tag is the only server-side filter)
  useEffect(() => {
    if (!currentAgent) return;
    const params: api.TaskListParams = { limit: 200 };
    if (selectedTag) params.tag = selectedTag;
    fetchTasks(params).then(r => setTasks(r.tasks || []));
  }, [selectedTag, currentAgent, fetchTasks]);

  // Counts per status/priority (computed from all tasks, before filtering)
  const statusCounts = useMemo(() => {
    const counts: Record<string, number> = {};
    for (const t of tasks) {
      if (!t.parent_id) counts[t.status] = (counts[t.status] || 0) + 1;
    }
    return counts;
  }, [tasks]);

  const priorityCounts = useMemo(() => {
    const counts: Record<string, number> = {};
    for (const t of tasks) {
      if (!t.parent_id) counts[t.priority] = (counts[t.priority] || 0) + 1;
    }
    return counts;
  }, [tasks]);

  // Client-side filtering
  const { topLevel: filtered, childrenMap } = useMemo(() => {
    let result = tasks;

    // Status filter: if nothing selected, show active statuses only
    if (selectedStatuses.size > 0) {
      result = result.filter(t => selectedStatuses.has(t.status));
    } else {
      result = result.filter(t => !doneStatuses.includes(t.status));
    }

    if (selectedPriorities.size > 0) {
      result = result.filter(t => selectedPriorities.has(t.priority));
    }

    if (selectedAssignee) {
      result = result.filter(t => t.assigned_to === selectedAssignee);
    }

    // Build parent → children map and separate top-level from subtasks
    const cMap = new Map<string, Task[]>();
    for (const t of result) {
      if (t.parent_id) {
        const list = cMap.get(t.parent_id) || [];
        list.push(t);
        cMap.set(t.parent_id, list);
      }
    }
    for (const [key, children] of cMap) {
      cMap.set(key, sortTasks(children));
    }

    return { topLevel: sortTasks(result.filter(t => !t.parent_id)), childrenMap: cMap };
  }, [tasks, selectedStatuses, selectedPriorities, selectedAssignee]);

  function toggleStatus(s: string) {
    setSelectedStatuses(prev => {
      const next = new Set(prev);
      if (next.has(s)) next.delete(s);
      else next.add(s);
      return next;
    });
  }

  function togglePriority(p: string) {
    setSelectedPriorities(prev => {
      const next = new Set(prev);
      if (next.has(p)) next.delete(p);
      else next.add(p);
      return next;
    });
  }

  const toggleParent = useCallback((id: string) => {
    setExpandedParents(prev => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }, []);

  const hasAnyFilter = selectedStatuses.size > 0 || selectedPriorities.size > 0 || selectedTag !== null || selectedAssignee !== null;

  function clearAllFilters() {
    setSelectedStatuses(new Set());
    setSelectedPriorities(new Set());
    setSelectedTag(null);
    setSelectedAssignee(null);
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center py-20">
        <p style={{ color: "var(--text-faint)" }} className="text-sm">Loading...</p>
      </div>
    );
  }

  return (
    <div style={{ maxWidth: viewMode === "board" || viewMode === "workload" ? 1200 : 900, margin: "0 auto", padding: "24px 24px" }}>
      {/* Toolbar row */}
      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: 12 }}>
        <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
          <button
            onClick={() => setFiltersVisible(v => !v)}
            style={{
              display: "flex", alignItems: "center", gap: 4,
              fontSize: 12, fontWeight: 500, padding: "5px 10px", borderRadius: 9999,
              border: `1px solid ${filtersVisible || hasAnyFilter ? "var(--accent)" : "var(--border)"}`,
              background: filtersVisible || hasAnyFilter ? "rgba(0,236,151,0.10)" : "transparent",
              color: filtersVisible || hasAnyFilter ? "var(--accent-text, #059669)" : "var(--text-muted)",
              cursor: "pointer", transition: "all 0.15s",
            }}
          >
            <svg width="14" height="14" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" d="M12 3c2.755 0 5.455.232 8.083.678.533.09.917.556.917 1.096v1.044a2.25 2.25 0 0 1-.659 1.591l-5.432 5.432a2.25 2.25 0 0 0-.659 1.591v2.927a2.25 2.25 0 0 1-1.244 2.013L9.75 21v-6.568a2.25 2.25 0 0 0-.659-1.591L3.659 7.409A2.25 2.25 0 0 1 3 5.818V4.774c0-.54.384-1.006.917-1.096A48.32 48.32 0 0 1 12 3Z" />
            </svg>
            Filter{hasAnyFilter ? "s on" : ""}
          </button>
          {hasAnyFilter && (
            <button
              onClick={clearAllFilters}
              style={{
                fontSize: 11, padding: "4px 10px", borderRadius: 9999,
                border: "1px solid var(--border)", background: "transparent",
                color: "var(--text-faint)", cursor: "pointer",
              }}
            >
              Clear
            </button>
          )}
          <span style={{ fontSize: 12, color: "var(--text-faint)" }}>
            {filtered.length} task{filtered.length !== 1 ? "s" : ""}
          </span>
        </div>
        <div style={{ display: "flex", gap: 2, background: "var(--bg-secondary)", borderRadius: 8, padding: 2 }}>
          <ViewButton icon="list" active={viewMode === "list"} onClick={() => setViewMode("list")} title="List" />
          <ViewButton icon="board" active={viewMode === "board"} onClick={() => setViewMode("board")} title="Board" />
          <ViewButton icon="calendar" active={viewMode === "calendar"} onClick={() => setViewMode("calendar")} title="Calendar" />
          <ViewButton icon="workload" active={viewMode === "workload"} onClick={() => setViewMode("workload")} title="Workload" />
          <ViewButton icon="activity" active={viewMode === "activity"} onClick={() => setViewMode("activity")} title="Activity" />
        </div>
      </div>

      {/* Inline filter pills */}
      {filtersVisible && <div style={{ display: "flex", flexDirection: "column", gap: 8, marginBottom: 16 }}>
        {/* Status pills */}
        <div style={{ display: "flex", alignItems: "center", gap: 6, flexWrap: "wrap" }}>
          <span style={{ fontSize: 10, color: "var(--text-faint)", textTransform: "uppercase", letterSpacing: "0.05em", width: 52, flexShrink: 0 }}>Status</span>
          {allStatuses.map(s => {
            const sl = statusLabel[s] || statusLabel.pending;
            const count = statusCounts[s] || 0;
            const isActive = selectedStatuses.has(s);
            const isDefaultHidden = selectedStatuses.size === 0 && doneStatuses.includes(s);
            return (
              <button
                key={s}
                onClick={() => toggleStatus(s)}
                style={{
                  display: "flex", alignItems: "center", gap: 5,
                  padding: "3px 10px", borderRadius: 9999, fontSize: 12, fontWeight: 500,
                  border: `1px solid ${isActive ? sl.color : "var(--border)"}`,
                  background: isActive ? `${sl.color}15` : "transparent",
                  color: isActive ? sl.color : isDefaultHidden ? "var(--text-faint)" : "var(--text-muted)",
                  cursor: "pointer", transition: "all 0.15s",
                  opacity: count === 0 && !isActive ? 0.5 : 1,
                }}
              >
                <span style={{ fontSize: 11 }}>{sl.icon}</span>
                {s.replace("_", " ")}
                <span style={{
                  fontSize: 10, fontWeight: 600, minWidth: 16, textAlign: "center",
                  padding: "0 4px", borderRadius: 9999,
                  background: isActive ? `${sl.color}25` : "var(--bg-tertiary)",
                  color: isActive ? sl.color : "var(--text-faint)",
                }}>{count}</span>
              </button>
            );
          })}
        </div>

        {/* Priority pills */}
        <div style={{ display: "flex", alignItems: "center", gap: 6, flexWrap: "wrap" }}>
          <span style={{ fontSize: 10, color: "var(--text-faint)", textTransform: "uppercase", letterSpacing: "0.05em", width: 52, flexShrink: 0 }}>Priority</span>
          {allPriorities.map(p => {
            const pl = priorityLabel[p];
            const count = priorityCounts[p] || 0;
            const isActive = selectedPriorities.has(p);
            return (
              <button
                key={p}
                onClick={() => togglePriority(p)}
                style={{
                  display: "flex", alignItems: "center", gap: 5,
                  padding: "3px 10px", borderRadius: 9999, fontSize: 12, fontWeight: 500,
                  border: `1px solid ${isActive ? pl.color : "var(--border)"}`,
                  background: isActive ? `${pl.color}15` : "transparent",
                  color: isActive ? pl.color : "var(--text-muted)",
                  cursor: "pointer", transition: "all 0.15s",
                  opacity: count === 0 && !isActive ? 0.5 : 1,
                }}
              >
                {p}
                <span style={{
                  fontSize: 10, fontWeight: 600, minWidth: 16, textAlign: "center",
                  padding: "0 4px", borderRadius: 9999,
                  background: isActive ? `${pl.color}25` : "var(--bg-tertiary)",
                  color: isActive ? pl.color : "var(--text-faint)",
                }}>{count}</span>
              </button>
            );
          })}
        </div>

        {/* Tag + Assignee row */}
        <div style={{ display: "flex", alignItems: "center", gap: 6, flexWrap: "wrap" }}>
          <span style={{ fontSize: 10, color: "var(--text-faint)", textTransform: "uppercase", letterSpacing: "0.05em", width: 52, flexShrink: 0 }}>More</span>

          {/* Tag dropdown */}
          <div style={{ position: "relative" }}>
            <button
              onClick={() => { setTagMenuOpen(!tagMenuOpen); setAssigneeMenuOpen(false); }}
              style={{
                display: "flex", alignItems: "center", gap: 4,
                padding: "3px 10px", borderRadius: 9999, fontSize: 12, fontWeight: 500,
                border: `1px solid ${selectedTag ? "var(--accent)" : "var(--border)"}`,
                background: selectedTag ? "rgba(0,236,151,0.10)" : "transparent",
                color: selectedTag ? "var(--accent-text, #059669)" : "var(--text-muted)",
                cursor: "pointer", transition: "all 0.15s",
              }}
            >
              {selectedTag ? `tag: ${selectedTag}` : "Tag"}
              {selectedTag && (
                <span
                  onClick={e => { e.stopPropagation(); setSelectedTag(null); }}
                  style={{ fontSize: 13, lineHeight: 1, cursor: "pointer", marginLeft: 2 }}
                >
                  ×
                </span>
              )}
            </button>
            {tagMenuOpen && (
              <FilterDropdown onClose={() => setTagMenuOpen(false)}>
                {tags.map(t => (
                  <DropdownItem
                    key={t.id}
                    active={selectedTag === t.name}
                    onClick={() => { setSelectedTag(selectedTag === t.name ? null : t.name); setTagMenuOpen(false); }}
                  >
                    {t.name}
                  </DropdownItem>
                ))}
                {tags.length === 0 && <DropdownItem disabled>No tags</DropdownItem>}
              </FilterDropdown>
            )}
          </div>

          {/* Assignee dropdown */}
          <div style={{ position: "relative" }}>
            <button
              onClick={() => { setAssigneeMenuOpen(!assigneeMenuOpen); setTagMenuOpen(false); }}
              style={{
                display: "flex", alignItems: "center", gap: 4,
                padding: "3px 10px", borderRadius: 9999, fontSize: 12, fontWeight: 500,
                border: `1px solid ${selectedAssignee ? "var(--accent)" : "var(--border)"}`,
                background: selectedAssignee ? "rgba(0,236,151,0.10)" : "transparent",
                color: selectedAssignee ? "var(--accent-text, #059669)" : "var(--text-muted)",
                cursor: "pointer", transition: "all 0.15s",
              }}
            >
              {selectedAssignee ? `assignee: ${getAgentName(selectedAssignee)}` : "Assignee"}
              {selectedAssignee && (
                <span
                  onClick={e => { e.stopPropagation(); setSelectedAssignee(null); }}
                  style={{ fontSize: 13, lineHeight: 1, cursor: "pointer", marginLeft: 2 }}
                >
                  ×
                </span>
              )}
            </button>
            {assigneeMenuOpen && (
              <FilterDropdown onClose={() => setAssigneeMenuOpen(false)}>
                {agents.filter(a => a.active).map(a => (
                  <DropdownItem
                    key={a.id}
                    active={selectedAssignee === a.id}
                    onClick={() => { setSelectedAssignee(selectedAssignee === a.id ? null : a.id); setAssigneeMenuOpen(false); }}
                  >
                    {a.email ?? a.id}
                  </DropdownItem>
                ))}
              </FilterDropdown>
            )}
          </div>
        </div>
      </div>}

      {/* Content */}
      {filtered.length === 0 ? (
        <div style={{ textAlign: "center", padding: "48px 0", color: "var(--text-faint)", fontSize: 14 }}>
          {emptyMessage}
          {hasAnyFilter && " matching these filters"}
        </div>
      ) : viewMode === "list" ? (
        <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
          {filtered.map(task => {
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
          })}
        </div>
      ) : viewMode === "board" ? (
        <KanbanBoard tasks={filtered} showCompleted={selectedStatuses.size > 0 && [...selectedStatuses].some(s => doneStatuses.includes(s))} />
      ) : viewMode === "calendar" ? (
        <CalendarView tasks={filtered} />
      ) : viewMode === "workload" ? (
        <WorkloadView tasks={filtered} />
      ) : (
        <ActivityView tasks={filtered} />
      )}
    </div>
  );
}

// ── Sub-components ─────────────────────────────────────

function ViewButton({ icon, active, onClick, title }: { icon: string; active: boolean; onClick: () => void; title: string }) {
  const paths: Record<string, React.ReactNode> = {
    list: <path strokeLinecap="round" strokeLinejoin="round" d="M3.75 6.75h16.5M3.75 12h16.5m-16.5 5.25h16.5" />,
    board: <path strokeLinecap="round" strokeLinejoin="round" d="M3.75 6A2.25 2.25 0 0 1 6 3.75h2.25A2.25 2.25 0 0 1 10.5 6v2.25a2.25 2.25 0 0 1-2.25 2.25H6a2.25 2.25 0 0 1-2.25-2.25V6ZM3.75 15.75A2.25 2.25 0 0 1 6 13.5h2.25a2.25 2.25 0 0 1 2.25 2.25V18a2.25 2.25 0 0 1-2.25 2.25H6A2.25 2.25 0 0 1 3.75 18v-2.25ZM13.5 6a2.25 2.25 0 0 1 2.25-2.25H18A2.25 2.25 0 0 1 20.25 6v2.25A2.25 2.25 0 0 1 18 10.5h-2.25a2.25 2.25 0 0 1-2.25-2.25V6ZM13.5 15.75a2.25 2.25 0 0 1 2.25-2.25H18a2.25 2.25 0 0 1 2.25 2.25V18A2.25 2.25 0 0 1 18 20.25h-2.25A2.25 2.25 0 0 1 13.5 18v-2.25Z" />,
    calendar: <path strokeLinecap="round" strokeLinejoin="round" d="M6.75 3v2.25M17.25 3v2.25M3 18.75V7.5a2.25 2.25 0 0 1 2.25-2.25h13.5A2.25 2.25 0 0 1 21 7.5v11.25m-18 0A2.25 2.25 0 0 0 5.25 21h13.5A2.25 2.25 0 0 0 21 18.75m-18 0v-7.5A2.25 2.25 0 0 1 5.25 9h13.5A2.25 2.25 0 0 1 21 11.25v7.5" />,
    workload: <path strokeLinecap="round" strokeLinejoin="round" d="M3 13.125C3 12.504 3.504 12 4.125 12h2.25c.621 0 1.125.504 1.125 1.125v6.75C7.5 20.496 6.996 21 6.375 21h-2.25A1.125 1.125 0 0 1 3 19.875v-6.75ZM9.75 8.625c0-.621.504-1.125 1.125-1.125h2.25c.621 0 1.125.504 1.125 1.125v11.25c0 .621-.504 1.125-1.125 1.125h-2.25a1.125 1.125 0 0 1-1.125-1.125V8.625ZM16.5 4.125c0-.621.504-1.125 1.125-1.125h2.25C20.496 3 21 3.504 21 4.125v15.75c0 .621-.504 1.125-1.125 1.125h-2.25a1.125 1.125 0 0 1-1.125-1.125V4.125Z" />,
    activity: <path strokeLinecap="round" strokeLinejoin="round" d="M12 6v6h4.5m4.5 0a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z" />,
  };
  return (
    <button
      onClick={onClick}
      title={title}
      style={{
        padding: 5, borderRadius: 6, border: "none", cursor: "pointer",
        background: active ? "var(--surface)" : "transparent",
        boxShadow: active ? "0 1px 2px rgba(0,0,0,0.06)" : "none",
        color: active ? "var(--text)" : "var(--text-faint)",
        display: "flex", alignItems: "center", justifyContent: "center",
      }}
    >
      <svg width="16" height="16" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor">
        {paths[icon]}
      </svg>
    </button>
  );
}

function FilterDropdown({ children, onClose }: { children: React.ReactNode; onClose: () => void }) {
  return (
    <>
      <div style={{ position: "fixed", inset: 0, zIndex: 40 }} onClick={onClose} />
      <div style={{
        position: "absolute", left: 0, top: "calc(100% + 4px)", zIndex: 50,
        minWidth: 160, borderRadius: 10, padding: 4,
        border: "1px solid var(--border)", background: "var(--surface)",
        boxShadow: "0 4px 12px rgba(0,0,0,0.08)",
      }}>
        {children}
      </div>
    </>
  );
}

function DropdownItem({ children, onClick, disabled, active }: { children: React.ReactNode; onClick?: () => void; disabled?: boolean; active?: boolean }) {
  return (
    <button
      onClick={disabled ? undefined : onClick}
      disabled={disabled}
      style={{
        display: "block", width: "100%", textAlign: "left",
        padding: "6px 10px", borderRadius: 6, border: "none",
        background: active ? "var(--bg-secondary)" : "transparent",
        fontSize: 12, cursor: disabled ? "default" : "pointer",
        color: active ? "var(--text)" : disabled ? "var(--text-faint)" : "var(--text-muted)",
        fontWeight: active ? 600 : 400,
      }}
      onMouseEnter={e => { if (!disabled) (e.target as HTMLElement).style.background = "var(--bg-secondary)"; }}
      onMouseLeave={e => { if (!active) (e.target as HTMLElement).style.background = "transparent"; }}
    >
      {children}
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
      {/* Expand/collapse toggle for parents with subtasks */}
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
            border: `1px solid ${isSubtask ? "var(--border)" : "var(--border)"}`,
            background: isSubtask ? "var(--bg)" : "var(--surface)",
            cursor: "pointer", transition: "all 0.15s",
            fontSize: isSubtask ? 13 : undefined,
          }}
          className="hover:shadow-sm"
          onMouseEnter={e => { (e.currentTarget as HTMLElement).style.borderColor = "var(--border-hover, #D1D5DB)"; }}
          onMouseLeave={e => { (e.currentTarget as HTMLElement).style.borderColor = "var(--border)"; }}
        >
          <span style={{ fontSize: isSubtask ? 14 : 16, color: s.color, flexShrink: 0, width: 20, textAlign: "center" }} title={task.status}>
            {s.icon}
          </span>

          <div style={{ flex: 1, minWidth: 0 }}>
            <div style={{ fontSize: isSubtask ? 13 : 14, fontWeight: 500, lineHeight: 1.4, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
              {task.title}
            </div>
            <div style={{ display: "flex", alignItems: "center", gap: 8, marginTop: 2, fontSize: 11, color: "var(--text-faint)" }}>
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
              background: "var(--bg-tertiary)", color: "var(--text-muted)",
            }}>
              {subtaskCount} subtask{subtaskCount > 1 ? "s" : ""}
            </span>
          )}

          <span style={{
            fontSize: 11, fontWeight: 500, padding: "2px 8px", borderRadius: 9999, flexShrink: 0,
            background: task.priority === "emergency" ? "rgba(239,68,68,0.1)" :
                        task.priority === "urgent" ? "rgba(245,158,11,0.1)" :
                        task.priority === "standard" ? "rgba(0,236,151,0.10)" : "var(--bg-tertiary)",
            color: task.priority === "emergency" ? "#EF4444" :
                   task.priority === "urgent" ? "#D97706" :
                   task.priority === "standard" ? "#059669" : "var(--text-faint)",
          }}>
            {task.priority}
          </span>

          {daysLeft !== null && (
            <span style={{
              fontSize: 11, fontWeight: 500, flexShrink: 0,
              color: isOverdue ? "#EF4444" : daysLeft <= 2 ? "#D97706" : "var(--text-faint)",
            }}>
              {isOverdue ? `${Math.abs(daysLeft)}d overdue` : `${daysLeft}d`}
            </span>
          )}
        </div>
      </Link>
    </div>
  );
}
