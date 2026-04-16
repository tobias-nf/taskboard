"use client";

import Link from "next/link";
import type { Task, Agent } from "@/lib/api";
import { useTaskboard } from "@/lib/taskboard-context";

interface AgentWorkload {
  agent: Agent;
  pending: number;
  in_progress: number;
  blocked: number;
  review: number;
  completed: number;
  overdue: number;
  total_active: number;
}

function computeWorkload(taskList: Task[], agents: Agent[]): AgentWorkload[] {
  const agentIds = new Set(taskList.map(t => t.assigned_to).filter(Boolean) as string[]);

  return Array.from(agentIds).map(id => {
    const agent = agents.find(a => a.id === id) ?? { id, name: id, type: "user" as const, description: "", active: true, created_at: "", updated_at: "" };
    const myTasks = taskList.filter(t => t.assigned_to === id);
    const active = myTasks.filter(t => !["completed", "failed", "cancelled"].includes(t.status));

    return {
      agent,
      pending: myTasks.filter(t => t.status === "pending").length,
      in_progress: myTasks.filter(t => t.status === "in_progress").length,
      blocked: myTasks.filter(t => t.status === "blocked").length,
      review: myTasks.filter(t => t.status === "review").length,
      completed: myTasks.filter(t => t.status === "completed").length,
      overdue: myTasks.filter(t => t.deadline && new Date(t.deadline) < new Date() && !["completed", "cancelled", "failed"].includes(t.status)).length,
      total_active: active.length,
    };
  }).sort((a, b) => b.total_active - a.total_active);
}

const barColors: Record<string, string> = {
  pending: "bg-[var(--text-muted)]",
  in_progress: "bg-indigo-500",
  blocked: "bg-red-500",
  review: "bg-amber-500",
};

export function WorkloadView({ tasks }: { tasks: Task[] }) {
  const { agents } = useTaskboard();
  const workload = computeWorkload(tasks, agents);
  const maxActive = Math.max(...workload.map(w => w.total_active), 1);

  return (
    <div className="bg-[var(--surface)] border border-[var(--border)] rounded-lg shadow-sm p-4">
      <div className="space-y-4">
        {workload.map(w => (
          <div key={w.agent.id}>
            <div className="flex items-center justify-between mb-1.5">
              <Link href={`/agents/${w.agent.id}/tasks`} className="flex items-center gap-2 hover:text-[var(--accent)] transition-colors">
                <span className="text-xs">{w.agent.type === "admin" ? "⚙️" : "👤"}</span>
                <span className="text-sm font-medium">{w.agent.email ?? w.agent.id}</span>
              </Link>
              <div className="flex items-center gap-3 text-[10px]">
                <span className="text-[var(--text-muted)]">{w.total_active} active</span>
                {w.overdue > 0 && <span className="text-red-500 dark:text-red-400 font-medium">{w.overdue} overdue</span>}
                <span className="text-[var(--text-faint)]">{w.completed} done</span>
              </div>
            </div>

            {/* Stacked bar */}
            <div className="flex h-5 rounded-md overflow-hidden bg-[var(--bg-tertiary)]">
              {(["pending", "in_progress", "blocked", "review"] as const).map(status => {
                const count = w[status];
                if (count === 0) return null;
                const width = (count / maxActive) * 100;
                return (
                  <div
                    key={status}
                    className={`${barColors[status]} flex items-center justify-center`}
                    style={{ width: `${Math.max(width, 3)}%` }}
                    title={`${status.replace("_", " ")}: ${count}`}
                  >
                    {count > 0 && <span className="text-[9px] text-white font-medium">{count}</span>}
                  </div>
                );
              })}
            </div>

            {/* Legend row */}
            <div className="flex gap-3 mt-1">
              {w.pending > 0 && <span className="text-[9px] text-[var(--text-faint)]">⬤ {w.pending} pending</span>}
              {w.in_progress > 0 && <span className="text-[9px] text-indigo-500">⬤ {w.in_progress} in progress</span>}
              {w.blocked > 0 && <span className="text-[9px] text-red-500">⬤ {w.blocked} blocked</span>}
              {w.review > 0 && <span className="text-[9px] text-amber-500">⬤ {w.review} review</span>}
            </div>
          </div>
        ))}

        {workload.length === 0 && (
          <p className="text-sm text-[var(--text-faint)] text-center py-4">No tasks with assignees</p>
        )}
      </div>
    </div>
  );
}
