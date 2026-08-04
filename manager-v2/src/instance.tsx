import { useEffect, useState } from "react";
import {
  normalizeBaseUrl,
  type AdvancedInstanceSettings,
  type EvolutionApi,
  type EvolutionConnection,
  type ManagedInstance,
} from "./api";

const EMPTY_ADVANCED_SETTINGS: AdvancedInstanceSettings = {
  alwaysOnline: false,
  rejectCall: false,
  msgRejectCall: "",
  readMessages: false,
  ignoreGroups: false,
  ignoreStatus: false,
};

type Notice = { tone: "success" | "error"; message: string } | null;

function instanceLabel(instance: ManagedInstance): string {
  return instance.name || instance.id || "Instância sem nome";
}

function formatDate(value?: string): string {
  if (!value) return "Data não disponível";
  const date = new Date(value);
  return Number.isNaN(date.getTime())
    ? "Data não disponível"
    : new Intl.DateTimeFormat("pt-BR", { dateStyle: "medium" }).format(date);
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
          <span className="eyebrow">Acesso à API</span>
          <h2>Configurações de conexão</h2>
        </div>
        <span className={`status-dot ${draft.apiKey || draft.adminApiKey ? "online" : "offline"}`} />
      </div>
      <p className="section-description">A chave global libera a criação, listagem e exclusão. A chave da instância é usada para desconectar e salvar as configurações dela.</p>
      <div className="form-grid connection-form-grid">
        <label>
          <span>URL da API</span>
          <input value={draft.baseUrl} onChange={(event) => setDraft({ ...draft, baseUrl: event.target.value })} placeholder="https://evolution.exemplo.com" />
        </label>
        <label>
          <span>ID da instância selecionada</span>
          <input value={draft.instanceId} onChange={(event) => setDraft({ ...draft, instanceId: event.target.value })} placeholder="Selecione uma instância na lista" />
        </label>
        <label>
          <span>Chave da instância</span>
          <input type="password" autoComplete="off" value={draft.apiKey} onChange={(event) => setDraft({ ...draft, apiKey: event.target.value })} placeholder="Necessária para desconectar e configurar" />
        </label>
        <label>
          <span>Chave global</span>
          <input type="password" autoComplete="off" value={draft.adminApiKey} onChange={(event) => setDraft({ ...draft, adminApiKey: event.target.value })} placeholder="Necessária para administrar instâncias" />
        </label>
      </div>
      <label className="check-row">
        <input type="checkbox" checked={draft.remember} onChange={(event) => setDraft({ ...draft, remember: event.target.checked })} />
        <span>{draft.remember ? "Salvar este acesso neste navegador" : "Manter o acesso apenas durante esta sessão"}</span>
      </label>
      <div className="button-row">
        {saved && <span className="save-feedback">Configurações salvas</span>}
        <button type="button" className="button primary" onClick={save}>Salvar configurações</button>
      </div>
    </section>
  );
}

