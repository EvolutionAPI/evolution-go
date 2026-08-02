export interface EvolutionConnection {
  baseUrl: string;
  apiKey: string;
  remember: boolean;
}

export type CallDirection = "incoming" | "outgoing";
export type CallState = "idle" | "ringing" | "connecting" | "active" | "ended" | "failed";

export interface EvolutionCall {
  id: string;
  peer: string;
  direction: CallDirection;
  state: CallState;
  video?: boolean;
  endReason?: string;
  createdAt?: string;
  updatedAt?: string;
}

export interface CallStatusSnapshot {
  instanceId?: string;
  connected: boolean;
  calls: EvolutionCall[];
}

export interface WebRTCSessionResponse {
  sessionId: string;
  answer: RTCSessionDescriptionInit;
}

const PERSISTENT_KEY = "evolution.managerV2.connection.v1";
const SESSION_KEY = "evolution.managerV2.connection.session.v1";

export function loadConnection(): EvolutionConnection {
  const parse = (value: string | null): Partial<EvolutionConnection> | null => {
    if (!value) return null;
    try {
      return JSON.parse(value) as Partial<EvolutionConnection>;
    } catch {
      return null;
    }
  };
  const persistent = parse(localStorage.getItem(PERSISTENT_KEY));
  const temporary = parse(sessionStorage.getItem(SESSION_KEY));
  const value = persistent ?? temporary ?? {};
  return {
    baseUrl: normalizeBaseUrl(value.baseUrl || window.location.origin),
    apiKey: value.apiKey || "",
    remember: Boolean(persistent),
  };
}

export function saveConnection(connection: EvolutionConnection): void {
  const normalized = { ...connection, baseUrl: normalizeBaseUrl(connection.baseUrl) };
  if (connection.remember) {
    localStorage.setItem(PERSISTENT_KEY, JSON.stringify(normalized));
    sessionStorage.removeItem(SESSION_KEY);
  } else {
    sessionStorage.setItem(SESSION_KEY, JSON.stringify(normalized));
    localStorage.removeItem(PERSISTENT_KEY);
  }
}

export function normalizeBaseUrl(value: string): string {
  return (value.trim() || window.location.origin).replace(/\/+$/, "");
}

export function normalizePhone(value: string): string {
  return value.replace(/\D/g, "");
}

export function displayPhone(value: string): string {
  return String(value || "").replace(/:\d+@/, "@").split("@")[0] || "Número não identificado";
}

export class EvolutionApi {
  private readonly baseUrl: string;
  private readonly apiKey: string;

  constructor(connection: EvolutionConnection) {
    this.baseUrl = normalizeBaseUrl(connection.baseUrl);
    this.apiKey = connection.apiKey.trim();
  }

  private async request<T>(path: string, init: RequestInit = {}): Promise<T> {
    if (!this.apiKey) throw new Error("Informe a API key da instância");
    const headers = new Headers(init.headers);
    headers.set("apikey", this.apiKey);
    if (init.body !== undefined && !headers.has("Content-Type")) {
      headers.set("Content-Type", "application/json");
    }
    const response = await fetch(`${this.baseUrl}${path}`, { ...init, headers });
    const body = (await response.json().catch(() => ({}))) as Record<string, unknown>;
    if (!response.ok) {
      const message = typeof body.error === "string"
        ? body.error
        : typeof body.message === "string"
          ? body.message
          : `HTTP ${response.status}`;
      throw new Error(message);
    }
    return body as T;
  }

  callStatus(): Promise<CallStatusSnapshot> {
    return this.request<CallStatusSnapshot>("/call/status");
  }

  startCall(number: string): Promise<EvolutionCall> {
    return this.request<EvolutionCall>("/call/start", {
      method: "POST",
      body: JSON.stringify({ number: normalizePhone(number), video: false }),
    });
  }

  acceptCall(callId: string): Promise<unknown> {
    return this.request(`/call/${encodeURIComponent(callId)}/accept`, { method: "POST" });
  }

  rejectCall(call: EvolutionCall): Promise<unknown> {
    return this.request("/call/reject", {
      method: "POST",
      body: JSON.stringify({ number: call.peer, callCreator: call.peer, callId: call.id }),
    });
  }

  terminateCall(callId: string): Promise<unknown> {
    return this.request(`/call/${encodeURIComponent(callId)}`, { method: "DELETE" });
  }

  createWebRTC(callId: string, offer: RTCSessionDescriptionInit): Promise<WebRTCSessionResponse> {
    return this.request<WebRTCSessionResponse>(`/call/${encodeURIComponent(callId)}/webrtc`, {
      method: "POST",
      body: JSON.stringify({ offer }),
    });
  }

  closeWebRTC(callId: string, sessionId: string): Promise<unknown> {
    return this.request(
      `/call/${encodeURIComponent(callId)}/webrtc/${encodeURIComponent(sessionId)}`,
      { method: "DELETE" },
    );
  }
}
