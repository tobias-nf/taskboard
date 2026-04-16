"use client";

import type { Task } from "@/lib/api";
import { TaskCard } from "./task-card";

interface Column {
  key: string;
  label: string;
  color: string;
}

const columns: Column[] = [
  { key: "pending", label: "Pending", color: "border-[var(--text-muted)]" },
  { key: "in_progress", label: "In Progress", color: "border-indigo-400" },
  { key: "blocked", label: "Blocked", color: "border-red-400" },
  { key: "review", label: "Review", color: "border-amber-400" },
  { key: "completed", label: "Completed", color: "border-green-400" },
];

export function KanbanBoard({
  tasks,
  showCompleted = false,
}: {
  tasks: Task[];
  showCompleted?: boolean;
}) {
  const visibleColumns = showCompleted ? columns : columns.filter(c => c.key !== "completed");

  return (
    <div className="flex gap-4 overflow-x-auto pb-4">
      {visibleColumns.map((col) => {
        const colTasks = tasks.filter(t => t.status === col.key);
        return (
          <div key={col.key} className="flex-shrink-0 w-72">
            <div className={`flex items-center gap-2 mb-3 pb-2 border-b-2 ${col.color}`}>
              <h3 className="text-sm font-medium">{col.label}</h3>
              <span className="text-xs text-[var(--text-muted)] bg-[var(--bg-tertiary)] px-1.5 py-0.5 rounded-full">
                {colTasks.length}
              </span>
            </div>
            <div className="space-y-2">
              {colTasks.length === 0 && (
                <p className="text-xs text-[var(--text-faint)] italic py-4 text-center">No tasks</p>
              )}
              {colTasks.map((task) => (
                <TaskCard key={task.id} task={task} />
              ))}
            </div>
          </div>
        );
      })}
    </div>
  );
}
