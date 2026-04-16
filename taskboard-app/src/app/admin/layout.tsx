"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useTaskboard } from "@/lib/taskboard-context";

const adminNav = [
  { href: "/admin", label: "Overview", exact: true },
  { href: "/admin/agents", label: "Agents" },
];

export default function AdminLayout({ children }: { children: React.ReactNode }) {
  const { currentAgent } = useTaskboard();
  const pathname = usePathname();

  if (!currentAgent || currentAgent.type !== "admin") {
    return (
      <div className="flex items-center justify-center h-full">
        <div className="text-center">
          <p className="text-sm text-[var(--text-muted)]">Admin access required</p>
          <p className="text-xs text-[var(--text-faint)] mt-1">You must be logged in as an admin agent to access this panel.</p>
        </div>
      </div>
    );
  }

  return (
    <div>
      <div className="mb-6">
        <h2 className="text-xl font-semibold">Admin</h2>
        <p className="text-sm text-[var(--text-muted)] mt-1">System administration and agent management</p>
      </div>

      <nav className="flex gap-1 mb-6 border-b border-[var(--border)]">
        {adminNav.map((item) => {
          const isActive = item.exact
            ? pathname === item.href
            : pathname.startsWith(item.href);
          return (
            <Link
              key={item.href}
              href={item.href}
              className={`px-3 py-2 text-xs font-medium border-b-2 transition-colors -mb-px ${
                isActive
                  ? "border-[var(--accent)] text-[var(--accent)]"
                  : "border-transparent text-[var(--text-muted)] hover:text-[var(--text)] hover:border-[var(--border)]"
              }`}
            >
              {item.label}
            </Link>
          );
        })}
      </nav>

      {children}
    </div>
  );
}
