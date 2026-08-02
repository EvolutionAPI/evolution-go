import { useEffect, useMemo, useRef, useState } from "react";
import {
  EvolutionApi,
  displayPhone,
  loadConnection,
  normalizeBaseUrl,
  normalizePhone,
  saveConnection,
  type EvolutionCall,
  type EvolutionConnection,
} from "./api";
import { useCallDesk } from "./calls";
import { ContactsWorkspace, MessagingWorkspace } from "./messaging";
import { EvolutionPcmBridge, type MediaStats, type MediaStatus } from "./pcm";

type View = "overview" | "calls" | "instances" | "messages" | "contacts" | "settings";

interface LogEntry {
  id: number;
  timestamp: string;
  message: string;
  details?: string;
}

const NAV_ITEMS: Array<{ id: View; icon: string; label: string }> = [
  { id: "overview", icon: "⌂", label: "Visão geral" },
  { id: "calls", icon: "☎", label: "Chamadas" },
  { id: "instances", icon: "◫", label: "Instâncias" },
  { id: "messages", icon: "✉", label: "Conversas" },
  { id: "contacts", icon: "◎", label: "Contatos" },
  { id: "settings", icon: "⚙", label: "Configurações" },
];

function stateLabel(state: EvolutionCall["state"]): string {
  return {
    idle: "Inativa",
    ringing: "Chamando",
    connecting: "Conectando",
    active: "Ativa",
    ended: "Encerrada",
    failed: "Falhou",
  }[state] || state;
}

function ConnectionEditor({
  value,
  onSave,
}: {
  value: EvolutionConnection;
  onSave: (connection: EvolutionConnection) => void;
}) {
  const [draft, setDraft] = useState(value);
  useEffect(() => setDraft(value), [value]);
  return (
    <section className="card connection-card">
      <div className="section-heading">
        <div>
          <span className="eyebrow">Conexão</span>
          <h2>Vincular uma instância</h2>
        </div>
        <span className={`status-dot ${draft.apiKey ? "online" : "offline"}`} />
      </div>
      <div className="form-grid">
        <label>
          <span>URL da API</span>
          <input
            value={draft.baseUrl}
            onChange={(event) => setDraft({ ...draft, baseUrl: event.target.value })}
            placeholder="https://evolution.exemplo.com"
          />
        </label>
        <label>
          <span>API key da instância</span>
          <input
            type="password"
            autoComplete="off"
            value={draft.apiKey}
            onChange={(event) => setDraft({ ...draft, apiKey: event.target.value })}
            placeholder="••••••••••••••••"
          />
        </label>
      </div>
      <label className="check-row">
        <input
          type="checkbox"
          checked={draft.remember}
          onChange={(event) => setDraft({ ...draft, remember: event.target.checked })}
        />
        <span>Salvar a chave neste navegador</span>
      </label>
      <div className="button-row">
        <button
          className="button primary"
          onClick={() => onSave({
            ...draft,
            baseUrl: normalizeBaseUrl(draft.baseUrl),
            apiKey: draft.apiKey.trim(),
          })}
        >
          Salvar conexão
        </button>
      </div>
    </section>
  );
}

function EmptyModule({ title, description }: { title: string; description: string }) {
  return (
    <section className="empty-module card">
      <div className="empty-icon">◇</div>
      <h2>{title}</h2>
      <p>{description}</p>
      <span>Estrutura preparada para a próxima fase do Manager V2.</span>
    </section>
  );
}

