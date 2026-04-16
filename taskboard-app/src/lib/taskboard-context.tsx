"use client";

import { createContext, useContext, useState, useEffect, useCallback, type ReactNode } from "react";
import * as api from "./api";
import type { Agent } from "./api";

interface TaskboardContextType {
  currentAgent: Agent | null;
  agents: Agent[];
  loading: boolean;
  authenticated: boolean;
  handleAuthCallback: (token: string) => Promise<boolean>;
  logout: () => void;
  refresh: () => Promise<void>;
  getAgentName: (id: string) => string;
}

const TaskboardContext = createContext<TaskboardContextType>({
  currentAgent: null,
  agents: [],
  loading: true,
  authenticated: false,
  handleAuthCallback: async () => false,
  logout: () => {},
  refresh: async () => {},
  getAgentName: (id) => id,
});

export function useTaskboard() {
  return useContext(TaskboardContext);
}

export function TaskboardProvider({ children }: { children: ReactNode }) {
  const [currentAgent, setCurrentAgent] = useState<Agent | null>(null);
  const [agents, setAgents] = useState<Agent[]>([]);
  const [loading, setLoading] = useState(true);
  const [authenticated, setAuthenticated] = useState(false);

  const loadData = useCallback(async () => {
    try {
      const [me, agentsRes] = await Promise.all([
        api.getMe(),
        api.listAgents(),
      ]);
      setCurrentAgent(me);
      setAgents(agentsRes.agents || []);
      setAuthenticated(true);
    } catch {
      setCurrentAgent(null);
      setAuthenticated(false);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (api.hasSession()) {
      loadData();
    } else {
      setLoading(false);
    }
  }, [loadData]);

  const handleAuthCallback = async (token: string): Promise<boolean> => {
    api.setSessionToken(token);
    try {
      await loadData();
      return true;
    } catch {
      api.clearSession();
      setAuthenticated(false);
      return false;
    }
  };

  const logout = () => {
    api.clearSession();
    setCurrentAgent(null);
    setAgents([]);
    setAuthenticated(false);
  };

  const getAgentName = (id: string) => {
    const agent = agents.find(a => a.id === id);
    return agent?.email ?? agent?.id ?? id;
  };

  return (
    <TaskboardContext.Provider value={{
      currentAgent, agents, loading, authenticated,
      handleAuthCallback, logout, refresh: loadData, getAgentName,
    }}>
      {children}
    </TaskboardContext.Provider>
  );
}
