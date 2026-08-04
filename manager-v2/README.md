# Evolution GO API Test Manager

Interface de desenvolvimento do Evolution GO, escrita do zero em React e TypeScript.

O Manager V2 não é uma caixa de entrada nem um sistema de atendimento. Ele existe para:

- criar ou configurar o acesso a uma instância;
- conectar e salvar a sessão do WhatsApp;
- gerar QR Code ou código de pareamento;
- testar as funções públicas da API;
- inspecionar respostas e erros;
- reproduzir chamadas com cURL;
- validar telefonia WebRTC no navegador.

## Princípios

- nenhuma dependência do bundle compilado do Manager legado;
- nenhuma cópia de componentes do AstraCalls/AGPL;
- `/manager` permanece disponível como fallback;
- `/manager-v2` é uma ferramenta técnica de conexão e testes;
- nenhuma API nova é inventada apenas para alimentar a interface;
- cada rota nova registrada no backend deve entrar no catálogo do API Lab.

## Áreas

### Instância

O perfil de conexão armazena:

- URL do Evolution GO;
- ID da instância;
- API key da instância;
- a chave global é obtida no servidor pelo `.env`, sem ser enviada ao navegador;
- preferência para salvar permanentemente ou apenas durante a sessão do navegador.

A tela possui ações rápidas para:

- consultar status;
- conectar;
- gerar QR Code;
- reconectar;
- desconectar;
- fazer logout.

O API Lab também contém as rotas administrativas para criar, listar, consultar, excluir e configurar proxy de instâncias.

### Acesso ao Manager

O Manager V2 possui uma conta administrativa local, independente das chaves de
cada instância:

- no primeiro acesso a `/manager-v2`, o navegador solicita o cadastro do
  administrador;
- os próximos acessos exigem e-mail e senha;
- a sessão é um JWT guardado em cookie `HttpOnly`, portanto a chave global não
  é exposta ao JavaScript ou ao armazenamento do navegador;
- `GLOBAL_API_KEY` continua sendo carregada pelo servidor a partir do `.env`;
- `MANAGER_JWT_SECRET` é opcional e permite usar um segredo separado para
  assinar as sessões. Se não for definido, o servidor usa `GLOBAL_API_KEY`.

Use uma senha de pelo menos 12 caracteres na criação da conta administrativa.

### API Lab

O catálogo cobre as rotas registradas atualmente no roteador Go, incluindo:

- servidor e instâncias;
- texto, link, mídia, figurinha, localização e contato;
- botões reply, copy, URL, call e PIX;
- listas e carrosséis;
- enquetes e status;
- usuários, privacidade, perfil e bloqueios;
- ações de mensagens e chats;
- grupos e comunidades;
- labels;
- newsletters;
- chamadas e sessões WebRTC.

Cada operação fornece:

- exemplo inicial de payload;
- método HTTP editável;
- caminho editável;
- autenticação por chave da instância, global ou sem chave;
- corpo JSON, multipart ou sem corpo;
- upload de arquivo com nome de campo editável;
- status HTTP e duração;
- resposta completa;
- cURL equivalente;
- histórico dos últimos testes da sessão.

A operação **Requisição personalizada** permite testar rotas novas ou variações sem esperar uma tela específica.

### Chamadas

A central de voz é um teste especializado para recursos que não podem ser validados apenas com JSON:

- início, aceite, recusa e encerramento;
- acompanhamento de `/call/status`;
- criação da sessão WebRTC;
- captura e reprodução PCM por AudioWorklet;
- mute;
- contadores de frames enviados, recebidos e descartados;
- diagnóstico local de WebRTC, relay e SRTP.

HTTPS é necessário para acesso ao microfone fora de `localhost`.

## Cobertura automática das rotas

O comando abaixo compara `pkg/routes/routes.go` com o catálogo do frontend:

```bash
npm run check:catalog
```

O CI falha quando uma rota registrada no Evolution GO não possui entrada no API Lab. Rotas de interface, favicon e Swagger são ignoradas.

## Desenvolvimento

```bash
cd manager-v2
npm install
npm run check:catalog
npm run typecheck
npm run dev
```

O Vite inicia em:

```text
http://localhost:5173/manager-v2/
```

## Build

```bash
npm run check:catalog
npm run typecheck
npm run build
```

O resultado é criado em `manager-v2/dist`.

## Docker e publicação

O Dockerfile compila o frontend em um estágio Node separado e copia o resultado para a imagem final. O servidor Go publica:

```text
/manager     Manager legado
/manager-v2  API Test Manager
```

Para compilar com telefonia Pion:

```bash
docker build --build-arg GO_BUILD_TAGS=voip_pion -t evolution-go:manager-v2 .
```

Depois do deploy:

```text
https://SEU_DOMINIO/manager-v2
```
