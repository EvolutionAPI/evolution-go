# API de Mensagens Interativas

Documentação completa dos endpoints para enviar mensagens interativas no WhatsApp: botões, listas e carrosséis. Todos os endpoints são compatíveis com **Android**, **iOS (iPhone)** e **WhatsApp Web/Desktop**.

## 📋 Índice

- [Enviar Botões](#enviar-botões)
- [Enviar Lista](#enviar-lista)
- [Enviar Carrossel](#enviar-carrossel)
- [Evento ButtonClick (Rastreio)](#evento-buttonclick)
- [Tipos de Botões](#tipos-de-botões)
- [Notas de Compatibilidade](#notas-de-compatibilidade)

---

## Enviar Botões

Envia uma mensagem com botões interativos. Suporta botões de resposta rápida (reply), URL, ligação (call), cópia (copy) e PIX.

**Endpoint**: `POST /send/button`

**Headers**:
```
Content-Type: application/json
apikey: SUA-CHAVE-API
```

**Body**:
```json
{
  "number": "5511999999999",
  "title": "Título da Mensagem",
  "description": "Texto do corpo da mensagem",
  "footer": "Texto do rodapé",
  "buttons": [
    {
      "type": "reply",
      "displayText": "Texto do Botão",
      "id": "identificador_unico"
    }
  ],
  "delay": 1000,
  "quoted": {
    "messageId": "BAE5..."
  }
}
```

**Parâmetros**:

| Campo | Tipo | Obrigatório | Descrição |
|-------|------|-------------|-----------|
| `number` | string | ✅ Sim | Número do destinatário (formato: DDI + DDD + número) |
| `description` | string | ✅ Sim | Texto do corpo da mensagem |
| `buttons` | array | ✅ Sim | Array de botões (ver [Tipos de Botões](#tipos-de-botões)) |
| `title` | string | ❌ Não | Título exibido em negrito acima do corpo |
| `footer` | string | ❌ Não | Texto pequeno no rodapé. **Não enviar vazio** |
| `delay` | int32 | ❌ Não | Delay em milissegundos antes de enviar |
| `mentionedJid` | string | ❌ Não | JID do usuário a mencionar |
| `mentionAll` | bool | ❌ Não | Mencionar todos os participantes (apenas grupos) |
| `formatJid` | bool | ❌ Não | Formatar número automaticamente (padrão: true) |
| `quoted` | object | ❌ Não | Mensagem a ser citada |

**Resposta de Sucesso (200)**:
```json
{
  "message": "success",
  "data": {
    "Info": {
      "ID": "3EB034D434158AD2CC0B9A",
      "Chat": "5511999999999@s.whatsapp.net",
      "Type": "ButtonMessage",
      "Timestamp": "2026-04-01T17:54:09Z"
    }
  }
}
```

**Resposta de Erro (400)**:
```json
{
  "error": "phone number is required"
}
```

### Exemplo 1: Botões Quick Reply

Máximo 3 botões. Não pode misturar com outros tipos.

```bash
curl -X POST http://localhost:4000/send/button \
  -H "Content-Type: application/json" \
  -H "apikey: SUA-CHAVE-API" \
  -d '{
    "number": "5511999999999",
    "title": "Atendimento",
    "description": "Como podemos ajudar você hoje?",
    "footer": "Responda clicando em um botão",
    "buttons": [
      {"type": "reply", "displayText": "Suporte Técnico", "id": "suporte"},
      {"type": "reply", "displayText": "Financeiro", "id": "financeiro"},
      {"type": "reply", "displayText": "Vendas", "id": "vendas"}
    ]
  }'
```

### Exemplo 2: Botões Mistos (URL + Call + Copy)

Podem ser combinados entre si livremente.

```bash
curl -X POST http://localhost:4000/send/button \
  -H "Content-Type: application/json" \
  -H "apikey: SUA-CHAVE-API" \
  -d '{
    "number": "5511999999999",
    "title": "Nossos Canais",
    "description": "Escolha como deseja entrar em contato:",
    "footer": "Empresa XYZ",
    "buttons": [
      {"type": "url", "displayText": "Acessar Site", "url": "https://empresa.com"},
      {"type": "call", "displayText": "Ligar para Nós", "phoneNumber": "+5511999999999"},
      {"type": "copy", "displayText": "Copiar Email", "copyCode": "contato@empresa.com"}
    ]
  }'
```

### Exemplo 3: Botão PIX

Deve ser enviado **sozinho**, sem outros botões.

```bash
curl -X POST http://localhost:4000/send/button \
  -H "Content-Type: application/json" \
  -H "apikey: SUA-CHAVE-API" \
  -d '{
    "number": "5511999999999",
    "title": "Pagamento PIX",
    "description": "Realize o pagamento via PIX:",
    "buttons": [
      {
        "type": "pix",
        "currency": "BRL",
        "name": "Empresa XYZ LTDA",
        "keyType": "CNPJ",
        "key": "12345678000199"
      }
    ]
  }'
```

---

## Enviar Lista

Envia uma mensagem com lista de opções organizadas em seções. O usuário toca no botão para abrir a lista e seleciona uma opção.

**Endpoint**: `POST /send/list`

**Headers**:
```
Content-Type: application/json
apikey: SUA-CHAVE-API
```

**Body**:
```json
{
  "number": "5511999999999",
  "title": "Título da Lista",
  "description": "Texto do corpo da mensagem",
  "buttonText": "Texto do botão que abre a lista",
  "footerText": "Texto do rodapé",
  "sections": [
    {
      "title": "Nome da Seção",
      "rows": [
        {
          "title": "Título da Opção",
          "description": "Descrição da opção",
          "rowId": "identificador_unico"
        }
      ]
    }
  ],
  "delay": 1000,
  "quoted": {
    "messageId": "BAE5..."
  }
}
```

**Parâmetros**:

| Campo | Tipo | Obrigatório | Descrição |
|-------|------|-------------|-----------|
| `number` | string | ✅ Sim | Número do destinatário |
| `description` | string | ✅ Sim | Texto do corpo da mensagem |
| `sections` | array | ✅ Sim | Array de seções contendo rows |
| `buttonText` | string | ❌ Não | Texto do botão que abre a lista (padrão: "Ver Menu") |
| `title` | string | ❌ Não | Título em negrito no topo |
| `footerText` | string | ❌ Não | Texto do rodapé |
| `delay` | int32 | ❌ Não | Delay em milissegundos antes de enviar |
| `mentionedJid` | string | ❌ Não | JID do usuário a mencionar |
| `mentionAll` | bool | ❌ Não | Mencionar todos (apenas grupos) |
| `formatJid` | bool | ❌ Não | Formatar número automaticamente |
| `quoted` | object | ❌ Não | Mensagem a ser citada |

**Parâmetros da Seção**:

| Campo | Tipo | Obrigatório | Descrição |
|-------|------|-------------|-----------|
| `title` | string | ✅ Sim | Nome da seção |
| `rows` | array | ✅ Sim | Array de opções da seção |

**Parâmetros da Row**:

| Campo | Tipo | Obrigatório | Descrição |
|-------|------|-------------|-----------|
| `title` | string | ✅ Sim | Texto da opção |
| `description` | string | ❌ Não | Descrição da opção |
| `rowId` | string | ✅ Sim | ID único para rastreio do clique |

**Resposta de Sucesso (200)**:
```json
{
  "message": "success",
  "data": {
    "Info": {
      "ID": "3EB0C5A277F7F9B6C599",
      "Chat": "5511999999999@s.whatsapp.net",
      "Type": "ListMessage",
      "Timestamp": "2026-04-01T18:00:00Z"
    }
  }
}
```

**Resposta de Erro (400)**:
```json
{
  "error": "sections are required"
}
```

### Exemplo 1: Cardápio Digital

```bash
curl -X POST http://localhost:4000/send/list \
  -H "Content-Type: application/json" \
  -H "apikey: SUA-CHAVE-API" \
  -d '{
    "number": "5511999999999",
    "title": "Cardápio Digital",
    "description": "Escolha um item do nosso cardápio:",
    "buttonText": "Ver Cardápio",
    "footerText": "Restaurante XYZ",
    "sections": [
      {
        "title": "Pratos Principais",
        "rows": [
          {"title": "Filé Mignon", "description": "Com arroz e batata - R$ 45,90", "rowId": "file_mignon"},
          {"title": "Salmão Grelhado", "description": "Com legumes - R$ 52,90", "rowId": "salmao"},
          {"title": "Frango Parmegiana", "description": "Com purê - R$ 35,90", "rowId": "frango"}
        ]
      },
      {
        "title": "Bebidas",
        "rows": [
          {"title": "Suco Natural", "description": "Laranja, Limão ou Maracujá - R$ 8,00", "rowId": "suco"},
          {"title": "Refrigerante", "description": "Lata 350ml - R$ 6,00", "rowId": "refri"},
          {"title": "Água Mineral", "description": "500ml - R$ 4,00", "rowId": "agua"}
        ]
      },
      {
        "title": "Sobremesas",
        "rows": [
          {"title": "Pudim", "description": "R$ 12,00", "rowId": "pudim"},
          {"title": "Petit Gâteau", "description": "R$ 18,00", "rowId": "petit_gateau"}
        ]
      }
    ]
  }'
```

### Exemplo 2: Menu de Serviços

```bash
curl -X POST http://localhost:4000/send/list \
  -H "Content-Type: application/json" \
  -H "apikey: SUA-CHAVE-API" \
  -d '{
    "number": "5511999999999",
    "description": "Selecione o serviço desejado:",
    "buttonText": "Ver Serviços",
    "sections": [
      {
        "title": "Atendimento",
        "rows": [
          {"title": "Falar com Atendente", "description": "Atendimento humano", "rowId": "atendente"},
          {"title": "FAQ", "description": "Perguntas frequentes", "rowId": "faq"},
          {"title": "Abrir Ticket", "description": "Registrar ocorrência", "rowId": "ticket"}
        ]
      }
    ]
  }'
```

---

## Enviar Carrossel

Envia uma mensagem com cards deslizáveis (carrossel), cada um com imagem, texto, rodapé e botões. Ideal para catálogos de produtos, planos e portfólios.

**Endpoint**: `POST /send/carousel`

**Headers**:
```
Content-Type: application/json
apikey: SUA-CHAVE-API
```

**Body**:
```json
{
  "number": "5511999999999",
  "body": "Texto do corpo principal",
  "footer": "Texto do rodapé principal",
  "cards": [
    {
      "header": {
        "title": "Título do Card",
        "imageUrl": "https://url-da-imagem.com/foto.jpg"
      },
      "body": "Texto do corpo do card",
      "footer": "Rodapé do card",
      "buttons": [
        {
          "type": "REPLY",
          "displayText": "Texto do Botão",
          "id": "id_rastreio"
        }
      ]
    }
  ],
  "delay": 1000,
  "quoted": {
    "messageId": "BAE5..."
  }
}
```

**Parâmetros**:

| Campo | Tipo | Obrigatório | Descrição |
|-------|------|-------------|-----------|
| `number` | string | ✅ Sim | Número do destinatário |
| `cards` | array | ✅ Sim | Array de cards (mínimo 2, máximo ~10) |
| `body` | string | ❌ Não | Texto do corpo principal (acima dos cards) |
| `footer` | string | ❌ Não | Rodapé principal |
| `delay` | int32 | ❌ Não | Delay em milissegundos antes de enviar |
| `formatJid` | bool | ❌ Não | Formatar número automaticamente |
| `quoted` | object | ❌ Não | Mensagem a ser citada |

**Parâmetros do Card**:

| Campo | Tipo | Obrigatório | Descrição |
|-------|------|-------------|-----------|
| `header` | object | ✅ Sim | Cabeçalho do card (deve conter imagem) |
| `header.title` | string | ❌ Não | Título do card |
| `header.subtitle` | string | ❌ Não | Subtítulo do card |
| `header.imageUrl` | string | ✅ Sim* | URL da imagem do card |
| `header.videoUrl` | string | ❌ Não | URL de vídeo (alternativa à imagem) |
| `body` | string | ✅ Sim | Texto do corpo do card |
| `footer` | string | ❌ Não | Rodapé do card |
| `buttons` | array | ❌ Não | Array de botões do card |

> ⚠️ **Importante**: Todos os cards **devem** ter imagem (`imageUrl`) para o carrossel renderizar corretamente em todos os dispositivos. As imagens recebem thumbnail JPEG automaticamente para carregamento instantâneo.

**Parâmetros do Botão do Card**:

| Campo | Tipo | Obrigatório | Descrição |
|-------|------|-------------|-----------|
| `type` | string | ❌ Não | Tipo do botão: `REPLY`, `URL`, `CALL`, `COPY` (padrão: `REPLY`) |
| `displayText` | string | ✅ Sim | Texto visível do botão |
| `id` | string | ✅ Sim* | ID para rastreio (reply) ou URL/telefone (url/call) |
| `copyCode` | string | ✅ Sim* | Texto a copiar (apenas tipo `COPY`) |

**Resposta de Sucesso (200)**:
```json
{
  "message": "success",
  "data": {
    "Info": {
      "ID": "3EB0C5A277F7F9B6C599",
      "Chat": "5511999999999@s.whatsapp.net",
      "Type": "InteractiveMessage",
      "Timestamp": "2026-04-01T18:30:00Z"
    }
  }
}
```

**Resposta de Erro (400)**:
```json
{
  "error": "cards are required (minimum 1)"
}
```

### Exemplo 1: Catálogo de Produtos

```bash
curl -X POST http://localhost:4000/send/carousel \
  -H "Content-Type: application/json" \
  -H "apikey: SUA-CHAVE-API" \
  -d '{
    "number": "5511999999999",
    "body": "Confira nossos produtos em destaque!",
    "footer": "Loja Virtual XYZ",
    "cards": [
      {
        "header": {
          "title": "Smartphone Pro Max",
          "imageUrl": "https://placehold.co/600x400/1a1a2e/white?text=Smartphone"
        },
        "body": "Tela 6.7 AMOLED, 256GB, Câmera 108MP.\nDe R$ 4.999 por R$ 3.799!",
        "footer": "12x sem juros",
        "buttons": [
          {"type": "REPLY", "displayText": "Comprar", "id": "comprar_smartphone"},
          {"type": "URL", "displayText": "Ver Detalhes", "id": "https://loja.com/smartphone"}
        ]
      },
      {
        "header": {
          "title": "Notebook Ultra",
          "imageUrl": "https://placehold.co/600x400/16213e/white?text=Notebook"
        },
        "body": "Intel i7, 16GB RAM, SSD 512GB, Tela 15.6 FHD.\nPor apenas R$ 5.299!",
        "footer": "Frete grátis",
        "buttons": [
          {"type": "REPLY", "displayText": "Comprar", "id": "comprar_notebook"},
          {"type": "URL", "displayText": "Ver Detalhes", "id": "https://loja.com/notebook"}
        ]
      },
      {
        "header": {
          "title": "Fone Bluetooth",
          "imageUrl": "https://placehold.co/600x400/0f3460/white?text=Fone"
        },
        "body": "Cancelamento de ruído ativo, 30h bateria.\nR$ 299,90",
        "footer": "Envio imediato",
        "buttons": [
          {"type": "REPLY", "displayText": "Comprar", "id": "comprar_fone"},
          {"type": "CALL", "displayText": "Ligar p/ Comprar", "id": "+5511999999999"}
        ]
      }
    ]
  }'
```

### Exemplo 2: Planos de Serviço

```bash
curl -X POST http://localhost:4000/send/carousel \
  -H "Content-Type: application/json" \
  -H "apikey: SUA-CHAVE-API" \
  -d '{
    "number": "5511999999999",
    "body": "Conheça nossos planos:",
    "cards": [
      {
        "header": {
          "title": "Plano Básico",
          "imageUrl": "https://placehold.co/600x400/2ecc71/white?text=Basico"
        },
        "body": "Ideal para pequenas empresas.\n- 1.000 mensagens/mês\n- 1 instância\n- Suporte email\n\nR$ 97/mês",
        "buttons": [
          {"type": "REPLY", "displayText": "Contratar Básico", "id": "plano_basico"},
          {"type": "COPY", "displayText": "Copiar Link", "copyCode": "https://planos.com/basico"}
        ]
      },
      {
        "header": {
          "title": "Plano Pro",
          "imageUrl": "https://placehold.co/600x400/3498db/white?text=Pro"
        },
        "body": "Para empresas em crescimento.\n- 10.000 mensagens/mês\n- 5 instâncias\n- Suporte prioritário\n\nR$ 297/mês",
        "buttons": [
          {"type": "REPLY", "displayText": "Contratar Pro", "id": "plano_pro"},
          {"type": "COPY", "displayText": "Copiar Link", "copyCode": "https://planos.com/pro"}
        ]
      },
      {
        "header": {
          "title": "Plano Enterprise",
          "imageUrl": "https://placehold.co/600x400/9b59b6/white?text=Enterprise"
        },
        "body": "Solução completa.\n- Mensagens ilimitadas\n- Instâncias ilimitadas\n- Suporte 24/7\n\nSob consulta",
        "buttons": [
          {"type": "REPLY", "displayText": "Solicitar Orçamento", "id": "plano_enterprise"},
          {"type": "CALL", "displayText": "Falar com Vendas", "id": "+5511999999999"}
        ]
      }
    ]
  }'
```

---

## Evento ButtonClick

Quando um usuário clica em um botão, seleciona um item de lista ou interage com um card de carrossel, a API dispara o evento `ButtonClick` para todos os canais configurados na instância.

### Canais de Disparo

- **Webhook** - Callback HTTP para URL configurada
- **WebSocket** - Evento em tempo real
- **RabbitMQ** - Mensagem na fila
- **NATS** - Mensagem no tópico

### Requisito

A instância deve ter o evento `BUTTON_CLICK` ou `MESSAGE` habilitado na configuração de events.

### Payload do Evento

```json
{
  "event": "ButtonClick",
  "data": {
    "buttonId": "suporte",
    "buttonText": "Suporte Técnico",
    "type": "native_flow_response",
    "phone": "5511999999999",
    "jid": "5511999999999@s.whatsapp.net",
    "pushName": "João Silva",
    "messageId": "3EB034D434158AD2CC0B9A",
    "chat": "5511999999999@s.whatsapp.net",
    "fromMe": false,
    "timestamp": 1711990500,
    "extraData": {}
  },
  "instanceToken": "token_da_instancia",
  "instanceId": "uuid-da-instancia",
  "instanceName": "nome_da_instancia"
}
```

### Campos do Evento

| Campo | Tipo | Descrição |
|-------|------|-----------|
| `buttonId` | string | ID do botão/row clicado (conforme definido no envio) |
| `buttonText` | string | Texto exibido no botão/row |
| `type` | string | Tipo de resposta (ver tabela abaixo) |
| `phone` | string | Número do usuário que clicou |
| `jid` | string | JID completo do usuário |
| `pushName` | string | Nome do contato no WhatsApp |
| `messageId` | string | ID da mensagem de resposta |
| `chat` | string | JID do chat |
| `fromMe` | bool | Se a mensagem é própria |
| `timestamp` | int64 | Timestamp Unix do clique |

### Tipos de Resposta

| Valor de `type` | Origem |
|-----------------|--------|
| `native_flow_response` | Clique em botão enviado via `/send/button` ou `/send/carousel` |
| `list_response` | Seleção de item em lista enviada via `/send/list` |
| `buttons_response` | Resposta a botões legados (ButtonsMessage) |
| `template_button_reply` | Resposta a template buttons |

### Exemplo: Botão Reply Clicado

```json
{
  "event": "ButtonClick",
  "data": {
    "buttonId": "vendas",
    "buttonText": "Vendas",
    "type": "native_flow_response",
    "phone": "5511999999999",
    "jid": "5511999999999@s.whatsapp.net",
    "pushName": "Maria Santos",
    "messageId": "3EB0XXXXXXXXXXXXXX",
    "chat": "5511999999999@s.whatsapp.net",
    "fromMe": false,
    "timestamp": 1711991000
  },
  "instanceToken": "teste123",
  "instanceId": "4ee8ab07-8a67-42a8-a029-f382315912b1",
  "instanceName": "minha_instancia"
}
```

### Exemplo: Item de Lista Selecionado

```json
{
  "event": "ButtonClick",
  "data": {
    "buttonId": "file_mignon",
    "buttonText": "Filé Mignon",
    "type": "list_response",
    "phone": "5511999999999",
    "jid": "5511999999999@s.whatsapp.net",
    "pushName": "Pedro Oliveira",
    "messageId": "3EB0XXXXXXXXXXXXXX",
    "chat": "5511999999999@s.whatsapp.net",
    "fromMe": false,
    "timestamp": 1711991500
  },
  "instanceToken": "teste123",
  "instanceId": "4ee8ab07-8a67-42a8-a029-f382315912b1",
  "instanceName": "minha_instancia"
}
```

### Exemplo: Botão de Carrossel Clicado

```json
{
  "event": "ButtonClick",
  "data": {
    "buttonId": "comprar_smartphone",
    "buttonText": "Comprar",
    "type": "native_flow_response",
    "phone": "5511999999999",
    "jid": "5511999999999@s.whatsapp.net",
    "pushName": "Ana Costa",
    "messageId": "3EB0XXXXXXXXXXXXXX",
    "chat": "5511999999999@s.whatsapp.net",
    "fromMe": false,
    "timestamp": 1711992000
  },
  "instanceToken": "teste123",
  "instanceId": "4ee8ab07-8a67-42a8-a029-f382315912b1",
  "instanceName": "minha_instancia"
}
```

---

## Tipos de Botões

### Resumo por Endpoint

| Tipo | SendButton | SendCarousel | Campo obrigatório |
|------|-----------|-------------|-------------------|
| Reply | `"reply"` | `"REPLY"` (padrão) | `displayText`, `id` |
| URL | `"url"` | `"URL"` | `displayText`, `url` ou `id` |
| Call | `"call"` | `"CALL"` | `displayText`, `phoneNumber` ou `id` |
| Copy | `"copy"` | `"COPY"` | `displayText`, `copyCode` |
| PIX | `"pix"` | N/A | `currency`, `name`, `keyType`, `key` |

### Regras de Combinação

- **Reply**: Máximo 3 botões. **Não** pode misturar com outros tipos no mesmo envio.
- **PIX**: Deve ser enviado **sozinho**, sem outros botões.
- **URL / Call / Copy**: Podem ser combinados livremente entre si.
- **Carrossel**: Todos os tipos (exceto PIX) podem ser combinados no mesmo card.
- **Lista**: Não possui botões nos items. Usa `rowId` para rastreio.

### Estrutura de Cada Tipo

**Reply** (Resposta Rápida):
```json
{"type": "reply", "displayText": "Suporte", "id": "btn_suporte"}
```

**URL** (Abre Link):
```json
{"type": "url", "displayText": "Acessar Site", "url": "https://empresa.com"}
```

**Call** (Ligação):
```json
{"type": "call", "displayText": "Ligar Agora", "phoneNumber": "+5511999999999"}
```

**Copy** (Copiar Texto):
```json
{"type": "copy", "displayText": "Copiar Código", "copyCode": "DESCONTO20"}
```

**PIX** (Pagamento):
```json
{
  "type": "pix",
  "currency": "BRL",
  "name": "Empresa XYZ",
  "keyType": "CPF",
  "key": "12345678901"
}
```

---

## Notas de Compatibilidade

### Regras para Funcionamento em Todos os Dispositivos

| Regra | Detalhe |
|-------|---------|
| Footer vazio | **Não** envie campo `footer` vazio. Omita o campo ou preencha com texto |
| Imagens no carrossel | **Todos** os cards devem ter `imageUrl` para renderizar corretamente |
| Mínimo de cards | Carrossel requer no **mínimo 2 cards** |
| Thumbnails | Gerados automaticamente pela API (72px JPEG) para carregamento instantâneo |
| Formato do número | DDI + DDD + número, sem caracteres especiais (`5511999999999`) |
| Grupos | Use o JID do grupo (ex: `120363XXXXX@g.us`) |
| Mensagem quoted | Adicione `"quoted": {"messageId": "ID"}` ao payload |

### Compatibilidade Testada

| Tipo de Mensagem | Android | iOS (iPhone) | WhatsApp Web |
|------------------|---------|--------------|--------------|
| Botões (reply) | ✅ | ✅ | ✅ |
| Botões (url) | ✅ | ✅ | ✅ |
| Botões (call) | ✅ | ✅ | ✅ |
| Botões (copy) | ✅ | ✅ | ✅ |
| Botões (pix) | ✅ | ✅ | ✅ |
| Lista | ✅ | ✅ | ✅ |
| Carrossel | ✅ | ✅ | ✅ |
