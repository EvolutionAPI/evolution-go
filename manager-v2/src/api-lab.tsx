import { useMemo, useState } from "react";
import type {
  ApiAuthMode,
  ApiExecutionResult,
  EvolutionApi,
  EvolutionConnection,
} from "./api";

type BodyMode = "none" | "json" | "multipart";

type ApiOperation = {
  id: string;
  category: string;
  title: string;
  method: string;
  path: string;
  auth: ApiAuthMode;
  bodyMode?: BodyMode;
  sample?: unknown;
  description: string;
  fileField?: string;
};

const quoted = { messageId: "", participant: "" };
const commonSend = { delay: 0, mentionAll: false, mentionedJid: [], formatJid: true, quoted };
const operation = (
  id: string,
  category: string,
  title: string,
  method: string,
  path: string,
  description: string,
  options: Partial<Pick<ApiOperation, "auth" | "bodyMode" | "sample" | "fileField">> = {},
): ApiOperation => ({
  id,
  category,
  title,
  method,
  path,
  description,
  auth: options.auth ?? "instance",
  bodyMode: options.bodyMode ?? (["GET", "DELETE"].includes(method) ? "none" : "json"),
  sample: options.sample,
  fileField: options.fileField,
});

export const API_OPERATIONS: ApiOperation[] = [
  operation("server-ok", "Servidor", "Verificar servidor", "GET", "/server/ok", "Confirma que o processo HTTP está respondendo.", { auth: "none" }),

  operation("instance-status", "Instância", "Status da instância", "GET", "/instance/status", "Retorna conexão e informações da sessão atual."),
  operation("instance-connect", "Instância", "Conectar sessão", "POST", "/instance/connect", "Inicia ou continua o processo de conexão.", { sample: {} }),
  operation("instance-qr", "Instância", "Obter QR Code", "GET", "/instance/qr", "Obtém o QR Code da sessão ainda não pareada."),
  operation("instance-pair", "Instância", "Parear por telefone", "POST", "/instance/pair", "Solicita código de pareamento pelo número.", { sample: { number: "5562999999999" } }),
  operation("instance-reconnect", "Instância", "Reconectar", "POST", "/instance/reconnect", "Reconecta o cliente WhatsApp.", { sample: {} }),
  operation("instance-disconnect", "Instância", "Desconectar", "POST", "/instance/disconnect", "Desconecta sem remover a sessão.", { sample: {} }),
  operation("instance-logout", "Instância", "Encerrar sessão", "DELETE", "/instance/logout", "Faz logout e remove a sessão WhatsApp."),
  operation("instance-all", "Administração", "Listar instâncias", "GET", "/instance/all", "Lista todas as instâncias usando a chave global.", { auth: "admin" }),
  operation("instance-info", "Administração", "Detalhes da instância", "GET", "/instance/info/:instanceId", "Consulta uma instância pelo ID.", { auth: "admin" }),
  operation("instance-force", "Administração", "Forçar reconexão", "POST", "/instance/forcereconnect/:instanceId", "Força reconexão administrativa.", { auth: "admin", sample: {} }),
  operation("instance-logs", "Administração", "Logs da instância", "GET", "/instance/logs/:instanceId", "Retorna logs da instância.", { auth: "admin" }),
  operation("instance-advanced-get", "Administração", "Ler configurações avançadas", "GET", "/instance/:instanceId/advanced-settings", "Consulta configurações avançadas."),
  operation("instance-advanced-put", "Administração", "Atualizar configurações avançadas", "PUT", "/instance/:instanceId/advanced-settings", "Atualiza opções avançadas com JSON livre.", { sample: {} }),

  operation("send-text", "Envio", "Texto comum", "POST", "/send/text", "Envia texto, com suporte a menções, resposta e atraso.", {
    sample: { number: "5562999999999", text: "Mensagem de teste", ...commonSend },
  }),
  operation("send-link", "Envio", "Link com prévia", "POST", "/send/link", "Envia texto com metadados de link.", {
    sample: { number: "5562999999999", text: "Confira https://evolution-api.com", title: "", url: "", description: "", imgUrl: "", ...commonSend },
  }),
  operation("send-media", "Envio", "Mídia por arquivo", "POST", "/send/media", "Envia imagem, vídeo, áudio ou documento via multipart.", {
    bodyMode: "multipart",
    fileField: "file",
    sample: { number: "5562999999999", type: "image", caption: "Teste de mídia", filename: "arquivo.jpg", delay: 0, mentionAll: false },
  }),
  operation("send-media-url", "Envio", "Mídia por URL", "POST", "/send/media", "Envia mídia usando URL pública ou base64.", {
    sample: { number: "5562999999999", type: "image", url: "https://picsum.photos/800/600", caption: "Imagem de teste", filename: "imagem.jpg", ...commonSend },
  }),
  operation("send-poll", "Envio", "Enquete", "POST", "/send/poll", "Envia uma enquete com duas ou mais opções.", {
    sample: { number: "5562999999999", question: "Qual opção você prefere?", maxAnswer: 1, options: ["Opção A", "Opção B"], ...commonSend },
  }),
  operation("send-sticker", "Envio", "Figurinha", "POST", "/send/sticker", "Converte uma imagem pública em figurinha WebP.", {
    sample: { number: "5562999999999", sticker: "https://picsum.photos/512/512", ...commonSend },
  }),
  operation("send-location", "Envio", "Localização", "POST", "/send/location", "Envia coordenadas, nome e endereço.", {
    sample: { number: "5562999999999", name: "Local de teste", latitude: -16.6869, longitude: -49.2648, address: "Goiânia - GO", ...commonSend },
  }),
  operation("send-contact", "Envio", "Contato vCard", "POST", "/send/contact", "Envia um cartão de contato.", {
    sample: { number: "5562999999999", vcard: { fullName: "Contato Teste", phone: "5562888888888", organization: "Evolution GO" }, ...commonSend },
  }),
  operation("send-button", "Envio", "Botões", "POST", "/send/button", "Testa botões reply, copy, URL, call ou PIX.", {
    sample: {
      number: "5562999999999",
      title: "Oferta especial",
      description: "Escolha uma opção",
      footer: "Evolution GO",
      buttons: [
        { type: "reply", displayText: "Quero saber mais", id: "btn_info" },
        { type: "reply", displayText: "Agora não", id: "btn_no" },
      ],
      ...commonSend,
    },
  }),
  operation("send-list", "Envio", "Lista interativa", "POST", "/send/list", "Envia lista de seleção única.", {
    sample: {
      number: "5562999999999",
      title: "Nossos planos",
      description: "Escolha uma opção",
      buttonText: "Abrir menu",
      footerText: "Evolution GO",
      sections: [{ title: "Planos", rows: [{ title: "Plano básico", description: "R$ 29,90/mês", rowId: "plan_basic" }] }],
      ...commonSend,
    },
  }),
  operation("send-carousel", "Envio", "Carrossel", "POST", "/send/carousel", "Envia cartões interativos com mídia e botões.", {
    sample: {
      number: "5562999999999",
      body: "Confira nossas novidades",
      footer: "Evolution GO",
      delay: 0,
      formatJid: true,
      quoted,
      cards: [{
        header: { title: "Oferta do dia", subtitle: "Somente hoje", imageUrl: "https://picsum.photos/seed/evolution/600/400" },
        body: { text: "Card de demonstração" },
        footer: "Por tempo limitado",
        buttons: [{ type: "REPLY", displayText: "Tenho interesse", id: "card_interest" }],
      }],
    },
  }),
  operation("send-status-text", "Envio", "Status de texto", "POST", "/send/status/text", "Publica um status de texto.", { sample: { text: "Status enviado pelo Evolution GO", id: "" } }),
  operation("send-status-media", "Envio", "Status com mídia", "POST", "/send/status/media", "Publica imagem ou vídeo no status.", {
    bodyMode: "multipart",
    fileField: "file",
    sample: { type: "image", caption: "Status de teste", id: "" },
  }),

  operation("user-check", "Usuário", "Verificar números", "POST", "/user/check", "Verifica se números estão registrados no WhatsApp.", { sample: { number: ["5562999999999"], formatJid: false } }),
  operation("user-info", "Usuário", "Informações do usuário", "POST", "/user/info", "Consulta status, dispositivos, LID e nome verificado.", { sample: { number: ["5562999999999"] } }),
  operation("user-avatar", "Usuário", "Avatar", "POST", "/user/avatar", "Consulta a foto de perfil.", { sample: { number: "5562999999999", preview: true } }),
  operation("user-contacts", "Usuário", "Contatos", "GET", "/user/contacts", "Lista contatos sincronizados."),
  operation("user-privacy", "Usuário", "Privacidade", "GET", "/user/privacy", "Consulta configurações de privacidade."),
  operation("user-block", "Usuário", "Bloquear contato", "POST", "/user/block", "Bloqueia um número.", { sample: { number: "5562999999999" } }),
  operation("user-unblock", "Usuário", "Desbloquear contato", "POST", "/user/unblock", "Remove um número da lista de bloqueio.", { sample: { number: "5562999999999" } }),
  operation("user-blocklist", "Usuário", "Lista de bloqueio", "GET", "/user/blocklist", "Obtém contatos bloqueados."),
  operation("user-profile-name", "Usuário", "Nome do perfil", "POST", "/user/profileName", "Atualiza o nome do perfil.", { sample: { name: "Evolution GO" } }),
  operation("user-profile-status", "Usuário", "Recado do perfil", "POST", "/user/profileStatus", "Atualiza o recado do perfil.", { sample: { status: "Disponível" } }),

  operation("message-presence", "Mensagem", "Presença no chat", "POST", "/message/presence", "Simula digitando, pausado ou gravando áudio.", { sample: { number: "5562999999999", presence: "composing", delay: 1000 } }),
  operation("message-react", "Mensagem", "Reagir à mensagem", "POST", "/message/react", "Adiciona ou remove reação.", { sample: { number: "5562999999999", messageId: "ID_DA_MENSAGEM", emoji: "👍" } }),
  operation("message-read", "Mensagem", "Marcar como lida", "POST", "/message/markread", "Marca uma mensagem como lida.", { sample: { number: "5562999999999", messageId: "ID_DA_MENSAGEM" } }),
  operation("message-played", "Mensagem", "Marcar áudio reproduzido", "POST", "/message/markplayed", "Marca áudio como reproduzido.", { sample: { number: "5562999999999", messageId: "ID_DA_MENSAGEM" } }),
  operation("message-status", "Mensagem", "Status da mensagem", "POST", "/message/status", "Consulta o status de uma mensagem.", { sample: { messageId: "ID_DA_MENSAGEM" } }),
  operation("message-edit", "Mensagem", "Editar mensagem", "POST", "/message/edit", "Edita uma mensagem enviada.", { sample: { number: "5562999999999", messageId: "ID_DA_MENSAGEM", text: "Texto editado" } }),
  operation("message-delete", "Mensagem", "Apagar para todos", "POST", "/message/delete", "Apaga uma mensagem enviada.", { sample: { number: "5562999999999", messageId: "ID_DA_MENSAGEM" } }),
  operation("message-download", "Mensagem", "Baixar mídia", "POST", "/message/downloadmedia", "Baixa mídia usando o objeto da mensagem.", { sample: {} }),

  operation("chat-archive", "Chat", "Arquivar", "POST", "/chat/archive", "Arquiva uma conversa.", { sample: { number: "5562999999999" } }),
  operation("chat-unarchive", "Chat", "Desarquivar", "POST", "/chat/unarchive", "Desarquiva uma conversa.", { sample: { number: "5562999999999" } }),
  operation("chat-mute", "Chat", "Silenciar", "POST", "/chat/mute", "Silencia uma conversa.", { sample: { number: "5562999999999", duration: 86400 } }),
  operation("chat-unmute", "Chat", "Remover silêncio", "POST", "/chat/unmute", "Remove o silêncio de uma conversa.", { sample: { number: "5562999999999" } }),
  operation("chat-history", "Chat", "Solicitar histórico", "POST", "/chat/history-sync", "Solicita sincronização de histórico.", { sample: {} }),

  operation("group-list", "Grupo", "Listar grupos", "GET", "/group/list", "Lista grupos conhecidos."),
  operation("group-info", "Grupo", "Informações do grupo", "POST", "/group/info", "Consulta metadados do grupo.", { sample: { number: "120363000000000000@g.us" } }),
  operation("group-create", "Grupo", "Criar grupo", "POST", "/group/create", "Cria um grupo com participantes.", { sample: { name: "Grupo de teste", participants: ["5562999999999"] } }),
  operation("group-participant", "Grupo", "Atualizar participante", "POST", "/group/participant", "Adiciona, remove, promove ou rebaixa participantes.", { sample: { number: "120363000000000000@g.us", participants: ["5562999999999"], action: "add" } }),
  operation("group-invite", "Grupo", "Link de convite", "POST", "/group/invitelink", "Obtém link de convite.", { sample: { number: "120363000000000000@g.us" } }),
  operation("group-join", "Grupo", "Entrar por convite", "POST", "/group/join", "Entra em grupo usando código ou URL.", { sample: { code: "CODIGO_DO_CONVITE" } }),
  operation("group-leave", "Grupo", "Sair do grupo", "POST", "/group/leave", "Sai de um grupo.", { sample: { number: "120363000000000000@g.us" } }),
  operation("group-name", "Grupo", "Alterar nome", "POST", "/group/name", "Atualiza o nome do grupo.", { sample: { number: "120363000000000000@g.us", name: "Novo nome" } }),
  operation("group-description", "Grupo", "Alterar descrição", "POST", "/group/description", "Atualiza a descrição.", { sample: { number: "120363000000000000@g.us", description: "Descrição de teste" } }),

  operation("call-status", "Chamadas", "Status das chamadas", "GET", "/call/status", "Lista chamadas em memória."),
  operation("call-start", "Chamadas", "Iniciar chamada", "POST", "/call/start", "Inicia chamada de voz ou vídeo.", { sample: { number: "5562999999999", video: false } }),
  operation("call-accept", "Chamadas", "Aceitar chamada", "POST", "/call/:callId/accept", "Aceita uma chamada recebida.", { sample: {} }),
  operation("call-webrtc-list", "Chamadas", "Listar WebRTC", "GET", "/call/:callId/webrtc", "Lista sessões WebRTC da chamada."),
  operation("call-terminate", "Chamadas", "Encerrar chamada", "DELETE", "/call/:callId", "Encerra uma chamada."),
  operation("call-reject", "Chamadas", "Recusar chamada", "POST", "/call/reject", "Recusa chamada recebida.", { sample: { number: "5562999999999", callCreator: "5562999999999@s.whatsapp.net", callId: "CALL_ID" } }),

  operation("label-list", "Labels", "Listar labels", "GET", "/label/list", "Lista etiquetas da instância."),
  operation("label-edit", "Labels", "Criar ou editar label", "POST", "/label/edit", "Cria ou edita uma etiqueta.", { sample: { id: "", name: "Cliente", color: 1, predefinedId: "" } }),
  operation("label-chat", "Labels", "Aplicar label ao chat", "POST", "/label/chat", "Aplica uma etiqueta à conversa.", { sample: { number: "5562999999999", labelId: "LABEL_ID" } }),
  operation("unlabel-chat", "Labels", "Remover label do chat", "POST", "/unlabel/chat", "Remove uma etiqueta da conversa.", { sample: { number: "5562999999999", labelId: "LABEL_ID" } }),

  operation("community-create", "Comunidade", "Criar comunidade", "POST", "/community/create", "Cria uma comunidade.", { sample: { name: "Comunidade de teste", description: "Criada pelo API Lab" } }),
  operation("community-add", "Comunidade", "Adicionar grupo", "POST", "/community/add", "Adiciona um grupo a uma comunidade.", { sample: { number: "120363000000000000@g.us", communityId: "120363000000000001@g.us" } }),
  operation("community-remove", "Comunidade", "Remover grupo", "POST", "/community/remove", "Remove um grupo da comunidade.", { sample: { number: "120363000000000000@g.us", communityId: "120363000000000001@g.us" } }),

  operation("newsletter-list", "Newsletter", "Listar canais", "GET", "/newsletter/list", "Lista newsletters/canais."),
  operation("newsletter-create", "Newsletter", "Criar canal", "POST", "/newsletter/create", "Cria uma newsletter.", { sample: { name: "Canal de teste", description: "Criado pelo Evolution GO" } }),
  operation("newsletter-info", "Newsletter", "Informações do canal", "POST", "/newsletter/info", "Consulta uma newsletter.", { sample: { newsletterId: "120363000000000000@newsletter" } }),
  operation("newsletter-link", "Newsletter", "Link do canal", "POST", "/newsletter/link", "Obtém link de convite.", { sample: { newsletterId: "120363000000000000@newsletter" } }),
  operation("newsletter-subscribe", "Newsletter", "Inscrever-se", "POST", "/newsletter/subscribe", "Inscreve a sessão em um canal.", { sample: { newsletterId: "120363000000000000@newsletter" } }),
  operation("newsletter-messages", "Newsletter", "Mensagens do canal", "POST", "/newsletter/messages", "Consulta mensagens recentes.", { sample: { newsletterId: "120363000000000000@newsletter", count: 20 } }),
  operation("poll-results", "Enquetes", "Resultados da enquete", "GET", "/polls/:pollMessageId/results", "Consulta votos de uma enquete."),
];

