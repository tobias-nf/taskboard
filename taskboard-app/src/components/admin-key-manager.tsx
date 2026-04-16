"use client";

import { useState } from "react";
import { adminRotateKey } from "@/lib/api";

interface AdminKeyManagerProps {
  agentId: string;
  agentName: string;
}

export function AdminKeyManager({ agentId, agentName }: AdminKeyManagerProps) {
  const [copied, setCopied] = useState(false);
  const [showRotateConfirm, setShowRotateConfirm] = useState(false);
  const [rotatedKey, setRotatedKey] = useState<string | null>(null);
  const [rotating, setRotating] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const maskedKey = "hive_sk_" + agentId + "_••••••••••••••••";

  function handleCopy() {
    navigator.clipboard.writeText(maskedKey);
    setCopied(true);
    setTimeout(() => setCopied(false), 1500);
  }

  async function handleRotateKey() {
    setRotating(true);
    setError(null);
    try {
      const result = await adminRotateKey(agentId);
      setRotatedKey(result.api_key);
      setShowRotateConfirm(false);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Failed to rotate key");
    } finally {
      setRotating(false);
    }
  }

  return (
    <div className="bg-[var(--surface)] border border-[var(--border)] rounded-lg p-4 shadow-sm">
      <h3 className="text-sm font-medium text-[var(--text-muted)] mb-3">API Key (Admin)</h3>

      <div className="mb-3">
        <label className="block text-xs text-[var(--text-faint)] mb-1">Key for {agentName}</label>
        <button
          onClick={handleCopy}
          className="w-full flex items-center gap-2 bg-[var(--bg-tertiary)] border border-[var(--border)] rounded-md px-3 py-2 text-sm font-mono text-[var(--text-faint)] hover:border-[var(--accent)]/30 transition-colors text-left"
        >
          <span className="flex-1 truncate">{maskedKey}</span>
          <span className="text-xs flex-shrink-0">{copied ? "✓ Copied" : "📋 Copy"}</span>
        </button>
      </div>

      {rotatedKey && (
        <div className="mb-3 p-3 bg-green-500/10 border border-green-500/30 rounded-lg">
          <p className="text-xs font-medium text-green-600 dark:text-green-400 mb-1">New API key for {agentName}. Copy it now — it won&apos;t be shown again.</p>
          <div className="flex items-center gap-2">
            <code className="flex-1 text-xs bg-[var(--bg)] border border-[var(--border)] rounded px-2 py-1.5 font-mono break-all">{rotatedKey}</code>
            <button
              onClick={() => { navigator.clipboard.writeText(rotatedKey); }}
              className="px-2 py-1.5 bg-[var(--bg-tertiary)] border border-[var(--border)] rounded text-xs hover:bg-[var(--bg)] transition-colors flex-shrink-0"
            >
              Copy
            </button>
          </div>
        </div>
      )}

      {!showRotateConfirm ? (
        <button
          onClick={() => setShowRotateConfirm(true)}
          className="px-3 py-1.5 bg-[var(--bg-tertiary)] border border-[var(--border)] rounded-md text-xs text-amber-600 dark:text-amber-400 hover:bg-amber-500/10 transition-colors"
        >
          Rotate Key for {agentName}
        </button>
      ) : (
        <div className="p-3 bg-amber-500/10 border border-amber-500/30 rounded-lg">
          <p className="text-xs text-amber-600 dark:text-amber-400 mb-2">
            This will invalidate {agentName}&apos;s current API key immediately. Their agent will stop working until the new key is configured.
          </p>
          {error && <p className="text-xs text-red-500 mb-2">{error}</p>}
          <div className="flex gap-2">
            <button
              onClick={handleRotateKey}
              disabled={rotating}
              className="px-3 py-1.5 bg-amber-600 text-white rounded-md text-xs font-medium hover:bg-amber-500 transition-colors disabled:opacity-50"
            >
              {rotating ? "Rotating..." : "Confirm Rotation"}
            </button>
            <button
              onClick={() => setShowRotateConfirm(false)}
              className="px-3 py-1.5 bg-[var(--bg-tertiary)] border border-[var(--border)] rounded-md text-xs text-[var(--text-muted)] hover:bg-[var(--bg)] transition-colors"
            >
              Cancel
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
