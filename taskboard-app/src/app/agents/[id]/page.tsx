"use client";

import { useEffect, useState } from "react";
import { useParams } from "next/navigation";
import Link from "next/link";
import { useTaskboard } from "@/lib/taskboard-context";
import * as api from "@/lib/api";
import { agentLabel, agentInitials } from "@/lib/api";
import type { Task } from "@/lib/api";

export default function AgentDetail() {
  const params = useParams();
  const id = params.id as string;
  const { agents, currentAgent, getAgentName } = useTaskboard();
  const agent = agents.find(a => a.id === id);

  const [assignedTasks, setAssignedTasks] = useState<Task[]>([]);
  const [createdTasks, setCreatedTasks] = useState<Task[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (!agent) return;
    Promise.all([
      api.getMyTasks(),
      api.getMyCreatedTasks(),
    ]).then(([assigned, created]) => {
      setAssignedTasks((assigned.tasks || []).filter(t => t.assigned_to === id));
      setCreatedTasks((created.tasks || []).filter(t => t.created_by === id));
      setLoading(false);
    }).catch(() => setLoading(false));
  }, [id, agent]);

  if (!agent) return <div className="text-[var(--text-muted)]">Agent not found: {id}</div>;
  if (loading) return <div className="text-sm text-[var(--text-muted)]">Loading...</div>;

  return (
    <div className="max-w-4xl mx-auto">
      <Link href={`/agents/${id}/tasks`} className="text-sm text-[var(--accent)] hover:underline mb-4 inline-block">&larr; Tasks</Link>

      {/* Header */}
      <div className="flex items-start gap-4 mb-6">
        <div className="w-12 h-12 rounded-full bg-[var(--accent)] flex items-center justify-center text-lg font-medium text-white">
          {agent.type === "admin" ? "⚙️" : agentInitials(agent)}
        </div>
        <div className="flex-1">
          <h2 className="text-xl font-semibold">{agentLabel(agent)}</h2>
          <div className="flex items-center gap-2 mt-1.5">
            <span className={`text-xs px-2 py-0.5 rounded ${
              agent.type === "admin" ? "bg-amber-500/20 text-amber-600 dark:text-amber-400" :
              "bg-blue-500/20 text-blue-600 dark:text-blue-400"
            }`}>{agent.type}</span>
            {agent.active ? (
              <span className="text-xs text-green-600 dark:text-green-400">Active</span>
            ) : (
              <span className="text-xs text-amber-600 dark:text-amber-400">Pending Approval</span>
            )}
          </div>
        </div>
      </div>

      <div className="grid grid-cols-2 gap-6">
        {/* Profile */}
        <section className="bg-[var(--surface)] border border-[var(--border)] rounded-lg p-4 space-y-3 shadow-sm">
          <h3 className="text-sm font-medium text-[var(--text-muted)]">Profile</h3>
          {agent.email && <div><p className="text-[10px] text-[var(--text-faint)] uppercase">Email</p><p className="text-sm">{agent.email}</p></div>}
          {agent.slack_id && <div><p className="text-[10px] text-[var(--text-faint)] uppercase">Slack</p><p className="text-sm font-mono">{agent.slack_id}</p></div>}
          <div><p className="text-[10px] text-[var(--text-faint)] uppercase">Agent ID</p><p className="text-xs font-mono text-[var(--text-muted)]">{agent.id}</p></div>
          {agent.last_seen_at && <div><p className="text-[10px] text-[var(--text-faint)] uppercase">Last Seen</p><p className="text-xs text-[var(--text-muted)]">{new Date(agent.last_seen_at).toLocaleString()}</p></div>}
        </section>

        {/* Task Summary */}
        <section className="bg-[var(--surface)] border border-[var(--border)] rounded-lg p-4 shadow-sm">
          <h3 className="text-sm font-medium text-[var(--text-muted)] mb-3">Tasks</h3>
          <div className="grid grid-cols-2 gap-3 mb-4">
            <div className="bg-[var(--bg)] rounded p-3 text-center border border-[var(--border)]">
              <p className="text-2xl font-semibold">{assignedTasks.filter(t => !["completed", "failed", "cancelled"].includes(t.status)).length}</p>
              <p className="text-[10px] text-[var(--text-muted)]">Active (assigned)</p>
            </div>
            <div className="bg-[var(--bg)] rounded p-3 text-center border border-[var(--border)]">
              <p className="text-2xl font-semibold">{createdTasks.filter(t => !["completed", "failed", "cancelled"].includes(t.status)).length}</p>
              <p className="text-[10px] text-[var(--text-muted)]">Active (created)</p>
            </div>
            <div className="bg-[var(--bg)] rounded p-3 text-center border border-[var(--border)]">
              <p className="text-2xl font-semibold">{assignedTasks.filter(t => t.status === "completed").length}</p>
              <p className="text-[10px] text-[var(--text-muted)]">Completed</p>
            </div>
            <div className="bg-[var(--bg)] rounded p-3 text-center border border-[var(--border)]">
              <p className="text-2xl font-semibold text-red-500 dark:text-red-400">{assignedTasks.filter(t => t.deadline && new Date(t.deadline) < new Date() && !["completed","cancelled","failed"].includes(t.status)).length}</p>
              <p className="text-[10px] text-[var(--text-muted)]">Overdue</p>
            </div>
          </div>

          <h4 className="text-xs text-[var(--text-faint)] uppercase mb-2">Recent tasks</h4>
          <div className="space-y-1.5">
            {assignedTasks.slice(0, 5).map(t => (
              <Link key={t.id} href={`/tasks/${t.id}`} className="flex items-center justify-between py-1.5 hover:bg-[var(--bg-tertiary)] rounded px-2 -mx-2 transition-colors">
                <span className="text-xs truncate">{t.title}</span>
                <span className={`text-[10px] px-1.5 py-0.5 rounded flex-shrink-0 ml-2 ${
                  t.status === "completed" ? "bg-green-500/20 text-green-600 dark:text-green-400" :
                  t.status === "blocked" ? "bg-red-500/20 text-red-600 dark:text-red-400" :
                  "bg-[var(--bg-tertiary)] text-[var(--text-muted)]"
                }`}>{t.status.replace("_", " ")}</span>
              </Link>
            ))}
          </div>
        </section>

        <section className="col-span-2 bg-[var(--surface)] border border-[var(--border)] rounded-lg p-4 shadow-sm">
          <h3 className="text-sm font-medium text-[var(--text-muted)] mb-2">Dashboard Access</h3>
          <p className="text-xs text-[var(--text-muted)]">
            Administrative changes for agents are available in the admin dashboard.
          </p>
          {!agent.active && (
            <p className="text-xs text-amber-600 dark:text-amber-400 mt-2">
              This agent is pending approval.
            </p>
          )}
          {currentAgent?.type === "admin" && (
            <p className="text-xs text-[var(--text-faint)] mt-2">
              Use <Link href="/admin/agents" className="text-[var(--accent)] hover:underline">Admin &gt; Agents</Link> to edit metadata, rotate keys, or change this agent&apos;s status.
            </p>
          )}
        </section>
      </div>
    </div>
  );
}