function replaceInstanceId(path: string, connection: EvolutionConnection): string {
  return path.replaceAll(":instanceId", connection.instanceId || "INSTANCE_ID");
}

function stringifySample(sample: unknown): string {
  return sample === undefined ? "" : JSON.stringify(sample, null, 2);
}

function appendFormValue(form: FormData, key: string, value: unknown): void {
  if (value === undefined || value === null) return;
  if (typeof value === "object") form.set(key, JSON.stringify(value));
  else form.set(key, String(value));
}

function responseText(result: ApiExecutionResult | null): string {
  if (!result) return "Execute uma operação para visualizar a resposta.";
  if (typeof result.data === "string") return result.data;
  return JSON.stringify(result.data, null, 2);
}

function buildCurl(
  connection: EvolutionConnection,
  operationValue: ApiOperation,
  path: string,
  body: string,
): string {
  const keyLabel = operationValue.auth === "admin" ? "SUA_CHAVE_GLOBAL" : operationValue.auth === "none" ? "" : "SUA_CHAVE_DA_INSTANCIA";
  const parts = [`curl -X ${operationValue.method} '${connection.baseUrl}${path}'`];
  if (keyLabel) parts.push(`-H 'apikey: ${keyLabel}'`);
  if (operationValue.bodyMode === "json" && body.trim()) {
    parts.push("-H 'Content-Type: application/json'");
    parts.push(`--data '${body.replaceAll("'", "'\\''")}'`);
  }
  if (operationValue.bodyMode === "multipart") {
    try {
      const parsed = JSON.parse(body || "{}") as Record<string, unknown>;
      Object.entries(parsed).forEach(([key, value]) => parts.push(`-F '${key}=${typeof value === "object" ? JSON.stringify(value) : String(value)}'`));
    } catch {
      // Keep the cURL useful even while the editor contains invalid JSON.
    }
    parts.push(`-F '${operationValue.fileField || "file"}=@/caminho/arquivo'`);
  }
  return parts.join(" \\\n  ");
}

