import { useEffect, useRef, useState } from "react";
import { displayPhone, normalizePhone, type EvolutionApi, type EvolutionCall, type EvolutionConnection, type InstanceLogEntry, type ManagedInstance } from "./api";
import { useCallDesk } from "./calls";
import { EvolutionPcmBridge, type MediaStats, type MediaStatus } from "./pcm";

interface LogEntry {
  id: number;
  timestamp: string;
  message: string;
  details?: string;
}

const CALL_LOG_PATTERN = /\b(call|chamada|ligaç[aã]o|voip|webrtc|audio|áudio|media|mídia|relay|srtp|pcm)\b/i;

function formatLogTimestamp(value: string): string {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : new Intl.DateTimeFormat("pt-BR", { dateStyle: "short", timeStyle: "medium" }).format(date);
}

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

export function CallWorkspace({
  api,
  connection,
  onSelectInstance,
}: {
  api: EvolutionApi | null;
  connection: EvolutionConnection;
  onSelectInstance: (connection: EvolutionConnection) => void;
}) {
  const [instances, setInstances] = useState<ManagedInstance[]>([]);
  const [loadingInstances, setLoadingInstances] = useState(true);
  const [instanceError, setInstanceError] = useState("");
  const callsApi = connection.instanceId.trim() && connection.apiKey.trim() ? api : null;
  const desk = useCallDesk(callsApi);
  const [number, setNumber] = useState("");
  const [mediaStatus, setMediaStatus] = useState<MediaStatus>("idle");
  const [stats, setStats] = useState<MediaStats>({ sent: 0, received: 0, dropped: 0 });
  const [muted, setMuted] = useState(false);
  const [logs, setLogs] = useState<LogEntry[]>([]);
  const [callLogs, setCallLogs] = useState<InstanceLogEntry[]>([]);
  const [loadingCallLogs, setLoadingCallLogs] = useState(false);
  const [callLogsError, setCallLogsError] = useState("");
  const [callLogsRefresh, setCallLogsRefresh] = useState(0);
  const [showHistory, setShowHistory] = useState(false);
  const [showLogs, setShowLogs] = useState(false);
  const [autoConnectId, setAutoConnectId] = useState("");
  const bridgeRef = useRef<EvolutionPcmBridge | null>(null);
  const selectedInstance = instances.find((instance) => instance.id === connection.instanceId) ?? null;

  useEffect(() => {
    let active = true;
    if (!api) {
      setInstances([]);
      setLoadingInstances(false);
      return () => { active = false; };
    }
    setLoadingInstances(true);
    void api.listInstances()
      .then((value) => {
        if (!active) return;
        setInstances(value);
        setInstanceError("");
      })
      .catch((cause) => {
        if (!active) return;
        setInstances([]);
        setInstanceError(cause instanceof Error ? cause.message : "Não foi possível carregar as instâncias.");
      })
      .finally(() => { if (active) setLoadingInstances(false); });
    return () => { active = false; };
  }, [api]);

  useEffect(() => {
    setCallLogs([]);
    setCallLogsError("");
  }, [connection.instanceId]);

  useEffect(() => {
    if (!showLogs) return;
    if (!api || !connection.instanceId) {
      setCallLogs([]);
      setCallLogsError("Selecione uma instância para consultar os logs.");
      return;
    }
    let active = true;
    setLoadingCallLogs(true);
    setCallLogsError("");
    void api.getInstanceLogs(connection.instanceId)
      .then((entries) => {
        if (!active) return;
        setCallLogs(entries.filter((entry) => CALL_LOG_PATTERN.test(entry.message)));
      })
      .catch((cause) => {
        if (active) setCallLogsError(cause instanceof Error ? cause.message : "Não foi possível carregar os logs de chamadas.");
      })
      .finally(() => { if (active) setLoadingCallLogs(false); });
    return () => { active = false; };
  }, [api, connection.instanceId, showLogs, callLogsRefresh]);

  const log = (message: string, details?: unknown) => {
    setLogs((current) => [...current.slice(-119), {
      id: Date.now() + Math.random(),
      timestamp: new Date().toLocaleTimeString(),
      message,
      details: details === undefined ? undefined : typeof details === "string" ? details : JSON.stringify(details),
    }]);
  };

  useEffect(() => {
    if (!callsApi) {
      void bridgeRef.current?.disconnect(false);
      bridgeRef.current = null;
      return;
    }
    const bridge = new EvolutionPcmBridge(callsApi, {
      onStatus: setMediaStatus,
      onStats: setStats,
      onLog: log,
    });
    bridgeRef.current = bridge;
    return () => {
      void bridge.disconnect();
      bridgeRef.current = null;
    };
  }, [callsApi]);

  const chooseInstance = (instanceId: string) => {
    const next = instances.find((instance) => instance.id === instanceId);
    if (!next) {
      onSelectInstance({ ...connection, instanceId: "", apiKey: "" });
      return;
    }
    const token = next.token || (connection.instanceId === next.id ? connection.apiKey : "");
    if (!token) {
      setInstanceError("Não foi possível obter a chave desta instância para as chamadas.");
      return;
    }
    setInstanceError("");
    onSelectInstance({ ...connection, instanceId: next.id, apiKey: token });
  };

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
    <div className="call-layout call-layout-single">
      <div className="call-main">
        <section className="card call-instance-selector">
          <div>
            <span className="eyebrow">Instância de chamadas</span>
            <h2>Escolha a instância ativa</h2>
            <p>As chamadas, o diagnóstico e o áudio desta tela serão executados somente na instância selecionada.</p>
          </div>
          <label>
            <span>Instância</span>
            <select value={connection.instanceId} disabled={loadingInstances || instances.length === 0} onChange={(event) => chooseInstance(event.target.value)}>
              <option value="">{loadingInstances ? "Carregando instâncias…" : "Selecione uma instância"}</option>
              {instances.map((instance) => <option key={instance.id} value={instance.id}>{instance.name || instance.id}{instance.connected ? " — conectada" : " — desconectada"}</option>)}
            </select>
          </label>
          <span className={`connection-pill ${selectedInstance?.connected ? "connected" : "disconnected"}`}>
            {selectedInstance ? (selectedInstance.connected ? "WhatsApp conectado" : "WhatsApp desconectado") : "Nenhuma instância selecionada"}
          </span>
          {instanceError && <div className="alert error" role="alert">{instanceError}</div>}
        </section>

        <section className="hero card">
          <div>
            <span className="eyebrow">Teste especializado</span>
            <h1>Telefonia WhatsApp no navegador</h1>
            <p>Validação de sinalização, WebRTC, microfone, relay e áudio recebido.</p>
          </div>
          <div className="hero-metrics">
            <div><strong>{live}</strong><span>em andamento</span></div>
            <div><strong>{incoming}</strong><span>tocando</span></div>
            <div><strong>{stats.received}</strong><span>frames recebidos</span></div>
          </div>
        </section>

        <section className="card dialer-card">
          <div className="section-heading">
            <div><span className="eyebrow">POST /call/start</span><h2>Discador de teste</h2></div>
            <span className={`connection-pill ${desk.snapshot.connected ? "connected" : "disconnected"}`}>
              {desk.snapshot.connected ? "WhatsApp conectado" : "WhatsApp desconectado"}
            </span>
          </div>
          <div className="dial-row">
            <div className="country-code">DDI</div>
            <input
              value={number}
              inputMode="numeric"
              placeholder="5562986124920"
              onChange={(event) => setNumber(event.target.value)}
              onKeyDown={(event) => { if (event.key === "Enter") void beginCall(); }}
            />
            <button className="button call-button" disabled={!callsApi || desk.loading || !selectedInstance?.connected} onClick={() => void beginCall()}><span>☎</span> Ligar</button>
          </div>
          {desk.error && <div className="alert error">{desk.error}</div>}
        </section>

        {selected ? (
          <section className="card active-call-card">
            <div className="call-avatar">{displayPhone(selected.peer).slice(-2)}</div>
            <div className="active-call-content">
              <span className="eyebrow">{selected.direction === "incoming" ? "Chamada recebida" : "Chamada realizada"}</span>
              <h2>{displayPhone(selected.peer)}</h2>
              <div className="call-meta"><span className={`state-badge state-${selected.state}`}>{stateLabel(selected.state)}</span><span>ID {selected.id}</span></div>
              <div className="media-strip"><span>Áudio: {mediaStatus}</span><span>↑ {stats.sent}</span><span>↓ {stats.received}</span><span>Descartados {stats.dropped}</span></div>
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
              {selected.state === "active" && mediaStatus === "idle" && <button className="button secondary" onClick={() => void connectAudio()}>Conectar áudio</button>}
              {mediaStatus === "connected" && (
                <button className="button secondary" onClick={() => {
                  const next = !muted;
                  setMuted(next);
                  bridgeRef.current?.setMuted(next);
                }}>{muted ? "Ativar microfone" : "Silenciar"}</button>
              )}
              {!(["ended", "failed"] as string[]).includes(selected.state) && <button className="round-action danger" title="Encerrar" onClick={() => void terminate(selected)}>⌁</button>}
            </div>
          </section>
        ) : (
          <section className="card active-call-card placeholder-call"><div className="call-avatar">☎</div><div><span className="eyebrow">Nenhuma chamada selecionada</span><h2>Teste de voz pronto</h2><p>Inicie uma chamada ou aguarde uma ligação recebida.</p></div></section>
        )}

        <section className="card call-utility-card">
          <div>
            <span className="eyebrow">Monitoramento</span>
            <h2>Controle da sessão</h2>
            <p>Áudio: <strong>{mediaStatus}</strong> · {stats.received > 0 ? "Fluxo de mídia recebido" : "Aguardando mídia"}</p>
          </div>
          <div className="call-utility-actions">
            <button type="button" className="button secondary" onClick={() => setShowHistory(true)}>Histórico <span>{desk.snapshot.calls.length}</span></button>
            <button type="button" className="button secondary" onClick={() => setShowLogs(true)}>Logs <span>{callLogs.length || "–"}</span></button>
          </div>
        </section>
      </div>

      {showHistory && (
        <div className="dialog-backdrop" role="presentation">
          <section className="card call-dialog" role="dialog" aria-modal="true" aria-labelledby="call-history-title">
            <div className="section-heading"><div><span className="eyebrow">GET /call/status</span><h2 id="call-history-title">Histórico de chamadas</h2></div><div className="call-dialog-actions"><button type="button" className="icon-button" title="Atualizar histórico" onClick={() => void desk.refresh(false)} disabled={desk.loading}>↻</button><button type="button" className="text-button" onClick={() => setShowHistory(false)}>Fechar</button></div></div>
            <div className="call-table call-dialog-scroll">
              {desk.snapshot.calls.length === 0 ? <div className="table-empty">Nenhuma chamada registrada.</div> : [...desk.snapshot.calls].reverse().map((call) => (
                <button key={call.id} className={`call-row ${desk.selectedCallId === call.id ? "selected" : ""}`} onClick={() => { desk.setSelectedCallId(call.id); setShowHistory(false); }}>
                  <span className={`direction-icon ${call.direction}`}>{call.direction === "incoming" ? "↙" : "↗"}</span>
                  <span className="call-person"><strong>{displayPhone(call.peer)}</strong><small>{call.id}</small></span>
                  <span>{call.direction === "incoming" ? "Recebida" : "Realizada"}</span>
                  <span className={`state-badge state-${call.state}`}>{stateLabel(call.state)}</span>
                </button>
              ))}
            </div>
          </section>
        </div>
      )}

      {showLogs && (
        <div className="dialog-backdrop" role="presentation">
          <section className="card call-dialog" role="dialog" aria-modal="true" aria-labelledby="call-logs-title">
            <div className="section-heading"><div><span className="eyebrow">Logs da instância</span><h2 id="call-logs-title">Logs de chamadas</h2></div><div className="call-dialog-actions"><button type="button" className="icon-button" title="Atualizar logs" onClick={() => setCallLogsRefresh((current) => current + 1)} disabled={loadingCallLogs}>↻</button><button type="button" className="text-button" onClick={() => setShowLogs(false)}>Fechar</button></div></div>
            <p className="call-logs-description">Eventos registrados pelo servidor para a instância selecionada, filtrados por chamadas, WebRTC, áudio e relay.</p>
            <div className="call-server-log-list call-dialog-scroll">
              {loadingCallLogs ? <p>Carregando logs de chamadas…</p> : callLogsError ? <div className="alert error" role="alert">{callLogsError}</div> : callLogs.length === 0 ? <p>Nenhum log de chamada foi encontrado para esta instância.</p> : callLogs.map((entry, index) => (
                <article className={`call-server-log level-${entry.level.toLowerCase()}`} key={`${entry.timestamp}-${index}`}>
                  <div><time>{formatLogTimestamp(entry.timestamp)}</time><span>{entry.level}</span></div>
                  <p>{entry.message}</p>
                </article>
              ))}
            </div>
          </section>
        </div>
      )}
    </div>
  );
}
