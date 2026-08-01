# API de chamadas — integração experimental

Esta branch adiciona a integração experimental WaCalls/AstraCalls ao Evolution Go.

> **Estado atual:** a sinalização é real e permite iniciar, receber, aceitar, rejeitar e encerrar chamadas no nível do protocolo. A variante experimental também consegue negociar DataChannels com os relays usando Pion. RTP/SRTP e áudio ainda não foram conectados. Não use em produção.

Todas as rotas usam a autenticação normal da instância do Evolution.

## Consultar o runtime

```http
GET /call/status
```

Os monitores de chamada são anexados automaticamente quando o Evolution cria o `whatsmeow.Client`. Durante reconexão, logout ou remoção da instância, handlers, DataChannels e material privado são removidos antes que o novo cliente seja registrado. Não é necessário chamar `/call/status` para ativar o monitoramento.

Exemplo de resposta:

```json
{
  "instanceId": "INSTANCE_ID",
  "connected": true,
  "calls": []
}
```

Chaves de chamada, JIDs internos de dispositivos, tokens de relay e outros dados privados não fazem parte dessa resposta. Eles ficam somente em memória e são apagados quando a chamada termina, é rejeitada ou a sessão é desconectada.

Instâncias configuradas com rejeição automática continuam rastreando o estado público da chamada. Elas não descriptografam ofertas recebidas nem enviam `preaccept`, mas continuam podendo iniciar chamadas e armazenar sua negociação privada de saída.

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

A oferta é enviada como uma consulta do protocolo. Quando o ACK contém relays estruturados, a chave gerada, os participantes e os candidatos são copiados para o registro privado da chamada antes de o resultado transitório ser sobrescrito.

A resposta HTTP `201` contém somente o estado público:

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

`CallAccept` não marca mais a chamada como `active` sozinho. Na variante Pion, `active` é publicado somente depois que um DataChannel de relay abre e o callback `media_connected` é aplicado.

Se a preparação criptográfica do evento ainda não terminou, a API retorna um erro informando que a chamada ainda não está pronta para aceite.

## Encerrar uma chamada

```http
DELETE /call/{callId}
apikey: INSTANCE_TOKEN
```

A rota envia `terminate` para chamadas realizadas e recebidas. Em seguida, o runtime muda o estado público para `ended` e remove chave, candidatos, DataChannels e demais dados privados da chamada.

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

O snapshot público usa:

- `ringing`
- `connecting`
- `active`
- `ended`
- `failed`

Internamente, a negociação usa uma máquina de estados estrita com:

- `initiating`
- `ringing`
- `incoming_ringing`
- `connecting`
- `active`
- `on_hold`
- `ended`

Transições inválidas, como marcar mídia conectada antes do aceite ou aceitar remotamente uma chamada recebida, são rejeitadas sem alterar o estado.

## Relay e transporte Pion

O módulo de sinalização reconhece os dois formatos encontrados nas respostas do WhatsApp:

- candidatos com atributos diretos, como `ip`, `port`, `token`, `relay-id` e `c2r-rtt`;
- respostas estruturadas `te2`, com tokens binários, `auth_token`, participantes, UUID, PIDs, HBH key, protocolo e endereço codificado em seis bytes.

Os candidatos são ordenados pelo menor RTT e associados ao material privado pelo `callId`. Atualizações posteriores recebidas em `CallTransport` substituem os candidatos anteriores sem apagar a chave da chamada.

A implementação experimental Pion inclui:

- PeerConnection e DataChannel `wa-web-call` por relay;
- transformação do SDP para credenciais e fingerprint do relay;
- registro STUN com subscriptions de SSRC;
- requisição de allocation;
- tentativas adicionais de registro;
- keepalive proprietário do WhatsApp;
- broadcast e recebimento de frames do DataChannel;
- timeout, fechamento e limpeza de buffers;
- SSRC determinístico derivado de `callId` e JID do dispositivo.

A build padrão continua usando um transportador sem rede. Para compilar a variante experimental:

```bash
go build -tags=voip_pion ./cmd/evolution-go
```

O workflow do PR testa permanentemente as duas variantes:

```bash
go test -race ./pkg/call/...
go test -race -tags=voip_pion ./pkg/call/...
```

## Limitações atuais

- sem áudio bidirecional;
- os frames recebidos pelo DataChannel ainda não são processados por uma sessão RTP/SRTP;
- sem derivação e instalação das chaves SRTP;
- sem codecs Opus ou MLow conectados ao runtime;
- sem WebRTC para navegador;
- a conexão real com um relay WhatsApp ainda precisa ser validada de ponta a ponta com uma conta conectada;
- as chaves ficam somente em memória e não sobrevivem a reinícios;
- API e formatos podem mudar enquanto o PR estiver em rascunho.
