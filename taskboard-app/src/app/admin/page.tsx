"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useTaskboard } from "@/lib/taskboard-context";
import * as api from "@/lib/api";
import type { AuditEntry } from "@/lib/api";

const actionColors: Record<string, string> = {
  agent_registered: "text-blue-600 dark:text-blue-400",
  agent_approved: "text-green-600 dark:text-green-400",
  agent_suspended: "text-red-600 dark:text-red-400",
  grant_created: "text-indigo-600 dark:text-indigo-400",
  grant_removed: "text-amber-600 dark:text-amber-400",
  key_rotated: "text-amber-600 dark:text-amber-400",
};

export default function AdminOverview() {
  const { agents } = useTaskboard();
  const [recentAudit, setRecentAudit] = useState<AuditEntry[]>([]);
  const [taskStats, setTaskStats] = useState({ active: 0, completed: 0, total: 0 });
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    Promise.all([
      api.getAuditLog(10),
      api.getVisibleTasks({ status: "pending,in_progress,blocked", limit: 1 }),
      api.getVisibleTasks({ status: "completed", limit: 1 }),
      api.getVisibleTasks({ limit: 1 }),
    ]).then(([audit, active, completed, all]) => {
      setRecentAudit(audit.entries || []);
      setTaskStats({
        active: active.total,
        completed: completed.total,
        total: all.total,
      });
      setLoading(false);
    }).catch(() => setLoading(false));
  }, []);

  const activeAgents = agents.filter(a => a.active).length;
  const pendingAgents = agents.filter(a => !a.active).length;
  const totalAgents = agents.length;

  return (
    <div>
      {/* Stats Grid */}
      <div className="grid grid-cols-4 gap-4 mb-6">
        <StatCard label="Active Agents" value={activeAgents} href="/admin/agents" />
        <StatCard label="Pending Approval" value={pendingAgents} href="/admin/agents" accent={pendingAgents > 0 ? "amber" : undefined} />
        <StatCard label="Total Agents" value={totalAgents} />
        <StatCard label="Active Tasks" value={taskStats.active} />
        <StatCard label="Completed Tasks" value={taskStats.completed} />
        <StatCard label="Total Tasks" value={taskStats.total} />
        <StatCard label="Agents Total" value={agents.length} />
      </div>

      {/* Recent Audit */}
      <div className="bg-[var(--surface)] border border-[var(--border)] rounded-lg shadow-sm">
        <div className="flex items-center justify-between px-4 py-3 border-b border-[var(--border)]">
          <h3 className="text-sm font-medium">Recent Activity</h3>
          <Link href="/audit" className="text-xs text-[var(--accent)] hover:underline">View all</Link>
        </div>
        {loading ? (
          <div className="px-4 py-6 text-sm text-[var(--text-muted)]">Loading...</div>
        ) : (
          <div className="divide-y divide-[var(--border)]/50">
            {recentAudit.slice(0, 8).map((entry) => (
              <div key={entry.id} className="flex items-center gap-3 px-4 py-2.5">
                <span className={`text-xs font-medium ${actionColors[entry.action] ?? "text-[var(--text-muted)]"}`}>
                  {entry.action.replace(/_/g, " ")}
                </span>
                <span className="text-xs text-[var(--text-muted)] truncate flex-1">{entry.target_id}</span>
                <span className="text-[10px] text-[var(--text-faint)] flex-shrink-0">
                  {new Date(entry.created_at).toLocaleString()}
                </span>
              </div>
            ))}
            {recentAudit.length === 0 && (
              <div className="px-4 py-6 text-center text-sm text-[var(--text-faint)]">No recent activity</div>
            )}
          </div>
        )}
      </div>
    </div>
  );
}

function StatCard({ label, value, href, accent }: { label: string; value: number; href?: string; accent?: string }) {
  const content = (
    <div className={`bg-[var(--surface)] border border-[var(--border)] rounded-lg p-4 shadow-sm ${href ? "hover:bg-[var(--bg-tertiary)] transition-colors cursor-pointer" : ""}`}>
      <p className={`text-2xl font-semibold ${accent === "amber" ? "text-amber-500" : ""}`}>{value}</p>
      <p className="text-[10px] text-[var(--text-muted)] mt-0.5">{label}</p>
    </div>
  );
  return href ? <Link href={href}>{content}</Link> : content;
}