function CallWorkspace({ api }: { api: EvolutionApi | null }) {
  const desk = useCallDesk(api);
  const [number, setNumber] = useState("");
  const [mediaStatus, setMediaStatus] = useState<MediaStatus>("idle");
  const [stats, setStats] = useState<MediaStats>({ sent: 0, received: 0, dropped: 0 });
  const [muted, setMuted] = useState(false);
  const [logs, setLogs] = useState<LogEntry[]>([]);
  const [autoConnectId, setAutoConnectId] = useState("");
  const bridgeRef = useRef<EvolutionPcmBridge | null>(null);

  const log = (message: string, details?: unknown) => {
    setLogs((current) => [...current.slice(-119), {
      id: Date.now() + Math.random(),
      timestamp: new Date().toLocaleTimeString(),
      message,
      details: details === undefined ? undefined : typeof details === "string" ? details : JSON.stringify(details),
    }]);
  };

  useEffect(() => {
    if (!api) {
      void bridgeRef.current?.disconnect(false);
      bridgeRef.current = null;
      return;
    }
    const bridge = new EvolutionPcmBridge(api, {
      onStatus: setMediaStatus,
      onStats: setStats,
      onLog: log,
    });
    bridgeRef.current = bridge;
    return () => {
      void bridge.disconnect();
      bridgeRef.current = null;
    };
  }, [api]);

  useEffect(() => {
    const selected = desk.selectedCall;
    const activeMediaCall = bridgeRef.current?.activeCallId;
    if (activeMediaCall) {
      const current = desk.snapshot.calls.find((call) => call.id === activeMediaCall);
      if (!current || ["ended", "failed"].includes(current.state)) {
        void bridgeRef.current?.disconnect(false);
      }
    }
    if (selected?.id === autoConnectId && selected.state === "active" && mediaStatus === "idle") {
      setAutoConnectId("");
      void bridgeRef.current?.connect(selected.id).catch((cause) => {
        log("Conexão automática do áudio falhou", cause instanceof Error ? cause.message : cause);
      });
    }
  }, [autoConnectId, desk.selectedCall, desk.snapshot.calls, mediaStatus]);

  const selected = desk.selectedCall;
  const incoming = desk.snapshot.calls.filter((call) => call.direction === "incoming" && call.state === "ringing").length;
  const live = desk.snapshot.calls.filter((call) => !["ended", "failed"].includes(call.state)).length;

  const beginCall = async () => {
    const normalized = normalizePhone(number);
    if (normalized.length < 8 || normalized.length > 20) {
      log("Número inválido", "Informe o número completo com DDI");
      return;
    }
    try {
      const call = await desk.start(normalized);
      setNumber(normalized);
      setAutoConnectId(call.id);
      log("Chamada iniciada", { callId: call.id, peer: call.peer });
    } catch {
      // The hook exposes the error in the workspace.
    }
  };

  const connectAudio = async () => {
    if (!selected || selected.state !== "active") return;
    try {
      await bridgeRef.current?.connect(selected.id);
    } catch (cause) {
      log("Falha ao conectar áudio", cause instanceof Error ? cause.message : cause);
    }
  };

  const terminate = async (call: EvolutionCall) => {
    if (bridgeRef.current?.activeCallId === call.id) await bridgeRef.current.disconnect();
    await desk.terminate(call).catch(() => undefined);
    log("Chamada encerrada", call.id);
  };

  return (
    <div className="call-layout">
      <div className="call-main">
        <section className="hero card">
          <div>
            <span className="eyebrow">Central de voz</span>
            <h1>Chamadas WhatsApp no navegador</h1>
            <p>Discagem, chamadas recebidas e mídia WebRTC em uma única área de trabalho.</p>
          </div>
          <div className="hero-metrics">
            <div><strong>{live}</strong><span>em andamento</span></div>
            <div><strong>{incoming}</strong><span>tocando</span></div>
            <div><strong>{stats.received}</strong><span>frames recebidos</span></div>
          </div>
        </section>

        <section className="card dialer-card">
          <div className="section-heading">
            <div>
              <span className="eyebrow">Nova chamada</span>
              <h2>Discador</h2>
            </div>
            <span className={`connection-pill ${desk.snapshot.connected ? "connected" : "disconnected"}`}>
              {desk.snapshot.connected ? "WhatsApp conectado" : "WhatsApp desconectado"}
            </span>
          </div>
          <div className="dial-row">
            <div className="country-code">+55</div>
            <input
              value={number}
              inputMode="numeric"
              placeholder="62 98612-4920"
              onChange={(event) => setNumber(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === "Enter") void beginCall();
              }}
            />
            <button className="button call-button" disabled={!api || desk.loading} onClick={() => void beginCall()}>
              <span>☎</span> Ligar
            </button>
          </div>
          {desk.error && <div className="alert error">{desk.error}</div>}
        </section>

        {selected ? (
          <section className="card active-call-card">
            <div className="call-avatar">{displayPhone(selected.peer).slice(-2)}</div>
            <div className="active-call-content">
              <span className="eyebrow">{selected.direction === "incoming" ? "Chamada recebida" : "Chamada realizada"}</span>
              <h2>{displayPhone(selected.peer)}</h2>
              <div className="call-meta">
                <span className={`state-badge state-${selected.state}`}>{stateLabel(selected.state)}</span>
                <span>ID {selected.id}</span>
              </div>
              <div className="media-strip">
                <span>Áudio: {mediaStatus}</span>
                <span>↑ {stats.sent}</span>
                <span>↓ {stats.received}</span>
                <span>Descartados {stats.dropped}</span>
              </div>
            </div>
            <div className="call-actions">
              {selected.direction === "incoming" && selected.state === "ringing" && (
                <>
                  <button className="round-action accept" title="Atender" onClick={() => {
                    setAutoConnectId(selected.id);
                    void desk.accept(selected).then(() => log("Chamada aceita", selected.id)).catch(() => undefined);
                  }}>✓</button>
                  <button className="round-action danger" title="Recusar" onClick={() => void desk.reject(selected).then(() => log("Chamada recusada", selected.id)).catch(() => undefined)}>×</button>
                </>
              )}
              {selected.state === "active" && mediaStatus === "idle" && (
                <button className="button secondary" onClick={() => void connectAudio()}>Conectar áudio</button>
              )}
              {mediaStatus === "connected" && (
                <button className="button secondary" onClick={() => {
                  const next = !muted;
                  setMuted(next);
                  bridgeRef.current?.setMuted(next);
                }}>{muted ? "Ativar microfone" : "Silenciar"}</button>
              )}
              {!(["ended", "failed"] as string[]).includes(selected.state) && (
                <button className="round-action danger" title="Encerrar" onClick={() => void terminate(selected)}>⌁</button>
              )}
            </div>
          </section>
        ) : (
          <section className="card active-call-card placeholder-call">
            <div className="call-avatar">☎</div>
            <div><span className="eyebrow">Nenhuma chamada selecionada</span><h2>A central está pronta</h2><p>Inicie uma chamada ou aguarde uma ligação recebida.</p></div>
          </section>
        )}

        <section className="card call-history">
          <div className="section-heading">
            <div><span className="eyebrow">Tempo real</span><h2>Chamadas da instância</h2></div>
            <button className="icon-button" onClick={() => void desk.refresh(false)} disabled={desk.loading}>↻</button>
          </div>
          <div className="call-table">
            {desk.snapshot.calls.length === 0 ? (
              <div className="table-empty">Nenhuma chamada registrada nesta sessão.</div>
            ) : [...desk.snapshot.calls].reverse().map((call) => (
              <button
                key={call.id}
                className={`call-row ${desk.selectedCallId === call.id ? "selected" : ""}`}
                onClick={() => desk.setSelectedCallId(call.id)}
              >
                <span className={`direction-icon ${call.direction}`}>{call.direction === "incoming" ? "↙" : "↗"}</span>
                <span className="call-person"><strong>{displayPhone(call.peer)}</strong><small>{call.id}</small></span>
                <span>{call.direction === "incoming" ? "Recebida" : "Realizada"}</span>
                <span className={`state-badge state-${call.state}`}>{stateLabel(call.state)}</span>
              </button>
            ))}
          </div>
        </section>
      </div>

      <aside className="call-aside">
        <section className="card diagnostic-card">
          <div className="section-heading"><div><span className="eyebrow">Operação</span><h2>Diagnóstico</h2></div><span className="live-dot" /></div>
          <div className="log-list">
            {logs.length === 0 ? <p>Nenhum evento local registrado.</p> : [...logs].reverse().map((entry) => (
              <div className="log-entry" key={entry.id}>
                <time>{entry.timestamp}</time>
                <strong>{entry.message}</strong>
                {entry.details && <span>{entry.details}</span>}
              </div>
            ))}
          </div>
        </section>
        <section className="card quality-card">
          <span className="eyebrow">Qualidade da chamada</span>
          <h2>{stats.received > 0 ? "Fluxo bidirecional" : mediaStatus === "connected" ? "Aguardando retorno" : "Sem mídia ativa"}</h2>
          <div className="quality-bars"><i /><i /><i className={stats.received > 0 ? "active" : ""} /><i className={stats.received > 10 ? "active" : ""} /></div>
          <p>Os contadores facilitam separar falhas do navegador, WebRTC, relay e SRTP.</p>
        </section>
      </aside>
    </div>
  );
}

