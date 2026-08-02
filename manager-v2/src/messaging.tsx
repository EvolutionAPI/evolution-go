import { useCallback, useEffect, useMemo, useState } from "react";
import {
  displayPhone,
  normalizePhone,
  type EvolutionApi,
  type EvolutionContact,
  type MessageSendResult,
} from "./api";

const MESSAGE_STORE_KEY = "evolution.managerV2.messages.session.v1";
const MAX_LOCAL_MESSAGES = 500;

type LocalMessageStatus = "sending" | "sent" | "failed";
type LocalMessageKind = "text" | "media";

interface LocalMessage {
  id: string;
  recipient: string;
  recipientKey: string;
  text: string;
  fileName?: string;
  kind: LocalMessageKind;
  status: LocalMessageStatus;
  createdAt: string;
  serverId?: string;
  error?: string;
}

interface ContactState {
  contacts: EvolutionContact[];
  loading: boolean;
  error: string;
  refresh: () => Promise<void>;
}

function useContacts(api: EvolutionApi | null): ContactState {
  const [contacts, setContacts] = useState<EvolutionContact[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  const refresh = useCallback(async () => {
    if (!api) {
      setContacts([]);
      return;
    }
    setLoading(true);
    setError("");
    try {
      const result = await api.contacts();
      setContacts([...result].sort((left, right) => contactName(left).localeCompare(contactName(right), "pt-BR")));
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "Não foi possível carregar os contatos");
    } finally {
      setLoading(false);
    }
  }, [api]);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  return { contacts, loading, error, refresh };
}

function contactName(contact: EvolutionContact): string {
  return contact.BusinessName || contact.FullName || contact.PushName || contact.FirstName || displayPhone(contact.Jid);
}

function contactInitials(contact: EvolutionContact): string {
  const parts = contactName(contact).trim().split(/\s+/).filter(Boolean);
  return (parts.length > 1 ? `${parts[0][0]}${parts.at(-1)?.[0] ?? ""}` : parts[0]?.slice(0, 2) || "WA").toUpperCase();
}

function recipientIdentity(value: string): string {
  const visible = displayPhone(value);
  return normalizePhone(visible) || visible.toLowerCase();
}

function loadMessages(): LocalMessage[] {
  try {
    const parsed = JSON.parse(sessionStorage.getItem(MESSAGE_STORE_KEY) || "[]") as unknown;
    if (!Array.isArray(parsed)) return [];
    return parsed.filter((item): item is LocalMessage => {
      if (!item || typeof item !== "object") return false;
      const candidate = item as Partial<LocalMessage>;
      return typeof candidate.id === "string"
        && typeof candidate.recipient === "string"
        && typeof candidate.recipientKey === "string"
        && typeof candidate.text === "string"
        && typeof candidate.createdAt === "string";
    }).slice(-MAX_LOCAL_MESSAGES);
  } catch {
    return [];
  }
}

function persistMessages(messages: LocalMessage[]): void {
  sessionStorage.setItem(MESSAGE_STORE_KEY, JSON.stringify(messages.slice(-MAX_LOCAL_MESSAGES)));
}

function localMessageId(): string {
  return typeof crypto !== "undefined" && "randomUUID" in crypto
    ? crypto.randomUUID()
    : `${Date.now()}-${Math.random().toString(16).slice(2)}`;
}

function responseMessageId(result: MessageSendResult): string | undefined {
  if (typeof result.id === "string" && result.id) return result.id;
  if (typeof result.messageId === "string" && result.messageId) return result.messageId;
  return undefined;
}

function formatMessageTime(value: string): string {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? "agora" : date.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
}

async function resolveRecipient(api: EvolutionApi, value: string): Promise<string> {
  const trimmed = value.trim();
  if (trimmed.includes("@")) return trimmed;
  const normalized = normalizePhone(trimmed);
  if (normalized.length < 8 || normalized.length > 20) {
    throw new Error("Informe um número completo com DDI");
  }
  const user = await api.checkUser(normalized);
  if (!user || !user.IsInWhatsapp) {
    throw new Error("O número não foi encontrado no WhatsApp");
  }
  return user.RemoteJID || user.JID || normalized;
}

