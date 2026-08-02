# Painel de chamadas no `/manager`

O Manager carrega um módulo de chamadas independente do bundle React existente. Um botão de telefone aparece no canto inferior direito de `/manager` e usa as rotas autenticadas de chamadas da instância.

## Requisitos

- servidor compilado com `-tags=voip_pion`;
- instância WhatsApp conectada;
- página do Manager em HTTPS ou `localhost` para acesso ao microfone;
- para redes diferentes, `CALL_WEBRTC_PUBLIC_IP` e `CALL_WEBRTC_MEDIA_PORT` configurados e a porta liberada em UDP e TCP.

## Uso

1. Abra `https://SEU_DOMINIO/manager`.
2. Clique no botão de telefone.
3. Confirme a URL da API. Quando o Manager e a API usam o mesmo domínio, o valor padrão já é correto.
4. Informe a API key da instância.
5. Clique em **Salvar e consultar**.
6. Digite o número completo com DDI e clique em **Ligar**.
7. Quando a chamada ficar `active`, o painel tenta conectar o áudio automaticamente. Também é possível usar **Conectar áudio** manualmente.

O painel permite:

- iniciar chamadas de voz;
- acompanhar `ringing`, `connecting`, `active`, `ended` e `failed`;
- atender ou recusar chamadas recebidas;
- conectar microfone e alto-falante pelo DataChannel PCM;
- silenciar o microfone;
- encerrar a chamada;
- consultar contadores de frames enviados, recebidos e descartados;
- visualizar diagnóstico local.

## Armazenamento da chave

Por padrão, a configuração fica em `sessionStorage` e desaparece quando a sessão do navegador é encerrada. A opção **Salvar chave neste navegador** usa `localStorage` para manter a API key entre acessos.

Não habilite a persistência em computadores compartilhados. A chave nunca é colocada na URL ou no conteúdo do log do painel.

## Implementação

O Manager versionado contém apenas o bundle compilado original. Por isso, o módulo foi integrado como assets isolados:

```text
manager/dist/assets/call-manager.js
manager/dist/assets/call-manager.css
```

E carregado por:

```text
manager/dist/index.html
```

O módulo usa o mesmo protocolo da página de teste independente:

```text
DataChannel: evolution-call-pcm
Protocol: evcall.pcm.v1
PCM: float32 little-endian, mono, 16 kHz
Frame nominal: 960 amostras / 60 ms
```

## Diagnóstico

Se o painel mostrar `501`, a imagem foi compilada sem `voip_pion`.

Se a chamada fica ativa, mas o canal de áudio não abre:

- confirme o log `browser WebRTC fixed ICE endpoint enabled`;
- libere `CALL_WEBRTC_MEDIA_PORT` em UDP e TCP;
- confira os candidatos em `chrome://webrtc-internals`;
- confirme que o Manager está sendo acessado por HTTPS.

O painel fecha tracks do microfone, AudioContext, DataChannel e PeerConnection ao desconectar o áudio. O backend também remove a sessão WebRTC quando a chamada termina, a instância desconecta ou o cliente WhatsApp é substituído.
