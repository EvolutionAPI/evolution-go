import type { ApiAuthMode } from "./api";

export type BodyMode = "none" | "json" | "multipart";

export type ApiOperation = {
  id: string;
  category: string;
  title: string;
  method: string;
  path: string;
  auth: ApiAuthMode;
  bodyMode: BodyMode;
  sample?: unknown;
  description: string;
  fileField?: string;
};

const quoted = { messageId: "", participant: "" };
const commonSend = { delay: 0, mentionAll: false, mentionedJid: [], formatJid: true, quoted };

function op(
  id: string,
  category: string,
  title: string,
  method: string,
  path: string,
  description: string,
  options: Partial<Pick<ApiOperation, "auth" | "bodyMode" | "sample" | "fileField">> = {},
): ApiOperation {
  return {
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
  };
}

export const API_OPERATIONS: ApiOperation[] = [
  op("custom", "Personalizado", "Requisição personalizada", "POST", "/send/text", "Edite método, caminho, autenticação e corpo para testar qualquer rota.", { sample: { number: "5562999999999", text: "Teste personalizado" } }),
  op("server-ok", "Servidor", "Verificar servidor", "GET", "/server/ok", "Confirma que o servidor HTTP está respondendo.", { auth: "none" }),

  op("instance-create", "Instância", "Criar instância", "POST", "/instance/create", "Cria uma instância e define sua API key.", { auth: "admin", sample: { instanceId: "minha-instancia", name: "Minha instância", token: "CHAVE_DA_INSTANCIA", proxy: null, advancedSettings: null } }),
  op("instance-all", "Instância", "Listar instâncias", "GET", "/instance/all", "Lista todas as instâncias.", { auth: "admin" }),
  op("instance-info", "Instância", "Detalhes da instância", "GET", "/instance/info/:instanceId", "Consulta uma instância pelo ID.", { auth: "admin" }),
  op("instance-delete", "Instância", "Excluir instância", "DELETE", "/instance/delete/:instanceId", "Exclui permanentemente uma instância.", { auth: "admin" }),
  op("instance-proxy-set", "Instância", "Configurar proxy", "POST", "/instance/proxy/:instanceId", "Configura proxy da instância.", { auth: "admin", sample: { protocol: "http", host: "127.0.0.1", port: "8080", username: "usuario", password: "senha" } }),
  op("instance-proxy-delete", "Instância", "Remover proxy", "DELETE", "/instance/proxy/:instanceId", "Remove a configuração de proxy.", { auth: "admin" }),
  op("instance-force", "Instância", "Forçar reconexão", "POST", "/instance/forcereconnect/:instanceId", "Força a atualização e reconexão administrativa.", { auth: "admin", sample: { number: "5562999999999" } }),
  op("instance-logs", "Instância", "Logs da instância", "GET", "/instance/logs/:instanceId", "Consulta logs; parâmetros de data, nível e limite podem ser adicionados na query string.", { auth: "admin" }),
  op("instance-connect", "Instância", "Conectar", "POST", "/instance/connect", "Inicia a conexão da sessão.", { sample: { webhookUrl: "", subscribe: [], immediate: false, phone: "", rabbitmqEnable: "", webSocketEnable: "", natsEnable: "" } }),
  op("instance-status", "Instância", "Status", "GET", "/instance/status", "Consulta estado conectado/logado."),
  op("instance-qr", "Instância", "QR Code", "GET", "/instance/qr", "Obtém QR Code, código ou etapa de passkey."),
  op("instance-pair", "Instância", "Parear por telefone", "POST", "/instance/pair", "Solicita código de pareamento.", { sample: { phone: "5562999999999", subscribe: [] } }),
  op("instance-disconnect", "Instância", "Desconectar", "POST", "/instance/disconnect", "Desconecta sem apagar a sessão.", { sample: {} }),
  op("instance-reconnect", "Instância", "Reconectar", "POST", "/instance/reconnect", "Reinicia a conexão preservando credenciais.", { sample: {} }),
  op("instance-logout", "Instância", "Logout", "DELETE", "/instance/logout", "Desvincula o aparelho e remove a sessão."),
  op("instance-advanced-get", "Instância", "Ler configurações avançadas", "GET", "/instance/:instanceId/advanced-settings", "Consulta configurações avançadas."),
  op("instance-advanced-put", "Instância", "Atualizar configurações avançadas", "PUT", "/instance/:instanceId/advanced-settings", "Atualiza configurações avançadas com JSON livre.", { sample: {} }),

  op("send-text", "Envio", "Texto comum", "POST", "/send/text", "Envia texto com menções, resposta, atraso e encaminhamento.", { sample: { number: "5562999999999", text: "Mensagem de teste", forwardingScore: 0, ...commonSend } }),
  op("send-link", "Envio", "Link com prévia", "POST", "/send/link", "Envia link e gera metadados de prévia.", { sample: { number: "5562999999999", text: "Confira https://evolution-api.com", title: "", url: "", description: "", imgUrl: "", ...commonSend } }),
  op("send-media-file", "Envio", "Mídia por arquivo", "POST", "/send/media", "Upload multipart de imagem, vídeo, áudio ou documento.", { bodyMode: "multipart", fileField: "file", sample: { number: "5562999999999", type: "image", caption: "Teste de mídia", filename: "arquivo.jpg", delay: 0, mentionAll: false } }),
  op("send-media-url", "Envio", "Mídia por URL/base64", "POST", "/send/media", "Envia mídia por URL pública ou base64.", { sample: { number: "5562999999999", url: "https://picsum.photos/800/600", type: "image", caption: "Imagem de teste", filename: "imagem.jpg", forwardingScore: 0, ...commonSend } }),
  op("send-poll", "Envio", "Enquete", "POST", "/send/poll", "Envia enquete com duas ou mais opções.", { sample: { number: "5562999999999", question: "Qual opção você prefere?", maxAnswer: 1, options: ["Opção A", "Opção B"], ...commonSend } }),
  op("send-sticker", "Envio", "Figurinha", "POST", "/send/sticker", "Baixa uma imagem pública e envia como WebP.", { sample: { number: "5562999999999", sticker: "https://picsum.photos/512/512", ...commonSend } }),
  op("send-location", "Envio", "Localização", "POST", "/send/location", "Envia latitude, longitude, nome e endereço.", { sample: { number: "5562999999999", name: "Local de teste", latitude: -16.6869, longitude: -49.2648, address: "Goiânia - GO", ...commonSend } }),
  op("send-contact", "Envio", "Contato vCard", "POST", "/send/contact", "Envia um contato em formato vCard.", { sample: { number: "5562999999999", vcard: { fullName: "Contato Teste", phone: "5562888888888", organization: "Evolution GO" }, ...commonSend } }),
  op("send-button", "Envio", "Botões", "POST", "/send/button", "Testa reply, copy, URL, call e PIX.", { sample: { number: "5562999999999", title: "Oferta especial", description: "Escolha uma opção", footer: "Evolution GO", buttons: [{ type: "reply", displayText: "Quero saber mais", id: "btn_info" }, { type: "reply", displayText: "Agora não", id: "btn_no" }], imageUrl: "", videoUrl: "", ...commonSend } }),
  op("send-list", "Envio", "Lista interativa", "POST", "/send/list", "Envia uma lista de seleção única.", { sample: { number: "5562999999999", title: "Nossos planos", description: "Escolha uma opção", buttonText: "Abrir menu", footerText: "Evolution GO", sections: [{ title: "Planos", rows: [{ title: "Plano básico", description: "R$ 29,90/mês", rowId: "plan_basic" }] }], ...commonSend } }),
  op("send-carousel", "Envio", "Carrossel", "POST", "/send/carousel", "Envia cartões interativos com mídia e botões.", { sample: { number: "5562999999999", body: "Confira nossas novidades", footer: "Evolution GO", delay: 0, formatJid: true, quoted, cards: [{ header: { title: "Oferta do dia", subtitle: "Somente hoje", imageUrl: "https://picsum.photos/seed/evolution/600/400", videoUrl: "" }, body: { text: "Card de demonstração" }, footer: "Por tempo limitado", buttons: [{ type: "REPLY", displayText: "Tenho interesse", id: "card_interest", copyCode: "" }] }] } }),
  op("send-status-text", "Envio", "Status de texto", "POST", "/send/status/text", "Publica texto no status.", { sample: { text: "Status enviado pelo Evolution GO", id: "" } }),
  op("send-status-media-file", "Envio", "Status com arquivo", "POST", "/send/status/media", "Publica imagem ou vídeo por multipart.", { bodyMode: "multipart", fileField: "file", sample: { type: "image", caption: "Status de teste", id: "" } }),
  op("send-status-media-url", "Envio", "Status por URL", "POST", "/send/status/media", "Publica imagem ou vídeo por URL.", { sample: { type: "image", url: "https://picsum.photos/1080/1920", caption: "Status de teste", id: "" } }),

  op("user-info", "Usuário", "Informações do usuário", "POST", "/user/info", "Consulta status, dispositivos, LID e nome verificado.", { sample: { number: ["5562999999999"] } }),
  op("user-check", "Usuário", "Verificar números", "POST", "/user/check", "Verifica se números estão no WhatsApp.", { sample: { number: ["5562999999999"], formatJid: false } }),
  op("user-avatar", "Usuário", "Avatar", "POST", "/user/avatar", "Consulta foto de perfil.", { sample: { number: "5562999999999", preview: true } }),
  op("user-contacts", "Usuário", "Contatos", "GET", "/user/contacts", "Lista contatos sincronizados."),
  op("user-privacy-get", "Usuário", "Ler privacidade", "GET", "/user/privacy", "Consulta as configurações de privacidade."),
  op("user-privacy-set", "Usuário", "Atualizar privacidade", "POST", "/user/privacy", "Atualiza todas as opções de privacidade.", { sample: { groupAdd: "all", lastSeen: "all", status: "all", profile: "all", readReceipts: "all", callAdd: "all", online: "all" } }),
  op("user-block", "Usuário", "Bloquear", "POST", "/user/block", "Bloqueia um contato.", { sample: { number: "5562999999999" } }),
  op("user-unblock", "Usuário", "Desbloquear", "POST", "/user/unblock", "Desbloqueia um contato.", { sample: { number: "5562999999999" } }),
  op("user-blocklist", "Usuário", "Lista de bloqueio", "GET", "/user/blocklist", "Lista contatos bloqueados."),
  op("user-profile-picture", "Usuário", "Foto do perfil", "POST", "/user/profilePicture", "Atualiza foto por URL ou base64 conforme o backend.", { sample: { image: "BASE64_OU_URL" } }),
  op("user-profile-name", "Usuário", "Nome do perfil", "POST", "/user/profileName", "Atualiza o nome do perfil.", { sample: { name: "Evolution GO" } }),
  op("user-profile-status", "Usuário", "Recado do perfil", "POST", "/user/profileStatus", "Atualiza o recado do perfil.", { sample: { status: "Disponível" } }),

  op("message-react", "Mensagem", "Reagir", "POST", "/message/react", "Adiciona ou remove reação.", { sample: { number: "5562999999999", messageId: "ID_DA_MENSAGEM", emoji: "👍" } }),
  op("message-presence", "Mensagem", "Presença no chat", "POST", "/message/presence", "Envia composing, paused ou recording.", { sample: { number: "5562999999999", presence: "composing", delay: 1000 } }),
  op("message-read", "Mensagem", "Marcar como lida", "POST", "/message/markread", "Marca mensagem como lida.", { sample: { number: "5562999999999", messageId: "ID_DA_MENSAGEM" } }),
  op("message-played", "Mensagem", "Marcar como reproduzida", "POST", "/message/markplayed", "Marca mídia de áudio como reproduzida.", { sample: { number: "5562999999999", messageId: "ID_DA_MENSAGEM" } }),
  op("message-download", "Mensagem", "Baixar mídia", "POST", "/message/downloadmedia", "Baixa mídia usando os metadados da mensagem.", { sample: {} }),
  op("message-status", "Mensagem", "Status da mensagem", "POST", "/message/status", "Consulta status por ID.", { sample: { messageId: "ID_DA_MENSAGEM" } }),
  op("message-delete", "Mensagem", "Apagar para todos", "POST", "/message/delete", "Apaga uma mensagem enviada.", { sample: { number: "5562999999999", messageId: "ID_DA_MENSAGEM" } }),
  op("message-edit", "Mensagem", "Editar", "POST", "/message/edit", "Edita texto enviado.", { sample: { number: "5562999999999", messageId: "ID_DA_MENSAGEM", text: "Texto editado" } }),

  op("chat-pin", "Chat", "Fixar", "POST", "/chat/pin", "Fixa uma conversa.", { sample: { number: "5562999999999" } }),
  op("chat-unpin", "Chat", "Desafixar", "POST", "/chat/unpin", "Remove conversa dos fixados.", { sample: { number: "5562999999999" } }),
  op("chat-archive", "Chat", "Arquivar", "POST", "/chat/archive", "Arquiva uma conversa.", { sample: { number: "5562999999999" } }),
  op("chat-unarchive", "Chat", "Desarquivar", "POST", "/chat/unarchive", "Desarquiva uma conversa.", { sample: { number: "5562999999999" } }),
  op("chat-mute", "Chat", "Silenciar", "POST", "/chat/mute", "Silencia uma conversa.", { sample: { number: "5562999999999", duration: 86400 } }),
  op("chat-unmute", "Chat", "Remover silêncio", "POST", "/chat/unmute", "Remove o silêncio.", { sample: { number: "5562999999999" } }),
  op("chat-history", "Chat", "Sincronizar histórico", "POST", "/chat/history-sync", "Solicita history sync.", { sample: {} }),

  op("group-list", "Grupo", "Listar grupos", "GET", "/group/list", "Lista grupos conhecidos."),
  op("group-info", "Grupo", "Informações", "POST", "/group/info", "Consulta metadados do grupo.", { sample: { number: "120363000000000000@g.us" } }),
  op("group-invite", "Grupo", "Link de convite", "POST", "/group/invitelink", "Obtém link de convite.", { sample: { number: "120363000000000000@g.us" } }),
  op("group-photo", "Grupo", "Foto", "POST", "/group/photo", "Atualiza foto do grupo.", { sample: { number: "120363000000000000@g.us", image: "BASE64_OU_URL" } }),
  op("group-name", "Grupo", "Nome", "POST", "/group/name", "Altera nome do grupo.", { sample: { number: "120363000000000000@g.us", name: "Novo nome" } }),
  op("group-description", "Grupo", "Descrição", "POST", "/group/description", "Altera descrição do grupo.", { sample: { number: "120363000000000000@g.us", description: "Descrição de teste" } }),
  op("group-create", "Grupo", "Criar grupo", "POST", "/group/create", "Cria grupo com participantes.", { sample: { name: "Grupo de teste", participants: ["5562999999999"] } }),
  op("group-participant", "Grupo", "Participantes", "POST", "/group/participant", "Adiciona, remove, promove ou rebaixa.", { sample: { number: "120363000000000000@g.us", participants: ["5562999999999"], action: "add" } }),
  op("group-myall", "Grupo", "Meus grupos", "GET", "/group/myall", "Consulta grupos da conta."),
  op("group-join", "Grupo", "Entrar por convite", "POST", "/group/join", "Entra por URL ou código.", { sample: { code: "CODIGO_DO_CONVITE" } }),
  op("group-leave", "Grupo", "Sair", "POST", "/group/leave", "Sai de um grupo.", { sample: { number: "120363000000000000@g.us" } }),
  op("group-settings", "Grupo", "Configurações", "POST", "/group/settings", "Atualiza configurações do grupo.", { sample: { number: "120363000000000000@g.us", action: "announcement" } }),

  op("call-status", "Chamadas", "Status", "GET", "/call/status", "Lista chamadas da instância."),
  op("call-start", "Chamadas", "Iniciar", "POST", "/call/start", "Inicia chamada de voz ou vídeo.", { sample: { number: "5562999999999", video: false } }),
  op("call-accept", "Chamadas", "Aceitar", "POST", "/call/:callId/accept", "Aceita chamada recebida.", { sample: {} }),
  op("call-webrtc-create", "Chamadas", "Criar WebRTC", "POST", "/call/:callId/webrtc", "Cria sessão WebRTC a partir de uma oferta SDP.", { sample: { offer: { type: "offer", sdp: "COLE_O_SDP" } } }),
  op("call-webrtc-list", "Chamadas", "Listar WebRTC", "GET", "/call/:callId/webrtc", "Lista sessões WebRTC."),
  op("call-webrtc-close", "Chamadas", "Fechar WebRTC", "DELETE", "/call/:callId/webrtc/:sessionId", "Fecha uma sessão WebRTC."),
  op("call-terminate", "Chamadas", "Encerrar", "DELETE", "/call/:callId", "Encerra uma chamada."),
  op("call-reject", "Chamadas", "Recusar", "POST", "/call/reject", "Recusa chamada recebida.", { sample: { number: "5562999999999", callCreator: "5562999999999@s.whatsapp.net", callId: "CALL_ID" } }),

  op("community-create", "Comunidade", "Criar", "POST", "/community/create", "Cria comunidade.", { sample: { name: "Comunidade de teste", description: "Criada pelo API Lab" } }),
  op("community-add", "Comunidade", "Adicionar grupo", "POST", "/community/add", "Adiciona grupo à comunidade.", { sample: { number: "120363000000000000@g.us", communityId: "120363000000000001@g.us" } }),
  op("community-remove", "Comunidade", "Remover grupo", "POST", "/community/remove", "Remove grupo da comunidade.", { sample: { number: "120363000000000000@g.us", communityId: "120363000000000001@g.us" } }),

  op("label-chat", "Labels", "Aplicar no chat", "POST", "/label/chat", "Aplica label à conversa.", { sample: { number: "5562999999999", labelId: "LABEL_ID" } }),
  op("label-message", "Labels", "Aplicar na mensagem", "POST", "/label/message", "Aplica label à mensagem.", { sample: { messageId: "ID_DA_MENSAGEM", labelId: "LABEL_ID" } }),
  op("label-edit", "Labels", "Criar ou editar", "POST", "/label/edit", "Cria ou edita label.", { sample: { id: "", name: "Cliente", color: 1, predefinedId: "" } }),
  op("label-list", "Labels", "Listar", "GET", "/label/list", "Lista labels."),
  op("unlabel-chat", "Labels", "Remover do chat", "POST", "/unlabel/chat", "Remove label da conversa.", { sample: { number: "5562999999999", labelId: "LABEL_ID" } }),
  op("unlabel-message", "Labels", "Remover da mensagem", "POST", "/unlabel/message", "Remove label da mensagem.", { sample: { messageId: "ID_DA_MENSAGEM", labelId: "LABEL_ID" } }),

  op("newsletter-create", "Newsletter", "Criar canal", "POST", "/newsletter/create", "Cria newsletter/canal.", { sample: { name: "Canal de teste", description: "Criado pelo Evolution GO" } }),
  op("newsletter-list", "Newsletter", "Listar canais", "GET", "/newsletter/list", "Lista canais."),
  op("newsletter-info", "Newsletter", "Informações", "POST", "/newsletter/info", "Consulta canal.", { sample: { newsletterId: "120363000000000000@newsletter" } }),
  op("newsletter-link", "Newsletter", "Link", "POST", "/newsletter/link", "Obtém link do canal.", { sample: { newsletterId: "120363000000000000@newsletter" } }),
  op("newsletter-subscribe", "Newsletter", "Inscrever-se", "POST", "/newsletter/subscribe", "Inscreve a conta no canal.", { sample: { newsletterId: "120363000000000000@newsletter" } }),
  op("newsletter-messages", "Newsletter", "Mensagens", "POST", "/newsletter/messages", "Consulta mensagens recentes.", { sample: { newsletterId: "120363000000000000@newsletter", count: 20 } }),

  op("poll-results", "Enquetes", "Resultados", "GET", "/polls/:pollMessageId/results", "Consulta votos de uma enquete."),
];