function ContactList({
  contacts,
  selectedRecipient,
  query,
  onQuery,
  onSelect,
}: {
  contacts: EvolutionContact[];
  selectedRecipient: string;
  query: string;
  onQuery: (value: string) => void;
  onSelect: (contact: EvolutionContact) => void;
}) {
  const normalizedQuery = query.trim().toLocaleLowerCase("pt-BR");
  const visible = contacts.filter((contact) => {
    if (!normalizedQuery) return true;
    return `${contactName(contact)} ${contact.Jid}`.toLocaleLowerCase("pt-BR").includes(normalizedQuery);
  });

  return (
    <>
      <div className="message-search">
        <span>⌕</span>
        <input value={query} onChange={(event) => onQuery(event.target.value)} placeholder="Buscar contato" />
      </div>
      <div className="message-contact-list">
        {visible.length === 0 ? (
          <div className="message-empty-small">Nenhum contato encontrado.</div>
        ) : visible.map((contact) => (
          <button
            type="button"
            key={contact.Jid}
            className={`message-contact ${recipientIdentity(selectedRecipient) === recipientIdentity(contact.Jid) ? "selected" : ""}`}
            onClick={() => onSelect(contact)}
          >
            <span className="message-avatar">{contactInitials(contact)}</span>
            <span className="message-contact-copy">
              <strong>{contactName(contact)}</strong>
              <small>{displayPhone(contact.Jid)}</small>
            </span>
            <span className={`contact-found ${contact.Found ? "yes" : "no"}`} title={contact.Found ? "Contato sincronizado" : "Contato não confirmado"} />
          </button>
        ))}
      </div>
    </>
  );
}

