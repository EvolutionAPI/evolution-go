# API de chamadas — integração experimental

Esta branch adiciona a integração experimental WaCalls/AstraCalls ao Evolution Go.

> **Estado atual:** a sinalização é real e permite iniciar, receber, aceitar, rejeitar e encerrar chamadas no nível do protocolo. Chaves e metadados de relay já são associados ao estado privado de cada chamada, mas o transporte de áudio/WebRTC/SRTP ainda não foi conectado. Não use em produção.

Todas as rotas usam a autenticação normal da instância do Evolution.

## Consultar o runtime

```http
GET /call/status
```

Os monitores de chamada são anexados automaticamente quando o Evolution cria o `whatsmeow.Client`. Durante reconexão, logout ou remoção da instância, os handlers anteriores são removidos e o material privado é apagado antes que o novo cliente seja registrado. Não é necessário chamar `/call/status` para ativar o monitoramento.

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

Se a preparação criptográfica do evento ainda não terminou, a API retorna um erro informando que a chamada ainda não está pronta para aceite.

## Encerrar uma chamada

```http
DELETE /call/{callId}
apikey: INSTANCE_TOKEN
```

A rota envia `terminate` para chamadas realizadas e recebidas. Em seguida, o runtime muda o estado público para `ended` e remove a chave e os dados privados da chamada.

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

Internamente, a negociação também usa uma máquina de estados estrita com:

- `initiating`
- `ringing`
- `incoming_ringing`
- `connecting`
- `active`
- `on_hold`
- `ended`

Transições inválidas, como marcar mídia conectada antes do aceite ou aceitar remotamente uma chamada recebida, são rejeitadas sem alterar o estado.

## Relay e transporte

O módulo de sinalização reconhece os dois formatos encontrados nas respostas do WhatsApp:

- candidatos com atributos diretos, como `ip`, `port`, `token`, `relay-id` e `c2r-rtt`;
- respostas estruturadas `te2`, com tokens binários, `auth_token`, participantes, UUID, PIDs, HBH key, protocolo e endereço codificado em seis bytes.

Os candidatos são ordenados pelo menor RTT e associados ao material privado pelo `callId`. Atualizações posteriores recebidas em `CallTransport` substituem os candidatos anteriores sem apagar a chave da chamada.

O pacote `pkg/call/voip/transport` define o contrato usado pelo futuro gerenciador SCTP. Ele:

- converte somente candidatos UDP utilizáveis;
- aplica a porta padrão do relay;
- remove duplicados;
- copia tokens binários para buffers independentes;
- oferece limpeza explícita desses buffers;
- usa um transportador desativado por padrão que falha de forma segura sem abrir sockets.

## Limitações atuais

- sem áudio bidirecional;
- sem WebRTC para navegador;
- o contrato SCTP existe, mas a implementação Pion ainda não está habilitada;
- sem RTP ou SRTP;
- aceitar a sinalização não estabelece o caminho de mídia;
- as chaves ficam somente em memória e não sobrevivem a reinícios;
- API e formatos podem mudar enquanto o PR estiver em rascunho.
