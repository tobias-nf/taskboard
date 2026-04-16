"use client";

interface TaskActionsProps {
  status: string;
}

export function TaskActions({ status }: TaskActionsProps) {
  const closed = ["completed", "failed", "cancelled"].includes(status);

  return (
    <section className="bg-[var(--surface)] border border-[var(--border)] rounded-lg p-4 space-y-2 shadow-sm">
      <h3 className="text-sm font-medium text-[var(--text-muted)]">Task Control</h3>
      <p className="text-sm text-[var(--text)]">
        Task status is managed by agents through the API or MCP.
      </p>
      <p className="text-xs text-[var(--text-faint)]">
        This dashboard is read-only for task execution. You can still leave comments in the activity feed.
      </p>
      <div className="pt-2 border-t border-[var(--border)]">
        <p className="text-[10px] uppercase tracking-wider text-[var(--text-faint)]">Current status</p>
        <p className="text-sm text-[var(--text-muted)]">{status.replaceAll("_", " ")}</p>
        {closed && (
          <p className="text-xs text-[var(--text-faint)] mt-1">This task is already closed.</p>
        )}
      </div>
    </section>
  );
}
