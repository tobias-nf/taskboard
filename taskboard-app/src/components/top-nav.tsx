"use client";

import { useState } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { useTaskboard } from "@/lib/taskboard-context";
import { agentLabel, agentInitials } from "@/lib/api";

const navLinks = [
  { href: "/assigned", label: "My Tasks" },
  { href: "/owed", label: "Owed to Me" },
  { href: "/tasks", label: "All Tasks" },
];

export function TopNav() {
  const pathname = usePathname();
  const { currentAgent, logout } = useTaskboard();
  const [menuOpen, setMenuOpen] = useState(false);

  if (!currentAgent) return null;

  const initials = agentInitials(currentAgent);

  return (
    <>
      <nav
        className="sticky top-0 z-50 flex items-center justify-between px-6 py-3 border-b"
        style={{ background: "var(--bg)", borderColor: "var(--border)" }}
      >
        {/* Left: Logo + Title */}
        <div className="flex items-center gap-3">
          <div
            className="w-6 h-6 rounded-md flex items-center justify-center text-[10px] font-bold"
            style={{ background: "var(--accent)", color: "#111827" }}
          >
            T
          </div>
          <div className="w-px h-5 hidden sm:block" style={{ background: "var(--border)" }} />
          <span
            className="text-sm font-semibold tracking-tight hidden sm:block"
            style={{ color: "var(--text)" }}
          >
            Taskboard
          </span>
        </div>

        {/* Center: Pill nav */}
        <div
          className="hidden md:flex items-center gap-1 rounded-full px-1 py-1"
          style={{ background: "var(--bg-secondary)" }}
        >
          {navLinks.map((link) => {
            const isActive =
              link.href === "/"
                ? pathname === "/"
                : pathname === link.href || pathname.startsWith(link.href + "/");
            return (
              <Link
                key={link.href}
                href={link.href}
                className="px-4 py-1.5 rounded-full text-xs font-medium transition-all whitespace-nowrap"
                style={{
                  background: isActive ? "var(--surface)" : "transparent",
                  color: isActive ? "var(--text)" : "var(--text-muted)",
                  boxShadow: isActive ? "0 1px 2px rgba(0,0,0,0.06)" : "none",
                }}
              >
                {link.label}
              </Link>
            );
          })}
        </div>

        {/* Right: Admin + User */}
        <div className="flex items-center gap-3">
          {currentAgent.type === "admin" && (
            <Link
              href="/admin"
              className="text-xs font-medium transition-colors"
              style={{
                color: pathname.startsWith("/admin")
                  ? "var(--text)"
                  : "var(--text-faint)",
              }}
            >
              Admin
            </Link>
          )}
          <div className="relative">
            <button
              onClick={() => setMenuOpen((prev) => !prev)}
              className="flex items-center gap-2"
            >
              <div
                className="w-7 h-7 rounded-full flex items-center justify-center text-[11px] font-bold"
                style={{ background: "var(--accent)", color: "#111827" }}
              >
                {initials}
              </div>
            </button>

            {menuOpen && (
              <>
                <div
                  className="fixed inset-0 z-40"
                  onClick={() => setMenuOpen(false)}
                />
                <div
                  className="absolute right-0 top-10 z-50 w-56 rounded-xl border shadow-lg p-3"
                  style={{
                    background: "var(--surface)",
                    borderColor: "var(--border)",
                  }}
                >
                  <div
                    className="px-1 pb-2 mb-2 border-b"
                    style={{ borderColor: "var(--border)" }}
                  >
                    <p
                      className="text-xs font-medium truncate"
                      style={{ color: "var(--text)" }}
                    >
                      {agentLabel(currentAgent)}
                    </p>
                    <p
                      className="text-[11px] truncate"
                      style={{ color: "var(--text-faint)" }}
                    >
                      {currentAgent.id}
                    </p>
                    {currentAgent.type === "admin" && (
                      <span
                        className="inline-flex mt-1 px-1.5 py-0.5 rounded text-[9px] font-semibold uppercase"
                        style={{ background: "#ECFDF5", color: "#059669" }}
                      >
                        admin
                      </span>
                    )}
                  </div>
                  <Link
                    href="/settings"
                    onClick={() => setMenuOpen(false)}
                    className="w-full block text-left px-1 py-1.5 rounded-md text-xs transition-colors"
                    style={{ color: "var(--text-muted)" }}
                    onMouseEnter={(e) =>
                      ((e.target as HTMLElement).style.background =
                        "var(--bg-secondary)")
                    }
                    onMouseLeave={(e) =>
                      ((e.target as HTMLElement).style.background =
                        "transparent")
                    }
                  >
                    Settings
                  </Link>
                  <Link
                    href="/agents"
                    onClick={() => setMenuOpen(false)}
                    className="w-full block text-left px-1 py-1.5 rounded-md text-xs transition-colors"
                    style={{ color: "var(--text-muted)" }}
                    onMouseEnter={(e) =>
                      ((e.target as HTMLElement).style.background =
                        "var(--bg-secondary)")
                    }
                    onMouseLeave={(e) =>
                      ((e.target as HTMLElement).style.background =
                        "transparent")
                    }
                  >
                    Agents
                  </Link>
                  <Link
                    href="/audit"
                    onClick={() => setMenuOpen(false)}
                    className="w-full block text-left px-1 py-1.5 rounded-md text-xs transition-colors"
                    style={{ color: "var(--text-muted)" }}
                    onMouseEnter={(e) =>
                      ((e.target as HTMLElement).style.background =
                        "var(--bg-secondary)")
                    }
                    onMouseLeave={(e) =>
                      ((e.target as HTMLElement).style.background =
                        "transparent")
                    }
                  >
                    Audit Log
                  </Link>
                  <div
                    className="my-2 border-t"
                    style={{ borderColor: "var(--border)" }}
                  />
                  <button
                    onClick={() => {
                      logout();
                      setMenuOpen(false);
                    }}
                    className="w-full text-left px-1 py-1.5 rounded-md text-xs transition-colors"
                    style={{ color: "var(--text-muted)" }}
                    onMouseEnter={(e) =>
                      ((e.target as HTMLElement).style.background =
                        "var(--bg-secondary)")
                    }
                    onMouseLeave={(e) =>
                      ((e.target as HTMLElement).style.background =
                        "transparent")
                    }
                  >
                    Sign out
                  </button>
                </div>
              </>
            )}
          </div>
        </div>
      </nav>

      {/* Bottom tab bar — mobile only */}
      <div
        className="fixed bottom-0 left-0 right-0 z-50 md:hidden border-t flex items-stretch justify-around"
        style={{
          background: "var(--bg)",
          borderColor: "var(--border)",
          paddingBottom: "env(safe-area-inset-bottom)",
        }}
      >
        {navLinks.map((link) => {
          const isActive =
            link.href === "/"
              ? pathname === "/"
              : pathname === link.href || pathname.startsWith(link.href + "/");
          return (
            <Link
              key={link.href}
              href={link.href}
              className="flex flex-col items-center justify-center gap-0.5 py-2 flex-1 transition-colors"
              style={{
                color: isActive ? "var(--text)" : "var(--text-faint)",
              }}
            >
              <span className="text-[11px] font-medium">{link.label}</span>
            </Link>
          );
        })}
      </div>
    </>
  );
}
