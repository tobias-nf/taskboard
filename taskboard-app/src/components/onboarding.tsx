"use client";

import { useState } from "react";
import * as api from "@/lib/api";
import { useTaskboard } from "@/lib/taskboard-context";

type Step = "slack" | "api-key" | "overview";

const SLACK_ID_PATTERN = /^U[A-Z0-9]{8,}$/;

export function OnboardingFlow() {
  const { refresh } = useTaskboard();
  const [step, setStep] = useState<Step>("slack");
  const [slackId, setSlackId] = useState("");
  const [slackError, setSlackError] = useState("");
  const [saving, setSaving] = useState(false);
  const [apiKey, setApiKey] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);

  async function handleSlackSubmit(e: React.FormEvent) {
    e.preventDefault();
    setSlackError("");

    const trimmed = slackId.trim();
    if (!SLACK_ID_PATTERN.test(trimmed)) {
      setSlackError("That doesn't look like a valid Slack member ID. It should start with 'U' followed by letters and numbers (e.g. U0AJ1HULDL3).");
      return;
    }

    setSaving(true);
    try {
      await api.updateMe({ slack_id: trimmed });
      // Generate API key for the new user
      const res = await api.rotateMyKey();
      setApiKey(res.api_key);
      setStep("api-key");
    } catch (err) {
      setSlackError("Failed to save. " + (err instanceof Error ? err.message : ""));
    } finally {
      setSaving(false);
    }
  }

  function handleCopy() {
    if (!apiKey) return;
    navigator.clipboard.writeText(apiKey);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  }

  async function handleFinish() {
    await refresh();
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-[var(--bg)]">
      <div className="w-full max-w-md">
        <div className="text-center mb-6">
          <h1 className="text-2xl font-semibold">Welcome to Taskboard</h1>
          <p className="text-sm text-[var(--text-muted)] mt-1">Let&apos;s get you set up</p>
        </div>

        {/* Step indicators */}
        <div className="flex items-center justify-center gap-2 mb-8">
          {(["slack", "api-key", "overview"] as Step[]).map((s, i) => (
            <div key={s} className="flex items-center gap-2">
              <div className={`w-2 h-2 rounded-full ${
                s === step ? "bg-[var(--accent)]" :
                (["slack", "api-key", "overview"].indexOf(step) > i ? "bg-[var(--accent)] opacity-40" : "bg-[var(--border)]")
              }`} />
              {i < 2 && <div className="w-8 h-px bg-[var(--border)]" />}
            </div>
          ))}
        </div>

        <div className="bg-[var(--surface)] border border-[var(--border)] rounded-lg p-6 shadow-sm">

          {/* Step 1: Slack ID */}
          {step === "slack" && (
            <form onSubmit={handleSlackSubmit}>
              <h2 className="text-sm font-medium mb-1">Your Slack ID</h2>
              <p className="text-xs text-[var(--text-muted)] mb-4">
                We use this to link your Taskboard account to Slack for notifications and mentions.
              </p>

              <div className="bg-[var(--bg)] border border-[var(--border)] rounded-md p-3 mb-4">
                <p className="text-xs text-[var(--text-muted)] mb-2 font-medium">How to find your Slack member ID:</p>
                <ol className="text-xs text-[var(--text-muted)] space-y-1 list-decimal list-inside">
                  <li>Open Slack and click on your profile picture (bottom left)</li>
                  <li>Click <strong>Profile</strong></li>
                  <li>Click the <strong>three dots (...)</strong> menu</li>
                  <li>Select <strong>Copy member ID</strong></li>
                </ol>
                <p className="text-xs text-[var(--text-faint)] mt-2">It looks like: <code className="font-mono">U0AJ1HULDL3</code></p>
              </div>

              <input
                type="text"
                value={slackId}
                onChange={e => setSlackId(e.target.value.toUpperCase())}
                placeholder="U..."
                autoFocus
                className="w-full bg-[var(--bg)] border border-[var(--border)] rounded-md px-3 py-2 text-sm font-mono focus:outline-none focus:border-[var(--accent)] mb-2"
              />
              {slackError && <p className="text-xs text-red-500 mb-2">{slackError}</p>}
              <button
                type="submit"
                disabled={!slackId.trim() || saving}
                className="w-full mt-2 px-4 py-2 bg-[var(--accent)] text-white rounded-md text-sm font-medium hover:bg-[var(--accent-hover)] transition-colors disabled:opacity-50"
              >
                {saving ? "Saving..." : "Continue"}
              </button>
            </form>
          )}

          {/* Step 2: API Key */}
          {step === "api-key" && apiKey && (
            <div>
              <h2 className="text-sm font-medium mb-1">Your API Key</h2>
              <p className="text-xs text-[var(--text-muted)] mb-4">
                Use this key to connect MCP clients (Claude Code, Claude Desktop) and for direct API access.
              </p>

              <div className="flex items-center gap-2 mb-2">
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
              <p className="text-xs text-amber-600 dark:text-amber-400 mb-4">
                Save this key now — it won&apos;t be shown again. You can always generate a new one from Settings.
              </p>

              <button
                onClick={() => setStep("overview")}
                className="w-full px-4 py-2 bg-[var(--accent)] text-white rounded-md text-sm font-medium hover:bg-[var(--accent-hover)] transition-colors"
              >
                Continue
              </button>
            </div>
          )}

          {/* Step 3: Overview */}
          {step === "overview" && (
            <div>
              <h2 className="text-sm font-medium mb-3">Quick Overview</h2>
              <div className="space-y-3 text-xs text-[var(--text-muted)]">
                <div>
                  <p className="font-medium text-[var(--text)]">My Tasks</p>
                  <p>Tasks assigned to you. Accept, work on, and complete them.</p>
                </div>
                <div>
                  <p className="font-medium text-[var(--text)]">Owed to Me</p>
                  <p>Tasks where you&apos;re a stakeholder — you&apos;re waiting on someone else&apos;s work.</p>
                </div>
                <div>
                  <p className="font-medium text-[var(--text)]">All Tasks</p>
                  <p>Browse all public tasks across the organization.</p>
                </div>
                <div>
                  <p className="font-medium text-[var(--text)]">MCP Integration</p>
                  <p>Connect Claude Code or Claude Desktop to Taskboard using your API key. The MCP endpoint is at <code className="font-mono text-[var(--text-faint)]">/mcp/sse</code>.</p>
                </div>
              </div>

              <button
                onClick={handleFinish}
                className="w-full mt-6 px-4 py-2 bg-[var(--accent)] text-white rounded-md text-sm font-medium hover:bg-[var(--accent-hover)] transition-colors"
              >
                Get Started
              </button>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
