"use client";

import Link from "next/link";
import type { Task } from "@/lib/api";

const priorityDots: Record<string, string> = {
  emergency: "bg-red-500",
  urgent: "bg-amber-500",
  standard: "bg-blue-500",
  low: "bg-[var(--text-faint)]",
};

export function CalendarView({ tasks }: { tasks: Task[] }) {
  const today = new Date();
  const startOfWeek = new Date(today);
  startOfWeek.setDate(today.getDate() - today.getDay() + 1); // Monday

  // Generate 4 weeks
  const weeks: Date[][] = [];
  const cursor = new Date(startOfWeek);
  for (let w = 0; w < 4; w++) {
    const week: Date[] = [];
    for (let d = 0; d < 7; d++) {
      week.push(new Date(cursor));
      cursor.setDate(cursor.getDate() + 1);
    }
    weeks.push(week);
  }

  function getTasksForDate(date: Date): Task[] {
    const dateStr = date.toISOString().split("T")[0];
    return tasks.filter(t => t.deadline && t.deadline.startsWith(dateStr));
  }

  const dayNames = ["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"];
  const todayStr = today.toISOString().split("T")[0];

  return (
    <div className="bg-[var(--surface)] border border-[var(--border)] rounded-lg shadow-sm overflow-hidden">
      {/* Header */}
      <div className="grid grid-cols-7 border-b border-[var(--border)]">
        {dayNames.map(d => (
          <div key={d} className="px-2 py-2 text-[10px] uppercase tracking-wider text-[var(--text-faint)] font-medium text-center">
            {d}
          </div>
        ))}
      </div>

      {/* Weeks */}
      {weeks.map((week, wi) => (
        <div key={wi} className="grid grid-cols-7 border-b border-[var(--border)]/50 last:border-0">
          {week.map((date, di) => {
            const dateStr = date.toISOString().split("T")[0];
            const isToday = dateStr === todayStr;
            const isWeekend = di >= 5;
            const dayTasks = getTasksForDate(date);

            return (
              <div
                key={di}
                className={`min-h-[90px] p-1.5 border-r border-[var(--border)]/30 last:border-0 ${
                  isWeekend ? "bg-[var(--bg-tertiary)]/50" : ""
                }`}
              >
                <div className={`text-xs mb-1 ${
                  isToday
                    ? "w-5 h-5 rounded-full bg-[var(--accent)] text-white flex items-center justify-center font-medium"
                    : "text-[var(--text-faint)]"
                }`}>
                  {date.getDate()}
                  {date.getDate() === 1 && !isToday && (
                    <span className="ml-1 text-[var(--text-faint)]">{date.toLocaleDateString("en", { month: "short" })}</span>
                  )}
                </div>
                <div className="space-y-0.5">
                  {dayTasks.slice(0, 3).map(task => (
                    <Link key={task.id} href={`/tasks/${task.id}`}>
                      <div className="flex items-center gap-1 px-1 py-0.5 rounded text-[10px] hover:bg-[var(--bg-tertiary)] transition-colors truncate cursor-pointer">
                        <span className={`w-1.5 h-1.5 rounded-full flex-shrink-0 ${priorityDots[task.priority]}`} />
                        <span className="truncate">{task.title.length > 25 ? task.title.slice(0, 25) + "..." : task.title}</span>
                      </div>
                    </Link>
                  ))}
                  {dayTasks.length > 3 && (
                    <p className="text-[9px] text-[var(--text-faint)] px-1">+{dayTasks.length - 3} more</p>
                  )}
                </div>
              </div>
            );
          })}
        </div>
      ))}
    </div>
  );
}
