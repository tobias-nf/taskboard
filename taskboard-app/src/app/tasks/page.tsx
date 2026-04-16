"use client";

import { useCallback } from "react";
import { TaskListPage } from "@/components/task-list-page";
import * as api from "@/lib/api";

export default function AllTasksPage() {
  const fetchTasks = useCallback((params?: api.TaskListParams) => api.getVisibleTasks(params), []);
  return <TaskListPage title="All Tasks" emptyMessage="No visible tasks" fetchTasks={fetchTasks} />;
}
