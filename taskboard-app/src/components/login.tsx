"use client";

import { AUTH_BASE } from "@/lib/api";

const DEV_MODE = process.env.NEXT_PUBLIC_DEV_MODE === "true";

export function LoginScreen() {
  function handleGoogleLogin() {
    window.location.href = `${AUTH_BASE}/google`;
  }

  function handleDevLogin(email: string) {
    window.location.href = `${AUTH_BASE}/dev-login?email=${encodeURIComponent(email)}`;
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-[var(--bg)]">
      <div className="w-full max-w-sm">
        <div className="text-center mb-8">
          <h1 className="text-2xl font-semibold">Taskboard</h1>
          <p className="text-sm text-[var(--text-muted)] mt-1">Agent Task Management</p>
        </div>

        <div className="bg-[var(--surface)] border border-[var(--border)] rounded-lg p-6 shadow-sm">
          <button
            onClick={handleGoogleLogin}
            className="w-full flex items-center justify-center gap-3 px-4 py-2.5 bg-white border border-gray-300 rounded-md text-sm font-medium text-gray-700 hover:bg-gray-50 transition-colors"
          >
            <svg className="w-5 h-5" viewBox="0 0 24 24">
              <path fill="#4285F4" d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92a5.06 5.06 0 0 1-2.2 3.32v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.1z" />
              <path fill="#34A853" d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z" />
              <path fill="#FBBC05" d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z" />
              <path fill="#EA4335" d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z" />
            </svg>
            Sign in with Google
          </button>

          <p className="text-xs text-[var(--text-faint)] text-center mt-4">
            Restricted to @near.foundation accounts
          </p>
        </div>

        {DEV_MODE && (
          <div className="mt-4 bg-[var(--surface)] border border-dashed border-[var(--border)] rounded-lg p-4">
            <p className="text-xs text-[var(--text-muted)] mb-3 font-medium">Dev Login</p>
            <div className="space-y-2">
              {["alice@near.foundation", "bob@near.foundation", "tobias.holenstein@near.foundation", "admin@near.foundation"].map((email) => (
                <button
                  key={email}
                  onClick={() => handleDevLogin(email)}
                  className="w-full text-left px-3 py-1.5 text-xs font-mono text-[var(--text-muted)] hover:text-[var(--text)] hover:bg-[var(--bg-tertiary)] rounded transition-colors"
                >
                  {email}
                </button>
              ))}
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
