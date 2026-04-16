"use client";

import Link from "next/link";
import type { Task } from "@/lib/api";
import { useTaskboard } from "@/lib/taskboard-context";

const priorityColors: Record<string, string> = {
  emergency: "bg-red-500/20 text-red-600 dark:text-red-400 border-red-500/30",
  urgent: "bg-amber-500/20 text-amber-600 dark:text-amber-400 border-amber-500/30",
  standard: "bg-blue-500/20 text-blue-600 dark:text-blue-400 border-blue-500/30",
  low: "bg-[var(--bg-tertiary)] text-[var(--text-muted)] border-[var(--border)]",
};

const priorityDots: Record<string, string> = {
  emergency: "bg-red-500 dark:bg-red-400",
  urgent: "bg-amber-500 dark:bg-amber-400",
  standard: "bg-blue-500 dark:bg-blue-400",
  low: "bg-[var(--text-faint)]",
};

export function TaskCard({ task }: { task: Task }) {
  const { getAgentName } = useTaskboard();
  const isOverdue = task.deadline && new Date(task.deadline) < new Date() && !["completed", "cancelled", "failed"].includes(task.status);
  const daysLeft = task.deadline ? Math.ceil((new Date(task.deadline).getTime() - Date.now()) / 86400000) : null;

  return (
    <Link href={`/tasks/${task.id}`} className="block">
      <div className="bg-[var(--surface)] border border-[var(--border)] rounded-lg p-3 hover:border-[var(--accent)]/30 transition-colors cursor-pointer shadow-sm">
        <div className="flex items-start justify-between gap-2 mb-2">
          <span className={`inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-medium border ${priorityColors[task.priority]}`}>
            <span className={`w-1.5 h-1.5 rounded-full ${priorityDots[task.priority]}`} />
            {task.priority}
          </span>
        </div>

        <h3 className="text-sm font-medium mb-1.5 leading-snug">{task.title}</h3>

        <div className="flex items-center gap-3 text-[10px] text-[var(--text-muted)]">
          {task.assigned_to && (
            <span>{getAgentName(task.assigned_to)}</span>
          )}
          {task.visibility === "private" && (
            <span className="bg-[var(--bg-tertiary)] px-1.5 py-0.5 rounded" title="Private task">
              private
            </span>
          )}
        </div>

        <div className="flex items-center justify-between mt-2 text-[10px] text-[var(--text-faint)]">
          <span className="font-mono">{task.id}</span>
          <div className="flex gap-1">
            {task.description && <span title="Has description">📋</span>}
          </div>
        </div>

        {daysLeft !== null && (
          <div className={`mt-2 text-[10px] font-medium ${isOverdue ? "text-red-500 dark:text-red-400" : daysLeft <= 2 ? "text-amber-500 dark:text-amber-400" : "text-[var(--text-muted)]"}`}>
            {isOverdue ? `⚠ Overdue by ${Math.abs(daysLeft)}d` : `${daysLeft}d remaining`}
          </div>
        )}
      </div>
    </Link>
  );
}
