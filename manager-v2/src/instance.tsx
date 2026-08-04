import { useEffect, useState } from "react";
import {
  normalizeBaseUrl,
  type AdvancedInstanceSettings,
  type EvolutionApi,
  type EvolutionConnection,
  type InstanceProxy,
  type ManagedInstance,
  type InstanceQRCode,
} from "./api";

const EMPTY_ADVANCED_SETTINGS: AdvancedInstanceSettings = {
  alwaysOnline: false,
  rejectCall: false,
  msgRejectCall: "",
  readMessages: false,
  ignoreGroups: false,
  ignoreStatus: false,
};

const WEBHOOK_EVENTS = [
  "MESSAGE", "SEND_MESSAGE", "READ_RECEIPT", "PRESENCE", "HISTORY_SYNC", "CHAT_PRESENCE", "CALL", "CONNECTION",
  "LABEL", "CONTACT", "GROUP", "NEWSLETTER", "QRCODE", "BUTTON_CLICK", "PICTURE", "USER_ABOUT",
];

const EMPTY_INSTANCE_PROXY: InstanceProxy = {
  protocol: "http",
  host: "",
  port: "",
  username: "",
  password: "",
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
        <span className={`status-dot ${draft.apiKey ? "online" : "offline"}`} />
      </div>
      <p className="section-description">A chave global é lida com segurança pelo servidor a partir do arquivo de ambiente. Informe somente a chave da instância selecionada para desconectar ou alterar as configurações dela.</p>
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
  onNotice,
}: {
  api: EvolutionApi | null;
  instance: ManagedInstance;
  canManage: boolean;
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
          <span className="eyebrow">Comportamento da instância</span>
          <h2 id="advanced-settings-title">Configurações avançadas</h2>
          <p>Defina como a instância <strong>{instanceLabel(instance)}</strong> deve se comportar no WhatsApp.</p>
        </div>
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

function InstanceConfigurationPanel({
  api,
  instance,
  canManage,
  onDisconnect,
  disconnecting,
  onDelete,
  onNotice,
  onSaved,
}: {
  api: EvolutionApi | null;
  instance: ManagedInstance;
  canManage: boolean;
  onDisconnect: () => void;
  disconnecting: boolean;
  onDelete: () => void;
  onNotice: (notice: Notice) => void;
  onSaved: () => void;
}) {
  const [settings, setSettings] = useState({
    webhookUrl: instance.webhook || "",
    subscribe: instance.events ? instance.events.split(",").filter(Boolean) : ["MESSAGE"],
    rabbitmqEnable: instance.rabbitmqEnable || "",
    websocketEnable: instance.websocketEnable || "",
    natsEnable: instance.natsEnable || "",
  });
  const [saving, setSaving] = useState(false);
  const selectedAll = settings.subscribe.includes("ALL");

  useEffect(() => {
    setSettings({
      webhookUrl: instance.webhook || "",
      subscribe: instance.events ? instance.events.split(",").filter(Boolean) : ["MESSAGE"],
      rabbitmqEnable: instance.rabbitmqEnable || "",
      websocketEnable: instance.websocketEnable || "",
      natsEnable: instance.natsEnable || "",
    });
  }, [instance]);

  const toggleEvent = (event: string) => {
    setSettings((current) => ({
      ...current,
      subscribe: current.subscribe.includes(event)
        ? current.subscribe.filter((item) => item !== event)
        : [...current.subscribe.filter((item) => item !== "ALL"), event],
    }));
  };

  const toggleAll = () => setSettings((current) => ({ ...current, subscribe: current.subscribe.includes("ALL") ? [] : ["ALL"] }));

  const saveWebhook = async () => {
    if (!api || !canManage || saving) return;
    setSaving(true);
    try {
      await api.configureInstance(settings);
      onNotice({ tone: "success", message: "Configurações de webhook salvas." });
      onSaved();
    } catch (cause) {
      onNotice({ tone: "error", message: cause instanceof Error ? cause.message : "Não foi possível salvar o webhook." });
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="instance-configuration-stack">
      <section className="card instance-info-card">
        <div className="section-heading"><div><span className="eyebrow">Instância</span><h2>Informações da instância</h2></div></div>
        <dl className="configuration-info-grid">
          <div><dt>Nome da instância</dt><dd>{instanceLabel(instance)}</dd></div>
          <div><dt>Status</dt><dd>{instance.connected ? "Conectada" : "Desconectada"}</dd></div>
          <div><dt>Número</dt><dd>{instance.jid || "Ainda não vinculado"}</dd></div>
          <div><dt>Token</dt><dd>{canManage ? "••••••••••••••••" : "Informe em Configurações"}</dd></div>
        </dl>
      </section>

      <section className="card webhook-settings-card">
        <div className="section-heading"><div><span className="eyebrow">Eventos</span><h2>Configurações de webhook</h2></div></div>
        {!canManage ? <div className="inline-empty">Informe a chave desta instância em Configurações antes de editar o webhook.</div> : <>
          <label className="webhook-url-field"><span>URL do webhook</span><input type="url" value={settings.webhookUrl} onChange={(event) => setSettings((current) => ({ ...current, webhookUrl: event.target.value }))} placeholder="https://seu-servidor.com/webhook" /><small>URL que receberá os eventos do WhatsApp.</small></label>
          <div className="events-box">
            <div className="events-title"><strong>Eventos para webhook</strong><label className="all-events"><input type="checkbox" checked={selectedAll} onChange={toggleAll} />Todos os eventos</label></div>
            <div className="events-grid">
              {WEBHOOK_EVENTS.map((event) => <label key={event}><input type="checkbox" disabled={selectedAll} checked={selectedAll || settings.subscribe.includes(event)} onChange={() => toggleEvent(event)} />{event}</label>)}
            </div>
          </div>
          <div className="form-grid delivery-grid">
            <label><span>RabbitMQ</span><select value={settings.rabbitmqEnable} onChange={(event) => setSettings((current) => ({ ...current, rabbitmqEnable: event.target.value }))}><option value="">Padrão</option><option value="enabled">Habilitado</option><option value="disabled">Desabilitado</option></select></label>
            <label><span>WebSocket</span><select value={settings.websocketEnable} onChange={(event) => setSettings((current) => ({ ...current, websocketEnable: event.target.value }))}><option value="">Padrão</option><option value="enabled">Habilitado</option><option value="disabled">Desabilitado</option></select></label>
            <label><span>NATS</span><select value={settings.natsEnable} onChange={(event) => setSettings((current) => ({ ...current, natsEnable: event.target.value }))}><option value="">Padrão</option><option value="enabled">Habilitado</option><option value="disabled">Desabilitado</option></select></label>
          </div>
          <div className="button-row"><button type="button" className="button primary" disabled={saving} onClick={() => void saveWebhook()}>{saving ? "Salvando…" : "Salvar webhook"}</button></div>
        </>}
      </section>

      <AdvancedSettings api={api} instance={instance} canManage={canManage} onNotice={onNotice} />

      <section className="danger-zone-card">
        <div><span className="eyebrow">Ação irreversível</span><h2>Zona de perigo</h2></div>
        <div className="danger-zone-row"><div><strong>Desconectar instância</strong><small>Desconecta a instância do WhatsApp.</small></div><button type="button" className="button danger-button" disabled={!canManage || disconnecting} onClick={onDisconnect}>{disconnecting ? "Desconectando…" : "Desconectar"}</button></div>
        <div className="danger-zone-row"><div><strong>Deletar instância</strong><small>Remove permanentemente esta instância.</small></div><button type="button" className="button danger-button" onClick={onDelete}>Deletar</button></div>
      </section>
    </div>
  );
}

export function InstanceSettingsWorkspace({
  api,
  connection,
  onSave,
  instanceId,
  onBack,
}: {
  api: EvolutionApi | null;
  connection: EvolutionConnection;
  onSave: (connection: EvolutionConnection) => void;
  instanceId: string;
  onBack: () => void;
}) {
  const [instance, setInstance] = useState<ManagedInstance | null>(null);
  const [loading, setLoading] = useState(true);
  const [notice, setNotice] = useState<Notice>(null);
  const [isDisconnecting, setIsDisconnecting] = useState(false);
  const [isDeleting, setIsDeleting] = useState(false);
  const [showDeleteConfirmation, setShowDeleteConfirmation] = useState(false);
  const [deleteError, setDeleteError] = useState("");

  const refreshInstance = async () => {
    if (!api) return;
    setLoading(true);
    try {
      const found = (await api.listInstances()).find((item) => item.id === instanceId) ?? null;
      setInstance(found);
      if (!found) {
        setNotice({ tone: "error", message: "Esta instância não foi encontrada ou já foi removida." });
        return;
      }

      const token = found.token || (connection.instanceId === found.id ? connection.apiKey : "");
      if (token && (connection.instanceId !== found.id || connection.apiKey !== token)) {
        onSave({ ...connection, instanceId: found.id, apiKey: token });
      }
    } catch (cause) {
      setNotice({ tone: "error", message: cause instanceof Error ? cause.message : "Não foi possível carregar a instância." });
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void refreshInstance();
  // A API é recriada quando a chave desta instância é carregada; isso confirma a tela com o acesso correto.
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [api, instanceId]);

  const canManage = Boolean(instance && connection.instanceId === instance.id && connection.apiKey.trim());

  const disconnect = async () => {
    if (!api || !instance || !canManage || isDisconnecting) return;
    setIsDisconnecting(true);
    try {
      await api.disconnectInstance();
      setNotice({ tone: "success", message: `Instância ${instanceLabel(instance)} desconectada.` });
      await refreshInstance();
    } catch (cause) {
      setNotice({ tone: "error", message: cause instanceof Error ? cause.message : "Não foi possível desconectar a instância." });
    } finally {
      setIsDisconnecting(false);
    }
  };

  const deleteInstance = async () => {
    if (!api || !instance || isDeleting) return;
    setIsDeleting(true);
    setDeleteError("");
    try {
      await api.deleteInstance(instance.id);
      if (connection.instanceId === instance.id) onSave({ ...connection, instanceId: "", apiKey: "" });
      onBack();
    } catch (cause) {
      setDeleteError(cause instanceof Error ? cause.message : "Não foi possível excluir a instância.");
    } finally {
      setIsDeleting(false);
    }
  };

  return (
    <div className="instance-settings-workspace">
      <section className="instance-settings-header">
        <div>
          <span className="eyebrow">Configuração dedicada</span>
          <h1>{instance ? `Configurações de ${instanceLabel(instance)}` : "Configurações da instância"}</h1>
          <p>Webhook, eventos, comportamento da conexão e ações de segurança desta instância.</p>
        </div>
        <button type="button" className="button secondary" onClick={onBack}>Voltar para instâncias</button>
      </section>

      {notice && <div className={`alert ${notice.tone}`} role="status">{notice.message}</div>}

      {loading ? (
        <section className="card instance-settings-loading">Carregando configurações da instância…</section>
      ) : instance ? (
        <InstanceConfigurationPanel
          api={api}
          instance={instance}
          canManage={canManage}
          onDisconnect={() => void disconnect()}
          disconnecting={isDisconnecting}
          onDelete={() => { setDeleteError(""); setShowDeleteConfirmation(true); }}
          onNotice={setNotice}
          onSaved={() => void refreshInstance()}
        />
      ) : (
        <section className="card instance-settings-loading">
          <p>A instância não está mais disponível.</p>
          <button type="button" className="button secondary" onClick={onBack}>Voltar para instâncias</button>
        </section>
      )}

      {showDeleteConfirmation && instance && (
        <div className="dialog-backdrop" role="presentation">
          <section className="card confirm-dialog" role="dialog" aria-modal="true" aria-labelledby="instance-settings-delete-title">
            <span className="eyebrow">Ação permanente</span>
            <h2 id="instance-settings-delete-title">Excluir {instanceLabel(instance)}?</h2>
            <p>Essa ação remove a instância e não pode ser desfeita.</p>
            {deleteError && <div className="delete-error" role="alert">{deleteError}</div>}
            <div className="dialog-actions">
              <button type="button" className="button secondary" disabled={isDeleting} onClick={() => { setDeleteError(""); setShowDeleteConfirmation(false); }}>Cancelar</button>
              <button type="button" className="button danger-button" disabled={isDeleting} onClick={() => void deleteInstance()}>{isDeleting ? "Excluindo…" : "Excluir instância"}</button>
            </div>
          </section>
        </div>
      )}
    </div>
  );
}

export function InstanceWorkspace({
  api,
  connection,
  onSave,
  onOpenSettings,
  onOpenInstanceSettings,
  canManage,
}: {
  api: EvolutionApi | null;
  connection: EvolutionConnection;
  onSave: (connection: EvolutionConnection) => void;
  onOpenSettings: () => void;
  onOpenInstanceSettings: (instanceId: string) => void;
  canManage: boolean;
}) {
  const [instances, setInstances] = useState<ManagedInstance[]>([]);
  const [selectedId, setSelectedId] = useState("");
  const [isLoading, setIsLoading] = useState(false);
  const [isCreating, setIsCreating] = useState(false);
  const [showCreate, setShowCreate] = useState(false);
  const [newInstanceName, setNewInstanceName] = useState("");
  const [newInstanceToken, setNewInstanceToken] = useState("");
  const [newInstanceProxyEnabled, setNewInstanceProxyEnabled] = useState(false);
  const [newInstanceProxy, setNewInstanceProxy] = useState<InstanceProxy>(EMPTY_INSTANCE_PROXY);
  const [createError, setCreateError] = useState("");
  const [pendingDelete, setPendingDelete] = useState<ManagedInstance | null>(null);
  const [notice, setNotice] = useState<Notice>(null);
  const [isDisconnecting, setIsDisconnecting] = useState(false);
  const [isDeleting, setIsDeleting] = useState(false);
  const [deleteError, setDeleteError] = useState("");
  const [isGeneratingQr, setIsGeneratingQr] = useState(false);
  const [qrCode, setQrCode] = useState<InstanceQRCode | null>(null);

  const refreshInstances = async (silent = false) => {
    if (!api || !canManage) return;
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
    if (!api || !canManage) {
      setInstances([]);
      setSelectedId("");
      return;
    }
    void refreshInstances();
  // A mudança de acesso deve disparar uma nova listagem. refreshInstances usa os valores atuais do render.
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [api, canManage]);

  const selected = instances.find((instance) => instance.id === selectedId) ?? null;
  const selectedUsesSavedKey = Boolean(selected && connection.instanceId === selected.id && connection.apiKey.trim());

  const selectInstance = (instance: ManagedInstance) => {
    const token = instance.token || (connection.instanceId === instance.id ? connection.apiKey : "");
    onSave({ ...connection, instanceId: instance.id, apiKey: token });
    setSelectedId(instance.id);
    setNotice(null);
  };

  const openSelectedSettings = () => {
    if (!selected) return;
    const token = selected.token || (connection.instanceId === selected.id ? connection.apiKey : "");
    if (token) onSave({ ...connection, instanceId: selected.id, apiKey: token });
    onOpenInstanceSettings(selected.id);
  };

  const createInstance = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!api || !canManage || isCreating) return;
    const name = newInstanceName.trim();
    const token = newInstanceToken.trim();
    if (!name) {
      setCreateError("Informe o nome da instância.");
      return;
    }
    if (newInstanceProxyEnabled && (!newInstanceProxy.host.trim() || !newInstanceProxy.port.trim())) {
      setCreateError("Informe host e porta do proxy.");
      return;
    }
    setIsCreating(true);
    setCreateError("");
    try {
      const proxy = newInstanceProxyEnabled
        ? {
            ...newInstanceProxy,
            host: newInstanceProxy.host.trim(),
            port: newInstanceProxy.port.trim(),
            username: newInstanceProxy.username?.trim(),
            password: newInstanceProxy.password?.trim(),
          }
        : undefined;
      const created = await api.createInstance({ name, token: token || undefined, proxy });
      onSave({ ...connection, instanceId: created.id, apiKey: created.token || token });
      setNewInstanceName("");
      setNewInstanceToken("");
      setNewInstanceProxyEnabled(false);
      setNewInstanceProxy(EMPTY_INSTANCE_PROXY);
      setShowCreate(false);
      setSelectedId(created.id);
      setNotice({ tone: "success", message: `Instância ${instanceLabel(created)} criada com sucesso.` });
      await refreshInstances(true);
    } catch (cause) {
      const message = cause instanceof Error ? cause.message : "Não foi possível criar a instância.";
      setCreateError(message);
      setNotice({ tone: "error", message });
    } finally {
      setIsCreating(false);
    }
  };

  const deleteInstance = async () => {
    if (!api || !pendingDelete || isDeleting) return;
    const deleted = pendingDelete;
    setIsDeleting(true);
    setDeleteError("");
    try {
      await api.deleteInstance(deleted.id);
      const remaining = await api.listInstances();
      if (remaining.some((instance) => instance.id === deleted.id)) {
        throw new Error("O servidor confirmou a exclusão, mas a instância ainda está registrada.");
      }
      setInstances(remaining);
      setSelectedId((current) => current === deleted.id ? remaining[0]?.id || "" : current);
      if (connection.instanceId === deleted.id) onSave({ ...connection, instanceId: "", apiKey: "" });
      setPendingDelete(null);
      setNotice({ tone: "success", message: `Instância ${instanceLabel(deleted)} excluída.` });
    } catch (cause) {
      const message = cause instanceof Error ? cause.message : "Não foi possível excluir a instância.";
      setDeleteError(message);
      setNotice({ tone: "error", message });
    } finally {
      setIsDeleting(false);
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

  const generateQr = async () => {
    if (!api || !selected || !selectedUsesSavedKey || isGeneratingQr) return;
    setIsGeneratingQr(true);
    setNotice(null);
    try {
      await api.reconnectInstance();
      const nextQr = await api.getInstanceQr();
      setQrCode(nextQr);
      if (nextQr.qrcode) {
        setNotice({ tone: "success", message: "QR Code gerado. Leia-o no WhatsApp para conectar a instância." });
      }
    } catch (cause) {
      const message = cause instanceof Error ? cause.message : "Não foi possível conectar a instância.";
      if (message.includes("session already linked")) {
        setNotice({ tone: "success", message: "A sessão desta instância ainda é válida e está sendo reconectada; não é necessário ler um novo QR Code." });
        await refreshInstances(true);
      } else {
        setNotice({ tone: "error", message });
      }
    } finally {
      setIsGeneratingQr(false);
    }
  };

  if (!canManage) {
    return (
      <section className="card access-required">
        <span className="eyebrow">Acesso administrativo</span>
        <h1>Entre com uma conta de administrador</h1>
        <p>O acesso ao gerenciamento de instâncias requer uma sessão autenticada.</p>
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
          <button type="button" className="button primary" onClick={() => { setShowCreate(true); setCreateError(""); setNotice(null); }}>Nova instância</button>
        </div>
      </section>

      {notice && <div className={`alert ${notice.tone}`} role="status">{notice.message}</div>}

      {showCreate && (
        <div className="dialog-backdrop" role="presentation">
          <form className="card create-instance-dialog" role="dialog" aria-modal="true" aria-labelledby="create-instance-title" onSubmit={(event) => void createInstance(event)}>
            <div className="section-heading"><div><span className="eyebrow">Nova conexão</span><h2 id="create-instance-title">Criar instância</h2></div><button type="button" className="text-button" disabled={isCreating} onClick={() => setShowCreate(false)}>Fechar</button></div>
            <p>Defina um nome para a instância. A chave pode ser informada agora ou gerada automaticamente.</p>
            <div className="form-grid">
              <label>
                <span>Nome da instância *</span>
                <input autoFocus required value={newInstanceName} onChange={(event) => setNewInstanceName(event.target.value)} placeholder="Ex.: atendimento-principal" />
              </label>
              <label>
                <span>Token da instância <em>opcional</em></span>
                <input type="password" autoComplete="new-password" value={newInstanceToken} onChange={(event) => setNewInstanceToken(event.target.value)} placeholder="Vazio = gerar automaticamente" />
              </label>
            </div>
            <div className="token-hint">Se deixar o token em branco, uma chave segura será criada automaticamente e salva neste navegador.</div>
            <section className="instance-proxy-section">
              <label className="toggle-row"><input type="checkbox" checked={newInstanceProxyEnabled} onChange={(event) => setNewInstanceProxyEnabled(event.target.checked)} /><span><strong>Configurar proxy desta instância</strong><small>Este proxy fica vinculado somente a esta instância.</small></span></label>
              {newInstanceProxyEnabled && (
                <div className="form-grid create-proxy-grid">
                  <label><span>Protocolo</span><select value={newInstanceProxy.protocol} onChange={(event) => setNewInstanceProxy((current) => ({ ...current, protocol: event.target.value as InstanceProxy["protocol"] }))}><option value="http">HTTP</option><option value="https">HTTPS</option><option value="socks5">SOCKS5</option></select></label>
                  <label><span>Host *</span><input required value={newInstanceProxy.host} onChange={(event) => setNewInstanceProxy((current) => ({ ...current, host: event.target.value }))} placeholder="proxy.exemplo.com" /></label>
                  <label><span>Porta *</span><input required inputMode="numeric" value={newInstanceProxy.port} onChange={(event) => setNewInstanceProxy((current) => ({ ...current, port: event.target.value }))} placeholder="8080" /></label>
                  <label><span>Usuário <em>opcional</em></span><input value={newInstanceProxy.username} onChange={(event) => setNewInstanceProxy((current) => ({ ...current, username: event.target.value }))} placeholder="usuario" /></label>
                  <label className="span-two"><span>Senha <em>opcional</em></span><input type="password" autoComplete="new-password" value={newInstanceProxy.password} onChange={(event) => setNewInstanceProxy((current) => ({ ...current, password: event.target.value }))} placeholder="Senha do proxy" /></label>
                </div>
              )}
            </section>
            {createError && <div className="create-error" role="alert">{createError}</div>}
            <div className="dialog-actions"><button type="button" className="button secondary" disabled={isCreating} onClick={() => setShowCreate(false)}>Cancelar</button><button type="submit" className="button primary" disabled={isCreating}>{isCreating ? "Criando…" : "Criar instância"}</button></div>
          </form>
        </div>
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
                {!selected.connected && <button type="button" className="button primary" disabled={!selectedUsesSavedKey || isGeneratingQr} onClick={() => void generateQr()}>{isGeneratingQr ? "Conectando…" : "Conectar"}</button>}
                <button type="button" className="button secondary" onClick={openSelectedSettings}>Configurações</button>
                <button type="button" className="button danger-button" disabled={!selectedUsesSavedKey || isDisconnecting} onClick={() => void disconnect()}>{isDisconnecting ? "Desconectando…" : "Desconectar"}</button>
                <button type="button" className="button ghost-danger" disabled={isDeleting} onClick={() => { setDeleteError(""); setPendingDelete(selected); }}>Excluir</button>
              </div>
            </>
          )}
        </section>
      </div>

      {qrCode && (
        <div className="dialog-backdrop" role="presentation">
          <section className="card qr-dialog" role="dialog" aria-modal="true" aria-labelledby="qr-title">
            <div className="section-heading"><div><span className="eyebrow">Conectar WhatsApp</span><h2 id="qr-title">Leia o QR Code</h2></div><button type="button" className="text-button" onClick={() => setQrCode(null)}>Fechar</button></div>
            {qrCode.qrcode ? (
              <>
                <img src={qrCode.qrcode} alt="QR Code para conectar a instância ao WhatsApp" />
                <p>No WhatsApp, abra <strong>Dispositivos conectados</strong> e escolha <strong>Conectar dispositivo</strong>.</p>
              </>
            ) : (
              <div className="qr-passkey"><strong>Confirmação adicional solicitada</strong><span>{qrCode.passkeyStage || "O WhatsApp pediu uma etapa adicional para concluir a conexão."}</span>{qrCode.passkeyCode && <code>{qrCode.passkeyCode}</code>}{qrCode.passkeyOpenUrl && <a className="button primary" href={qrCode.passkeyOpenUrl} target="_blank" rel="noreferrer">Abrir WhatsApp Web</a>}</div>
            )}
            <div className="button-row"><button type="button" className="button secondary" disabled={isGeneratingQr} onClick={() => void generateQr()}>{isGeneratingQr ? "Atualizando…" : "Atualizar QR Code"}</button></div>
          </section>
        </div>
      )}

      {pendingDelete && (
        <div className="dialog-backdrop" role="presentation">
          <section className="card confirm-dialog" role="dialog" aria-modal="true" aria-labelledby="delete-title">
            <span className="eyebrow">Ação permanente</span>
            <h2 id="delete-title">Excluir {instanceLabel(pendingDelete)}?</h2>
            <p>Essa ação remove a instância e não pode ser desfeita.</p>
            {deleteError && <div className="delete-error" role="alert">{deleteError}</div>}
            <div className="dialog-actions">
              <button type="button" className="button secondary" disabled={isDeleting} onClick={() => { setDeleteError(""); setPendingDelete(null); }}>Cancelar</button>
              <button type="button" className="button danger-button" disabled={isDeleting} onClick={() => void deleteInstance()}>{isDeleting ? "Excluindo…" : "Excluir instância"}</button>
            </div>
          </section>
        </div>
      )}
    </div>
  );
}
