import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import type { EvolutionApi, EvolutionCall, CallStatusSnapshot } from "./api";

const FALLBACK_POLL_INTERVAL_MS = 1800;
const HEALTH_POLL_INTERVAL_MS = 15000;
const REALTIME_REFRESH_DELAY_MS = 80;
const OUTCOME_VISIBLE_MS = 12000;

export type CallRealtimeState = "connecting" | "connected" | "fallback";

export interface CallDeskState {
	snapshot: CallStatusSnapshot;
	selectedCall: EvolutionCall | null;
	selectedCallId: string;
	loading: boolean;
	error: string;
	realtime: CallRealtimeState;
	recentOutcome: EvolutionCall | null;
	setSelectedCallId: (callId: string) => void;
	dismissOutcome: () => void;
	refresh: (quiet?: boolean) => Promise<void>;
	start: (number: string) => Promise<EvolutionCall>;
	accept: (call: EvolutionCall) => Promise<void>;
	reject: (call: EvolutionCall) => Promise<void>;
	terminate: (call: EvolutionCall) => Promise<void>;
}

type ManagerCallEventType = "call.offer" | "call.accept" | "call.terminate" | "call.updated";

interface ManagerCallEvent {
	type: ManagerCallEventType;
	instanceId: string;
	call: EvolutionCall;
	occurredAt: string;
}

const EMPTY_SNAPSHOT: CallStatusSnapshot = { connected: false, calls: [] };

function isTerminal(call: EvolutionCall | undefined): boolean {
	return call?.state === "ended" || call?.state === "failed";
}

function normalizeSnapshot(snapshot: CallStatusSnapshot): CallStatusSnapshot {
	return { ...snapshot, calls: Array.isArray(snapshot.calls) ? snapshot.calls : [] };
}

function isCallEvent(value: unknown): value is ManagerCallEvent {
	if (!value || typeof value !== "object") return false;
	const event = value as Partial<ManagerCallEvent>;
	const call = event.call as Partial<EvolutionCall> | undefined;
	return ["call.offer", "call.accept", "call.terminate", "call.updated"].includes(event.type || "")
		&& typeof event.instanceId === "string"
		&& Boolean(call)
		&& typeof call?.id === "string"
		&& typeof call?.direction === "string"
		&& typeof call?.state === "string";
}

