# API de chamadas — integração experimental

Esta branch adiciona a primeira etapa da integração WaCalls/AstraCalls ao Evolution Go.

> **Estado atual:** a sinalização é real e permite iniciar, receber, aceitar, rejeitar e encerrar chamadas no nível do protocolo. O transporte de áudio/WebRTC/SRTP ainda não foi conectado. Não use em produção.

Todas as rotas usam a autenticação normal da instância do Evolution.

## Ativar e consultar o runtime

```http
GET /call/status
```

Além de retornar as chamadas conhecidas, essa rota anexa os monitores de eventos e de material criptográfico ao `whatsmeow.Client` da instância. Nesta etapa experimental, execute-a ao menos uma vez após conectar ou reconectar a instância para monitorar chamadas recebidas.

Exemplo de resposta:

```json
{
  "instanceId": "INSTANCE_ID",
  "connected": true,
  "calls": []
}
```

Chaves de chamada, JIDs internos de dispositivos e outros dados privados não fazem parte dessa resposta. Eles ficam somente em memória e são zerados quando a chamada termina, é rejeitada ou a sessão é desconectada.

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

## Aceitar uma chamada recebida

Quando uma chamada recebida aparecer em `GET /call/status`, use:

```http
POST /call/{callId}/accept
apikey: INSTANCE_TOKEN
```

O runtime descriptografa a chave recebida usando a sessão Signal já autenticada, envia `preaccept` automaticamente e mantém o material somente na memória privada. O endpoint envia a stanza `accept` e retorna a chamada no estado `connecting`.

```json
{
  "id": "CALL_ID",
  "peer": "5511999999999@s.whatsapp.net",
  "direction": "incoming",
  "state": "connecting",
  "video": false
}
```

Se a preparação criptográfica do evento ainda não terminou, a API retorna um erro informando que a chamada ainda não está pronta para aceite.

## Encerrar uma chamada

```http
DELETE /call/{callId}
apikey: INSTANCE_TOKEN
```

A rota envia `terminate` tanto para chamadas realizadas quanto para chamadas recebidas cujo material privado ainda está disponível em memória. Em seguida, o runtime muda o estado para `ended` e apaga a chave da chamada recebida.

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
- aceitar a sinalização não estabelece o caminho de mídia;
- as chaves ficam somente em memória e não sobrevivem a reinícios;
- o runtime ainda precisa ser ativado por uma rota de chamadas após reconexão;
- API e formatos podem mudar enquanto o PR estiver em rascunho.
