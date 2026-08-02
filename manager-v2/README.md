# Evolution GO Manager V2

Novo frontend do Evolution GO, escrito do zero em React e TypeScript.

## Princípios

- nenhuma dependência do bundle compilado do Manager antigo;
- nenhuma cópia de componentes do AstraCalls/AGPL;
- APIs, autenticação e protocolo WebRTC pertencem ao Evolution GO;
- migração gradual: `/manager` permanece intacto enquanto o V2 evolui;
- telefonia é um módulo central, não um widget anexado posteriormente.

## Primeira entrega

- shell responsivo com navegação;
- configuração segura da URL e API key da instância;
- consulta periódica de `/call/status`;
- início, aceite, recusa e encerramento de chamadas;
- ponte WebRTC PCM `evolution-call-pcm` / `evcall.pcm.v1`;
- captura e reprodução com AudioWorklet;
- mute, estatísticas e diagnóstico local;
- áreas reservadas para instâncias, conversas, contatos e dashboard.

## Desenvolvimento

```bash
cd manager-v2
npm install
npm run dev
```

O Vite inicia em `http://localhost:5173/manager-v2/`. Para testar chamadas contra uma API remota, informe a URL HTTPS e a API key no próprio Manager V2.

## Build

```bash
npm run typecheck
npm run build
```

O resultado é criado em `manager-v2/dist`. A publicação em `/manager-v2` será conectada ao servidor Go em uma etapa isolada, preservando `/manager` como fallback.
