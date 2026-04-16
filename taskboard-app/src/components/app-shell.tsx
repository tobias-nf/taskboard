"use client";

import { usePathname } from "next/navigation";
import { useTaskboard } from "@/lib/taskboard-context";
import { TopNav } from "./top-nav";
import { LoginScreen } from "./login";
import { OnboardingFlow } from "./onboarding";

// Routes that bypass auth (rendered outside AppShell chrome)
const PUBLIC_ROUTES = ["/auth/callback"];

export function AppShell({ children }: { children: React.ReactNode }) {
  const { loading, authenticated, currentAgent } = useTaskboard();
  const pathname = usePathname();

  // Let public routes (like OAuth callback) render without auth
  if (PUBLIC_ROUTES.some(r => pathname.startsWith(r))) {
    return <>{children}</>;
  }

  if (loading) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-[var(--bg)]">
        <div className="text-sm text-[var(--text-muted)]">Loading...</div>
      </div>
    );
  }

  if (!authenticated) {
    return <LoginScreen />;
  }

  // Show onboarding for user agents without a Slack ID
  if (currentAgent && currentAgent.type === "user" && !currentAgent.slack_id) {
    return <OnboardingFlow />;
  }

  return (
    <div className="min-h-screen" style={{ background: "var(--bg)" }}>
      <TopNav />
      <main className="px-6 py-6 pb-16 md:pb-6">
        {children}
      </main>
    </div>
  );
}
