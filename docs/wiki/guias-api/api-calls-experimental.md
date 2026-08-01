# API de chamadas — integração experimental

Esta branch adiciona a integração experimental WaCalls/AstraCalls ao Evolution Go.

> **Estado atual:** a sinalização é real e permite iniciar, receber, aceitar, rejeitar e encerrar chamadas no nível do protocolo. A variante `voip_pion` negocia DataChannels com os relays, processa RTP/SRTP autenticado e possui codec MLow, jitter buffer e entrada/saída PCM internas. Ainda não há áudio audível para o usuário porque microfone, reprodução e ponte WebRTC não foram conectados. Não use em produção.

Todas as rotas usam a autenticação normal da instância do Evolution.

## Consultar o runtime

```http
GET /call/status
```

Os monitores são anexados automaticamente quando o Evolution cria o `whatsmeow.Client`. Durante reconexão, logout ou remoção da instância, handlers, DataChannels, sessões RTP/SRTP, jitter buffers, codecs e material privado são removidos antes que o novo cliente seja registrado. Não é necessário chamar `/call/status` para ativar o monitoramento.

Exemplo de resposta:

```json
{
  "instanceId": "INSTANCE_ID",
  "connected": true,
  "calls": []
}
```

Chaves de chamada, JIDs internos de dispositivos, chaves SRTP, tokens de relay, pacotes enfileirados e buffers PCM não fazem parte dessa resposta. Eles ficam somente em memória e são apagados quando a chamada termina, é rejeitada ou a sessão é desconectada.

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

`CallAccept` não marca a chamada como `active` sozinho. Na variante Pion, `active` é publicado somente depois que o DataChannel abre e as sessões RTP/SRTP, jitter e MLow são criadas com sucesso.

Se a preparação criptográfica do evento ainda não terminou, a API retorna um erro informando que a chamada ainda não está pronta para aceite.

## Encerrar uma chamada

```http
DELETE /call/{callId}
apikey: INSTANCE_TOKEN
```

A rota envia `terminate` para chamadas realizadas e recebidas. Em seguida, o runtime muda o estado público para `ended` e remove chave, candidatos, DataChannels, contextos RTP/SRTP, jitter buffer, codec, buffers PCM e demais dados privados da chamada.

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

## RTP e SRTP

O caminho de pacotes inclui:

- RTP versão 2 com CSRC, extensões e padding validados;
- gerador concorrente de sequência e timestamp para payload type `120`;
- derivação HKDF-SHA256 por JID de dispositivo a partir da chave privada de 32 bytes da chamada;
- chave mestra AES de 16 bytes e salt de 14 bytes;
- AES-CTR para proteção do payload;
- autenticação HMAC-SHA1 truncada em quatro bytes;
- verificação da autenticação antes da descriptografia;
- rollover counter para a transição de sequência `65535 → 0`;
- janela antirreplay de 64 pacotes;
- suporte a pacotes autenticados fora de ordem dentro da janela;
- rejeição de reutilização do índice SRTP no envio;
- validação do SSRC remoto e do payload type de áudio;
- sessão independente por `callId`;
- limpeza sincronizada durante término, rejeição, logout ou reconexão.

## Jitter buffer e perda de pacotes

Cada chamada possui um jitter buffer independente antes do decoder MLow. A configuração padrão atual é fixa:

- duração de frame de 60 ms;
- atraso inicial de dois pacotes, aproximadamente 120 ms;
- limite de 64 pacotes enfileirados;
- no máximo cinco frames consecutivos de concealment por lacuna.

O buffer:

- usa número de sequência estendido para ordenar pacotes durante o rollover `65535 → 0`;
- aceita pacotes fora de ordem que ainda não perderam o prazo de reprodução;
- rejeita duplicatas, pacotes atrasados e estouro do limite sem derrubar a chamada;
- mantém contadores internos de recebidos, entregues, ocultados, duplicados, atrasados e descartados por limite;
- gera PLC chamando `Decode(nil)` somente quando um pacote futuro confirma uma lacuna;
- limita o PLC consecutivo e depois sincroniza novamente no próximo pacote disponível;
- não fabrica áudio no final do fluxo quando não existe pacote futuro;
- copia e apaga os payloads privados que mantém na fila;
- encerra o relógio de playout antes de destruir o codec.

Esta primeira versão não adapta automaticamente o atraso à variação observada da rede. A adaptação dinâmica será feita depois da validação com relays reais.

## Codec MLow e PCM

O codec MLow em Go puro foi portado da revisão MIT fixada `edeb31f0427aba896639db503153b777a405eccf` do WaCalls. O runtime não depende de CGO ou de uma instalação externa de `libopus` para essa etapa.

O pipeline interno agora:

- aceita PCM mono `float32` em 16 kHz;
- aceita chunks de tamanho arbitrário;
- acumula frames completos de 960 amostras, equivalentes a 60 ms;
- substitui `NaN` e infinito por silêncio;
- limita amplitudes ao intervalo `[-1, 1]`;
- codifica MLow e envia o payload pelo RTP/SRTP existente;
- preserva o marker RTP no primeiro frame transmitido;
- envia frames de silêncio quando a captura fica inativa;
- entrega RTP recebido ao jitter buffer antes da decodificação;
- decodifica payloads ordenados ou PLC para blocos PCM de 960 amostras;
- entrega uma cópia do PCM a um callback interno;
- serializa encoder e decoder por chamada;
- espera envios e playout em andamento antes do teardown.

As fronteiras internas disponíveis no `Coordinator` são:

- `FeedPCM(instanceID, callID, pcm)` para PCM mono/16 kHz;
- `SetOnPCM(callback)` para receber PCM decodificado;
- `SetOnRTP(callback)` para observação autenticada de baixo nível;
- `SendOpus(...)` para o caminho codificado já existente.

Essas funções ainda não são rotas HTTP. Expor áudio bruto sem autenticação de mídia, limite de fluxo e controle de sessão aumentaria a superfície de ataque.

## Build experimental

A build padrão continua usando um transportador sem rede:

```bash
go build ./cmd/evolution-go
```

Para compilar a variante com relay Pion:

```bash
go build -tags=voip_pion ./cmd/evolution-go
```

O workflow do PR testa permanentemente as duas variantes:

```bash
go test -race ./pkg/call/...
go test -race -tags=voip_pion ./pkg/call/...
```

## Limitações atuais

- sem captura real de microfone;
- sem reprodução em alto-falante;
- sem ponte WebRTC para navegador;
- sem resampling automático para fontes que não sejam mono/16 kHz;
- jitter buffer ainda estático, sem ajuste adaptativo por atraso e variação da rede;
- sem endpoint ou protocolo público de streaming de áudio;
- a conexão real com um relay WhatsApp ainda precisa ser validada de ponta a ponta com uma conta conectada;
- as chaves e sessões ficam somente em memória e não sobrevivem a reinícios;
- API e formatos podem mudar enquanto o PR estiver em rascunho.
