import type { Metadata } from "next";
import "./globals.css";
import { TaskboardProvider } from "@/lib/taskboard-context";
import { AppShell } from "@/components/app-shell";

export const metadata: Metadata = {
  title: "Taskboard — Agent Task Management",
  description: "Central control plane for the agent ecosystem",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en">
      <body className="bg-[var(--bg)] text-[var(--text)] font-sans antialiased">
        <TaskboardProvider>
          <AppShell>
            {children}
          </AppShell>
        </TaskboardProvider>
      </body>
    </html>
  );
}
