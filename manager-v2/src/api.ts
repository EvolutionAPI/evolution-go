export interface EvolutionConnection {
  baseUrl: string;
  apiKey: string;
  instanceId: string;
  remember: boolean;
}

export interface ManagedInstance {
  id: string;
  name: string;
  token?: string;
  webhook?: string;
  jid?: string;
  connected: boolean;
  createdAt?: string;
  clientName?: string;
  osName?: string;
  disconnect_reason?: string;
}

export interface AdvancedInstanceSettings {
  alwaysOnline: boolean;
  rejectCall: boolean;
  msgRejectCall: string;
  readMessages: boolean;
  ignoreGroups: boolean;
  ignoreStatus: boolean;
}

export interface CreateInstanceInput {
  name: string;
  token: string;
}

export type ApiAuthMode = "instance" | "admin" | "none";

export interface ApiExecutionRequest {
  method: string;
  path: string;
  auth?: ApiAuthMode;
  body?: BodyInit | null;
  headers?: HeadersInit;
}

export interface ApiExecutionResult {
  ok: boolean;
  status: number;
  statusText: string;
  durationMs: number;
  url: string;
  headers: Record<string, string>;
  data: unknown;
  rawText: string;
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

export interface EvolutionContact {
  Jid: string;
  Found: boolean;
  FirstName: string;
  FullName: string;
  PushName: string;
  BusinessName: string;
}

export interface CheckedUser {
  Query: string;
  IsInWhatsapp: boolean;
  JID: string;
  RemoteJID: string;
  LID?: string | null;
  VerifiedName?: string;
}

export interface MessageSendResult {
  id?: string;
  messageId?: string;
  timestamp?: number | string;
  [key: string]: unknown;
}

interface ApiEnvelope<T> {
  message: string;
  data: T;
}

interface CheckUserCollection {
  Users?: CheckedUser[];
  users?: CheckedUser[];
}

const PERSISTENT_KEY = "evolution.managerV2.connection.v2";
const SESSION_KEY = "evolution.managerV2.connection.session.v2";
const LEGACY_PERSISTENT_KEY = "evolution.managerV2.connection.v1";
const LEGACY_SESSION_KEY = "evolution.managerV2.connection.session.v1";

export function loadConnection(): EvolutionConnection {
  const parse = (value: string | null): Partial<EvolutionConnection> | null => {
    if (!value) return null;
    try {
      return JSON.parse(value) as Partial<EvolutionConnection>;
    } catch {
      return null;
    }
  };
  const persistent = parse(localStorage.getItem(PERSISTENT_KEY)) ?? parse(localStorage.getItem(LEGACY_PERSISTENT_KEY));
  const temporary = parse(sessionStorage.getItem(SESSION_KEY)) ?? parse(sessionStorage.getItem(LEGACY_SESSION_KEY));
  const value = persistent ?? temporary ?? {};
  return {
    baseUrl: normalizeBaseUrl(value.baseUrl || window.location.origin),
    apiKey: value.apiKey || "",
    instanceId: value.instanceId || "",
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
  localStorage.removeItem(LEGACY_PERSISTENT_KEY);
  sessionStorage.removeItem(LEGACY_SESSION_KEY);
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
  private readonly instanceApiKey: string;
  private readonly managerBaseUrl: string;

  constructor(connection: EvolutionConnection) {
    this.baseUrl = normalizeBaseUrl(connection.baseUrl);
    this.instanceApiKey = connection.apiKey.trim();
    this.managerBaseUrl = normalizeBaseUrl(window.location.origin);
  }

  async execute(request: ApiExecutionRequest): Promise<ApiExecutionResult> {
    const path = request.path.startsWith("/") ? request.path : `/${request.path}`;
    const auth = request.auth ?? "instance";
    const url = `${auth === "admin" ? this.managerBaseUrl : this.baseUrl}${path}`;
    const headers = new Headers(request.headers);
    if (auth === "instance") {
      if (!this.instanceApiKey) throw new Error("Informe a API key da instância em Configurações");
      headers.set("apikey", this.instanceApiKey);
    }
    const isFormData = typeof FormData !== "undefined" && request.body instanceof FormData;
    if (request.body !== undefined && request.body !== null && !isFormData && !headers.has("Content-Type")) {
      headers.set("Content-Type", "application/json");
    }

    const startedAt = performance.now();
    const response = await fetch(url, {
      method: request.method.toUpperCase(),
      headers,
      body: ["GET", "HEAD"].includes(request.method.toUpperCase()) ? undefined : request.body,
      credentials: "include",
    });
    const rawText = await response.text();
    let data: unknown = rawText;
    if (rawText) {
      try {
        data = JSON.parse(rawText) as unknown;
      } catch {
        data = rawText;
      }
    } else {
      data = null;
    }
    return {
      ok: response.ok,
      status: response.status,
      statusText: response.statusText,
      durationMs: Math.round(performance.now() - startedAt),
      url,
      headers: Object.fromEntries(response.headers.entries()),
      data,
      rawText,
    };
  }

  private async request<T>(path: string, init: RequestInit = {}): Promise<T> {
    const result = await this.execute({
      path,
      method: init.method || "GET",
      body: init.body,
      headers: init.headers,
      auth: "instance",
    });
    if (!result.ok) {
      const body = result.data as Record<string, unknown> | null;
      const message = body && typeof body === "object" && typeof body.error === "string"
        ? body.error
        : body && typeof body === "object" && typeof body.message === "string"
          ? body.message
          : `HTTP ${result.status}`;
      throw new Error(message);
    }
    return result.data as T;
  }

  private async adminRequest<T>(path: string, init: RequestInit = {}): Promise<T> {
    const result = await this.execute({
      path,
      method: init.method || "GET",
      body: init.body,
      headers: init.headers,
      auth: "admin",
    });
    if (!result.ok) {
      const body = result.data as Record<string, unknown> | null;
      const message = body && typeof body === "object" && typeof body.error === "string"
        ? body.error
        : body && typeof body === "object" && typeof body.message === "string"
          ? body.message
          : `HTTP ${result.status}`;
      throw new Error(message);
    }
    return result.data as T;
  }

  async listInstances(): Promise<ManagedInstance[]> {
    const response = await this.adminRequest<ApiEnvelope<ManagedInstance[]> | ManagedInstance[]>("/instance/all");
    const instances = Array.isArray(response) ? response : response.data;
    return Array.isArray(instances) ? instances : [];
  }

  async createInstance(input: CreateInstanceInput): Promise<ManagedInstance> {
    const response = await this.adminRequest<ApiEnvelope<ManagedInstance>>("/instance/create", {
      method: "POST",
      body: JSON.stringify({
        instanceId: input.name,
        name: input.name,
        token: input.token,
        proxy: null,
        advancedSettings: null,
      }),
    });
    return "data" in response ? response.data : response;
  }

  deleteInstance(instanceId: string): Promise<unknown> {
    return this.adminRequest(`/instance/delete/${encodeURIComponent(instanceId)}`, { method: "DELETE" });
  }

  getAdvancedSettings(instanceId: string): Promise<AdvancedInstanceSettings> {
    return this.request<AdvancedInstanceSettings>(`/instance/${encodeURIComponent(instanceId)}/advanced-settings`);
  }

  updateAdvancedSettings(instanceId: string, settings: AdvancedInstanceSettings): Promise<unknown> {
    return this.request(`/instance/${encodeURIComponent(instanceId)}/advanced-settings`, {
      method: "PUT",
      body: JSON.stringify(settings),
    });
  }

  disconnectInstance(): Promise<unknown> {
    return this.request("/instance/disconnect", { method: "POST", body: JSON.stringify({}) });
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

  async contacts(): Promise<EvolutionContact[]> {
    const response = await this.request<ApiEnvelope<EvolutionContact[]>>("/user/contacts");
    return Array.isArray(response.data) ? response.data : [];
  }

  async checkUser(number: string): Promise<CheckedUser | null> {
    const normalized = normalizePhone(number);
    if (!normalized) return null;
    const response = await this.request<ApiEnvelope<CheckUserCollection>>("/user/check", {
      method: "POST",
      body: JSON.stringify({ number: [normalized], formatJid: false }),
    });
    const users = response.data?.Users ?? response.data?.users ?? [];
    return users.find((user) => user.IsInWhatsapp) ?? users[0] ?? null;
  }

  async sendText(number: string, text: string): Promise<MessageSendResult> {
    const response = await this.request<ApiEnvelope<MessageSendResult>>("/send/text", {
      method: "POST",
      body: JSON.stringify({
        number,
        text,
        delay: 0,
        mentionAll: false,
        mentionedJid: [],
        quoted: { messageId: "", participant: "" },
      }),
    });
    return response.data ?? {};
  }

  async sendMedia(number: string, file: File, caption = ""): Promise<MessageSendResult> {
    const form = new FormData();
    form.set("number", number);
    form.set("type", mediaTypeForFile(file));
    form.set("caption", caption);
    form.set("filename", file.name);
    form.set("delay", "0");
    form.set("mentionAll", "false");
    form.set("file", file, file.name);
    const response = await this.request<ApiEnvelope<MessageSendResult>>("/send/media", {
      method: "POST",
      body: form,
    });
    return response.data ?? {};
  }
}

function mediaTypeForFile(file: File): "image" | "video" | "audio" | "document" {
  if (file.type.startsWith("image/")) return "image";
  if (file.type.startsWith("video/")) return "video";
  if (file.type.startsWith("audio/")) return "audio";
  return "document";
}