export function MessagingWorkspace({
  api,
  initialRecipient,
  onStartCall,
}: {
  api: EvolutionApi | null;
  initialRecipient?: string;
  onStartCall?: () => void;
}) {
  const contactState = useContacts(api);
  const [recipient, setRecipient] = useState(initialRecipient || "");
  const [recipientDraft, setRecipientDraft] = useState(initialRecipient ? displayPhone(initialRecipient) : "");
  const [contactQuery, setContactQuery] = useState("");
  const [text, setText] = useState("");
  const [file, setFile] = useState<File | null>(null);
  const [fileInputKey, setFileInputKey] = useState(0);
  const [messages, setMessages] = useState<LocalMessage[]>(loadMessages);
  const [sending, setSending] = useState(false);
  const [notice, setNotice] = useState("");
  const [error, setError] = useState("");

  useEffect(() => persistMessages(messages), [messages]);
  useEffect(() => {
    if (!initialRecipient) return;
    setRecipient(initialRecipient);
    setRecipientDraft(displayPhone(initialRecipient));
  }, [initialRecipient]);

  const selectedContact = useMemo(
    () => contactState.contacts.find((contact) => recipientIdentity(contact.Jid) === recipientIdentity(recipient)),
    [contactState.contacts, recipient],
  );
  const conversation = useMemo(
    () => messages.filter((message) => message.recipientKey === recipientIdentity(recipient)),
    [messages, recipient],
  );

  const selectContact = (contact: EvolutionContact) => {
    setRecipient(contact.Jid);
    setRecipientDraft(displayPhone(contact.Jid));
    setError("");
    setNotice("");
  };

  const openTypedRecipient = async () => {
    if (!api) return;
    setError("");
    setNotice("Verificando número…");
    try {
      const resolved = await resolveRecipient(api, recipientDraft);
      setRecipient(resolved);
      setRecipientDraft(displayPhone(resolved));
      setNotice("Número validado no WhatsApp.");
    } catch (cause) {
      setNotice("");
      setError(cause instanceof Error ? cause.message : "Não foi possível validar o número");
    }
  };

  const send = async () => {
    if (!api || sending) return;
    if (!text.trim() && !file) {
      setError("Digite uma mensagem ou escolha um arquivo");
      return;
    }
    setSending(true);
    setError("");
    setNotice("");
    let target = recipient;
    try {
      if (!target) target = await resolveRecipient(api, recipientDraft);
      else if (!target.includes("@")) target = await resolveRecipient(api, target);
      setRecipient(target);
      setRecipientDraft(displayPhone(target));

      const optimistic: LocalMessage = {
        id: localMessageId(),
        recipient: target,
        recipientKey: recipientIdentity(target),
        text: text.trim() || `Arquivo: ${file?.name ?? "mídia"}`,
        fileName: file?.name,
        kind: file ? "media" : "text",
        status: "sending",
        createdAt: new Date().toISOString(),
      };
      setMessages((current) => [...current, optimistic]);

      try {
        const result = file
          ? await api.sendMedia(target, file, text.trim())
          : await api.sendText(target, text.trim());
        setMessages((current) => current.map((message) => message.id === optimistic.id
          ? { ...message, status: "sent", serverId: responseMessageId(result) }
          : message));
        setText("");
        setFile(null);
        setFileInputKey((current) => current + 1);
        setNotice(file ? "Arquivo enviado com sucesso." : "Mensagem enviada com sucesso.");
      } catch (cause) {
        const message = cause instanceof Error ? cause.message : "Falha ao enviar mensagem";
        setMessages((current) => current.map((item) => item.id === optimistic.id
          ? { ...item, status: "failed", error: message }
          : item));
        throw cause;
      }
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "Falha ao enviar mensagem");
    } finally {
      setSending(false);
    }
  };

  const startCall = async () => {
    if (!api) return;
    setError("");
    try {
      const target = recipient || await resolveRecipient(api, recipientDraft);
      await api.startCall(displayPhone(target));
      setNotice("Chamada iniciada. Abrindo a central de voz…");
      onStartCall?.();
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "Não foi possível iniciar a chamada");
    }
  };

  const clearConversation = () => {
    const identity = recipientIdentity(recipient);
    setMessages((current) => current.filter((message) => message.recipientKey !== identity));
  };

  return (
    <div className="message-workspace">
      <aside className="card message-sidebar">
        <div className="message-pane-heading">
          <div><span className="eyebrow">Mensageria</span><h2>Contatos</h2></div>
          <button className="icon-button" type="button" disabled={contactState.loading} onClick={() => void contactState.refresh()}>↻</button>
        </div>
        <ContactList
          contacts={contactState.contacts}
          selectedRecipient={recipient}
          query={contactQuery}
          onQuery={setContactQuery}
          onSelect={selectContact}
        />
        {contactState.error && <div className="alert error">{contactState.error}</div>}
      </aside>

      <section className="card conversation-pane">
        <header className="conversation-header">
          <div className="conversation-person">
            <span className="message-avatar large">{selectedContact ? contactInitials(selectedContact) : "WA"}</span>
            <div>
              <span className="eyebrow">Conversa</span>
              <h2>{selectedContact ? contactName(selectedContact) : recipient ? displayPhone(recipient) : "Nova conversa"}</h2>
              <small>{recipient ? displayPhone(recipient) : "Escolha um contato ou informe um número"}</small>
            </div>
          </div>
          <button className="button secondary" type="button" disabled={!api || (!recipient && !recipientDraft)} onClick={() => void startCall()}>☎ Ligar</button>
        </header>

        {!recipient && (
          <div className="recipient-entry">
            <input
              value={recipientDraft}
              inputMode="tel"
              placeholder="Número completo com DDI"
              onChange={(event) => setRecipientDraft(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === "Enter") void openTypedRecipient();
              }}
            />
            <button className="button secondary" type="button" disabled={!api} onClick={() => void openTypedRecipient()}>Verificar</button>
          </div>
        )}

        <div className="conversation-scroll">
          {!recipient ? (
            <div className="conversation-empty"><span>✉</span><h3>Comece uma conversa</h3><p>Selecione um contato ou valide um número com DDI.</p></div>
          ) : conversation.length === 0 ? (
            <div className="conversation-empty"><span>✓</span><h3>Canal pronto</h3><p>As mensagens enviadas nesta sessão aparecerão aqui.</p></div>
          ) : conversation.map((message) => (
            <article className={`message-bubble outgoing ${message.status}`} key={message.id}>
              {message.fileName && <span className="message-file">▧ {message.fileName}</span>}
              <p>{message.text}</p>
              <footer>
                <time>{formatMessageTime(message.createdAt)}</time>
                <span>{message.status === "sending" ? "Enviando…" : message.status === "sent" ? "Enviada ✓" : "Falhou"}</span>
              </footer>
              {message.error && <small className="message-error">{message.error}</small>}
            </article>
          ))}
        </div>

        <div className="message-composer">
          {file && (
            <div className="attachment-chip">
              <span>▧ {file.name}</span>
              <button type="button" onClick={() => { setFile(null); setFileInputKey((current) => current + 1); }}>×</button>
            </div>
          )}
          <div className="composer-row">
            <label className="attachment-button" title="Anexar arquivo">
              <input
                key={fileInputKey}
                type="file"
                onChange={(event) => setFile(event.target.files?.[0] ?? null)}
              />
              ＋
            </label>
            <textarea
              value={text}
              rows={2}
              placeholder={file ? "Legenda opcional" : "Digite uma mensagem"}
              onChange={(event) => setText(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === "Enter" && !event.shiftKey) {
                  event.preventDefault();
                  void send();
                }
              }}
            />
            <button className="button primary send-button" type="button" disabled={!api || sending} onClick={() => void send()}>
              {sending ? "Enviando…" : "Enviar"}
            </button>
          </div>
          {(notice || error) && <div className={`composer-notice ${error ? "error" : "success"}`}>{error || notice}</div>}
        </div>
      </section>

      <aside className="card conversation-info">
        <span className="eyebrow">Detalhes</span>
        <div className="contact-hero">
          <span className="message-avatar xlarge">{selectedContact ? contactInitials(selectedContact) : "WA"}</span>
          <h2>{selectedContact ? contactName(selectedContact) : recipient ? displayPhone(recipient) : "Sem contato"}</h2>
          <p>{recipient || "Nenhum destinatário selecionado"}</p>
        </div>
        <dl className="contact-facts">
          <div><dt>Sincronizado</dt><dd>{selectedContact?.Found ? "Sim" : "Não confirmado"}</dd></div>
          <div><dt>Nome do WhatsApp</dt><dd>{selectedContact?.PushName || "Não disponível"}</dd></div>
          <div><dt>Empresa</dt><dd>{selectedContact?.BusinessName || "Não informada"}</dd></div>
          <div><dt>Mensagens locais</dt><dd>{conversation.length}</dd></div>
        </dl>
        <button className="button secondary full-button" type="button" disabled={!recipient || conversation.length === 0} onClick={clearConversation}>Limpar histórico local</button>
        <div className="message-limit-note">
          <strong>Histórico da API</strong>
          <p>O backend atual envia mensagens e lista contatos, mas ainda não fornece um endpoint de caixa de entrada. Esta tela guarda apenas os envios da sessão do navegador.</p>
        </div>
      </aside>
    </div>
  );
}

