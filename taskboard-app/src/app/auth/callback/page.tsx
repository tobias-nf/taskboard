"use client";

import { Suspense, useEffect, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { useTaskboard } from "@/lib/taskboard-context";

function AuthCallbackInner() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const { handleAuthCallback } = useTaskboard();
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const token = searchParams.get("token");
    const authError = searchParams.get("auth_error");

    if (authError) {
      const messages: Record<string, string> = {
        domain_not_allowed: "Only @near.foundation accounts are allowed.",
        email_not_verified: "Your email address is not verified with Google.",
        account_inactive: "Your account has been deactivated. Contact an admin.",
      };
      setError(messages[authError] || `Authentication failed: ${authError}`);
      return;
    }

    if (!token) {
      setError("No authentication token received.");
      return;
    }

    handleAuthCallback(token).then((ok) => {
      if (ok) {
        router.replace("/");
      } else {
        setError("Failed to validate session. The token may be expired.");
      }
    });
  }, [searchParams, handleAuthCallback, router]);

  if (error) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-[var(--bg)]">
        <div className="w-full max-w-sm text-center">
          <h1 className="text-xl font-semibold mb-2">Sign-in Failed</h1>
          <p className="text-sm text-red-500 mb-4">{error}</p>
          <a
            href="/"
            className="text-sm text-[var(--accent)] hover:underline"
          >
            Back to login
          </a>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-[var(--bg)]">
      <div className="text-sm text-[var(--text-muted)]">Signing in...</div>
    </div>
  );
}

export default function AuthCallback() {
  return (
    <Suspense fallback={
      <div className="min-h-screen flex items-center justify-center bg-[var(--bg)]">
        <div className="text-sm text-[var(--text-muted)]">Signing in...</div>
      </div>
    }>
      <AuthCallbackInner />
    </Suspense>
  );
}