export function App() {
  const [view, setView] = useState<View>("calls");
  const [connection, setConnection] = useState(loadConnection);
  const [messageRecipient, setMessageRecipient] = useState("");
  const api = useMemo(() => connection.apiKey ? new EvolutionApi(connection) : null, [connection]);

  const updateConnection = (next: EvolutionConnection) => {
    saveConnection(next);
    setConnection(next);
    setView("calls");
  };

  return (
    <div className="app-shell">
      <aside className="sidebar">
        <div className="brand"><span className="brand-mark">E</span><div><strong>Evolution GO</strong><small>Manager V2</small></div></div>
        <nav>
          {NAV_ITEMS.map((item) => (
            <button key={item.id} className={view === item.id ? "active" : ""} onClick={() => setView(item.id)}>
              <span>{item.icon}</span>{item.label}
            </button>
          ))}
        </nav>
        <div className="sidebar-foot">
          <span className={`status-dot ${connection.apiKey ? "online" : "offline"}`} />
          <div><strong>{connection.apiKey ? "Instância configurada" : "Sem instância"}</strong><small>{new URL(connection.baseUrl).host}</small></div>
        </div>
      </aside>

      <main>
        <header className="topbar">
          <div><span className="breadcrumb">Manager V2 /</span><strong>{NAV_ITEMS.find((item) => item.id === view)?.label}</strong></div>
          <div className="top-actions">
            <span className={`connection-pill ${connection.apiKey ? "connected" : "disconnected"}`}>{connection.apiKey ? "API configurada" : "Configuração necessária"}</span>
            <button className="profile-button">EO</button>
          </div>
        </header>
        <div className="content">
          {view === "calls" && <CallWorkspace api={api} />}
          {view === "messages" && (
            <MessagingWorkspace
              api={api}
              initialRecipient={messageRecipient}
              onStartCall={() => setView("calls")}
            />
          )}
          {view === "contacts" && (
            <ContactsWorkspace
              api={api}
              onMessage={(recipient) => {
                setMessageRecipient(recipient);
                setView("messages");
              }}
              onCallStarted={() => setView("calls")}
            />
          )}
          {view === "settings" && <ConnectionEditor value={connection} onSave={updateConnection} />}
          {view === "overview" && <EmptyModule title="Visão geral operacional" description="Indicadores de instâncias, mensagens e chamadas serão reunidos aqui." />}
          {view === "instances" && <EmptyModule title="Gerenciamento de instâncias" description="QR Code, conexão, reconexão e configurações avançadas serão migrados na próxima etapa." />}
          {!connection.apiKey && view !== "settings" && (
            <div className="setup-overlay"><ConnectionEditor value={connection} onSave={updateConnection} /></div>
          )}
        </div>
      </main>
    </div>
  );
}
