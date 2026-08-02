# Evolution GO Manager V2

Novo frontend do Evolution GO, escrito do zero em React e TypeScript.

## Princípios

- nenhuma dependência do bundle compilado do Manager antigo;
- nenhuma cópia de componentes do AstraCalls/AGPL;
- APIs, autenticação e protocolo WebRTC pertencem ao Evolution GO;
- migração gradual: `/manager` permanece intacto enquanto o V2 evolui;
- telefonia e mensageria fazem parte da mesma área operacional.

## Recursos atuais

### Telefonia

- consulta periódica de `/call/status`;
- início, aceite, recusa e encerramento de chamadas;
- ponte WebRTC PCM `evolution-call-pcm` / `evcall.pcm.v1`;
- captura e reprodução com AudioWorklet;
- mute, estatísticas e diagnóstico local;
- integração entre contatos, conversas e discador.

### Mensageria

- contatos reais carregados de `GET /user/contacts`;
- busca por nome, empresa ou número;
- validação de novo destinatário por `POST /user/check`;
- envio de texto por `POST /send/text`;
- envio multipart de imagem, vídeo, áudio e documento por `POST /send/media`;
- legenda opcional para anexos;
- estado local de envio, sucesso e falha;
- histórico dos envios mantido em `sessionStorage` durante a sessão do navegador;
- botão para iniciar chamada diretamente da conversa ou contato.

O backend atual não expõe uma rota de caixa de entrada/histórico completo. Portanto, o Manager V2 ainda não tenta inventar mensagens recebidas: ele mostra os envios locais e deixa a arquitetura preparada para consumir WebSocket, webhook persistido ou uma futura API de conversas.

## Desenvolvimento

```bash
cd manager-v2
npm install
npm run dev
```

O Vite inicia em `http://localhost:5173/manager-v2/`. Para testar contra uma API remota, informe a URL HTTPS e a API key da instância no próprio Manager V2.

## Build local

```bash
npm run typecheck
npm run build
```

O resultado é criado em `manager-v2/dist`.

## Docker e publicação

O `Dockerfile` compila o frontend em um estágio Node separado e copia o resultado para a imagem final. O servidor Go publica:

```text
/manager     Manager legado
/manager-v2  Manager novo
```

Para testar a imagem com telefonia Pion:

```bash
docker build --build-arg GO_BUILD_TAGS=voip_pion -t evolution-go:manager-v2 .
```

Depois do deploy, acesse:

```text
https://SEU_DOMINIO/manager-v2
```

HTTPS é necessário para acesso ao microfone fora de `localhost`.

## Próximas fases

1. gerenciamento de instâncias, QR Code, conexão e reconexão;
2. API persistente de conversas e mensagens recebidas;
3. eventos em tempo real por WebSocket/SSE;
4. confirmação de entrega e leitura;
5. resposta, edição, exclusão, reações e marcação como lida;
6. envio de localização, contato, enquete, botões e listas;
7. testes de interface e homologação ponta a ponta.
