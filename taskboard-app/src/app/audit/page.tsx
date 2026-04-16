"use client";

import { useEffect, useState } from "react";
import * as api from "@/lib/api";
import type { AuditEntry } from "@/lib/api";

const actionLabels: Record<string, string> = {
  agent_created: "Agent created",
  agent_registered: "Agent registered",
  agent_approved: "Agent approved",
  agent_suspended: "Agent suspended",
  agent_reactivated: "Agent reactivated",
  key_rotated: "API key rotated",
};

const actionColors: Record<string, string> = {
  agent_created: "text-blue-600 dark:text-blue-400",
  agent_registered: "text-blue-600 dark:text-blue-400",
  agent_approved: "text-green-600 dark:text-green-400",
  agent_suspended: "text-red-600 dark:text-red-400",
  agent_reactivated: "text-green-600 dark:text-green-400",
  key_rotated: "text-amber-600 dark:text-amber-400",
};

export default function Audit() {
  const [entries, setEntries] = useState<AuditEntry[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    api.getAuditLog().then(res => {
      setEntries(res.entries || []);
      setLoading(false);
    }).catch(() => setLoading(false));
  }, []);

  if (loading) return <div className="text-sm text-[var(--text-muted)]">Loading...</div>;

  return (
    <div>
      <div className="mb-6">
        <h2 className="text-xl font-semibold">Audit Log</h2>
        <p className="text-sm text-[var(--text-muted)] mt-1">Admin actions and system events</p>
      </div>

      <div className="bg-[var(--surface)] border border-[var(--border)] rounded-lg shadow-sm">
        <div className="divide-y divide-[var(--border)]/50">
          {entries.map((entry) => (
            <div key={entry.id} className="flex items-start gap-4 px-4 py-3">
              <div className="flex-shrink-0 w-1 h-8 rounded-full bg-[var(--border)] mt-1" />
              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-2">
                  <span className={`text-sm font-medium ${actionColors[entry.action] ?? "text-[var(--text-muted)]"}`}>
                    {actionLabels[entry.action] ?? entry.action}
                  </span>
                  <span className="text-[10px] text-[var(--text-faint)]">{entry.target_type}</span>
                </div>
                <p className="text-xs text-[var(--text-muted)] mt-0.5">
                  {entry.target_id}
                  {(() => {
                    if (entry.details && typeof entry.details === "object" && Object.keys(entry.details as Record<string, unknown>).length > 0) {
                      return <span className="ml-2 text-[var(--text-faint)]">{JSON.stringify(entry.details)}</span>;
                    }
                    return null;
                  })()}
                </p>
              </div>
              <div className="flex-shrink-0 text-right">
                <p className="text-xs text-[var(--text-faint)]">{entry.actor}</p>
                <p className="text-[10px] text-[var(--text-faint)]">{new Date(entry.created_at).toLocaleString()}</p>
              </div>
            </div>
          ))}
          {entries.length === 0 && (
            <div className="px-4 py-8 text-center text-sm text-[var(--text-faint)]">No audit entries</div>
          )}
        </div>
      </div>
    </div>
  );
}
