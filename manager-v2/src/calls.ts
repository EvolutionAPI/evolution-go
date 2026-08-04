import { useCallback, useEffect, useMemo, useState } from "react";
import type { EvolutionApi, EvolutionCall, CallStatusSnapshot } from "./api";

const POLL_INTERVAL_MS = 1800;

export interface CallDeskState {
  snapshot: CallStatusSnapshot;
  selectedCall: EvolutionCall | null;
  selectedCallId: string;
  loading: boolean;
  error: string;
  setSelectedCallId: (callId: string) => void;
  refresh: (quiet?: boolean) => Promise<void>;
  start: (number: string) => Promise<EvolutionCall>;
  accept: (call: EvolutionCall) => Promise<void>;
  reject: (call: EvolutionCall) => Promise<void>;
  terminate: (call: EvolutionCall) => Promise<void>;
}

const EMPTY_SNAPSHOT: CallStatusSnapshot = { connected: false, calls: [] };

export function useCallDesk(api: EvolutionApi | null): CallDeskState {
  const [snapshot, setSnapshot] = useState<CallStatusSnapshot>(EMPTY_SNAPSHOT);
  const [selectedCallId, setSelectedCallId] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  const chooseCall = useCallback((next: CallStatusSnapshot, currentId: string): string => {
    if (currentId && next.calls.some((call) => call.id === currentId)) return currentId;
    const live = [...next.calls].reverse().find((call) => !["ended", "failed"].includes(call.state));
    return live?.id || next.calls.at(-1)?.id || "";
  }, []);

  const refresh = useCallback(async (quiet = false) => {
    if (!api) {
      setSnapshot(EMPTY_SNAPSHOT);
      return;
    }
    if (!quiet) setLoading(true);
    try {
      const next = await api.callStatus();
      next.calls = Array.isArray(next.calls) ? next.calls : [];
      setSnapshot(next);
      setSelectedCallId((current) => chooseCall(next, current));
      setError("");
    } catch (cause) {
      if (!quiet) setError(cause instanceof Error ? cause.message : "Falha ao consultar chamadas");
    } finally {
      if (!quiet) setLoading(false);
    }
  }, [api, chooseCall]);

  useEffect(() => {
    void refresh(false);
    if (!api) return;
    const timer = window.setInterval(() => void refresh(true), POLL_INTERVAL_MS);
    return () => window.clearInterval(timer);
  }, [api, refresh]);

  const run = useCallback(async <T,>(operation: () => Promise<T>): Promise<T> => {
    setLoading(true);
    setError("");
    try {
      return await operation();
    } catch (cause) {
      const message = cause instanceof Error ? cause.message : "Operação não concluída";
      setError(message);
      throw cause;
    } finally {
      setLoading(false);
    }
  }, []);

  const start = useCallback(async (number: string) => {
    if (!api) throw new Error("Configure a conexão da instância");
    const call = await run(() => api.startCall(number));
    setSelectedCallId(call.id);
    await refresh(true);
    return call;
  }, [api, refresh, run]);

  const accept = useCallback(async (call: EvolutionCall) => {
    if (!api) throw new Error("Configure a conexão da instância");
    await run(() => api.acceptCall(call.id));
    await refresh(true);
  }, [api, refresh, run]);

  const reject = useCallback(async (call: EvolutionCall) => {
    if (!api) throw new Error("Configure a conexão da instância");
    await run(() => api.rejectCall(call));
    await refresh(true);
  }, [api, refresh, run]);

  const terminate = useCallback(async (call: EvolutionCall) => {
    if (!api) throw new Error("Configure a conexão da instância");
    await run(() => api.terminateCall(call.id));
    await refresh(true);
  }, [api, refresh, run]);

  const selectedCall = useMemo(
    () => snapshot.calls.find((call) => call.id === selectedCallId) ?? null,
    [selectedCallId, snapshot.calls],
  );

  return {
    snapshot,
    selectedCall,
    selectedCallId,
    loading,
    error,
    setSelectedCallId,
    refresh,
    start,
    accept,
    reject,
    terminate,
  };
}
