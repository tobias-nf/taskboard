"use client";

import { useMemo, useState } from "react";
import Link from "next/link";
import { useTaskboard } from "@/lib/taskboard-context";
import { agentLabel } from "@/lib/api";

const typeColors: Record<string, string> = {
  user: "bg-blue-500/20 text-blue-600 dark:text-blue-400",
  admin: "bg-amber-500/20 text-amber-600 dark:text-amber-400",
};

export default function Agents() {
  const { agents } = useTaskboard();
  const [search, setSearch] = useState("");

  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase();
    if (!q) return agents;
    return agents.filter((agent) => {
      const haystack = [
        agent.id,
        agent.type,
        agent.email,
      ]
        .filter(Boolean)
        .join(" ")
        .toLowerCase();
      return haystack.includes(q);
    });
  }, [agents, search]);

  return (
    <div>
      <div className="mb-6">
        <div>
          <h2 className="text-xl font-semibold">Agents</h2>
          <p className="text-sm text-[var(--text-muted)] mt-1">
            {filtered.length} shown &middot; {agents.filter(a => a.active).length} active &middot; {agents.filter(a => !a.active).length} pending
          </p>
        </div>
      </div>

      <div className="mb-4 p-3 bg-[var(--surface)] border border-[var(--border)] rounded-lg text-xs text-[var(--text-muted)] shadow-sm">
        Search the agent directory by name, ID, or title. Registration and approval still happen through the API or MCP.
      </div>

      <div className="mb-4">
        <input
          type="search"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder="Search agents..."
          className="w-full bg-[var(--surface)] border border-[var(--border)] rounded-lg px-4 py-2.5 text-sm shadow-sm"
        />
      </div>

      <div className="bg-[var(--surface)] border border-[var(--border)] rounded-lg overflow-hidden shadow-sm">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-[var(--border)]">
              <th className="text-left px-4 py-3 text-[10px] uppercase tracking-wider text-[var(--text-faint)] font-medium">Agent</th>
              <th className="text-left px-4 py-3 text-[10px] uppercase tracking-wider text-[var(--text-faint)] font-medium">Type</th>
              <th className="text-left px-4 py-3 text-[10px] uppercase tracking-wider text-[var(--text-faint)] font-medium">Status</th>
              <th className="text-left px-4 py-3 text-[10px] uppercase tracking-wider text-[var(--text-faint)] font-medium">Last Seen</th>
            </tr>
          </thead>
          <tbody>
            {filtered.map((agent) => (
              <tr key={agent.id} className="border-b border-[var(--border)]/50 hover:bg-[var(--bg-tertiary)] transition-colors">
                <td className="px-4 py-3">
                  <Link href={`/agents/${agent.id}`} className="hover:text-[var(--accent)]">
                    <div>
                      <span className="font-medium">{agentLabel(agent)}</span>
                    </div>
                    <span className="text-[10px] font-mono text-[var(--text-faint)]">{agent.id}</span>
                  </Link>
                </td>
                <td className="px-4 py-3">
                  <span className={`px-2 py-0.5 rounded text-xs ${typeColors[agent.type]}`}>{agent.type}</span>
                </td>
                <td className="px-4 py-3">
                  {agent.active ? (
                    <span className="text-green-600 dark:text-green-400 text-xs">Active</span>
                  ) : (
                    <span className="text-amber-600 dark:text-amber-400 text-xs">Pending</span>
                  )}
                </td>
                <td className="px-4 py-3 text-xs text-[var(--text-faint)]">
                  {agent.last_seen_at ? new Date(agent.last_seen_at).toLocaleString() : "—"}
                </td>
            </tr>
            ))}
            {filtered.length === 0 && (
              <tr>
                <td colSpan={4} className="px-4 py-8 text-center text-sm text-[var(--text-faint)]">No agents match your search</td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}
