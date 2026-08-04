import { useEffect, useState } from "react";
import { type EvolutionApi, type InfrastructureSettings } from "./api";

const EMPTY_SETTINGS: InfrastructureSettings = {
  amqpEnabled: false,
  amqpUrl: "",
  amqpGlobalEnabled: false,
  webhookUrl: "",
  proxyEnabled: false,
  proxyProtocol: "http",
  proxyHost: "",
  proxyPort: "",
  proxyUsername: "",
  proxyPassword: "",
  minioEnabled: false,
  minioEndpoint: "",
  minioAccessKey: "",
  minioSecretKey: "",
  minioBucket: "",
  minioUseSsl: false,
};

export function InfrastructureSettingsPanel({ api }: { api: EvolutionApi }) {
  const [draft, setDraft] = useState<InfrastructureSettings>(EMPTY_SETTINGS);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [notice, setNotice] = useState("");
  const [error, setError] = useState("");

  useEffect(() => {
    let active = true;
    void api.infrastructureSettings()
      .then((settings) => { if (active) setDraft({ ...EMPTY_SETTINGS, ...settings }); })
      .catch((cause) => { if (active) setError(cause instanceof Error ? cause.message : "Não foi possível carregar as configurações de infraestrutura."); })
      .finally(() => { if (active) setLoading(false); });
    return () => { active = false; };
  }, [api]);

  const save = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (saving) return;
    setSaving(true);
    setError("");
    setNotice("");
    try {
      const response = await api.saveInfrastructureSettings(draft);
      setNotice(response.restartRequired ? "Configurações salvas. Reinicie o servidor para aplicar todas as integrações." : "Configurações salvas.");
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "Não foi possível salvar as configurações.");
    } finally {
      setSaving(false);
    }
  };

  const update = <K extends keyof InfrastructureSettings>(key: K, value: InfrastructureSettings[K]) => {
    setDraft((current) => ({ ...current, [key]: value }));
  };

  return (
    <form className="infrastructure-settings" onSubmit={(event) => void save(event)}>
      <section className="card infrastructure-card">
        <div className="section-heading">
          <div><span className="eyebrow">Infraestrutura</span><h2>Eventos e webhook</h2></div>
          <span className="restart-badge">Requer reinício</span>
        </div>
        <p className="section-description">Esses dados ficam protegidos no servidor e substituem os valores de integração definidos no ambiente após o próximo reinício.</p>
        <div className="infrastructure-grid">
          <section className="settings-group">
            <label className="toggle-row">
              <input type="checkbox" checked={draft.amqpEnabled} onChange={(event) => update("amqpEnabled", event.target.checked)} />
              <span><strong>RabbitMQ / AMQP</strong><small>Publica eventos globais da API.</small></span>
            </label>
            <label>
              <span>URL AMQP</span>
              <input disabled={!draft.amqpEnabled} value={draft.amqpUrl} onChange={(event) => update("amqpUrl", event.target.value)} placeholder="amqp://admin:senha@localhost:5672/default" />
            </label>
            <label className="check-row compact-check">
              <input type="checkbox" disabled={!draft.amqpEnabled} checked={draft.amqpGlobalEnabled} onChange={(event) => update("amqpGlobalEnabled", event.target.checked)} />
              <span>Habilitar eventos globais AMQP</span>
            </label>
          </section>
          <section className="settings-group">
            <label>
              <span>URL do webhook global</span>
              <input type="url" value={draft.webhookUrl} onChange={(event) => update("webhookUrl", event.target.value)} placeholder="https://seu-dominio.com/webhook" />
            </label>
            <p className="field-note">Deixe em branco para não enviar eventos por webhook.</p>
          </section>
        </div>
      </section>

      <section className="card infrastructure-card">
        <div className="section-heading"><div><span className="eyebrow">Rede</span><h2>Proxy padrão</h2></div><span className="restart-badge">Requer reinício</span></div>
        <p className="section-description">Use um proxy padrão somente quando novas instâncias precisarem sair pela mesma rede.</p>
        <div className="settings-group">
          <label className="toggle-row">
            <input type="checkbox" checked={draft.proxyEnabled} onChange={(event) => update("proxyEnabled", event.target.checked)} />
            <span><strong>Usar proxy padrão</strong><small>Aplicado às novas conexões das instâncias.</small></span>
          </label>
          <div className="form-grid proxy-grid">
            <label><span>Protocolo</span><select disabled={!draft.proxyEnabled} value={draft.proxyProtocol} onChange={(event) => update("proxyProtocol", event.target.value as InfrastructureSettings["proxyProtocol"])}><option value="http">HTTP</option><option value="https">HTTPS</option><option value="socks5">SOCKS5</option></select></label>
            <label><span>Host</span><input disabled={!draft.proxyEnabled} value={draft.proxyHost} onChange={(event) => update("proxyHost", event.target.value)} placeholder="proxy.exemplo.com" /></label>
            <label><span>Porta</span><input disabled={!draft.proxyEnabled} inputMode="numeric" value={draft.proxyPort} onChange={(event) => update("proxyPort", event.target.value)} placeholder="8080" /></label>
            <label><span>Usuário</span><input disabled={!draft.proxyEnabled} value={draft.proxyUsername} onChange={(event) => update("proxyUsername", event.target.value)} placeholder="usuario" /></label>
            <label className="span-two"><span>Senha</span><input disabled={!draft.proxyEnabled} type="password" autoComplete="new-password" value={draft.proxyPassword} onChange={(event) => update("proxyPassword", event.target.value)} placeholder="Senha do proxy" /></label>
          </div>
        </div>
      </section>

      <section className="card infrastructure-card">
        <div className="section-heading"><div><span className="eyebrow">Armazenamento</span><h2>MinIO</h2></div><span className="restart-badge">Requer reinício</span></div>
        <p className="section-description">Armazene as mídias recebidas e enviadas em um bucket compatível com S3.</p>
        <div className="settings-group">
          <label className="toggle-row">
            <input type="checkbox" checked={draft.minioEnabled} onChange={(event) => update("minioEnabled", event.target.checked)} />
            <span><strong>Ativar armazenamento MinIO</strong><small>Guarda mídias da API em um bucket externo.</small></span>
          </label>
          <div className="form-grid minio-grid">
            <label><span>Endpoint</span><input disabled={!draft.minioEnabled} value={draft.minioEndpoint} onChange={(event) => update("minioEndpoint", event.target.value)} placeholder="localhost:9000" /></label>
            <label><span>Bucket</span><input disabled={!draft.minioEnabled} value={draft.minioBucket} onChange={(event) => update("minioBucket", event.target.value)} placeholder="evolution-media" /></label>
            <label><span>Chave de acesso</span><input disabled={!draft.minioEnabled} value={draft.minioAccessKey} onChange={(event) => update("minioAccessKey", event.target.value)} placeholder="minioadmin" /></label>
            <label><span>Chave secreta</span><input disabled={!draft.minioEnabled} type="password" autoComplete="new-password" value={draft.minioSecretKey} onChange={(event) => update("minioSecretKey", event.target.value)} placeholder="Senha do MinIO" /></label>
          </div>
          <label className="check-row compact-check"><input type="checkbox" disabled={!draft.minioEnabled} checked={draft.minioUseSsl} onChange={(event) => update("minioUseSsl", event.target.checked)} /><span>Usar conexão SSL</span></label>
        </div>
      </section>

      {loading && <div className="alert">Carregando configurações de infraestrutura…</div>}
      {error && <div className="alert error" role="alert">{error}</div>}
      {notice && <div className="alert success" role="status">{notice}</div>}
      <div className="button-row infrastructure-save"><button type="submit" className="button primary" disabled={loading || saving}>{saving ? "Salvando…" : "Salvar infraestrutura"}</button></div>
    </form>
  );
}
