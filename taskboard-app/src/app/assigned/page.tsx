"use client";

import { useCallback } from "react";
import { TaskListPage } from "@/components/task-list-page";
import * as api from "@/lib/api";

export default function AssignedPage() {
  const fetchTasks = useCallback((params?: api.TaskListParams) => api.getMyTasks(params), []);
  return <TaskListPage title="My Tasks" emptyMessage="No tasks assigned to you" fetchTasks={fetchTasks} />;
}
