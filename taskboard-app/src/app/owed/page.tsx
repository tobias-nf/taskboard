"use client";

import { useCallback } from "react";
import { TaskListPage } from "@/components/task-list-page";
import * as api from "@/lib/api";

export default function OwedPage() {
  const fetchTasks = useCallback((params?: api.TaskListParams) => api.getTasksOwedToMe(params), []);
  return <TaskListPage title="Owed to Me" emptyMessage="No tasks owed to you" fetchTasks={fetchTasks} />;
}