export function ContactsWorkspace({
  api,
  onMessage,
  onCallStarted,
}: {
  api: EvolutionApi | null;
  onMessage: (recipient: string) => void;
  onCallStarted: () => void;
}) {
  const contactState = useContacts(api);
  const [query, setQuery] = useState("");
  const [notice, setNotice] = useState("");
  const normalizedQuery = query.trim().toLocaleLowerCase("pt-BR");
  const contacts = contactState.contacts.filter((contact) => {
    if (!normalizedQuery) return true;
    return `${contactName(contact)} ${contact.Jid}`.toLocaleLowerCase("pt-BR").includes(normalizedQuery);
  });

  const startCall = async (contact: EvolutionContact) => {
    if (!api) return;
    setNotice("");
    try {
      await api.startCall(displayPhone(contact.Jid));
      onCallStarted();
    } catch (cause) {
      setNotice(cause instanceof Error ? cause.message : "Falha ao iniciar chamada");
    }
  };

  return (
    <div className="contacts-workspace">
      <section className="hero card contacts-hero">
        <div><span className="eyebrow">Agenda WhatsApp</span><h1>Contatos da instância</h1><p>Pesquise, envie uma mensagem ou inicie uma chamada sem sair do Manager V2.</p></div>
        <div className="hero-metrics"><div><strong>{contactState.contacts.length}</strong><span>contatos</span></div><div><strong>{contactState.contacts.filter((contact) => contact.Found).length}</strong><span>sincronizados</span></div></div>
      </section>
      <section className="card contacts-panel">
        <div className="contacts-toolbar">
          <div className="message-search contacts-search"><span>⌕</span><input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Buscar por nome ou número" /></div>
          <button className="button secondary" type="button" disabled={contactState.loading} onClick={() => void contactState.refresh()}>Atualizar contatos</button>
        </div>
        {(contactState.error || notice) && <div className="alert error">{contactState.error || notice}</div>}
        <div className="contact-grid">
          {contacts.length === 0 ? <div className="table-empty">Nenhum contato encontrado.</div> : contacts.map((contact) => (
            <article className="contact-card" key={contact.Jid}>
              <span className="message-avatar xlarge">{contactInitials(contact)}</span>
              <div className="contact-card-copy"><h3>{contactName(contact)}</h3><p>{displayPhone(contact.Jid)}</p><small>{contact.BusinessName || contact.PushName || "Contato WhatsApp"}</small></div>
              <div className="contact-card-actions">
                <button className="button primary" type="button" onClick={() => onMessage(contact.Jid)}>✉ Conversar</button>
                <button className="button secondary" type="button" disabled={!api} onClick={() => void startCall(contact)}>☎ Ligar</button>
              </div>
            </article>
          ))}
        </div>
      </section>
    </div>
  );
}
