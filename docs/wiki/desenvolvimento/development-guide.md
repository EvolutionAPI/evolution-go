# Guia de Desenvolvimento

Guia completo para desenvolver e contribuir com o Evolution GO.

## Índice

- [Visão Geral](#visão-geral)
- [Requisitos](#requisitos)
- [Setup Inicial](#setup-inicial)
- [Estrutura do Projeto](#estrutura-do-projeto)
- [Executando Localmente](#executando-localmente)
- [Makefile e Comandos](#makefile-e-comandos)
- [Configuração de IDE](#configuração-de-ide)
- [Convenções de Código](#convenções-de-código)
- [Workflow de Desenvolvimento](#workflow-de-desenvolvimento)
- [Próximos Passos](#próximos-passos)

---

## Visão Geral

O Evolution GO é um **gateway de API WhatsApp** escrito em Go, utilizando:

- **Linguagem**: Go 1.24+
- **Framework Web**: Gin
- **ORM**: GORM
- **Banco de Dados**: PostgreSQL / SQLite
- **WhatsApp Library**: whatsmeow
- **Documentação API**: Swagger

---

## Requisitos

### Obrigatórios

- **Go 1.24+** ([Download](https://go.dev/dl/))
- **Git** ([Download](https://git-scm.com/downloads))
- **PostgreSQL 12+** ([Download](https://www.postgresql.org/download/))
  - Ou usar Docker: `docker run -d -p 5432:5432 -e POSTGRES_PASSWORD=postgres postgres:15-alpine`

### Opcionais

- **Make** (incluído no Linux/Mac, Windows: `choco install make`)
- **Docker** e **Docker Compose** (para desenvolvimento com containers)
- **RabbitMQ** (para testar sistema de eventos)
- **MinIO** (para testar armazenamento de mídia)

### Ferramentas Recomendadas

- **IDE**: VSCode, GoLand, ou Vim/Neovim
- **Postman** ou **Insomnia** (para testar API)
- **PostgreSQL Client**: pgAdmin, DBeaver, ou psql
- **Git GUI**: GitKraken, Sourcetree (opcional)

---

## Setup Inicial

### 1. Clonar o Repositório

```bash
# Via HTTPS
git clone https://git.evoai.app/Evolution/evolution-go.git
cd evolution-go

# Ou via SSH (se configurado)
git clone git@git.evochat.com:Evolution/evolution-go.git
cd evolution-go
```

### 2. Instalar Dependências Go

```bash
# Baixar dependências
go mod download

# Ou usando Make
make deps
```

### 3. Configurar PostgreSQL

**Opção A: PostgreSQL Local**

```bash
# Ubuntu/Debian
sudo apt-get install postgresql postgresql-contrib
sudo systemctl start postgresql

# macOS
brew install postgresql@15
brew services start postgresql@15

# Criar databases
sudo -u postgres psql << EOF
CREATE DATABASE evogo_auth;
CREATE DATABASE evogo_users;
EOF
```

**Opção B: PostgreSQL via Docker**

```bash
docker run -d \
  --name postgres \
  -p 5432:5432 \
  -e POSTGRES_PASSWORD=postgres \
  -v postgres_data:/var/lib/postgresql/data \
  postgres:15-alpine

# Criar databases
docker exec -i postgres psql -U postgres << EOF
CREATE DATABASE evogo_auth;
CREATE DATABASE evogo_users;
EOF
```

### 4. Configurar Variáveis de Ambiente

```bash
# Copiar arquivo de exemplo
cp .env.example .env

# Editar configurações
nano .env
```

**Configuração mínima (.env)**:

```env
# Servidor
SERVER_PORT=4000
CLIENT_NAME=evolution-dev

# API Key (gere uma segura)
GLOBAL_API_KEY=dev-key-12345

# PostgreSQL
POSTGRES_AUTH_DB=postgresql://postgres:postgres@localhost:5432/evogo_auth?sslmode=disable
POSTGRES_USERS_DB=postgresql://postgres:postgres@localhost:5432/evogo_users?sslmode=disable
DATABASE_SAVE_MESSAGES=false

# Logs
WADEBUG=DEBUG
LOGTYPE=console

# Configurações de desenvolvimento
CONNECT_ON_STARTUP=false
WEBHOOK_FILES=true
OS_NAME=Linux
```

### 5. Verificar Instalação

```bash
# Verificar versão do Go
go version
# Deve mostrar: go version go1.24.x ...

# Verificar dependências
go mod verify

# Compilar (teste)
go build ./cmd/evolution-go
```

---

## Estrutura do Projeto

```
evolution-go/
├── cmd/
│   └── evolution-go/
│       └── main.go              # Entry point da aplicação
│
├── pkg/                         # Pacotes principais
│   ├── config/                  # Configuração e env vars
│   │   ├── config.go
│   │   └── env.go
│   │
│   ├── instance/                # Gerenciamento de instâncias
│   │   ├── handler.go           # HTTP handlers
│   │   ├── service.go           # Lógica de negócio
│   │   ├── repository.go        # Acesso ao banco
│   │   └── model/
│   │       └── instance_model.go
│   │
│   ├── whatsmeow/               # Cliente WhatsApp
│   │   ├── whatsmeow.go
│   │   └── events.go
│   │
│   ├── message/                 # Mensagens
│   │   ├── handler.go
│   │   ├── service.go
│   │   ├── repository.go
│   │   └── model/
│   │
│   ├── sendMessage/             # Envio de mensagens
│   │   ├── handler.go
│   │   └── service.go
│   │
│   ├── events/                  # Produtores de eventos
│   │   ├── webhook/
│   │   ├── rabbitmq/
│   │   ├── nats/
│   │   └── websocket/
│   │
│   ├── storage/                 # Armazenamento de mídia
│   │   ├── media_storage.go
│   │   └── minio/
│   │
│   └── utils/                   # Utilitários
│       ├── logger/
│       └── validator/
│
├── docs/                        # Documentação Swagger
│   ├── docs.go
│   ├── swagger.json
│   └── swagger.yaml
│
├── docker/                      # Configurações Docker
│   └── examples/
│       ├── docker-compose.yml
│       ├── docker-compose.swarm.yml
│       └── .env.example
│
├── Dockerfile                   # Imagem Docker
├── Makefile                     # Comandos de automação
├── go.mod                       # Dependências Go
├── go.sum                       # Checksums de dependências
├── .env.example                 # Exemplo de variáveis de ambiente
└── README.md                    # Documentação principal
```

### Arquitetura em Camadas

Cada módulo segue o padrão **Handler → Service → Repository**:

```
┌─────────────┐
│   Handler   │  ← Recebe requests HTTP (Gin)
└──────┬──────┘
       │
       ▼
┌─────────────┐
│   Service   │  ← Lógica de negócio
└──────┬──────┘
       │
       ▼
┌─────────────┐
│ Repository  │  ← Acesso ao banco (GORM)
└─────────────┘
```

---

## Executando Localmente

### Modo Desenvolvimento (Hot Reload)

```bash
# Executar em modo dev
make dev

# Ou diretamente
go run cmd/evolution-go/main.go -dev
```

**Saída esperada**:

```
[GIN-debug] [WARNING] Creating an Engine instance with the Logger and Recovery middleware already attached.
[GIN-debug] [WARNING] Running in "debug" mode. Switch to "release" mode in production.
[GIN-debug] GET    /swagger/*any             --> github.com/swaggo/gin-swagger.CustomWrapHandler.func1 (3 handlers)
[GIN-debug] GET    /server/ok                --> main.main.func1 (3 handlers)
[GIN-debug] POST   /instance/create          --> evolution-go/pkg/instance.(*InstanceHandler).Create-fm (4 handlers)
...
[GIN-debug] Listening and serving HTTP on :4000
```

### Build e Executar

```bash
# Build local
make build-local

# Executar binário
./build/evolution-go
```

### Com Docker Compose

```bash
# Copiar exemplo
cp docker/examples/docker-compose.yml ./

# Editar GLOBAL_API_KEY
nano docker-compose.yml

# Iniciar
docker-compose up -d

# Ver logs
docker-compose logs -f evolution-go
```

### Acessar a Aplicação

- **API Base**: http://localhost:4000
- **Health Check**: http://localhost:4000/server/ok
- **Swagger UI**: http://localhost:4000/swagger/index.html

---

## Makefile e Comandos

### Comandos de Desenvolvimento

```bash
# Executar em modo dev (hot reload)
make dev

# Build para produção (Linux)
make build

# Build local (seu SO)
make build-local

# Build e executar
make run
```

### Comandos de Qualidade

```bash
# Instalar dependências
make deps

# Atualizar dependências
make deps-update

# Formatar código
make fmt

# Lint (fmt + vet)
make lint

# Verificar tudo (deps + lint)
make check
```

### Comandos de Documentação

```bash
# Gerar documentação Swagger
make swagger

# Requer: go install github.com/swaggo/swag/cmd/swag@latest
```

### Comandos Docker

```bash
# Build imagem Docker
make docker-build

# Executar container
make docker-run

# Executar em modo dev
make docker-run-dev

# Limpar imagens
make docker-clean
```

### Comandos Auxiliares

```bash
# Ver logs
make logs

# Limpar builds
make clean

# Setup completo (deps + tools)
make setup

# Quick start
make quick-start
```

---

## Configuração de IDE

### Visual Studio Code

#### Extensões Recomendadas

```json
{
  "recommendations": [
    "golang.go",                    // Go support
    "ms-vscode.makefile-tools",     // Makefile support
    "humao.rest-client",            // REST client
    "42crunch.vscode-openapi",      // Swagger/OpenAPI
    "streetsidesoftware.code-spell-checker"
  ]
}
```

Salvar em `.vscode/extensions.json`.

#### Settings

```json
{
  "go.useLanguageServer": true,
  "go.lintTool": "golangci-lint",
  "go.lintOnSave": "package",
  "go.formatTool": "goimports",
  "editor.formatOnSave": true,
  "go.testFlags": ["-v"],
  "go.coverOnSave": false,
  "[go]": {
    "editor.codeActionsOnSave": {
      "source.organizeImports": true
    }
  }
}
```

Salvar em `.vscode/settings.json`.

#### Tasks

```json
{
  "version": "2.0.0",
  "tasks": [
    {
      "label": "Run Dev",
      "type": "shell",
      "command": "make dev",
      "group": {
        "kind": "build",
        "isDefault": true
      }
    },
    {
      "label": "Build",
      "type": "shell",
      "command": "make build-local"
    },
    {
      "label": "Lint",
      "type": "shell",
      "command": "make lint"
    }
  ]
}
```

Salvar em `.vscode/tasks.json`.

#### Launch (Debug)

```json
{
  "version": "0.2.0",
  "configurations": [
    {
      "name": "Launch Evolution GO",
      "type": "go",
      "request": "launch",
      "mode": "debug",
      "program": "${workspaceFolder}/cmd/evolution-go",
      "args": ["-dev"],
      "env": {
        "WADEBUG": "DEBUG"
      },
      "showLog": true
    }
  ]
}
```

Salvar em `.vscode/launch.json`.

### GoLand / IntelliJ IDEA

#### Run Configuration

1. **Run → Edit Configurations**
2. **Add New Configuration → Go Build**
3. Configurar:
   - **Name**: Evolution GO Dev
   - **Run kind**: Directory
   - **Directory**: `cmd/evolution-go`
   - **Program arguments**: `-dev`
   - **Environment**: `WADEBUG=DEBUG`
   - **Working directory**: Raiz do projeto

#### File Watchers (opcional)

1. **Settings → Tools → File Watchers**
2. **Add → go fmt**
3. **Add → goimports**

---

## Convenções de Código

### Estrutura de Pacotes

```go
// Cada módulo deve ter:
package instance

// handler.go - HTTP handlers
type InstanceHandler struct {
    service InstanceService
}

func (h *InstanceHandler) Create(c *gin.Context) { }

// service.go - Lógica de negócio
type InstanceService interface {
    Create(data CreateInstanceDTO) (*Instance, error)
}

// repository.go - Banco de dados
type InstanceRepository interface {
    Save(instance *Instance) error
    FindByID(id string) (*Instance, error)
}

// model/instance_model.go - Structs
type Instance struct {
    Id   string
    Name string
}
```

### Nomenclatura

```go
// Variáveis: camelCase
var instanceName string

// Constantes: PascalCase ou UPPER_CASE
const MaxRetries = 5
const DEFAULT_TIMEOUT = 30

// Structs: PascalCase
type InstanceConfig struct { }

// Interfaces: PascalCase + sufixo "er" ou nome descritivo
type InstanceRepository interface { }
type MessageSender interface { }

// Métodos: PascalCase
func (h *Handler) CreateInstance() { }

// Funções privadas: camelCase
func parseConfig() { }

// Funções públicas: PascalCase
func NewInstanceHandler() { }
```

### Comentários

```go
// Comentários de package (em <package>_doc.go ou no arquivo principal)
// Package instance fornece gerenciamento de instâncias WhatsApp.
package instance

// Comentários de funções exportadas (obrigatório para Swagger)
// CreateInstance cria uma nova instância WhatsApp.
//
// @Summary Criar instância
// @Description Cria uma nova instância WhatsApp
// @Tags instance
// @Accept json
// @Produce json
// @Param data body CreateInstanceDTO true "Dados da instância"
// @Success 201 {object} Instance
// @Failure 400 {object} ErrorResponse
// @Router /instance/create [post]
func (h *InstanceHandler) CreateInstance(c *gin.Context) {
    // Implementação...
}

// Comentários inline para lógica complexa
// Gera UUID v4 para identificação única
instanceID := uuid.New().String()
```

### Tratamento de Erros

```go
// Sempre retornar erros, não panic
func CreateInstance(name string) (*Instance, error) {
    if name == "" {
        return nil, fmt.Errorf("name cannot be empty")
    }
    
    instance := &Instance{Name: name}
    if err := repository.Save(instance); err != nil {
        return nil, fmt.Errorf("failed to save instance: %w", err)
    }
    
    return instance, nil
}

// Em handlers HTTP
func (h *Handler) Create(c *gin.Context) {
    var dto CreateDTO
    if err := c.ShouldBindJSON(&dto); err != nil {
        c.JSON(400, gin.H{"error": err.Error()})
        return
    }
    
    instance, err := h.service.Create(dto)
    if err != nil {
        logger.LogError("Failed to create instance: %v", err)
        c.JSON(500, gin.H{"error": "Internal server error"})
        return
    }
    
    c.JSON(201, instance)
}
```

### Logging

```go
import "evolution-go/pkg/utils/logger"

// Níveis de log
logger.LogInfo("Instance %s created successfully", instanceName)
logger.LogWarn("Webhook URL not configured for instance %s", instanceName)
logger.LogError("Failed to connect to database: %v", err)
logger.LogDebug("Processing message: %+v", message)
```

---

## Workflow de Desenvolvimento

### 1. Criar Branch

```bash
# Atualizar main
git checkout main
git pull origin main

# Criar feature branch
git checkout -b feature/nome-da-feature

# Ou bugfix
git checkout -b fix/nome-do-bug
```

### 2. Desenvolver

```bash
# Executar em modo dev
make dev

# Em outro terminal, fazer mudanças no código
# O servidor reinicia automaticamente com hot-reload
```

### 3. Testar Manualmente

```bash
# Testar health check
curl http://localhost:4000/server/ok

# Criar instância
curl -X POST http://localhost:4000/instance/create \
  -H "Content-Type: application/json" \
  -H "apikey: dev-key-12345" \
  -d '{"instanceName": "teste"}'

# Verificar Swagger
open http://localhost:4000/swagger/index.html
```

### 4. Formatar e Lint

```bash
# Formatar código
make fmt

# Lint
make lint
```

### 5. Atualizar Swagger (se alterou endpoints)

```bash
# Gerar documentação
make swagger

# Verificar se docs/ foi atualizado
git status
```

### 6. Commit

```bash
# Add mudanças
git add .

# Commit com mensagem descritiva
git commit -m "feat: adiciona endpoint de listagem de instâncias

- Implementa GET /instance/list
- Adiciona paginação
- Atualiza documentação Swagger"

# Padrões de commit:
# feat: nova feature
# fix: correção de bug
# docs: documentação
# refactor: refatoração
# chore: tarefas de manutenção
```

### 7. Push e Pull Request

```bash
# Push para origin
git push origin feature/nome-da-feature

# Criar PR no GitLab/GitHub
# Preencher descrição detalhada
# Aguardar code review
```

---

## Próximos Passos

Após setup completo:

1. **[Como Contribuir](./contributing.md)** - Processo de contribuição e PR
2. **[Debugging](./debugging.md)** - Troubleshooting e resolução de problemas
3. **[Arquitetura](../conceitos-core/architecture.md)** - Entenda a arquitetura
4. **[API Overview](../guias-api/api-overview.md)** - Comece a desenvolver na API

---

## Recursos Adicionais

- **Documentação Go**: https://go.dev/doc/
- **GORM**: https://gorm.io/docs/
- **Gin Framework**: https://gin-gonic.com/docs/
- **Whatsmeow**: https://github.com/tulir/whatsmeow
- **Swagger**: https://swagger.io/docs/

---

**Dica**: Use `make help` para ver todos os comandos disponíveis!

**Mantido por**: Equipe EvoAI Services  
**Versão**: 1.0.0
