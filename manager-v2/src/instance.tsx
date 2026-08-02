import { useEffect, useMemo, useState } from "react";
import {
  normalizeBaseUrl,
  type ApiExecutionResult,
  type EvolutionApi,
  type EvolutionConnection,
} from "./api";

function pretty(value: unknown): string {
  if (typeof value === "string") return value;
  return JSON.stringify(value, null, 2);
}

function findQrImage(value: unknown): string {
  const visit = (candidate: unknown): string => {
    if (typeof candidate === "string") {
      if (candidate.startsWith("data:image/")) return candidate;
      if (candidate.length > 200 && /^[A-Za-z0-9+/=\r\n]+$/.test(candidate)) {
        return `data:image/png;base64,${candidate.replace(/\s/g, "")}`;
      }
      return "";
    }
    if (Array.isArray(candidate)) {
      for (const item of candidate) {
        const found = visit(item);
        if (found) return found;
      }
      return "";
    }
    if (candidate && typeof candidate === "object") {
      const record = candidate as Record<string, unknown>;
      const priority = ["qrcode", "qrCode", "base64", "image", "code"];
      for (const key of priority) {
        if (key in record) {
          const found = visit(record[key]);
          if (found) return found;
        }
      }
      for (const item of Object.values(record)) {
        const found = visit(item);
        if (found) return found;
      }
    }
    return "";
  };
  return visit(value);
}

export function ConnectionEditor({
  value,
  onSave,
  compact = false,
}: {
  value: EvolutionConnection;
  onSave: (connection: EvolutionConnection) => void;
  compact?: boolean;
}) {
  const [draft, setDraft] = useState(value);
  const [saved, setSaved] = useState(false);
  useEffect(() => setDraft(value), [value]);

  const save = () => {
    const normalized = {
      ...draft,
      baseUrl: normalizeBaseUrl(draft.baseUrl),
      apiKey: draft.apiKey.trim(),
      adminApiKey: draft.adminApiKey.trim(),
      instanceId: draft.instanceId.trim(),
    };
    onSave(normalized);
    setDraft(normalized);
    setSaved(true);
    window.setTimeout(() => setSaved(false), 1800);
  };

  return (
    <section className={`card connection-card ${compact ? "compact" : ""}`}>
      <div className="section-heading">
        <div>
          <span className="eyebrow">Perfil de acesso</span>
          <h2>Conexão com o Evolution GO</h2>
        </div>
        <span className={`status-dot ${draft.apiKey ? "online" : "offline"}`} />
      </div>
      <div className="form-grid connection-form-grid">
        <label>
          <span>URL da API</span>
          <input value={draft.baseUrl} onChange={(event) => setDraft({ ...draft, baseUrl: event.target.value })} placeholder="https://evolution.exemplo.com" />
        </label>
        <label>
          <span>ID da instância</span>
          <input value={draft.instanceId} onChange={(event) => setDraft({ ...draft, instanceId: event.target.value })} placeholder="minha-instancia" />
        </label>
        <label>
          <span>API key da instância</span>
          <input type="password" autoComplete="off" value={draft.apiKey} onChange={(event) => setDraft({ ...draft, apiKey: event.target.value })} placeholder="Usada em /send, /user, /call…" />
        </label>
        <label>
          <span>API key global (opcional)</span>
          <input type="password" autoComplete="off" value={draft.adminApiKey} onChange={(event) => setDraft({ ...draft, adminApiKey: event.target.value })} placeholder="Usada em /instance/all e rotas administrativas" />
        </label>
      </div>
      <label className="check-row">
        <input type="checkbox" checked={draft.remember} onChange={(event) => setDraft({ ...draft, remember: event.target.checked })} />
        <span>{draft.remember ? "Salvar este perfil no navegador" : "Manter somente nesta sessão do navegador"}</span>
      </label>
      <div className="button-row">
        {saved && <span className="save-feedback">Perfil salvo</span>}
        <button type="button" className="button primary" onClick={save}>Salvar conexão</button>
      </div>
    </section>
  );
}

type SessionAction = {
  id: string;
  title: string;
  description: string;
  method: string;
  path: string;
  danger?: boolean;
};

