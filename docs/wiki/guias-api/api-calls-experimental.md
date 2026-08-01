# API de chamadas — integração experimental

Esta branch adiciona a primeira etapa da integração WaCalls/AstraCalls ao Evolution Go.

> **Estado atual:** a sinalização é real e pode fazer o aparelho remoto tocar, mas o transporte de áudio/WebRTC/SRTP ainda não foi conectado. Não use em produção.

Todas as rotas usam a autenticação normal da instância do Evolution.

## Ativar e consultar o runtime

```http
GET /call/status
```

Além de retornar as chamadas conhecidas, essa rota anexa o monitor de eventos ao `whatsmeow.Client` da instância. Nesta etapa experimental, execute-a ao menos uma vez após conectar ou reconectar a instância para monitorar chamadas recebidas antes de qualquer operação de chamada.

Exemplo de resposta:

```json
{
  "instanceId": "INSTANCE_ID",
  "connected": true,
  "calls": []
}
```

## Iniciar uma chamada

```http
POST /call/start
Content-Type: application/json
apikey: INSTANCE_TOKEN
```

```json
{
  "number": "5511999999999",
  "video": false
}
```

A resposta HTTP `201` contém o `id` da chamada e o estado inicial `ringing`.

```json
{
  "id": "32_CHARACTER_CALL_ID",
  "peer": "5511999999999:DEVICE@s.whatsapp.net",
  "direction": "outgoing",
  "state": "ringing",
  "video": false,
  "createdAt": "2026-07-31T23:00:00Z",
  "updatedAt": "2026-07-31T23:00:00Z"
}
```

## Encerrar uma chamada realizada

```http
DELETE /call/{callId}
apikey: INSTANCE_TOKEN
```

Nesta etapa, o encerramento por essa rota está habilitado apenas para chamadas de saída registradas pelo runtime.

## Rejeitar uma chamada recebida

A rota existente foi preservada:

```http
POST /call/reject
Content-Type: application/json
apikey: INSTANCE_TOKEN
```

```json
{
  "callCreator": "5511999999999@s.whatsapp.net",
  "callId": "CALL_ID"
}
```

## Estados rastreados

- `ringing`
- `connecting`
- `active`
- `ended`
- `failed`

O runtime escuta `CallOffer`, `CallOfferNotice`, `CallPreAccept`, `CallAccept`, `CallTransport`, `CallReject`, `CallTerminate`, `Disconnected` e `LoggedOut` no mesmo cliente utilizado pela mensageria.

## Limitações atuais

- sem áudio bidirecional;
- sem WebRTC para navegador;
- sem SRTP/relay do WhatsApp;
- aceite de chamadas recebidas ainda não exposto;
- o runtime ainda precisa ser ativado por uma rota de chamadas após reconexão;
- API e formatos podem mudar enquanto o PR estiver em rascunho.