function AdvancedSettings({
  api,
  instance,
  canManage,
  onClose,
  onNotice,
}: {
  api: EvolutionApi | null;
  instance: ManagedInstance;
  canManage: boolean;
  onClose: () => void;
  onNotice: (notice: Notice) => void;
}) {
  const [settings, setSettings] = useState<AdvancedInstanceSettings>(EMPTY_ADVANCED_SETTINGS);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    let active = true;
    if (!api || !canManage) {
      setLoading(false);
      return () => { active = false; };
    }
    setLoading(true);
    void api.getAdvancedSettings(instance.id)
      .then((value) => {
        if (active) setSettings({ ...EMPTY_ADVANCED_SETTINGS, ...value });
      })
      .catch((cause) => {
        if (active) onNotice({ tone: "error", message: cause instanceof Error ? cause.message : "Não foi possível carregar as configurações." });
      })
      .finally(() => { if (active) setLoading(false); });
    return () => { active = false; };
  }, [api, canManage, instance.id, onNotice]);

  const save = async () => {
    if (!api || !canManage || saving) return;
    setSaving(true);
    try {
      await api.updateAdvancedSettings(instance.id, settings);
      onNotice({ tone: "success", message: "Configurações da instância salvas." });
    } catch (cause) {
      onNotice({ tone: "error", message: cause instanceof Error ? cause.message : "Não foi possível salvar as configurações." });
    } finally {
      setSaving(false);
    }
  };

  const toggle = (field: Exclude<keyof AdvancedInstanceSettings, "msgRejectCall">) => {
    setSettings((current) => ({ ...current, [field]: !current[field] }));
  };

  return (
    <section className="card advanced-settings-card" aria-labelledby="advanced-settings-title">
      <div className="section-heading">
        <div>
          <span className="eyebrow">Instância selecionada</span>
          <h2 id="advanced-settings-title">Configurações de {instanceLabel(instance)}</h2>
        </div>
        <button type="button" className="text-button" onClick={onClose}>Fechar</button>
      </div>
      {!canManage ? (
        <div className="inline-empty">Informe e salve a chave desta instância em Configurações antes de alterar estes itens.</div>
      ) : loading ? (
        <div className="inline-empty">Carregando configurações…</div>
      ) : (
        <>
          <div className="toggle-list">
            <label className="toggle-row">
              <input type="checkbox" checked={settings.alwaysOnline} onChange={() => toggle("alwaysOnline")} />
              <span><strong>Sempre online</strong><small>Mantém a presença da conta ativa.</small></span>
            </label>
            <label className="toggle-row">
              <input type="checkbox" checked={settings.readMessages} onChange={() => toggle("readMessages")} />
              <span><strong>Ler mensagens automaticamente</strong><small>Marca as mensagens recebidas como lidas.</small></span>
            </label>
            <label className="toggle-row">
              <input type="checkbox" checked={settings.ignoreGroups} onChange={() => toggle("ignoreGroups")} />
              <span><strong>Ignorar grupos</strong><small>Não processa eventos de grupos.</small></span>
            </label>
            <label className="toggle-row">
              <input type="checkbox" checked={settings.ignoreStatus} onChange={() => toggle("ignoreStatus")} />
              <span><strong>Ignorar status</strong><small>Não processa atualizações de status.</small></span>
            </label>
            <label className="toggle-row">
              <input type="checkbox" checked={settings.rejectCall} onChange={() => toggle("rejectCall")} />
              <span><strong>Recusar chamadas</strong><small>Recusa automaticamente chamadas recebidas.</small></span>
            </label>
          </div>
          {settings.rejectCall && (
            <label className="message-field">
              <span>Mensagem ao recusar chamada</span>
              <input value={settings.msgRejectCall} onChange={(event) => setSettings((current) => ({ ...current, msgRejectCall: event.target.value }))} placeholder="Ex.: Não atendemos chamadas." />
            </label>
          )}
          <div className="button-row">
            <button type="button" className="button primary" disabled={saving} onClick={() => void save}>{saving ? "Salvando…" : "Salvar configurações"}</button>
          </div>
        </>
      )}
    </section>
  );
}

