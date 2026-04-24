# Guia de Instalação

Métodos de instalação do Evolution GO para diferentes ambientes.

## Índice

- [Requisitos](#requisitos)
- [Instalação com Docker (Recomendado)](#instalação-com-docker-recomendado)
- [Instalação Local](#instalação-local)
- [Instalação com Docker Swarm](#instalação-com-docker-swarm)
- [Stack Completa](#stack-completa)
- [Verificação](#verificação)
- [Solução de Problemas](#solução-de-problemas)

---

## Requisitos

### Docker (Recomendado)

- Docker 20.10+
- Docker Compose 2.0+
- PostgreSQL 12+ (containerizado)
- Mínimo 1GB RAM
- Mínimo 2GB disco

### Instalação Local

- Go 1.24+
- PostgreSQL 12+
- Git
- Mínimo 1GB RAM
- Mínimo 2GB disco

### Opcionais

- RabbitMQ - Sistema de filas
- NATS - Mensageria distribuída
- MinIO - Storage S3-compatible

---

## Instalação com Docker (Recomendado)

Método mais simples e adequado para produção.

### 1. Obter Arquivos

**Opção A: Clonar repositório**

```bash
git clone https://git.evoai.app/Evolution/evolution-go.git
cd evolution-go
```

**Opção B: Download direto**

```bash
mkdir evolution-go-deploy && cd evolution-go-deploy

curl -o docker-compose.yml https://raw.githubusercontent.com/EvolutionAPI/evolution-go/main/docker/examples/docker-compose.yml
curl -o init-db.sql https://raw.githubusercontent.com/EvolutionAPI/evolution-go/main/docker/examples/init-db.sql
```

### 2. Configurar API Key

Gere uma chave segura:

```bash
# Linux/Mac
uuidgen

# Python
python3 -c "import uuid; print(uuid.uuid4())"

# Resultado exemplo: df16caad-d0d2-41b2-bec5-75b90048a0db
```

Edite `docker-compose.yml`:

```bash
nano docker-compose.yml
```

Localize e substitua:
```yaml
GLOBAL_API_KEY: "SUA-CHAVE-API-SEGURA-AQUI"
```

Por:
```yaml
GLOBAL_API_KEY: "df16caad-d0d2-41b2-bec5-75b90048a0db"
```

### 3. Iniciar Serviços

```bash
docker-compose up -d
```

Aguarde ~30 segundos para inicialização completa.

### 4. Verificar Logs

```bash
# Evolution GO
docker-compose logs -f evolution-go

# PostgreSQL
docker-compose logs -f postgres
```

### 5. Acessar Aplicação

- **API**: http://localhost:4000
- **Swagger**: http://localhost:4000/swagger/index.html
- **Health Check**: http://localhost:4000/server/ok

---

## Instalação Local

Para ambientes de desenvolvimento.

### 1. Instalar PostgreSQL

**Ubuntu/Debian:**
```bash
sudo apt-get update
sudo apt-get install postgresql postgresql-contrib
```

**macOS:**
```bash
brew install postgresql@15
brew services start postgresql@15
```

**Windows:**  
Download: https://www.postgresql.org/download/windows/

### 2. Configurar Banco de Dados

```bash
sudo -u postgres psql
```

```sql
CREATE DATABASE evogo_auth;
CREATE DATABASE evogo_users;
\q
```

### 3. Instalar Go

Download: https://go.dev/dl/

Verificar:
```bash
go version
```

### 4. Clonar Repositório

```bash
git clone https://git.evoai.app/Evolution/evolution-go.git
cd evolution-go
```

### 5. Instalar Dependências

```bash
go mod download
```

Ou:
```bash
make deps
```

### 6. Configurar Ambiente

```bash
cp .env.example .env
nano .env
```

Configuração mínima:

```env
SERVER_PORT=4000
GLOBAL_API_KEY=sua-chave-gerada

POSTGRES_AUTH_DB=postgresql://postgres:postgres@localhost:5432/evogo_auth?sslmode=disable
POSTGRES_USERS_DB=postgresql://postgres:postgres@localhost:5432/evogo_users?sslmode=disable
DATABASE_SAVE_MESSAGES=false

WADEBUG=DEBUG
LOGTYPE=console
CONNECT_ON_STARTUP=false
WEBHOOK_FILES=true
OS_NAME=Linux
```

### 7. Executar

**Modo desenvolvimento:**
```bash
make dev
```

Ou:
```bash
go run cmd/evolution-go/main.go -dev
```

**Build produção:**
```bash
make build-local
./build/evolution-go
```

---

## Instalação com Docker Swarm

Para ambientes com alta disponibilidade.

### 1. Inicializar Swarm

```bash
docker swarm init
```

### 2. Criar Recursos

```bash
docker volume create evolution_go_data
docker volume create evolution_go_logs
docker network create --driver overlay network_public
```

### 3. Configurar Stack

```bash
cp docker/examples/docker-compose.swarm.yml ./docker-compose.swarm.yml
nano docker-compose.swarm.yml
```

Configure:
- `GLOBAL_API_KEY`
- `POSTGRES_AUTH_DB`
- `POSTGRES_USERS_DB`
- Labels Traefik (se aplicável)

### 4. Deploy

```bash
docker stack deploy -c docker-compose.swarm.yml evolution
```

### 5. Verificar

```bash
docker service ls
docker service logs evolution_evolution_go -f
```

---

## Stack Completa

Incluindo RabbitMQ, MinIO e NATS.

```bash
curl -o docker-compose-full.yml https://raw.githubusercontent.com/EvolutionAPI/evolution-go/main/docker/examples/docker-compose.full.yml

nano docker-compose-full.yml  # Configurar API Key

docker-compose -f docker-compose-full.yml up -d
```

**Serviços incluídos:**

| Serviço | Porta | Função |
|---------|-------|--------|
| Evolution GO | 4000 | API principal |
| PostgreSQL | 5432 | Banco de dados |
| RabbitMQ | 5672, 15672 | Filas de mensagens |
| MinIO | 9000, 9001 | Storage de objetos |
| NATS | 4222 | Mensageria (opcional) |

**Acessos:**

- Evolution GO: http://localhost:4000
- Swagger: http://localhost:4000/swagger/index.html
- RabbitMQ: http://localhost:15672 (admin/admin)
- MinIO: http://localhost:9001 (minioadmin/minioadmin)

**Configuração MinIO (primeira vez):**

1. Acesse http://localhost:9001
2. Login: minioadmin / minioadmin
3. Create bucket: `evolution-media`
4. Configurar política de acesso

---

## Verificação

### Health Check

```bash
curl http://localhost:4000/server/ok
```

Resposta esperada:
```json
{
  "status": "ok"
}
```

### Swagger UI

Acesse: http://localhost:4000/swagger/index.html

### Criar Instância de Teste

```bash
curl -X POST http://localhost:4000/instance/create \
  -H "Content-Type: application/json" \
  -H "apikey: sua-chave-api-global" \
  -d '{
    "name": "teste",
    "token": "token-unico-para-esta-instancia"
  }'
```

> **Nota:** Use a `GLOBAL_API_KEY` no header e forneça `name` + `token` no body.

### Verificar Logs

**Docker:**
```bash
docker-compose logs -f evolution-go
```

**Local:**
```bash
tail -f logs/evolution-go.log
```

---

## Solução de Problemas

### Connection Refused (PostgreSQL)

**Diagnóstico:**
```bash
docker-compose ps postgres
# Ou local:
sudo systemctl status postgresql
```

**Verificar databases:**
```bash
docker-compose exec postgres psql -U postgres -c "\l"
```

### Porta em Uso

Alterar porta no `docker-compose.yml`:
```yaml
ports:
  - "4001:4000"
```

Acesso via: http://localhost:4001

### Container Reiniciando

**Ver logs:**
```bash
docker-compose logs evolution-go
```

**Causas comuns:**
- `GLOBAL_API_KEY` não definida
- PostgreSQL não acessível
- Credenciais incorretas

### Permissões de Volume

```bash
mkdir -p logs dbdata
chmod 777 logs dbdata
```

Ou usar bind mounts:
```yaml
volumes:
  - ./logs:/app/logs
  - ./dbdata:/app/dbdata
```

### MinIO Bucket Não Existe

1. Acessar console: http://localhost:9001
2. Login: minioadmin / minioadmin
3. Criar bucket: `evolution-media`

Ou via CLI:
```bash
docker-compose exec minio mc mb /data/evolution-media
```

---

## Comandos Úteis

### Docker Compose

```bash
# Iniciar
docker-compose up -d

# Parar
docker-compose stop

# Reiniciar
docker-compose restart

# Status
docker-compose ps

# Logs
docker-compose logs -f

# Remover
docker-compose down

# Atualizar
docker-compose pull
docker-compose up -d
```

### Docker Swarm

```bash
# Listar serviços
docker service ls

# Logs
docker service logs evolution_evolution_go -f

# Escalar
docker service scale evolution_evolution_go=3

# Atualizar
docker service update --image evoapicloud/evolution-go:latest evolution_evolution_go

# Remover
docker stack rm evolution
```

---

## Próximos Passos

- **[Configuração](./configuration.md)** - Variáveis de ambiente
- **[Início Rápido](./quickstart.md)** - Primeira instância
- **[API Overview](../guias-api/api-overview.md)** - Estrutura da API
- **[Docker Deployment](../deploy-producao/docker-deployment.md)** - Deploy avançado

---

## Arquivos de Exemplo

Disponíveis em `docker/examples/`:

- [docker-compose.yml](../../../docker/examples/docker-compose.yml)
- [docker-compose.swarm.yml](../../../docker/examples/docker-compose.swarm.yml)
- [docker-compose.full.yml](../../../docker/examples/docker-compose.full.yml)
- [init-db.sql](../../../docker/examples/init-db.sql)
- [.env.example](../../../docker/examples/.env.example)

---

**Documentação Evolution GO v1.0**
