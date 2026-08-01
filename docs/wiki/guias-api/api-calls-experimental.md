# API de chamadas — integração experimental

Esta branch adiciona a integração experimental WaCalls/AstraCalls ao Evolution Go.

> **Estado atual:** a sinalização é real e permite iniciar, receber, aceitar, rejeitar e encerrar chamadas no nível do protocolo. A variante `voip_pion` negocia DataChannels com os relays do WhatsApp, processa RTP/SRTP autenticado, usa codec MLow com jitter/PLC e oferece uma ponte WebRTC PCM para microfone e reprodução no navegador. A conexão com relays reais ainda precisa de validação ponta a ponta. Não use em produção.

Todas as rotas usam a autenticação normal da instância do Evolution.

## Consultar o runtime

```http
GET /call/status
apikey: INSTANCE_TOKEN
```

Os monitores são anexados automaticamente quando o Evolution cria o `whatsmeow.Client`. Durante reconexão, logout ou remoção da instância, handlers, DataChannels, sessões RTP/SRTP, jitter buffers, codecs, sessões WebRTC e material privado são removidos antes que o novo cliente seja registrado.

Exemplo de resposta:

```json
{
  "instanceId": "INSTANCE_ID",
  "connected": true,
  "calls": []
}
```

Chaves de chamada, JIDs internos de dispositivos, chaves SRTP, tokens de relay, pacotes enfileirados e buffers PCM nunca fazem parte dessa resposta.

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

A oferta é enviada como uma consulta do protocolo. Quando o ACK contém relays estruturados, a chave gerada, os participantes e os candidatos são copiados para o registro privado da chamada.

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

```http
POST /call/{callId}/accept
apikey: INSTANCE_TOKEN
```

O runtime descriptografa a chave recebida usando a sessão Signal já autenticada, envia `preaccept` automaticamente e mantém o material somente na memória privada. O endpoint envia a stanza `accept` e retorna a chamada no estado `connecting`.

`CallAccept` não marca a chamada como `active` sozinho. Na variante Pion, `active` é publicado somente depois que o relay abre e as sessões RTP/SRTP, jitter e MLow são criadas com sucesso.

## Encerrar ou rejeitar

```http
DELETE /call/{callId}
apikey: INSTANCE_TOKEN
```

A rota envia `terminate`, muda o estado público para `ended` e remove relays, contextos criptográficos, codec, jitter, buffers PCM e sessões WebRTC ligadas à chamada.

A rota de rejeição existente foi preservada:

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

O snapshot público usa `ringing`, `connecting`, `active`, `ended` e `failed`.

Internamente, a negociação usa uma máquina estrita com `initiating`, `ringing`, `incoming_ringing`, `connecting`, `active`, `on_hold` e `ended`. Transições inválidas são rejeitadas sem alterar o estado.

## Relay e transporte Pion

O módulo reconhece candidatos com atributos diretos e respostas estruturadas `te2`. Os candidatos são ordenados pelo menor RTT e associados ao material privado pelo `callId`.

A implementação Pion experimental inclui:

- PeerConnection e DataChannel `wa-web-call` por relay;
- transformação do SDP para credenciais e fingerprint do WhatsApp;
- registro STUN com subscriptions de SSRC;
- allocation, retries e keepalive;
- broadcast e recebimento de frames;
- timeout, fechamento e limpeza de buffers;
- SSRC determinístico por `callId` e JID de dispositivo.

## RTP e SRTP

O caminho de pacotes inclui:

- RTP versão 2 com CSRC, extensões e padding validados;
- payload type `120`;
- derivação HKDF-SHA256 por dispositivo;
- AES-CTR e HMAC-SHA1 truncado;
- autenticação verificada antes da descriptografia;
- rollover counter na transição `65535 → 0`;
- janela antirreplay de 64 pacotes;
- pacotes autenticados fora de ordem;
- rejeição de reutilização do índice de envio;
- sessão independente por `callId`;
- limpeza sincronizada durante término, rejeição, logout ou reconexão.

## Jitter buffer e perda de pacotes

Cada chamada possui um jitter buffer antes do decoder MLow. A configuração padrão atual é fixa:

- frames de 60 ms;
- atraso inicial de dois pacotes, aproximadamente 120 ms;
- limite de 64 pacotes enfileirados;
- até cinco frames consecutivos de concealment por lacuna.

O buffer usa sequência estendida para rollover, aceita pacotes fora de ordem antes do prazo, contabiliza duplicatas/atrasados/overflow e chama `Decode(nil)` somente quando um pacote futuro confirma a lacuna. Ele não fabrica áudio ao final do fluxo.

## Codec MLow e PCM

O codec MLow em Go puro foi portado da revisão MIT fixa `edeb31f0427aba896639db503153b777a405eccf` do WaCalls. Não há dependência de CGO ou `libopus`.

O pipeline:

- aceita PCM mono `float32` em 16 kHz;
- acumula frames de 960 amostras/60 ms;
- sanitiza `NaN`, infinito e amplitudes fora de `[-1, 1]`;
- codifica MLow e envia por RTP/SRTP;
- reordena e aplica PLC antes do decode recebido;
- entrega PCM por callback interno;
- serializa encoder/decoder por chamada;
- espera envio e playout antes do teardown.

## Ponte WebRTC do navegador

A ponte do navegador usa WebRTC para fornecer DTLS/SCTP, mas transmite PCM em um DataChannel em vez de uma media track. Isso evita uma segunda pilha Opus no servidor e reutiliza diretamente o pipeline MLow existente.

Ela só está disponível na build `voip_pion`. A build padrão responde `501 Not Implemented`.

### Criar sessão

A chamada deve estar em `active`.

```http
POST /call/{callId}/webrtc
Content-Type: application/json
apikey: INSTANCE_TOKEN
```

```json
{
  "offer": {
    "type": "offer",
    "sdp": "v=0\r\n..."
  }
}
```

O navegador deve criar previamente um DataChannel com:

```text
label: evolution-call-pcm
protocol: evcall.pcm.v1
ordered: true
```

Resposta `201`:

```json
{
  "sessionId": "UUID",
  "answer": {
    "type": "answer",
    "sdp": "v=0\r\n..."
  },
  "audio": {
    "dataChannel": "evolution-call-pcm",
    "protocol": "evcall.pcm.v1",
    "format": "f32le",
    "sampleRate": 16000,
    "channels": 1,
    "frameSamples": 960
  }
}
```

A API espera uma oferta completa com os candidatos ICE já coletados. Não há endpoint de trickle ICE nesta etapa.

### Listar e fechar sessões

```http
GET /call/{callId}/webrtc
apikey: INSTANCE_TOKEN
```

A resposta contém estado, frames de entrada/saída e descartes por sessão.

```http
DELETE /call/{callId}/webrtc/{sessionId}
apikey: INSTANCE_TOKEN
```

Há limite de quatro sessões por chamada. Todas são fechadas automaticamente quando a chamada termina, é rejeitada, a instância desconecta ou o cliente WhatsApp é substituído.

### Framing PCM `EVPC` versão 1

Cada mensagem binária possui:

| Offset | Tamanho | Campo |
|---:|---:|---|
| 0 | 4 | magic ASCII `EVPC` |
| 4 | 1 | versão `1` |
| 5 | 1 | tipo `1` para PCM |
| 6 | 2 | flags, atualmente zero, little-endian |
| 8 | 4 | sample rate `16000`, little-endian |
| 12 | 4 | número de amostras, little-endian |
| 16 | variável | amostras `float32` little-endian |

O servidor aceita no máximo 3840 amostras por mensagem. O frame nominal contém 960 amostras. Filas internas possuem oito frames e o envio é descartado quando o buffer SCTP ultrapassa 512 KiB.

### Exemplo pronto

Abra o arquivo:

```text
docs/examples/call-webrtc-pcm.html
```

Ele implementa:

- troca SDP autenticada;
- `getUserMedia` com cancelamento de eco, redução de ruído e ganho automático;
- resampling da taxa do `AudioContext` para 16 kHz;
- envio em frames de 960 amostras;
- resampling de 16 kHz para a taxa do dispositivo;
- reprodução por `AudioWorklet`;
- mute, backpressure e encerramento da sessão.

O navegador exige HTTPS ou `localhost` para liberar o microfone.

### Limitações de rede da ponte

A configuração atual não injeta servidores STUN/TURN no PeerConnection do navegador. Portanto, ela funciona diretamente quando navegador e Evolution conseguem trocar candidatos host, mas pode falhar através de NATs ou redes remotas. TURN configurável é uma etapa posterior.

## Build experimental

```bash
go build ./cmd/evolution-go
go build -tags=voip_pion ./cmd/evolution-go
```

O workflow testa permanentemente as duas variantes:

```bash
go test -race ./pkg/call/...
go test -race -tags=voip_pion ./pkg/call/...
```

## Segurança e limites

- todas as rotas SDP são autenticadas pela instância;
- a chamada deve estar `active` antes de criar a sessão;
- ofertas SDP acima de 256 KiB são rejeitadas;
- mensagens de texto, labels/protocolos incorretos e frames PCM inválidos são descartados;
- filas e buffer SCTP são limitados;
- payloads e PCM temporários são sobrescritos antes do descarte;
- chaves de chamada e SRTP nunca são entregues ao navegador;
- nenhuma rota HTTP recebe áudio bruto.

## Limitações atuais

- falta validar uma chamada real ponta a ponta com relay WhatsApp;
- o exemplo realiza resampling linear, ainda sem filtro de alta qualidade;
- não há STUN/TURN configurável para a ponte do navegador;
- jitter do WhatsApp ainda é estático, sem ajuste adaptativo pela rede;
- sessões e chaves ficam apenas em memória;
- publicação normalizada de estados nos produtores de eventos continua pendente;
- API e framing permanecem experimentais enquanto o PR estiver em rascunho.