export function InstanceWorkspace({
  api,
  connection,
  onSave,
  onOpenSettings,
}: {
  api: EvolutionApi | null;
  connection: EvolutionConnection;
  onSave: (connection: EvolutionConnection) => void;
  onOpenSettings: () => void;
}) {
  const hasAdminKey = Boolean(connection.adminApiKey.trim());
  const [instances, setInstances] = useState<ManagedInstance[]>([]);
  const [selectedId, setSelectedId] = useState("");
  const [isLoading, setIsLoading] = useState(false);
  const [isCreating, setIsCreating] = useState(false);
  const [showCreate, setShowCreate] = useState(false);
  const [newInstanceName, setNewInstanceName] = useState("");
  const [newInstanceToken, setNewInstanceToken] = useState("");
  const [pendingDelete, setPendingDelete] = useState<ManagedInstance | null>(null);
  const [showAdvancedSettings, setShowAdvancedSettings] = useState(false);
  const [notice, setNotice] = useState<Notice>(null);
  const [isDisconnecting, setIsDisconnecting] = useState(false);

  const refreshInstances = async (silent = false) => {
    if (!api || !hasAdminKey) return;
    if (!silent) setIsLoading(true);
    try {
      const nextInstances = await api.listInstances();
      setInstances(nextInstances);
      setSelectedId((current) => current && nextInstances.some((item) => item.id === current) ? current : nextInstances[0]?.id || "");
    } catch (cause) {
      setNotice({ tone: "error", message: cause instanceof Error ? cause.message : "Não foi possível listar as instâncias." });
    } finally {
      if (!silent) setIsLoading(false);
    }
  };

  useEffect(() => {
    if (!api || !hasAdminKey) {
      setInstances([]);
      setSelectedId("");
      return;
    }
    void refreshInstances();
  // A mudança de acesso deve disparar uma nova listagem. refreshInstances usa os valores atuais do render.
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [api, hasAdminKey]);

  const selected = instances.find((instance) => instance.id === selectedId) ?? null;
  const selectedUsesSavedKey = Boolean(selected && connection.instanceId === selected.id && connection.apiKey.trim());

  const selectInstance = (instance: ManagedInstance) => {
    const token = instance.token || (connection.instanceId === instance.id ? connection.apiKey : "");
    onSave({ ...connection, instanceId: instance.id, apiKey: token });
    setSelectedId(instance.id);
    setShowAdvancedSettings(false);
    setNotice(null);
  };

  const createInstance = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!api || !hasAdminKey || isCreating) return;
    const name = newInstanceName.trim();
    const token = newInstanceToken.trim();
    if (!name || !token) {
      setNotice({ tone: "error", message: "Informe o nome e a chave da nova instância." });
      return;
    }
    setIsCreating(true);
    try {
      const created = await api.createInstance({ name, token });
      onSave({ ...connection, instanceId: created.id, apiKey: token });
      setNewInstanceName("");
      setNewInstanceToken("");
      setShowCreate(false);
      setSelectedId(created.id);
      setNotice({ tone: "success", message: `Instância ${instanceLabel(created)} criada com sucesso.` });
      await refreshInstances(true);
    } catch (cause) {
      setNotice({ tone: "error", message: cause instanceof Error ? cause.message : "Não foi possível criar a instância." });
    } finally {
      setIsCreating(false);
    }
  };

  const deleteInstance = async () => {
    if (!api || !pendingDelete) return;
    const deleted = pendingDelete;
    setPendingDelete(null);
    try {
      await api.deleteInstance(deleted.id);
      if (connection.instanceId === deleted.id) onSave({ ...connection, instanceId: "", apiKey: "" });
      setShowAdvancedSettings(false);
      setNotice({ tone: "success", message: `Instância ${instanceLabel(deleted)} excluída.` });
      await refreshInstances(true);
    } catch (cause) {
      setNotice({ tone: "error", message: cause instanceof Error ? cause.message : "Não foi possível excluir a instância." });
    }
  };

  const disconnect = async () => {
    if (!api || !selected || !selectedUsesSavedKey || isDisconnecting) return;
    setIsDisconnecting(true);
    try {
      await api.disconnectInstance();
      setNotice({ tone: "success", message: `Instância ${instanceLabel(selected)} desconectada.` });
      await refreshInstances(true);
    } catch (cause) {
      setNotice({ tone: "error", message: cause instanceof Error ? cause.message : "Não foi possível desconectar a instância." });
    } finally {
      setIsDisconnecting(false);
    }
  };

  if (!hasAdminKey) {
    return (
      <section className="card access-required">
        <span className="eyebrow">Acesso administrativo</span>
        <h1>Configure a chave global para gerenciar instâncias</h1>
        <p>Com ela, você poderá listar, criar e excluir instâncias neste painel.</p>
        <button type="button" className="button primary" onClick={onOpenSettings}>Abrir configurações</button>
      </section>
    );
  }

  return (
    <div className="instance-workspace">
      <section className="instances-header">
        <div>
          <span className="eyebrow">Gerenciador</span>
          <h1>Instâncias</h1>
          <p>Crie, acompanhe e administre as suas conexões com o WhatsApp.</p>
        </div>
        <div className="instances-header-actions">
          <button type="button" className="button secondary" disabled={isLoading} onClick={() => void refreshInstances()}>{isLoading ? "Atualizando…" : "Atualizar lista"}</button>
          <button type="button" className="button primary" onClick={() => { setShowCreate((current) => !current); setNotice(null); }}>{showCreate ? "Cancelar" : "Nova instância"}</button>
        </div>
      </section>

      {notice && <div className={`alert ${notice.tone}`} role="status">{notice.message}</div>}

      {showCreate && (
        <form className="card create-instance-card" onSubmit={(event) => void createInstance(event)}>
          <div className="section-heading">
            <div><span className="eyebrow">Nova conexão</span><h2>Criar instância</h2></div>
          </div>
          <div className="form-grid">
            <label>
              <span>Nome da instância</span>
              <input autoFocus value={newInstanceName} onChange={(event) => setNewInstanceName(event.target.value)} placeholder="Ex.: atendimento-principal" />
            </label>
            <label>
              <span>Chave da instância</span>
              <input type="password" autoComplete="new-password" value={newInstanceToken} onChange={(event) => setNewInstanceToken(event.target.value)} placeholder="Defina uma chave exclusiva e segura" />
            </label>
          </div>
          <div className="button-row"><button type="submit" className="button primary" disabled={isCreating}>{isCreating ? "Criando…" : "Criar instância"}</button></div>
        </form>
      )}

      <div className="instances-layout">
        <section className="card instance-list-card" aria-label="Lista de instâncias">
          <div className="list-heading"><div><h2>Suas instâncias</h2><span>{instances.length} {instances.length === 1 ? "instância" : "instâncias"}</span></div></div>
          {isLoading ? (
            <div className="list-empty">Carregando instâncias…</div>
          ) : instances.length === 0 ? (
            <div className="list-empty"><strong>Nenhuma instância criada</strong><span>Crie a sua primeira instância para começar.</span></div>
          ) : (
            <div className="instance-list">
              {instances.map((instance) => (
                <button type="button" key={instance.id} className={`instance-list-item ${selected?.id === instance.id ? "selected" : ""}`} onClick={() => selectInstance(instance)}>
                  <span className={`instance-status ${instance.connected ? "connected" : "disconnected"}`} aria-hidden="true" />
                  <span className="instance-list-copy"><strong>{instanceLabel(instance)}</strong><small>{instance.jid || instance.id}</small></span>
                  <span className={`state-badge ${instance.connected ? "state-active" : "state-ended"}`}>{instance.connected ? "Conectada" : "Desconectada"}</span>
                </button>
              ))}
            </div>
          )}
        </section>

        <section className="card instance-detail-card">
          {!selected ? (
            <div className="detail-empty"><span className="empty-icon">+</span><h2>Selecione uma instância</h2><p>Os detalhes e ações aparecerão aqui.</p></div>
          ) : (
            <>
              <div className="instance-detail-heading">
                <div className="instance-avatar">{instanceLabel(selected).slice(0, 1).toUpperCase()}</div>
                <div><span className="eyebrow">Instância selecionada</span><h2>{instanceLabel(selected)}</h2><p>{selected.jid || "Ainda não vinculada a uma conta do WhatsApp"}</p></div>
                <span className={`state-badge ${selected.connected ? "state-active" : "state-ended"}`}>{selected.connected ? "Conectada" : "Desconectada"}</span>
              </div>
              <dl className="instance-meta">
                <div><dt>ID</dt><dd>{selected.id}</dd></div>
                <div><dt>Criada em</dt><dd>{formatDate(selected.createdAt)}</dd></div>
                <div><dt>Cliente</dt><dd>{selected.clientName || selected.osName || "Não informado"}</dd></div>
                <div><dt>Webhook</dt><dd>{selected.webhook || "Não configurado"}</dd></div>
              </dl>
              {!selectedUsesSavedKey && <div className="key-hint">Para desconectar ou alterar esta instância, informe a chave dela em Configurações.</div>}
              <div className="instance-actions">
                <button type="button" className="button secondary" onClick={() => setShowAdvancedSettings((current) => !current)}>Configurações</button>
                <button type="button" className="button danger-button" disabled={!selectedUsesSavedKey || isDisconnecting} onClick={() => void disconnect()}>{isDisconnecting ? "Desconectando…" : "Desconectar"}</button>
                <button type="button" className="button ghost-danger" onClick={() => setPendingDelete(selected)}>Excluir</button>
              </div>
            </>
          )}
        </section>
      </div>

      {showAdvancedSettings && selected && <AdvancedSettings api={api} instance={selected} canManage={selectedUsesSavedKey} onClose={() => setShowAdvancedSettings(false)} onNotice={setNotice} />}

      {pendingDelete && (
        <div className="dialog-backdrop" role="presentation">
          <section className="card confirm-dialog" role="dialog" aria-modal="true" aria-labelledby="delete-title">
            <span className="eyebrow">Ação permanente</span>
            <h2 id="delete-title">Excluir {instanceLabel(pendingDelete)}?</h2>
            <p>Essa ação remove a instância e não pode ser desfeita.</p>
            <div className="dialog-actions">
              <button type="button" className="button secondary" onClick={() => setPendingDelete(null)}>Cancelar</button>
              <button type="button" className="button danger-button" onClick={() => void deleteInstance()}>Excluir instância</button>
            </div>
          </section>
        </div>
      )}
    </div>
  );
}
