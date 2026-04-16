"use client";

import { Fragment, useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { useTaskboard } from "@/lib/taskboard-context";
import * as api from "@/lib/api";
import type { Agent, AgentCreateInput, AgentUpdateInput } from "@/lib/api";
import { agentLabel } from "@/lib/api";

const typeColors: Record<string, string> = {
  user: "bg-blue-500/20 text-blue-600 dark:text-blue-400",
  admin: "bg-amber-500/20 text-amber-600 dark:text-amber-400",
  service: "bg-purple-500/20 text-purple-600 dark:text-purple-400",
};

const inputClassName = "w-full rounded-md border border-[var(--border)] bg-[var(--bg)] px-3 py-2 text-sm outline-none transition-colors focus:border-[var(--accent)]";
const labelClassName = "block text-[10px] uppercase tracking-wider text-[var(--text-faint)] mb-1";

type Filter = "all" | "pending" | "active" | "suspended" | "user" | "admin";

type AgentDraft = {
  type: Agent["type"];
  email: string;
  slack_id: string;
  preferred_tool: string;
};

type NewAgentForm = {
  id: string;
  type: Agent["type"];
  email: string;
};

function agentDraftFromAgent(agent: Agent): AgentDraft {
  return {
    type: agent.type,
    email: agent.email || "",
    slack_id: agent.slack_id || "",
    preferred_tool: agent.preferred_tool || "",
  };
}

function getStatus(agent: Agent) {
  if (agent.active) {
    return { label: "Active", className: "text-green-600 dark:text-green-400" };
  }
  if (agent.approved_by) {
    return { label: "Suspended", className: "text-red-500" };
  }
  return { label: "Pending", className: "text-amber-600 dark:text-amber-400" };
}

function isConfiguredAdmin(agent: Agent) {
  return agent.id === "hive-admin";
}

export default function AdminAgents() {
  const { agents, refresh } = useTaskboard();
  const [filter, setFilter] = useState<Filter>("all");
  const [search, setSearch] = useState("");
  const [expandedAgentId, setExpandedAgentId] = useState<string | null>(null);
  const [editDrafts, setEditDrafts] = useState<Record<string, AgentDraft>>({});
  const [showCreate, setShowCreate] = useState(false);
  const [newAgent, setNewAgent] = useState<NewAgentForm>({
    id: "",
    type: "user",
    email: "",
  });
  const [createdKey, setCreatedKey] = useState<{ agentId: string; apiKey: string } | null>(null);
  const [rotatedKeys, setRotatedKeys] = useState<Record<string, string>>({});
  const [actionLoading, setActionLoading] = useState<string | null>(null);
  const [savingAgentId, setSavingAgentId] = useState<string | null>(null);
  const [creating, setCreating] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);

  useEffect(() => {
    setEditDrafts(prev => {
      const next = { ...prev };
      for (const agent of agents) {
        next[agent.id] ??= agentDraftFromAgent(agent);
      }
      return next;
    });
  }, [agents]);

  const filtered = useMemo(() => {
    const needle = search.trim().toLowerCase();
    return agents.filter(agent => {
      const matchesFilter = (() => {
        switch (filter) {
          case "pending":
            return !agent.active && !agent.approved_by;
          case "active":
            return agent.active;
          case "suspended":
            return !agent.active && !!agent.approved_by;
          case "user":
          case "admin":
            return agent.type === filter;
          default:
            return true;
        }
      })();

      if (!matchesFilter) return false;
      if (!needle) return true;

      return [
        agent.id,
        agent.email || "",
        agent.slack_id || "",
      ].some(value => value.toLowerCase().includes(needle));
    });
  }, [agents, filter, search]);

  function clearNotices() {
    setError(null);
    setSuccess(null);
  }

  function updateDraft(agentId: string, patch: Partial<AgentDraft>) {
    setEditDrafts(prev => ({
      ...prev,
      [agentId]: {
        ...(prev[agentId] || {
          type: "user",
          email: "",
          slack_id: "",
          preferred_tool: "",
        }),
        ...patch,
      },
    }));
  }

  function updateNewAgent(patch: Partial<NewAgentForm>) {
    setNewAgent(prev => ({ ...prev, ...patch }));
  }

  async function copyToClipboard(value: string, message: string) {
    await navigator.clipboard.writeText(value);
    setSuccess(message);
  }

  async function handleApprove(id: string) {
    setActionLoading(`approve:${id}`);
    clearNotices();
    try {
      await api.approveAgent(id);
      await refresh();
      setSuccess(`Approved ${id}.`);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to approve agent");
    } finally {
      setActionLoading(null);
    }
  }

  async function handleSuspend(id: string) {
    setActionLoading(`suspend:${id}`);
    clearNotices();
    try {
      await api.suspendAgent(id);
      await refresh();
      setSuccess(`Suspended ${id}.`);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to suspend agent");
    } finally {
      setActionLoading(null);
    }
  }

  async function handleReactivate(id: string) {
    setActionLoading(`reactivate:${id}`);
    clearNotices();
    try {
      await api.reactivateAgent(id);
      await refresh();
      setSuccess(`Reactivated ${id}.`);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to reactivate agent");
    } finally {
      setActionLoading(null);
    }
  }

  async function handleCreate() {
    const payload: AgentCreateInput = {
      id: newAgent.id.trim(),
      type: newAgent.type,
      email: newAgent.email.trim() || undefined,
    };

    if (!payload.id) {
      setError("Agent ID is required.");
      return;
    }
    if (payload.type !== "user" && payload.type !== "admin" && payload.type !== "service") {
      setError("Type must be 'user', 'admin', or 'service'.");
      return;
    }

    setCreating(true);
    clearNotices();
    try {
      const created = await api.createAgent(payload);
      await refresh();
      setCreatedKey({ agentId: created.id, apiKey: created.api_key });
      setExpandedAgentId(created.id);
      setNewAgent({
        id: "",
        type: "user",
        email: "",
      });
      setShowCreate(false);
      setSuccess(`Created ${payload.id}. Copy the API key before leaving this page.`);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to create agent");
    } finally {
      setCreating(false);
    }
  }

  async function handleSave(agentId: string) {
    const draft = editDrafts[agentId];
    if (!draft) return;

    const payload: AgentUpdateInput = {
      type: draft.type,
      email: draft.email.trim() || null,
      slack_id: draft.slack_id.trim() || null,
      preferred_tool: draft.preferred_tool.trim() || null,
    };

    setSavingAgentId(agentId);
    clearNotices();
    try {
      const updated = await api.updateAgent(agentId, payload);
      setEditDrafts(prev => ({ ...prev, [agentId]: agentDraftFromAgent(updated) }));
      await refresh();
      setSuccess(`Saved changes for ${agentLabel(updated)}.`);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to update agent");
    } finally {
      setSavingAgentId(null);
    }
  }

  async function handleRotateKey(agent: Agent) {
    if (!window.confirm(`Rotate the API key for ${agentLabel(agent)}? The current key will stop working immediately.`)) {
      return;
    }

    setActionLoading(`rotate:${agent.id}`);
    clearNotices();
    try {
      const result = await api.adminRotateKey(agent.id);
      setRotatedKeys(prev => ({ ...prev, [agent.id]: result.api_key }));
      setSuccess(`Rotated the API key for ${agentLabel(agent)}.`);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to rotate API key");
    } finally {
      setActionLoading(null);
    }
  }

  return (
    <div className="space-y-4">
      <div className="flex flex-col gap-3 rounded-lg border border-[var(--border)] bg-[var(--surface)] p-4 shadow-sm lg:flex-row lg:items-end lg:justify-between">
        <div className="flex-1">
          <label className={labelClassName}>Search Agents</label>
          <input
            value={search}
            onChange={e => setSearch(e.target.value)}
            placeholder="Search by name, id, email, or title"
            className={inputClassName}
          />
        </div>
        <div className="flex gap-2">
          <button
            onClick={() => setShowCreate(current => !current)}
            className="rounded-md border border-[var(--border)] bg-[var(--bg)] px-3 py-2 text-sm text-[var(--text)] transition-colors hover:border-[var(--accent)]/40"
          >
            {showCreate ? "Close Create Form" : "Create Agent"}
          </button>
        </div>
      </div>

      <div className="flex flex-wrap gap-1">
        {(["all", "pending", "active", "suspended", "user", "admin"] as Filter[]).map(item => {
          const count = item === "all"
            ? agents.length
            : item === "pending"
              ? agents.filter(agent => !agent.active && !agent.approved_by).length
              : item === "active"
                ? agents.filter(agent => agent.active).length
                : item === "suspended"
                  ? agents.filter(agent => !agent.active && !!agent.approved_by).length
                  : agents.filter(agent => agent.type === item).length;
          return (
            <button
              key={item}
              onClick={() => setFilter(item)}
              className={`rounded px-2.5 py-1 text-xs transition-colors ${
                filter === item
                  ? "bg-[var(--accent)]/10 font-medium text-[var(--accent)]"
                  : "text-[var(--text-muted)] hover:bg-[var(--bg-tertiary)]"
              }`}
            >
              {item.charAt(0).toUpperCase() + item.slice(1)} ({count})
            </button>
          );
        })}
      </div>

      {error && (
        <div className="rounded-lg border border-red-500/20 bg-red-500/10 p-3 text-xs text-red-500">{error}</div>
      )}
      {success && (
        <div className="rounded-lg border border-green-500/20 bg-green-500/10 p-3 text-xs text-green-700 dark:text-green-400">{success}</div>
      )}
      {createdKey && (
        <div className="rounded-lg border border-green-500/30 bg-green-500/10 p-4">
          <div className="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
            <div>
              <p className="text-sm font-medium text-green-700 dark:text-green-400">New API key for {createdKey.agentId}</p>
              <p className="mt-1 text-xs text-[var(--text-muted)]">This key is only shown once here. Store it in the agent runtime before leaving the page.</p>
            </div>
            <button
              onClick={() => copyToClipboard(createdKey.apiKey, `Copied the new API key for ${createdKey.agentId}.`)}
              className="rounded-md border border-green-500/40 bg-[var(--bg)] px-3 py-2 text-xs text-green-700 transition-colors hover:bg-green-500/10 dark:text-green-400"
            >
              Copy Key
            </button>
          </div>
          <code className="mt-3 block break-all rounded-md border border-[var(--border)] bg-[var(--bg)] px-3 py-2 text-xs font-mono">
            {createdKey.apiKey}
          </code>
        </div>
      )}

      {showCreate && (
        <section className="rounded-lg border border-[var(--border)] bg-[var(--surface)] p-4 shadow-sm">
          <div className="mb-4">
            <h3 className="text-sm font-medium text-[var(--text-muted)]">Create Agent</h3>
            <p className="mt-1 text-xs text-[var(--text-faint)]">Admins create active agents immediately.</p>
          </div>

          <div className="grid gap-3 md:grid-cols-2">
            <div>
              <label className={labelClassName}>Agent ID</label>
              <input value={newAgent.id} onChange={e => updateNewAgent({ id: e.target.value })} className={inputClassName} placeholder="ops-bot" />
            </div>
            <div>
              <label className={labelClassName}>Type</label>
              <select value={newAgent.type} onChange={e => updateNewAgent({ type: e.target.value as Agent["type"] })} className={inputClassName}>
                <option value="user">user</option>
                <option value="admin">admin</option>
                <option value="service">service</option>
              </select>
            </div>
            <div>
              <label className={labelClassName}>Email</label>
              <input value={newAgent.email} onChange={e => updateNewAgent({ email: e.target.value })} className={inputClassName} placeholder="optional" />
            </div>
          </div>

          <div className="mt-4 flex gap-2">
            <button
              onClick={handleCreate}
              disabled={creating}
              className="rounded-md bg-[var(--accent)] px-3 py-2 text-sm font-medium text-white transition-colors hover:opacity-90 disabled:opacity-50"
            >
              {creating ? "Creating..." : "Create Agent"}
            </button>
            <button
              onClick={() => setShowCreate(false)}
              className="rounded-md border border-[var(--border)] bg-[var(--bg)] px-3 py-2 text-sm text-[var(--text-muted)] transition-colors hover:border-[var(--accent)]/40"
            >
              Cancel
            </button>
          </div>
        </section>
      )}

      <div className="overflow-hidden rounded-lg border border-[var(--border)] bg-[var(--surface)] shadow-sm">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-[var(--border)]">
              <th className="px-4 py-3 text-left text-[10px] font-medium uppercase tracking-wider text-[var(--text-faint)]">Agent</th>
              <th className="px-4 py-3 text-left text-[10px] font-medium uppercase tracking-wider text-[var(--text-faint)]">Type</th>
              <th className="px-4 py-3 text-left text-[10px] font-medium uppercase tracking-wider text-[var(--text-faint)]">Status</th>
              <th className="px-4 py-3 text-left text-[10px] font-medium uppercase tracking-wider text-[var(--text-faint)]">Last Seen</th>
              <th className="px-4 py-3 text-right text-[10px] font-medium uppercase tracking-wider text-[var(--text-faint)]">Actions</th>
            </tr>
          </thead>
          <tbody>
            {filtered.map(agent => {
              const status = getStatus(agent);
              const envManaged = isConfiguredAdmin(agent);
              const isExpanded = expandedAgentId === agent.id;
              const draft = editDrafts[agent.id] || agentDraftFromAgent(agent);
              const rotatedKey = rotatedKeys[agent.id];

              return (
                <Fragment key={agent.id}>
                  <tr className="border-b border-[var(--border)]/50 transition-colors hover:bg-[var(--bg-tertiary)]">
                    <td className="px-4 py-3">
                      <Link href={`/agents/${agent.id}`} className="hover:text-[var(--accent)]">
                        <div className="font-medium">{agentLabel(agent)}</div>
                        <div className="mt-0.5 text-[10px] font-mono text-[var(--text-faint)]">{agent.id}</div>
                      </Link>
                    </td>
                    <td className="px-4 py-3">
                      <span className={`rounded px-2 py-0.5 text-xs ${typeColors[agent.type]}`}>{agent.type}</span>
                    </td>
                    <td className="px-4 py-3">
                      <span className={`text-xs ${status.className}`}>{status.label}</span>
                    </td>
                    <td className="px-4 py-3 text-xs text-[var(--text-faint)]">
                      {agent.last_seen_at ? new Date(agent.last_seen_at).toLocaleString() : "\u2014"}
                    </td>
                    <td className="px-4 py-3">
                      <div className="flex justify-end gap-1.5">
                        <button
                          onClick={() => setExpandedAgentId(current => (current === agent.id ? null : agent.id))}
                          className="rounded border border-[var(--border)] bg-[var(--bg)] px-2.5 py-1 text-xs text-[var(--text-muted)] transition-colors hover:border-[var(--accent)]/40"
                        >
                          {isExpanded ? "Close" : "Manage"}
                        </button>
                        {!agent.active && !agent.approved_by && (
                          <button
                            onClick={() => handleApprove(agent.id)}
                            disabled={actionLoading === `approve:${agent.id}`}
                            className="rounded bg-green-600 px-2.5 py-1 text-xs text-white transition-colors hover:bg-green-700 disabled:opacity-50"
                          >
                            Approve
                          </button>
                        )}
                        {agent.active && !envManaged && (
                          <button
                            onClick={() => handleSuspend(agent.id)}
                            disabled={actionLoading === `suspend:${agent.id}`}
                            className="rounded bg-red-500/10 px-2.5 py-1 text-xs text-red-500 transition-colors hover:bg-red-500/20 disabled:opacity-50"
                          >
                            Suspend
                          </button>
                        )}
                        {!agent.active && !!agent.approved_by && (
                          <button
                            onClick={() => handleReactivate(agent.id)}
                            disabled={actionLoading === `reactivate:${agent.id}`}
                            className="rounded bg-blue-500/10 px-2.5 py-1 text-xs text-blue-500 transition-colors hover:bg-blue-500/20 disabled:opacity-50"
                          >
                            Reactivate
                          </button>
                        )}
                      </div>
                    </td>
                  </tr>
                  {isExpanded && (
                    <tr className="border-b border-[var(--border)]/50 bg-[var(--bg)]/50">
                      <td colSpan={5} className="px-4 py-4">
                        <div className="grid gap-4 lg:grid-cols-[minmax(0,2fr)_minmax(280px,1fr)]">
                          <section className="rounded-lg border border-[var(--border)] bg-[var(--surface)] p-4">
                            <div className="mb-4 flex items-center justify-between">
                              <div>
                                <h3 className="text-sm font-medium text-[var(--text-muted)]">Agent Metadata</h3>
                                <p className="mt-1 text-xs text-[var(--text-faint)]">Update profile fields, agent type, and routing hints from the dashboard.</p>
                              </div>
                              {envManaged && (
                                <span className="rounded bg-amber-500/10 px-2 py-1 text-[10px] text-amber-600 dark:text-amber-400">
                                  Env-managed
                                </span>
                              )}
                            </div>

                            <div className="grid gap-3 md:grid-cols-2">
                              <div>
                                <label className={labelClassName}>Type</label>
                                <select
                                  value={draft.type}
                                  onChange={e => updateDraft(agent.id, { type: e.target.value as Agent["type"] })}
                                  className={inputClassName}
                                  disabled={envManaged}
                                >
                                  <option value="user">user</option>
                                  <option value="admin">admin</option>
                                  <option value="service">service</option>
                                </select>
                              </div>
                              <div>
                                <label className={labelClassName}>Email</label>
                                <input
                                  value={draft.email}
                                  onChange={e => updateDraft(agent.id, { email: e.target.value })}
                                  className={inputClassName}
                                  placeholder="optional"
                                  disabled={envManaged}
                                />
                              </div>
                              <div>
                                <label className={labelClassName}>Slack ID</label>
                                <input
                                  value={draft.slack_id}
                                  onChange={e => updateDraft(agent.id, { slack_id: e.target.value })}
                                  className={inputClassName}
                                  disabled={envManaged}
                                />
                              </div>
                              <div>
                                <label className={labelClassName}>Preferred Tool</label>
                                <input
                                  value={draft.preferred_tool}
                                  onChange={e => updateDraft(agent.id, { preferred_tool: e.target.value })}
                                  className={inputClassName}
                                  placeholder="claude-code, codex, browser"
                                  disabled={envManaged}
                                />
                              </div>
                            </div>

                            <div className="mt-4 flex flex-wrap items-center gap-2">
                              <button
                                onClick={() => handleSave(agent.id)}
                                disabled={savingAgentId === agent.id || envManaged}
                                className="rounded-md bg-[var(--accent)] px-3 py-2 text-sm font-medium text-white transition-colors hover:opacity-90 disabled:opacity-50"
                              >
                                {savingAgentId === agent.id ? "Saving..." : "Save Changes"}
                              </button>
                              <button
                                onClick={() => setEditDrafts(prev => ({ ...prev, [agent.id]: agentDraftFromAgent(agent) }))}
                                disabled={savingAgentId === agent.id}
                                className="rounded-md border border-[var(--border)] bg-[var(--bg)] px-3 py-2 text-sm text-[var(--text-muted)] transition-colors hover:border-[var(--accent)]/40"
                              >
                                Reset
                              </button>
                              {envManaged && (
                                <span className="text-xs text-[var(--text-faint)]">The configured admin account is synced from `TASKBOARD_ADMIN_API_KEY`.</span>
                              )}
                            </div>
                          </section>

                          <section className="space-y-4">
                            <div className="rounded-lg border border-[var(--border)] bg-[var(--surface)] p-4">
                              <h3 className="text-sm font-medium text-[var(--text-muted)]">Key Management</h3>
                              <p className="mt-1 text-xs text-[var(--text-faint)]">Rotate keys here when an agent is re-provisioned or a secret is exposed.</p>

                              {rotatedKey ? (
                                <div className="mt-3 rounded-lg border border-amber-500/30 bg-amber-500/10 p-3">
                                  <p className="text-xs text-amber-700 dark:text-amber-400">New API key for {agentLabel(agent)}. Copy it now; it will not be shown again after refresh.</p>
                                  <code className="mt-2 block break-all rounded-md border border-[var(--border)] bg-[var(--bg)] px-3 py-2 text-xs font-mono">
                                    {rotatedKey}
                                  </code>
                                  <button
                                    onClick={() => copyToClipboard(rotatedKey, `Copied the rotated API key for ${agentLabel(agent)}.`)}
                                    className="mt-3 rounded-md border border-[var(--border)] bg-[var(--bg)] px-3 py-2 text-xs text-[var(--text)] transition-colors hover:border-[var(--accent)]/40"
                                  >
                                    Copy Rotated Key
                                  </button>
                                </div>
                              ) : (
                                <div className="mt-3 rounded-lg border border-[var(--border)] bg-[var(--bg)] p-3 text-xs text-[var(--text-muted)]">
                                  No new key is currently pending copy for this agent.
                                </div>
                              )}

                              <button
                                onClick={() => handleRotateKey(agent)}
                                disabled={actionLoading === `rotate:${agent.id}` || envManaged}
                                className="mt-3 rounded-md bg-amber-600 px-3 py-2 text-sm font-medium text-white transition-colors hover:bg-amber-500 disabled:opacity-50"
                              >
                                {actionLoading === `rotate:${agent.id}` ? "Rotating..." : "Rotate API Key"}
                              </button>
                              {envManaged && <p className="mt-2 text-xs text-[var(--text-faint)]">The configured admin key is environment-managed and cannot be rotated here.</p>}
                            </div>

                            <div className="rounded-lg border border-[var(--border)] bg-[var(--surface)] p-4">
                              <h3 className="text-sm font-medium text-[var(--text-muted)]">Admin Notes</h3>
                              <div className="mt-3 space-y-2 text-xs text-[var(--text-muted)]">
                                <p>Suspending an agent immediately invalidates future requests made with its current key.</p>
                              </div>
                            </div>
                          </section>
                        </div>
                      </td>
                    </tr>
                  )}
                </Fragment>
              );
            })}

          </tbody>
        </table>
        {filtered.length === 0 && (
          <div className="px-4 py-8 text-center text-sm text-[var(--text-faint)]">
            No agents match this filter.
          </div>
        )}
      </div>
    </div>
  );
}
