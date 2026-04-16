"use client";

import { useState } from "react";
import { useTaskboard } from "@/lib/taskboard-context";
import { ThemeToggle } from "@/components/theme-toggle";
import * as api from "@/lib/api";

export default function Settings() {
  const { currentAgent, logout } = useTaskboard();
  const [apiKey, setApiKey] = useState<string | null>(null);
  const [rotating, setRotating] = useState(false);
  const [copied, setCopied] = useState(false);

  if (!currentAgent) return null;

  async function handleRotateKey() {
    if (!confirm(
      apiKey
        ? "Generate a new API key? The current one will stop working immediately."
        : "Generate an API key for MCP / API access? If you already have one, it will be replaced."
    )) return;

    setRotating(true);
    try {
      const res = await api.rotateMyKey();
      setApiKey(res.api_key);
      setCopied(false);
    } catch (err) {
      alert("Failed to generate API key. " + (err instanceof Error ? err.message : ""));
    } finally {
      setRotating(false);
    }
  }

  function handleCopy() {
    if (!apiKey) return;
    navigator.clipboard.writeText(apiKey);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  }

  return (
    <div className="max-w-2xl mx-auto">
      <div className="mb-6">
        <h2 className="text-xl font-semibold">Settings</h2>
        <p className="text-sm text-[var(--text-muted)] mt-1">Review your agent profile and local dashboard settings</p>
      </div>

      {/* Profile */}
      <section className="bg-[var(--surface)] border border-[var(--border)] rounded-lg p-5 shadow-sm mb-6">
        <h3 className="text-sm font-medium mb-4">Profile</h3>

        <div className="space-y-4">
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-xs text-[var(--text-muted)] mb-1">Agent ID</label>
              <input
                type="text"
                value={currentAgent.id}
                disabled
                className="w-full bg-[var(--bg-tertiary)] border border-[var(--border)] rounded-md px-3 py-2 text-sm text-[var(--text-faint)] cursor-not-allowed"
              />
            </div>
            <div>
              <label className="block text-xs text-[var(--text-muted)] mb-1">Type</label>
              <input
                type="text"
                value={currentAgent.type}
                disabled
                className="w-full bg-[var(--bg-tertiary)] border border-[var(--border)] rounded-md px-3 py-2 text-sm text-[var(--text-faint)] cursor-not-allowed"
              />
            </div>
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-xs text-[var(--text-muted)] mb-1">Email</label>
              <input
                type="email"
                value={currentAgent.email ?? ""}
                disabled
                className="w-full bg-[var(--bg-tertiary)] border border-[var(--border)] rounded-md px-3 py-2 text-sm text-[var(--text-faint)] cursor-not-allowed"
              />
            </div>
            <div>
              <label className="block text-xs text-[var(--text-muted)] mb-1">Slack ID</label>
              <input
                type="text"
                value={currentAgent.slack_id ?? ""}
                disabled
                className="w-full bg-[var(--bg-tertiary)] border border-[var(--border)] rounded-md px-3 py-2 text-sm font-mono text-[var(--text-faint)] cursor-not-allowed"
              />
            </div>
          </div>

        </div>
      </section>

      {/* API Key for MCP / programmatic access */}
      <section className="bg-[var(--surface)] border border-[var(--border)] rounded-lg p-5 shadow-sm mb-6">
        <h3 className="text-sm font-medium mb-2">API Key</h3>
        <p className="text-xs text-[var(--text-muted)] mb-4">
          Use an API key for MCP connections (Claude Code, Claude Desktop) and direct API access.
          The key is shown only once after generation.
        </p>

        {apiKey ? (
          <div className="space-y-3">
            <div className="flex items-center gap-2">
              <input
                type="text"
                value={apiKey}
                readOnly
                className="flex-1 bg-[var(--bg)] border border-[var(--border)] rounded-md px-3 py-2 text-xs font-mono select-all"
              />
              <button
                onClick={handleCopy}
                className="px-3 py-2 bg-[var(--bg-tertiary)] border border-[var(--border)] rounded-md text-xs hover:border-[var(--text-faint)] transition-colors"
              >
                {copied ? "Copied" : "Copy"}
              </button>
            </div>
            <p className="text-xs text-amber-600 dark:text-amber-400">
              Save this key now — it won&apos;t be shown again.
            </p>
          </div>
        ) : (
          <div className="flex items-center gap-3">
            <div className="flex-1 bg-[var(--bg-tertiary)] border border-[var(--border)] rounded-md px-3 py-2 text-sm font-mono text-[var(--text-faint)]">
              hive_sk_{currentAgent.id}_••••••••
            </div>
            <button
              onClick={handleRotateKey}
              disabled={rotating}
              className="px-4 py-2 bg-[var(--accent)] text-white rounded-md text-xs font-medium hover:bg-[var(--accent-hover)] transition-colors disabled:opacity-50"
            >
              {rotating ? "Generating..." : "Generate Key"}
            </button>
          </div>
        )}

        {apiKey && (
          <button
            onClick={handleRotateKey}
            disabled={rotating}
            className="mt-3 text-xs text-[var(--text-muted)] hover:text-[var(--text)] transition-colors"
          >
            {rotating ? "Rotating..." : "Rotate key (invalidates current)"}
          </button>
        )}
      </section>

      {/* Appearance */}
      <section className="bg-[var(--surface)] border border-[var(--border)] rounded-lg p-5 shadow-sm mb-6">
        <h3 className="text-sm font-medium mb-3">Appearance</h3>
        <div className="flex items-center justify-between">
          <span className="text-xs text-[var(--text-muted)]">Theme</span>
          <ThemeToggle />
        </div>
      </section>

      {/* Account */}
      <section className="bg-[var(--surface)] border border-[var(--border)] rounded-lg p-5 shadow-sm">
        <h3 className="text-sm font-medium mb-2">Account</h3>
        <div className="space-y-2 text-xs text-[var(--text-muted)]">
          <div className="flex justify-between">
            <span>Agent ID</span>
            <span className="font-mono">{currentAgent.id}</span>
          </div>
          <div className="flex justify-between">
            <span>Type</span>
            <span>{currentAgent.type}</span>
          </div>
          <div className="flex justify-between">
            <span>Status</span>
            <span className="text-green-600 dark:text-green-400">{currentAgent.active ? "Active" : "Inactive"}</span>
          </div>
          <div className="flex justify-between">
            <span>Registered</span>
            <span>{currentAgent.created_at ? new Date(currentAgent.created_at).toLocaleDateString() : "—"}</span>
          </div>
        </div>
        <div className="mt-4 pt-4 border-t border-[var(--border)]">
          <button
            onClick={logout}
            className="px-4 py-2 text-sm text-red-500 hover:text-red-400 hover:bg-red-500/10 rounded-md transition-colors"
          >
            Sign out
          </button>
        </div>
      </section>
    </div>
  );
}