const SESSION_ACTIONS: SessionAction[] = [
  { id: "status", title: "Consultar status", description: "Verifica se a sessão está conectada.", method: "GET", path: "/instance/status" },
  { id: "connect", title: "Conectar", description: "Inicia a conexão e prepara o QR Code.", method: "POST", path: "/instance/connect" },
  { id: "qr", title: "Gerar QR Code", description: "Busca o QR Code para pareamento.", method: "GET", path: "/instance/qr" },
  { id: "reconnect", title: "Reconectar", description: "Reinicia o cliente preservando a sessão.", method: "POST", path: "/instance/reconnect" },
  { id: "disconnect", title: "Desconectar", description: "Desconecta sem apagar credenciais.", method: "POST", path: "/instance/disconnect" },
  { id: "logout", title: "Fazer logout", description: "Remove a sessão vinculada ao WhatsApp.", method: "DELETE", path: "/instance/logout", danger: true },
];

export function InstanceWorkspace({
  api,
  connection,
  onSave,
}: {
  api: EvolutionApi | null;
  connection: EvolutionConnection;
  onSave: (connection: EvolutionConnection) => void;
}) {
  const [running, setRunning] = useState("");
  const [error, setError] = useState("");
  const [result, setResult] = useState<ApiExecutionResult | null>(null);
  const qrImage = useMemo(() => findQrImage(result?.data), [result]);

  const execute = async (action: SessionAction) => {
    if (!api || running) return;
    setRunning(action.id);
    setError("");
    try {
      const response = await api.execute({ method: action.method, path: action.path, auth: "instance", body: action.method === "POST" ? JSON.stringify({}) : undefined });
      setResult(response);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "Falha ao executar a ação");
    } finally {
      setRunning("");
    }
  };

  return (
    <div className="instance-workspace">
      <section className="hero card instance-hero">
        <div>
          <span className="eyebrow">Sessão WhatsApp</span>
          <h1>Conectar, salvar e testar</h1>
          <p>Este painel não é uma caixa de entrada. Ele mantém o acesso da instância e oferece ferramentas para validar todas as funções da API.</p>
        </div>
        <div className="hero-metrics">
          <div><strong>{connection.instanceId || "—"}</strong><span>instância</span></div>
          <div><strong>{connection.apiKey ? "OK" : "—"}</strong><span>chave local</span></div>
          <div><strong>{connection.adminApiKey ? "OK" : "—"}</strong><span>chave global</span></div>
        </div>
      </section>

      <ConnectionEditor value={connection} onSave={onSave} />

      <section className="session-grid">
        {SESSION_ACTIONS.map((action) => (
          <article className="card session-action-card" key={action.id}>
            <span className={`http-method method-${action.method.toLowerCase()}`}>{action.method}</span>
            <h3>{action.title}</h3>
            <p>{action.description}</p>
            <code>{action.path}</code>
            <button
              type="button"
              className={`button ${action.danger ? "danger-button" : "secondary"}`}
              disabled={!api || Boolean(running)}
              onClick={() => void execute(action)}
            >
              {running === action.id ? "Executando…" : action.title}
            </button>
          </article>
        ))}
      </section>

      {(error || result) && (
        <section className="session-result-grid">
          {qrImage && (
            <section className="card qr-card">
              <span className="eyebrow">Pareamento</span>
              <h2>QR Code da sessão</h2>
              <img src={qrImage} alt="QR Code para conectar o WhatsApp" />
              <p>Abra o WhatsApp no celular e use “Aparelhos conectados”.</p>
            </section>
          )}
          <section className="card session-response">
            <div className="section-heading">
              <div><span className="eyebrow">Resposta da API</span><h2>{result ? `${result.status} ${result.statusText}` : "Falha local"}</h2></div>
              {result && <span className={`response-status ${result.ok ? "ok" : "failed"}`}>{result.durationMs} ms</span>}
            </div>
            {error ? <div className="alert error">{error}</div> : <pre>{pretty(result?.data)}</pre>}
          </section>
        </section>
      )}
    </div>
  );
}
