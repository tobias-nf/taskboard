"use client";

import Link from "next/link";
import type { Task } from "@/lib/api";
import { useTaskboard } from "@/lib/taskboard-context";

interface ActivityEntry {
  id: string;
  task_id: string;
  task_title: string;
  type: string;
  actor: string;
  summary: string;
  timestamp: string;
}

// Generate activity from tasks
function generateActivity(tasks: Task[]): ActivityEntry[] {
  const entries: ActivityEntry[] = [];
  let id = 1;

  for (const task of tasks) {
    entries.push({
      id: String(id++),
      task_id: task.id,
      task_title: task.title,
      type: "created",
      actor: task.created_by,
      summary: `created task`,
      timestamp: task.created_at,
    });

    if (task.status !== "pending") {
      entries.push({
        id: String(id++),
        task_id: task.id,
        task_title: task.title,
        type: "status_changed",
        actor: task.assigned_to ?? task.created_by,
        summary: `changed status to ${task.status.replace("_", " ")}`,
        timestamp: task.updated_at,
      });
    }

    if (task.status === "completed") {
      entries.push({
        id: String(id++),
        task_id: task.id,
        task_title: task.title,
        type: "completed",
        actor: task.assigned_to ?? task.created_by,
        summary: `completed task`,
        timestamp: task.updated_at,
      });
    }

    if (task.status === "blocked") {
      entries.push({
        id: String(id++),
        task_id: task.id,
        task_title: task.title,
        type: "blocked",
        actor: task.assigned_to ?? task.created_by,
        summary: `marked as blocked`,
        timestamp: task.updated_at,
      });
    }
  }

  return entries.sort((a, b) => b.timestamp.localeCompare(a.timestamp));
}

const typeIcons: Record<string, string> = {
  created: "➕",
  status_changed: "🔄",
  completed: "✅",
  blocked: "🚫",
  commented: "💬",
  escalated: "⚡",
};

const typeColors: Record<string, string> = {
  created: "border-blue-400",
  status_changed: "border-indigo-400",
  completed: "border-green-400",
  blocked: "border-red-400",
  commented: "border-[var(--text-faint)]",
  escalated: "border-amber-400",
};

export function ActivityView({ tasks }: { tasks: Task[] }) {
  const { getAgentName } = useTaskboard();
  const activity = generateActivity(tasks);

  return (
    <div className="bg-[var(--surface)] border border-[var(--border)] rounded-lg shadow-sm p-4">
      <div className="space-y-0">
        {activity.map((entry, i) => (
          <div key={entry.id} className="flex gap-3 py-2.5">
            <div className="flex flex-col items-center">
              <div className={`w-7 h-7 rounded-full border-2 ${typeColors[entry.type] ?? "border-[var(--border)]"} flex items-center justify-center text-xs bg-[var(--bg)]`}>
                {typeIcons[entry.type] ?? "·"}
              </div>
              {i < activity.length - 1 && (
                <div className="w-px flex-1 bg-[var(--border)] mt-1" />
              )}
            </div>
            <div className="flex-1 min-w-0 pb-1">
              <p className="text-sm">
                <span className="font-medium">{getAgentName(entry.actor)}</span>
                <span className="text-[var(--text-muted)]"> {entry.summary}</span>
              </p>
              <Link href={`/tasks/${entry.task_id}`} className="text-xs text-[var(--accent)] hover:underline truncate block">
                {entry.task_id}: {entry.task_title}
              </Link>
              <p className="text-[10px] text-[var(--text-faint)] mt-0.5">
                {new Date(entry.timestamp).toLocaleString()}
              </p>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