export function ApiLab({
  api,
  connection,
}: {
  api: EvolutionApi | null;
  connection: EvolutionConnection;
}) {
  const [query, setQuery] = useState("");
  const [category, setCategory] = useState("Todos");
  const [selectedId, setSelectedId] = useState("send-text");
  const selected = API_OPERATIONS.find((item) => item.id === selectedId) ?? API_OPERATIONS[0];
  const [path, setPath] = useState(() => replaceInstanceId(selected.path, connection));
  const [body, setBody] = useState(() => stringifySample(selected.sample));
  const [auth, setAuth] = useState<ApiAuthMode>(selected.auth);
  const [file, setFile] = useState<File | null>(null);
  const [running, setRunning] = useState(false);
  const [error, setError] = useState("");
  const [result, setResult] = useState<ApiExecutionResult | null>(null);
  const [history, setHistory] = useState<Array<{ id: number; title: string; status: number; duration: number }>>([]);

  const categories = useMemo(() => ["Todos", ...Array.from(new Set(API_OPERATIONS.map((item) => item.category)))], []);
  const visible = useMemo(() => {
    const normalized = query.trim().toLocaleLowerCase("pt-BR");
    return API_OPERATIONS.filter((item) => {
      if (category !== "Todos" && item.category !== category) return false;
      if (!normalized) return true;
      return `${item.title} ${item.path} ${item.description}`.toLocaleLowerCase("pt-BR").includes(normalized);
    });
  }, [category, query]);

  const choose = (item: ApiOperation) => {
    setSelectedId(item.id);
    setPath(replaceInstanceId(item.path, connection));
    setBody(stringifySample(item.sample));
    setAuth(item.auth);
    setFile(null);
    setError("");
    setResult(null);
  };

  const run = async () => {
    if (!api || running) return;
    setRunning(true);
    setError("");
    try {
      let requestBody: BodyInit | null | undefined;
      if (selected.bodyMode === "json" && body.trim()) {
        requestBody = JSON.stringify(JSON.parse(body) as unknown);
      } else if (selected.bodyMode === "multipart") {
        const form = new FormData();
        const values = JSON.parse(body || "{}") as Record<string, unknown>;
        Object.entries(values).forEach(([key, value]) => appendFormValue(form, key, value));
        if (file) form.set(selected.fileField || "file", file, file.name);
        requestBody = form;
      }
      const response = await api.execute({ method: selected.method, path, auth, body: requestBody });
      setResult(response);
      setHistory((current) => [{ id: Date.now(), title: selected.title, status: response.status, duration: response.durationMs }, ...current].slice(0, 20));
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "Falha ao executar a requisição");
    } finally {
      setRunning(false);
    }
  };

  const curl = buildCurl(connection, { ...selected, auth }, path, body);

  return (
    <div className="api-lab-layout">
      <aside className="card api-catalog">
        <div className="section-heading">
          <div><span className="eyebrow">Catálogo</span><h2>{API_OPERATIONS.length} operações</h2></div>
        </div>
        <input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Buscar rota ou função" />
        <div className="api-category-strip">
          {categories.map((item) => <button type="button" className={category === item ? "active" : ""} key={item} onClick={() => setCategory(item)}>{item}</button>)}
        </div>
        <div className="api-operation-list">
          {visible.map((item) => (
            <button type="button" className={item.id === selected.id ? "selected" : ""} key={item.id} onClick={() => choose(item)}>
              <span className={`http-method method-${item.method.toLowerCase()}`}>{item.method}</span>
              <span><strong>{item.title}</strong><small>{item.path}</small></span>
            </button>
          ))}
        </div>
      </aside>

      <section className="api-console">
        <section className="card api-request-card">
          <div className="section-heading">
            <div><span className="eyebrow">{selected.category}</span><h2>{selected.title}</h2></div>
            <span className={`http-method method-${selected.method.toLowerCase()}`}>{selected.method}</span>
          </div>
          <p className="api-description">{selected.description}</p>
          <div className="api-route-row">
            <select value={selected.method} disabled aria-label="Método HTTP"><option>{selected.method}</option></select>
            <input value={path} onChange={(event) => setPath(event.target.value)} aria-label="Caminho da API" />
            <select value={auth} onChange={(event) => setAuth(event.target.value as ApiAuthMode)} aria-label="Tipo de autenticação">
              <option value="instance">Chave da instância</option>
              <option value="admin">Chave global</option>
              <option value="none">Sem autenticação</option>
            </select>
          </div>

          {selected.bodyMode !== "none" && (
            <label className="api-editor-label">
              <span>{selected.bodyMode === "multipart" ? "Campos multipart em JSON" : "Corpo JSON"}</span>
              <textarea value={body} onChange={(event) => setBody(event.target.value)} spellCheck={false} rows={16} />
            </label>
          )}
          {selected.bodyMode === "multipart" && (
            <label className="api-file-field">
              <span>Arquivo ({selected.fileField || "file"})</span>
              <input type="file" onChange={(event) => setFile(event.target.files?.[0] ?? null)} />
              <small>{file ? `${file.name} · ${Math.ceil(file.size / 1024)} KB` : "Selecione um arquivo quando a rota exigir upload."}</small>
            </label>
          )}

          {error && <div className="alert error">{error}</div>}
          <div className="api-actions">
            <button type="button" className="button secondary" onClick={() => void navigator.clipboard.writeText(curl)}>Copiar cURL</button>
            <button type="button" className="button primary" disabled={!api || running} onClick={() => void run()}>{running ? "Executando…" : "Executar teste"}</button>
          </div>
        </section>

        <section className="card api-response-card">
          <div className="section-heading">
            <div><span className="eyebrow">Resposta</span><h2>{result ? `${result.status} ${result.statusText}` : "Aguardando execução"}</h2></div>
            {result && <span className={`response-status ${result.ok ? "ok" : "failed"}`}>{result.durationMs} ms</span>}
          </div>
          {result && <div className="api-response-meta"><span>{result.url}</span><span>{result.ok ? "Sucesso" : "Erro HTTP"}</span></div>}
          <pre>{responseText(result)}</pre>
        </section>

        <section className="card api-curl-card">
          <div className="section-heading"><div><span className="eyebrow">Reprodução</span><h2>cURL equivalente</h2></div></div>
          <pre>{curl}</pre>
        </section>
      </section>

      <aside className="card api-history">
        <div className="section-heading"><div><span className="eyebrow">Sessão</span><h2>Últimos testes</h2></div></div>
        {history.length === 0 ? <p>Nenhuma requisição executada.</p> : history.map((item) => (
          <div className="api-history-item" key={item.id}>
            <strong>{item.title}</strong>
            <span className={item.status >= 200 && item.status < 300 ? "ok" : "failed"}>{item.status}</span>
            <small>{item.duration} ms</small>
          </div>
        ))}
      </aside>
    </div>
  );
}
