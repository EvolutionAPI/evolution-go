# API de Grupos

Documentação completa dos endpoints para gerenciar grupos WhatsApp.

## 📋 Índice

- [Listar Grupos](#listar-grupos)
- [Informações do Grupo](#informações-do-grupo)
- [Informações via Link de Convite](#informações-via-link-de-convite)
- [Link de Convite](#link-de-convite)
- [Criar Grupo](#criar-grupo)
- [Gerenciar Participantes](#gerenciar-participantes)
- [Configurações do Grupo](#configurações-do-grupo)
  - [Alterar Foto](#alterar-foto)
  - [Alterar Nome](#alterar-nome)
  - [Alterar Descrição](#alterar-descrição)
- [Meus Grupos (Admin)](#meus-grupos-admin)
- [Entrar via Link](#entrar-via-link)
- [Sair do Grupo](#sair-do-grupo)

---

## Listar Grupos

Lista todos os grupos que você participa.

**Endpoint**: `GET /group/list`

**Headers**:
```
apikey: SUA-CHAVE-API
```

**Resposta de Sucesso (200)**:
```json
{
  "message": "success",
  "data": [
    {
      "JID": "120363XXXXXXXXXX@g.us",
      "OwnerJID": "5511999999999@s.whatsapp.net",
      "GroupName": {
        "Name": "Equipe de Vendas",
        "NameSetAt": "2025-01-15T10:30:00Z",
        "NameSetBy": "5511999999999@s.whatsapp.net"
      },
      "GroupTopic": {
        "Topic": "Discussões sobre vendas e metas",
        "TopicID": "abc123",
        "TopicSetAt": "2025-01-15T10:35:00Z",
        "TopicSetBy": "5511999999999@s.whatsapp.net"
      },
      "GroupCreated": "2025-01-15T10:00:00Z",
      "Participants": [
        {
          "JID": "5511999999999@s.whatsapp.net",
          "IsAdmin": true,
          "IsSuperAdmin": true
        },
        {
          "JID": "5511888888888@s.whatsapp.net",
          "IsAdmin": false,
          "IsSuperAdmin": false
        }
      ],
      "ParticipantVersionID": "abc123"
    }
  ]
}
```

**Campos da Resposta**:
- `JID`: ID único do grupo (sempre termina com @g.us)
- `OwnerJID`: Criador do grupo
- `GroupName`: Nome e metadados
- `GroupTopic`: Descrição do grupo
- `Participants`: Lista de participantes com permissões

**Exemplo cURL**:
```bash
curl -X GET http://localhost:4000/group/list \
  -H "apikey: SUA-CHAVE-API"
```

---

## Informações do Grupo

Obtém informações detalhadas de um grupo específico.

**Endpoint**: `POST /group/info`

**Body**:
```json
{
  "groupJid": "120363XXXXXXXXXX@g.us"
}
```

**Parâmetros**:

| Campo | Tipo | Obrigatório | Descrição |
|-------|------|-------------|-----------|
| `groupJid` | string | ✅ Sim | JID do grupo |

**Resposta de Sucesso (200)**:
```json
{
  "message": "success",
  "data": {
    "JID": "120363XXXXXXXXXX@g.us",
    "OwnerJID": "5511999999999@s.whatsapp.net",
    "GroupName": {
      "Name": "Equipe de Vendas",
      "NameSetAt": "2025-01-15T10:30:00Z",
      "NameSetBy": "5511999999999@s.whatsapp.net"
    },
    "GroupTopic": {
      "Topic": "Discussões sobre vendas",
      "TopicID": "abc123",
      "TopicSetAt": "2025-01-15T10:35:00Z",
      "TopicSetBy": "5511999999999@s.whatsapp.net"
    },
    "GroupCreated": "2025-01-15T10:00:00Z",
    "Participants": [
      {
        "JID": "5511999999999@s.whatsapp.net",
        "IsAdmin": true,
        "IsSuperAdmin": true
      }
    ]
  }
}
```

**Exemplo cURL**:
```bash
curl -X POST http://localhost:4000/group/info \
  -H "Content-Type: application/json" \
  -H "apikey: SUA-CHAVE-API" \
  -d '{
    "groupJid": "120363XXXXXXXXXX@g.us"
  }'
```
}'

---

## Informações via Link de Convite

Obtém informações de um grupo através de um link de convite sem precisar entrar nele.

**Endpoint**: `POST /group/invite-info`

**Body**:
```json
{
"code": "https://chat.whatsapp.com/ABCDEFGHIJKLMNOP"
}
```

**Parâmetros**:

| Campo | Tipo | Obrigatório | Descrição |
|-------|------|-------------|-----------|
| `code` | string | ✅ Sim | Código do link ou URL completa |

**Resposta de Sucesso (200)**:
```json
{
"message": "success",
"data": {
  "JID": "120363XXXXXXXXXX@g.us",
  "OwnerJID": "5511999999999@s.whatsapp.net",
  "GroupName": {
    "Name": "Nome do Grupo",
    "NameSetAt": "2025-01-15T10:30:00Z",
    "NameSetBy": "5511999999999@s.whatsapp.net"
  },
  "GroupCreated": "2025-01-15T10:00:00Z"
}
}
```

**Exemplo cURL**:
```bash
curl -X POST http://localhost:4000/group/invite-info \
-H "Content-Type: application/json" \
-H "apikey: SUA-CHAVE-API" \
-d '{
  "code": "https://chat.whatsapp.com/ABCDEFGHIJKLMNOP"
}'
```

---

## Link de Convite
Obtém ou regenera o link de convite do grupo.

**Endpoint**: `POST /group/invitelink`

**Body**:
```json
{
  "groupJid": "120363XXXXXXXXXX@g.us",
  "reset": false
}
```

**Parâmetros**:

| Campo | Tipo | Obrigatório | Descrição |
|-------|------|-------------|-----------|
| `groupJid` | string | ✅ Sim | JID do grupo |
| `reset` | bool | ❌ Não | Se true, gera um novo link (invalida o anterior) |

**Nota**: Apenas admins podem obter/resetar links de convite.

**Resposta de Sucesso (200)**:
```json
{
  "message": "success",
  "data": "https://chat.whatsapp.com/ABCDEFGHIJKLMNOP"
}
```

**Exemplo cURL**:
```bash
# Obter link existente
curl -X POST http://localhost:4000/group/invitelink \
  -H "Content-Type: application/json" \
  -H "apikey: SUA-CHAVE-API" \
  -d '{
    "groupJid": "120363XXXXXXXXXX@g.us",
    "reset": false
  }'

# Gerar novo link
curl -X POST http://localhost:4000/group/invitelink \
  -H "Content-Type: application/json" \
  -H "apikey: SUA-CHAVE-API" \
  -d '{
    "groupJid": "120363XXXXXXXXXX@g.us",
    "reset": true
  }'
```

---

## Criar Grupo

Cria um novo grupo WhatsApp.

**Endpoint**: `POST /group/create`

**Body**:
```json
{
  "groupName": "Equipe de Vendas",
  "participants": [
    "5511999999999",
    "5511888888888",
    "5511777777777"
  ]
}
```

**Parâmetros**:

| Campo | Tipo | Obrigatório | Descrição |
|-------|------|-------------|-----------|
| `groupName` | string | ✅ Sim | Nome do grupo |
| `participants` | array | ✅ Sim | Array de números dos participantes (mínimo 1) |

**Resposta de Sucesso (200)**:
```json
{
  "message": "success",
  "data": {
    "jid": "120363XXXXXXXXXX@g.us",
    "name": "Equipe de Vendas",
    "owner": "5511999999999@s.whatsapp.net",
    "added": [
      "5511999999999@s.whatsapp.net",
      "5511888888888@s.whatsapp.net"
    ],
    "failed": [
      "5511777777777@s.whatsapp.net"
    ]
  }
}
```

**Campos da Resposta**:
- `jid`: ID do grupo criado
- `name`: Nome do grupo
- `owner`: Criador (você)
- `added`: Participantes adicionados com sucesso
- `failed`: Participantes que falharam (número inválido, bloqueado, etc)

**Nota**: É normal que alguns participantes falhem (números inexistentes, bloqueados, etc).

**Exemplo cURL**:
```bash
curl -X POST http://localhost:4000/group/create \
  -H "Content-Type: application/json" \
  -H "apikey: SUA-CHAVE-API" \
  -d '{
    "groupName": "Meu Grupo",
    "participants": ["5511999999999", "5511888888888"]
  }'
```

---

## Gerenciar Participantes

Adiciona, remove ou promove participantes do grupo.

**Endpoint**: `POST /group/participant`

**Body**:
```json
{
  "groupJid": "120363XXXXXXXXXX@g.us",
  "action": "add",
  "participants": ["5511999999999", "5511888888888"]
}
```

**Parâmetros**:

| Campo | Tipo | Obrigatório | Descrição |
|-------|------|-------------|-----------|
| `groupJid` | string | ✅ Sim | JID do grupo |
| `action` | string | ✅ Sim | Ação a executar |
| `participants` | array | ✅ Sim | Array de números |

**Ações Disponíveis**:

| Action | Descrição | Requer Admin |
|--------|-----------|--------------|
| `add` | Adicionar participantes | ✅ Sim |
| `remove` | Remover participantes | ✅ Sim |
| `promote` | Promover a admin | ✅ Sim |
| `demote` | Remover admin | ✅ Sim |

**Resposta de Sucesso (200)**:
```json
{
  "message": "success"
}
```

**Resposta de Erro (500)**:
```json
{
  "error": "error create group: 403 forbidden"
}
```

**Exemplos cURL**:

```bash
# Adicionar participantes
curl -X POST http://localhost:4000/group/participant \
  -H "Content-Type: application/json" \
  -H "apikey: SUA-CHAVE-API" \
  -d '{
    "groupJid": "120363XXXXXXXXXX@g.us",
    "action": "add",
    "participants": ["5511999999999"]
  }'

# Remover participantes
curl -X POST http://localhost:4000/group/participant \
  -H "Content-Type: application/json" \
  -H "apikey: SUA-CHAVE-API" \
  -d '{
    "groupJid": "120363XXXXXXXXXX@g.us",
    "action": "remove",
    "participants": ["5511888888888"]
  }'

# Promover a admin
curl -X POST http://localhost:4000/group/participant \
  -H "Content-Type: application/json" \
  -H "apikey: SUA-CHAVE-API" \
  -d '{
    "groupJid": "120363XXXXXXXXXX@g.us",
    "action": "promote",
    "participants": ["5511999999999"]
  }'

# Remover admin (rebaixar)
curl -X POST http://localhost:4000/group/participant \
  -H "Content-Type: application/json" \
  -H "apikey: SUA-CHAVE-API" \
  -d '{
    "groupJid": "120363XXXXXXXXXX@g.us",
    "action": "demote",
    "participants": ["5511999999999"]
  }'
```

---

## Configurações do Grupo

### Alterar Foto

Define a foto do grupo.

**Endpoint**: `POST /group/photo`

**Body**:
```json
{
  "groupJid": "120363XXXXXXXXXX@g.us",
  "image": "https://exemplo.com/logo.jpg"
}
```

Ou com base64:

```json
{
  "groupJid": "120363XXXXXXXXXX@g.us",
  "image": "data:image/jpeg;base64,/9j/4AAQSkZJRg..."
}
```

**Parâmetros**:

| Campo | Tipo | Obrigatório | Descrição |
|-------|------|-------------|-----------|
| `groupJid` | string | ✅ Sim | JID do grupo |
| `image` | string | ✅ Sim | URL ou base64 da imagem |

**Formatos Aceitos**: 
- URL (http:// ou https://)
- Base64 (data:image/jpeg;base64,... ou data:image/png;base64,...)

**Nota**: Apenas admins podem alterar a foto.

**Resposta de Sucesso (200)**:
```json
{
  "message": "success",
  "data": "abc123"
}
```

O campo `data` contém o ID da nova foto.

**Exemplo cURL**:
```bash
# Com URL
curl -X POST http://localhost:4000/group/photo \
  -H "Content-Type: application/json" \
  -H "apikey: SUA-CHAVE-API" \
  -d '{
    "groupJid": "120363XXXXXXXXXX@g.us",
    "image": "https://exemplo.com/logo.jpg"
  }'

# Com base64
curl -X POST http://localhost:4000/group/photo \
  -H "Content-Type: application/json" \
  -H "apikey: SUA-CHAVE-API" \
  -d '{
    "groupJid": "120363XXXXXXXXXX@g.us",
    "image": "data:image/jpeg;base64,/9j/4AAQSkZJRg..."
  }'
```

---

### Alterar Nome

Altera o nome do grupo.

**Endpoint**: `POST /group/name`

**Body**:
```json
{
  "groupJid": "120363XXXXXXXXXX@g.us",
  "name": "Novo Nome do Grupo"
}
```

**Parâmetros**:

| Campo | Tipo | Obrigatório | Descrição |
|-------|------|-------------|-----------|
| `groupJid` | string | ✅ Sim | JID do grupo |
| `name` | string | ✅ Sim | Novo nome (máx 25 caracteres) |

**Nota**: Apenas admins podem alterar o nome.

**Resposta de Sucesso (200)**:
```json
{
  "message": "success"
}
```

**Exemplo cURL**:
```bash
curl -X POST http://localhost:4000/group/name \
  -H "Content-Type: application/json" \
  -H "apikey: SUA-CHAVE-API" \
  -d '{
    "groupJid": "120363XXXXXXXXXX@g.us",
    "name": "Equipe Marketing 2025"
  }'
```

---

### Alterar Descrição

Altera a descrição do grupo.

**Endpoint**: `POST /group/description`

**Body**:
```json
{
  "groupJid": "120363XXXXXXXXXX@g.us",
  "description": "Grupo para discussões sobre marketing e estratégias de vendas."
}
```

**Parâmetros**:

| Campo | Tipo | Obrigatório | Descrição |
|-------|------|-------------|-----------|
| `groupJid` | string | ✅ Sim | JID do grupo |
| `description` | string | ✅ Sim | Nova descrição (máx 512 caracteres) |

**Nota**: Apenas admins podem alterar a descrição.

**Resposta de Sucesso (200)**:
```json
{
  "message": "success"
}
```

**Exemplo cURL**:
```bash
curl -X POST http://localhost:4000/group/description \
  -H "Content-Type: application/json" \
  -H "apikey: SUA-CHAVE-API" \
  -d '{
    "groupJid": "120363XXXXXXXXXX@g.us",
    "description": "Grupo oficial da empresa - Apenas assuntos profissionais"
  }'
```

---

## Meus Grupos (Admin)

Lista apenas os grupos onde você é o **proprietário/criador**.

**Endpoint**: `GET /group/myall`

**Headers**:
```
apikey: SUA-CHAVE-API
```

**Resposta de Sucesso (200)**:
```json
{
  "message": "success",
  "data": [
    {
      "JID": "120363XXXXXXXXXX@g.us",
      "OwnerJID": "5511999999999@s.whatsapp.net",
      "GroupName": {
        "Name": "Meu Grupo",
        "NameSetAt": "2025-01-15T10:30:00Z",
        "NameSetBy": "5511999999999@s.whatsapp.net"
      },
      "Participants": [...]
    }
  ]
}
```

**Diferença**: 
- `/group/list` - Todos os grupos que você participa
- `/group/myall` - Apenas grupos que você criou (owner)

**Exemplo cURL**:
```bash
curl -X GET http://localhost:4000/group/myall \
  -H "apikey: SUA-CHAVE-API"
```

---

## Entrar via Link

Entra em um grupo através de um link de convite.

**Endpoint**: `POST /group/join`

**Body**:
```json
{
  "code": "ABCDEFGHIJKLMNOP"
}
```

Ou com URL completa:

```json
{
  "code": "https://chat.whatsapp.com/ABCDEFGHIJKLMNOP"
}
```

**Parâmetros**:

| Campo | Tipo | Obrigatório | Descrição |
|-------|------|-------------|-----------|
| `code` | string | ✅ Sim | Código do link ou URL completa |

**Nota**: O código é a parte após `https://chat.whatsapp.com/`.

**Resposta de Sucesso (200)**:
```json
{
  "message": "success"
}
```

**Resposta de Erro (500)**:
```json
{
  "error": "error create group: 404 not found"
}
```

Erros comuns:
- `404 not found` - Link inválido ou expirado
- `403 forbidden` - Você foi banido ou removido
- `406 not acceptable` - Grupo cheio (256 participantes)

**Exemplo cURL**:
```bash
# Com código
curl -X POST http://localhost:4000/group/join \
  -H "Content-Type: application/json" \
  -H "apikey: SUA-CHAVE-API" \
  -d '{
    "code": "ABCDEFGHIJKLMNOP"
  }'

# Com URL completa
curl -X POST http://localhost:4000/group/join \
  -H "Content-Type: application/json" \
  -H "apikey: SUA-CHAVE-API" \
  -d '{
    "code": "https://chat.whatsapp.com/ABCDEFGHIJKLMNOP"
  }'
```

---

## Sair do Grupo

Sai de um grupo WhatsApp.

**Endpoint**: `POST /group/leave`

**Body**:
```json
{
  "groupJid": "120363XXXXXXXXXX@g.us"
}
```

**Parâmetros**:

| Campo | Tipo | Obrigatório | Descrição |
|-------|------|-------------|-----------|
| `groupJid` | string | ✅ Sim | JID do grupo |

**Nota**: Se você for o único admin, o grupo ficará sem admins até que outro seja promovido ou o grupo seja excluído.

**Resposta de Sucesso (200)**:
```json
{
  "message": "success"
}
```

**Exemplo cURL**:
```bash
curl -X POST http://localhost:4000/group/leave \
  -H "Content-Type: application/json" \
  -H "apikey: SUA-CHAVE-API" \
  -d '{
    "groupJid": "120363XXXXXXXXXX@g.us"
  }'
```

---

## Fluxo Completo de Gerenciamento

### 1. Criar e Configurar Grupo

```bash
# 1. Criar grupo
GROUP_JID=$(curl -X POST http://localhost:4000/group/create \
  -H "Content-Type: application/json" \
  -H "apikey: SUA-CHAVE-API" \
  -d '{
    "groupName": "Meu Grupo",
    "participants": ["5511999999999"]
  }' | jq -r '.data.jid')

# 2. Adicionar foto
curl -X POST http://localhost:4000/group/photo \
  -H "Content-Type: application/json" \
  -H "apikey: SUA-CHAVE-API" \
  -d "{
    "groupJid": "$GROUP_JID",
    "image": "https://exemplo.com/logo.jpg"
  }"

# 3. Adicionar descrição
curl -X POST http://localhost:4000/group/description \
  -H "Content-Type: application/json" \
  -H "apikey: SUA-CHAVE-API" \
  -d "{
    "groupJid": "$GROUP_JID",
    "description": "Grupo oficial da empresa"
  }"

# 4. Obter link de convite
curl -X POST http://localhost:4000/group/invitelink \
  -H "Content-Type: application/json" \
  -H "apikey: SUA-CHAVE-API" \
  -d "{
    "groupJid": "$GROUP_JID",
    "reset": false
  }"
```

### 2. Gerenciar Participantes

```bash
# Adicionar novos membros
curl -X POST http://localhost:4000/group/participant \
  -H "Content-Type: application/json" \
  -H "apikey: SUA-CHAVE-API" \
  -d '{
    "groupJid": "120363XXXXXXXXXX@g.us",
    "action": "add",
    "participants": ["5511888888888", "5511777777777"]
  }'

# Promover a admin
curl -X POST http://localhost:4000/group/participant \
  -H "Content-Type: application/json" \
  -H "apikey: SUA-CHAVE-API" \
  -d '{
    "groupJid": "120363XXXXXXXXXX@g.us",
    "action": "promote",
    "participants": ["5511888888888"]
  }'
```

---

## Códigos de Erro Comuns

| Código | Erro | Solução |
|--------|------|---------|
| 400 | `groupJid is required` | Forneça o JID do grupo |
| 400 | `groupName is required` | Forneça o nome do grupo |
| 400 | `participants are required` | Forneça array de participantes |
| 400 | `action is required` | Especifique a ação (add/remove/promote/demote) |
| 500 | `invalid group jid` | JID de grupo inválido (deve terminar com @g.us) |
| 500 | `403 forbidden` | Sem permissão (não é admin) |
| 500 | `404 not found` | Grupo não existe ou link inválido |

---

## Boas Práticas

### 1. Verificar Permissões
Sempre verifique se você é admin antes de tentar alterar configurações:
```bash
# 1. Obter info do grupo
INFO=$(curl -s -X POST http://localhost:4000/group/info \
  -H "Content-Type: application/json" \
  -H "apikey: SUA-CHAVE-API" \
  -d '{"groupJid": "120363XXXXXXXXXX@g.us"}')

# 2. Verificar se você é admin
echo $INFO | jq '.data.Participants[] | select(.IsAdmin == true)'
```

### 2. Validar Números
Valide números antes de adicionar ao grupo:
```bash
# 1. Verificar se número existe
curl -X POST http://localhost:4000/user/check \
  -H "Content-Type: application/json" \
  -H "apikey: SUA-CHAVE-API" \
  -d '{"number": ["5511999999999"]}'

# 2. Se IsInWhatsapp=true, adicionar ao grupo
curl -X POST http://localhost:4000/group/participant \
  -H "Content-Type: application/json" \
  -H "apikey: SUA-CHAVE-API" \
  -d '{
    "groupJid": "120363XXXXXXXXXX@g.us",
    "action": "add",
    "participants": ["5511999999999"]
  }'
```

### 3. Limites do WhatsApp
Respeite os limites do WhatsApp:
- **256 participantes** por grupo
- **25 caracteres** para nome do grupo
- **512 caracteres** para descrição
- Não adicione mais de **20 participantes de uma vez**

### 4. Tratamento de Falhas
Ao criar grupo, sempre verifique o campo `failed` na resposta:
- O campo `added` contém os participantes adicionados com sucesso
- O campo `failed` contém os que falharam (número inválido, não tem WhatsApp, etc)
- Se houver falhas, você pode tentar adicionar novamente após verificação

---

## Próximos Passos

- [API de Mensagens](./api-messages.md) - Enviar mensagens em grupos
- [API de Usuários](./api-user.md) - Gerenciar contatos
- [API de Chats](./api-chats.md) - Gerenciar conversas
- [Visão Geral da API](./api-overview.md)

---

**Documentação gerada para Evolution GO v1.0**