export function useCallDesk(api: EvolutionApi | null): CallDeskState {
	const [snapshot, setSnapshot] = useState<CallStatusSnapshot>(EMPTY_SNAPSHOT);
	const [selectedCallId, setSelectedCallId] = useState("");
	const [loading, setLoading] = useState(false);
	const [error, setError] = useState("");
	const [realtime, setRealtime] = useState<CallRealtimeState>("fallback");
	const [recentOutcome, setRecentOutcome] = useState<EvolutionCall | null>(null);
	const snapshotRef = useRef<CallStatusSnapshot>(EMPTY_SNAPSHOT);
	const updateVersion = useRef(0);
	const refreshTimer = useRef<number | undefined>(undefined);
	const outcomeTimer = useRef<number | undefined>(undefined);

	const chooseCall = useCallback((next: CallStatusSnapshot, currentId: string): string => {
		const current = currentId ? next.calls.find((call) => call.id === currentId) : undefined;
		// Keep the currently handled call visible. New incoming calls remain in
		// the global card and queue instead of unexpectedly replacing an active
		// conversation in the workspace.
		if (current && !isTerminal(current)) return current.id;
		const active = [...next.calls].reverse().find((call) => call.state === "active" || call.state === "connecting");
		if (active) return active.id;
		const incoming = [...next.calls].reverse().find((call) => call.direction === "incoming" && call.state === "ringing");
		if (incoming) return incoming.id;
		const live = [...next.calls].reverse().find((call) => !isTerminal(call));
		return live?.id || next.calls.at(-1)?.id || "";
	}, []);

	const dismissOutcome = useCallback(() => {
		if (outcomeTimer.current !== undefined) window.clearTimeout(outcomeTimer.current);
		outcomeTimer.current = undefined;
		setRecentOutcome(null);
	}, []);

	const showOutcome = useCallback((call: EvolutionCall) => {
		if (outcomeTimer.current !== undefined) window.clearTimeout(outcomeTimer.current);
		setRecentOutcome(call);
		outcomeTimer.current = window.setTimeout(() => {
			outcomeTimer.current = undefined;
			setRecentOutcome(null);
		}, OUTCOME_VISIBLE_MS);
	}, []);

	const applySnapshot = useCallback((value: CallStatusSnapshot) => {
		const next = normalizeSnapshot(value);
		const previous = snapshotRef.current;
		const outcome = next.calls.find((call) => {
			const before = previous.calls.find((previousCall) => previousCall.id === call.id);
			return call.direction === "incoming"
				&& before?.state === "ringing"
				&& isTerminal(call);
		});
		snapshotRef.current = next;
		setSnapshot(next);
		setSelectedCallId((current) => chooseCall(next, current));
		if (outcome) showOutcome(outcome);
	}, [chooseCall, showOutcome]);

	const refresh = useCallback(async (quiet = false) => {
		if (!api) {
			snapshotRef.current = EMPTY_SNAPSHOT;
			setSnapshot(EMPTY_SNAPSHOT);
			return;
		}
		const versionAtStart = updateVersion.current;
		if (!quiet) setLoading(true);
		try {
			const next = await api.callStatus();
			// Do not let an older HTTP response overwrite an event that was just
			// delivered over the authenticated Manager WebSocket.
			if (versionAtStart !== updateVersion.current) return;
			applySnapshot(next);
			setError("");
		} catch (cause) {
			if (!quiet) setError(cause instanceof Error ? cause.message : "Falha ao consultar chamadas");
		} finally {
			if (!quiet) setLoading(false);
		}
	}, [api, applySnapshot]);

	const applyCallEvent = useCallback((event: ManagerCallEvent) => {
		const { call } = event;
		updateVersion.current += 1;
		const current = snapshotRef.current;
		const calls = [...current.calls];
		const index = calls.findIndex((item) => item.id === call.id);
		if (index >= 0) {
			calls[index] = { ...calls[index], ...call };
		} else {
			calls.push(call);
		}
		applySnapshot({ ...current, instanceId: event.instanceId, calls });
		setError("");
	}, [applySnapshot]);

	const scheduleRefresh = useCallback(() => {
		if (refreshTimer.current !== undefined) window.clearTimeout(refreshTimer.current);
		refreshTimer.current = window.setTimeout(() => {
			refreshTimer.current = undefined;
			void refresh(true);
		}, REALTIME_REFRESH_DELAY_MS);
	}, [refresh]);

	useEffect(() => () => {
		if (refreshTimer.current !== undefined) window.clearTimeout(refreshTimer.current);
		if (outcomeTimer.current !== undefined) window.clearTimeout(outcomeTimer.current);
	}, []);

	useEffect(() => {
		updateVersion.current += 1;
		snapshotRef.current = EMPTY_SNAPSHOT;
		setSnapshot(EMPTY_SNAPSHOT);
		setSelectedCallId("");
		dismissOutcome();
		setRealtime(api ? "connecting" : "fallback");
		void refresh(false);
	}, [api, dismissOutcome, refresh]);

	useEffect(() => {
		if (!api) return;
		const interval = realtime === "connected" ? HEALTH_POLL_INTERVAL_MS : FALLBACK_POLL_INTERVAL_MS;
		const timer = window.setInterval(() => void refresh(true), interval);
		return () => window.clearInterval(timer);
	}, [api, realtime, refresh]);

	useEffect(() => {
		if (!api) return;
		const url = api.callEventsUrl();
		if (!url) {
			setRealtime("fallback");
			return;
		}
		const expectedInstanceId = new URL(url).searchParams.get("instanceId");

		let disposed = false;
		let socket: WebSocket | null = null;
		let retryTimer: number | undefined;
		let attempts = 0;

		const scheduleReconnect = () => {
			if (disposed) return;
			setRealtime("fallback");
			const delay = Math.min(30000, 1000 * (2 ** Math.min(attempts, 5)));
			attempts += 1;
			retryTimer = window.setTimeout(connect, delay);
		};

		const connect = () => {
			if (disposed) return;
			setRealtime("connecting");
			try {
				const currentSocket = new WebSocket(url);
				socket = currentSocket;
				currentSocket.onopen = () => {
					attempts = 0;
					setRealtime("connected");
					void refresh(true);
				};
				currentSocket.onmessage = (message) => {
					try {
						const event = JSON.parse(String(message.data)) as unknown;
						if (!isCallEvent(event) || event.instanceId !== expectedInstanceId) return;
						applyCallEvent(event);
						scheduleRefresh();
					} catch {
						// Polling remains the recovery source for malformed event frames.
					}
				};
				currentSocket.onerror = () => currentSocket.close();
				currentSocket.onclose = () => scheduleReconnect();
			} catch {
				scheduleReconnect();
			}
		};

		connect();
		return () => {
			disposed = true;
			if (retryTimer !== undefined) window.clearTimeout(retryTimer);
			socket?.close(1000, "Manager call monitor stopped");
		};
	}, [api, applyCallEvent, refresh, scheduleRefresh]);

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
		realtime,
		recentOutcome,
		setSelectedCallId,
		dismissOutcome,
		refresh,
		start,
		accept,
		reject,
		terminate,
	};
}
