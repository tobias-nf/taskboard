"use client";

import { useEffect, useState } from "react";
import { useParams } from "next/navigation";
import { useTaskboard } from "@/lib/taskboard-context";
import * as api from "@/lib/api";
import type { Task } from "@/lib/api";
import { KanbanBoard } from "@/components/kanban-board";
import { agentLabel, agentInitials } from "@/lib/api";
import Link from "next/link";

export default function AgentTasks() {
  const params = useParams();
  const id = params.id as string;
  const { agents } = useTaskboard();
  const agent = agents.find(a => a.id === id);

  const [assignedTasks, setAssignedTasks] = useState<Task[]>([]);
  const [createdTasks, setCreatedTasks] = useState<Task[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    Promise.all([api.getMyTasks(), api.getMyCreatedTasks()]).then(([assigned, created]) => {
      setAssignedTasks((assigned.tasks || []).filter(t => t.assigned_to === id));
      setCreatedTasks((created.tasks || []).filter(t => t.created_by === id && t.assigned_to !== id));
      setLoading(false);
    }).catch(() => setLoading(false));
  }, [id]);

  if (!agent) return <div className="text-[var(--text-muted)]">Agent not found: {id}</div>;
  if (loading) return <div className="text-sm text-[var(--text-muted)]">Loading...</div>;

  const activeTasks = assignedTasks.filter(t => !["completed", "failed", "cancelled"].includes(t.status));
  const overdue = assignedTasks.filter(t => t.deadline && new Date(t.deadline) < new Date() && !["completed", "failed", "cancelled"].includes(t.status));

  return (
    <div>
      <div className="flex items-center gap-3 mb-6">
        <div className="w-9 h-9 rounded-full bg-[var(--accent)] flex items-center justify-center text-sm font-medium text-white">
          {agent.type === "admin" ? "⚙️" : agentInitials(agent)}
        </div>
        <div>
          <div className="flex items-center gap-2">
            <h2 className="text-lg font-semibold">{agentLabel(agent)}</h2>
            <Link href={`/agents/${id}`} className="text-xs text-[var(--accent)] hover:underline">profile</Link>
          </div>
          <p className="text-xs text-[var(--text-muted)]">
            {activeTasks.length} active &middot; {overdue.length} overdue &middot; {assignedTasks.filter(t => t.status === "completed").length} completed
          </p>
        </div>
      </div>

      {assignedTasks.length > 0 ? (
        <div className="mb-8">
          <h3 className="text-sm font-medium text-[var(--text-muted)] mb-3">Assigned tasks</h3>
          <KanbanBoard tasks={assignedTasks} showCompleted={true} />
        </div>
      ) : (
        <p className="text-sm text-[var(--text-faint)] py-8 text-center">No tasks assigned to this agent.</p>
      )}

      {createdTasks.length > 0 && (
        <div>
          <h3 className="text-sm font-medium text-[var(--text-muted)] mb-3">Created by this agent (assigned to others)</h3>
          <KanbanBoard tasks={createdTasks} />
        </div>
      )}
    </div>
  );
}
