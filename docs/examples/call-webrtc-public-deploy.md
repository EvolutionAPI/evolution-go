# Implantação pública da ponte WebRTC de chamadas

Este guia publica a ponte navegador ⇄ Evolution em uma única porta UDP/TCP, sem depender de STUN ou TURN externo.

> A mídia do navegador só existe na build `voip_pion`. A chamada WhatsApp também precisa chegar ao estado `active` antes da criação da sessão WebRTC.

## Variáveis obrigatórias

Configure o IPv4 anunciado e a porta de mídia:

```env
CALL_WEBRTC_PUBLIC_IP=203.0.113.10
CALL_WEBRTC_MEDIA_PORT=50000
```

Em uma VPS cujo IPv4 público está diretamente associado à interface, também é possível usar:

```env
CALL_WEBRTC_PUBLIC_IP=auto
CALL_WEBRTC_MEDIA_PORT=50000
```

`auto` detecta o IPv4 escolhido pela rota padrão. Em servidores atrás de NAT, balanceador ou encaminhamento de porta, use explicitamente o endereço externo.

As duas variáveis devem ser definidas juntas. Configuração parcial, endereço inválido, porta inválida ou falha de bind impedem a criação da sessão WebRTC.

## Compilação direta

```bash
go build -tags=voip_pion -o evolution-go ./cmd/evolution-go

CALL_WEBRTC_PUBLIC_IP=203.0.113.10 \
CALL_WEBRTC_MEDIA_PORT=50000 \
./evolution-go
```

## Imagem Docker

O Dockerfile mantém a build padrão quando nenhum argumento é informado. Para incluir o Pion:

```bash
docker build \
  --build-arg GO_BUILD_TAGS=voip_pion \
  -t evolution-go:voip-pion .
```

### Rede host

É a opção mais simples em uma VPS Linux:

```bash
docker run --rm \
  --network host \
  -e CALL_WEBRTC_PUBLIC_IP=203.0.113.10 \
  -e CALL_WEBRTC_MEDIA_PORT=50000 \
  evolution-go:voip-pion
```

### Encaminhamento explícito

Quando a rede host não estiver disponível, publique a mesma porta nos dois protocolos:

```bash
docker run --rm \
  -p 8080:8080/tcp \
  -p 50000:50000/udp \
  -p 50000:50000/tcp \
  -e CALL_WEBRTC_PUBLIC_IP=203.0.113.10 \
  -e CALL_WEBRTC_MEDIA_PORT=50000 \
  evolution-go:voip-pion
```

Exemplo equivalente em Compose:

```yaml
services:
  evolution:
    build:
      context: .
      args:
        GO_BUILD_TAGS: voip_pion
    environment:
      CALL_WEBRTC_PUBLIC_IP: 203.0.113.10
      CALL_WEBRTC_MEDIA_PORT: "50000"
    ports:
      - "8080:8080/tcp"
      - "50000:50000/udp"
      - "50000:50000/tcp"
```

## Firewall e proxy

Libere no firewall ou security group:

```text
UDP 50000 entrada
TCP 50000 entrada
```

Traefik, Nginx ou Caddy podem publicar a API e a página em HTTPS, mas não devem intermediar a porta ICE. O tráfego WebRTC chega diretamente ao Evolution:

```text
navegador ── UDP 50000 ──► Evolution
          └─ TCP 50000 ──► Evolution, quando UDP falha
```

O HTTPS continua obrigatório para que navegadores remotos liberem `getUserMedia`.

## Comportamento do runtime

Quando as variáveis estão configuradas, o processo cria uma única API Pion compartilhada e:

- anuncia o IPv4 configurado por NAT 1:1;
- usa `ICEUDPMux` na porta fixa;
- usa `ICETCPMux` passivo na mesma porta;
- compartilha os muxes entre todas as sessões e chamadas;
- mantém os limites de quatro sessões por chamada e oito frames por fila;
- não entrega chaves WhatsApp ou SRTP ao navegador.

Sem as variáveis, a ponte continua usando candidatos host e portas efêmeras para desenvolvimento local.

## Quando TURN ainda é necessário

Esta estratégia cobre VPSs e servidores com uma porta pública encaminhável. TURN ainda pode ser necessário quando:

- o servidor está atrás de CGNAT sem encaminhamento;
- a rede do navegador bloqueia UDP e também ICE-TCP nessa porta;
- somente tráfego por 443 através de um relay é permitido;
- a implantação exige compatibilidade máxima em redes corporativas restritas.

## Checklist de validação

1. Confirme que a imagem foi compilada com `GO_BUILD_TAGS=voip_pion`.
2. Confirme que UDP e TCP estão liberados na porta configurada.
3. Inicie o Evolution com as duas variáveis.
4. Verifique o log `browser WebRTC fixed ICE endpoint enabled`.
5. Deixe a chamada WhatsApp chegar a `active`.
6. Abra `docs/examples/call-webrtc-pcm.html` por HTTPS.
7. Crie a sessão e confirme candidatos com o IPv4 e a porta pública no SDP answer.
8. Teste microfone e reprodução a partir de outra rede.
