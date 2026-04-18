# Deploy com Docker

Guia de deploy do Evolution GO usando Docker, Docker Compose, Swarm e Kubernetes.

## Índice

- [Visão Geral](#visão-geral)
- [Deploy com Docker Compose](#deploy-com-docker-compose)
- [Deploy com Docker Swarm](#deploy-com-docker-swarm)
- [Deploy com Kubernetes](#deploy-com-kubernetes)
- [Otimização e Gestão](#otimização-e-gestão)
- [Troubleshooting](#troubleshooting)

---

## Visão Geral

### Estratégias de Deploy

| Estratégia | Uso Recomendado | Complexidade | Escalabilidade |
|------------|-----------------|--------------|----------------|
| **Docker Compose** | Desenvolvimento, testes, deploys pequenos | Baixa | Limitada (single-host) |
| **Docker Swarm** | Produção pequena/média, HA | Média | Boa (multi-host) |
| **Kubernetes** | Produção enterprise, orquestração avançada | Alta | Excelente |

### Arquitetura de Componentes

```
┌─────────────────────────────────────────────────────────────────┐
│                     EVOLUTION GO STACK                          │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌──────────────┐      ┌──────────────┐      ┌──────────────┐ │
│  │ Evolution GO │◄────►│  PostgreSQL  │      │  RabbitMQ    │ │
│  │   (API)      │      │   (Auth DB)  │      │  (Events)    │ │
│  │  Port: 4000  │      │   (Users DB) │      │  Port: 5672  │ │
│  └──────┬───────┘      └──────────────┘      └──────────────┘ │
│         │                                                       │
│         │              ┌──────────────┐      ┌──────────────┐ │
│         └─────────────►│    MinIO     │      │     NATS     │ │
│                        │   (Media)    │      │  (Optional)  │ │
│                        │  Port: 9000  │      │  Port: 4222  │ │
│                        └──────────────┘      └──────────────┘ │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### Imagem Docker

- **Registry**: `evoapicloud/evolution-go`
- **Tags**: `latest`, `v1.x.x`
- **Base**: Alpine Linux 3.19.1
- **Tamanho**: ~50MB (compactada)
- **Arquiteturas**: amd64, arm64

---

## Deploy com Docker Compose

### Setup Básico

Configuração mínima com Evolution GO + PostgreSQL.

#### docker-compose.yml

```yaml
version: '3.8'

services:
  evolution-go:
    image: evoapicloud/evolution-go:latest
    container_name: evolution-go
    restart: unless-stopped
    ports:
      - "4000:4000"
    environment:
      SERVER_PORT: 4000
      CLIENT_NAME: "evolution"
      GLOBAL_API_KEY: "SUBSTITUA-POR-UUID-FORTE"

      POSTGRES_AUTH_DB: "postgresql://postgres:postgres@postgres:5432/evogo_auth?sslmode=disable"
      POSTGRES_USERS_DB: "postgresql://postgres:postgres@postgres:5432/evogo_users?sslmode=disable"
      DATABASE_SAVE_MESSAGES: "false"

      WADEBUG: "INFO"
      LOGTYPE: "console"
      CONNECT_ON_STARTUP: "false"
      WEBHOOK_FILES: "true"

    volumes:
      - evolution_data:/app/dbdata
      - evolution_logs:/app/logs
    networks:
      - evolution_network
    depends_on:
      - postgres

  postgres:
    image: postgres:15-alpine
    container_name: postgres
    restart: unless-stopped
    environment:
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: postgres
      POSTGRES_DB: postgres
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data
      - ./init-db.sql:/docker-entrypoint-initdb.d/init-db.sql
    networks:
      - evolution_network

volumes:
  evolution_data:
  evolution_logs:
  postgres_data:

networks:
  evolution_network:
    driver: bridge
```

#### init-db.sql

```sql
CREATE DATABASE evogo_auth;
CREATE DATABASE evogo_users;
SELECT 'Databases criados com sucesso!' as message;
```

#### Deploy

```bash
# Gerar API Key
uuidgen

# Editar docker-compose.yml e inserir API Key

# Iniciar
docker-compose up -d

# Verificar
docker-compose logs -f evolution-go
curl http://localhost:4000/server/ok
```

### Setup Completo

Incluindo RabbitMQ, MinIO e NATS.

```yaml
version: '3.8'

services:
  evolution-go:
    image: evoapicloud/evolution-go:latest
    restart: unless-stopped
    ports:
      - "4000:4000"
    environment:
      SERVER_PORT: 4000
      GLOBAL_API_KEY: "SUA-CHAVE-AQUI"

      POSTGRES_AUTH_DB: "postgresql://postgres:senha@postgres:5432/evogo_auth?sslmode=disable"
      POSTGRES_USERS_DB: "postgresql://postgres:senha@postgres:5432/evogo_users?sslmode=disable"
      DATABASE_SAVE_MESSAGES: "true"

      AMQP_URL: "amqp://admin:admin@rabbitmq:5672/default"
      AMQP_GLOBAL_ENABLED: "true"
      AMQP_GLOBAL_EVENTS: "messages.upsert,messages.update,connection.update"
      
      MINIO_ENABLED: "true"
      MINIO_ENDPOINT: "minio:9000"
      MINIO_ACCESS_KEY: "minioadmin"
      MINIO_SECRET_KEY: "minioadmin"
      MINIO_BUCKET: "evolution-media"
      MINIO_USE_SSL: "false"

    volumes:
      - evolution_data:/app/dbdata
      - evolution_logs:/app/logs
    depends_on:
      - postgres
      - rabbitmq
      - minio

  postgres:
    image: postgres:15-alpine
    restart: unless-stopped
    environment:
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: senha
      POSTGRES_DB: postgres
    volumes:
      - postgres_data:/var/lib/postgresql/data
      - ./init-db.sql:/docker-entrypoint-initdb.d/init-db.sql

  rabbitmq:
    image: rabbitmq:3-management-alpine
    restart: unless-stopped
    environment:
      RABBITMQ_DEFAULT_USER: admin
      RABBITMQ_DEFAULT_PASS: admin
      RABBITMQ_DEFAULT_VHOST: default
    ports:
      - "5672:5672"
      - "15672:15672"
    volumes:
      - rabbitmq_data:/var/lib/rabbitmq

  minio:
    image: minio/minio:latest
    restart: unless-stopped
    command: server /data --console-address ":9001"
    environment:
      MINIO_ROOT_USER: minioadmin
      MINIO_ROOT_PASSWORD: minioadmin
    ports:
      - "9000:9000"
      - "9001:9001"
    volumes:
      - minio_data:/data

  nats:
    image: nats:2-alpine
    restart: unless-stopped
    ports:
      - "4222:4222"
      - "8222:8222"

volumes:
  evolution_data:
  evolution_logs:
  postgres_data:
  rabbitmq_data:
  minio_data:
```

**Acessos:**
- Evolution GO: http://localhost:4000
- Swagger: http://localhost:4000/swagger/index.html
- RabbitMQ: http://localhost:15672 (admin/admin)
- MinIO: http://localhost:9001 (minioadmin/minioadmin)

### Configurações Avançadas

#### Arquivo .env

```bash
# .env
EVOLUTION_VERSION=latest
POSTGRES_VERSION=15-alpine

# Portas
EVOLUTION_PORT=4000
POSTGRES_PORT=5432

# Credenciais
POSTGRES_USER=postgres
POSTGRES_PASSWORD=senha_forte
RABBITMQ_USER=admin
RABBITMQ_PASS=senha_forte

# Evolution GO
GLOBAL_API_KEY=df16caad-d0d2-41b2-bec5-75b90048a0db
CLIENT_NAME=evolution-prod
```

Referência no compose:
```yaml
services:
  evolution-go:
    image: evoapicloud/evolution-go:${EVOLUTION_VERSION:-latest}
    ports:
      - "${EVOLUTION_PORT:-4000}:4000"
    environment:
      GLOBAL_API_KEY: "${GLOBAL_API_KEY}"
```

#### Health Checks

```yaml
services:
  evolution-go:
    healthcheck:
      test: ["CMD", "wget", "-q", "--spider", "http://localhost:4000/server/ok"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 40s

  postgres:
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U postgres"]
      interval: 10s
      timeout: 5s
      retries: 5

  rabbitmq:
    healthcheck:
      test: ["CMD", "rabbitmq-diagnostics", "ping"]
      interval: 30s
      timeout: 10s
      retries: 3
```

#### Resource Limits

```yaml
services:
  evolution-go:
    deploy:
      resources:
        limits:
          cpus: '2.0'
          memory: 2G
        reservations:
          cpus: '0.5'
          memory: 512M

  postgres:
    deploy:
      resources:
        limits:
          cpus: '1.0'
          memory: 1G
        reservations:
          cpus: '0.25'
          memory: 256M
```

---

## Deploy com Docker Swarm

### Inicialização

```bash
# Manager node
docker swarm init --advertise-addr 192.168.1.10

# Worker nodes
docker swarm join --token SWMTKN-1-xxxxx 192.168.1.10:2377

# Verificar cluster
docker node ls
```

### Preparação

```bash
# Volumes
docker volume create evolution_go_data
docker volume create evolution_go_logs

# Rede
docker network create --driver overlay network_public
```

### docker-compose.swarm.yml

```yaml
version: '3.8'

services:
  evolution_go:
    image: evoapicloud/evolution-go:latest
    networks:
      - network_public
    environment:
      SERVER_PORT: 4000
      GLOBAL_API_KEY: "sua-chave-api"
      POSTGRES_AUTH_DB: "postgresql://user:pass@postgres:5432/evogo_auth"
      POSTGRES_USERS_DB: "postgresql://user:pass@postgres:5432/evogo_users"

    volumes:
      - evolution_go_data:/app/dbdata
      - evolution_go_logs:/app/logs

    deploy:
      replicas: 3
      placement:
        constraints:
          - node.role == worker
      restart_policy:
        condition: on-failure
        delay: 5s
        max_attempts: 3
        window: 120s
      update_config:
        parallelism: 1
        delay: 10s
        failure_action: rollback
        order: start-first
      rollback_config:
        parallelism: 1
        delay: 5s
      labels:
        - traefik.enable=true
        - traefik.http.routers.evolution.rule=Host(`evolution.domain.com`)
        - traefik.http.routers.evolution.entrypoints=websecure
        - traefik.http.routers.evolution.tls.certresolver=letsencrypt
        - traefik.http.services.evolution.loadbalancer.server.port=4000

volumes:
  evolution_go_data:
    external: true
  evolution_go_logs:
    external: true

networks:
  network_public:
    external: true
```

### Deploy e Gerenciamento

```bash
# Deploy
docker stack deploy -c docker-compose.swarm.yml evolution

# Status
docker stack ls
docker service ls
docker service ps evolution_evolution_go

# Logs
docker service logs evolution_evolution_go -f

# Escalar
docker service scale evolution_evolution_go=5

# Atualizar (rolling update)
docker service update --image evoapicloud/evolution-go:v1.2.0 evolution_evolution_go

# Remover
docker stack rm evolution
```

---

## Deploy com Kubernetes

### Manifests Básicos

#### Namespace

```yaml
# namespace.yaml
apiVersion: v1
kind: Namespace
metadata:
    name: evolution-go
```

#### ConfigMap

```yaml
# configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: evolution-config
  namespace: evolution-go
data:
  SERVER_PORT: "4000"
  CLIENT_NAME: "evolution"
  WADEBUG: "INFO"
  LOGTYPE: "console"
  CONNECT_ON_STARTUP: "false"
  WEBHOOK_FILES: "true"
  DATABASE_SAVE_MESSAGES: "false"
```

#### Secrets

```bash
kubectl create secret generic evolution-secrets \
  --from-literal=GLOBAL_API_KEY=$(uuidgen) \
  --from-literal=POSTGRES_PASSWORD=$(openssl rand -base64 32) \
  --namespace=evolution-go
```

#### Deployment

```yaml
# deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: evolution-go
  namespace: evolution-go
spec:
  replicas: 3
  selector:
    matchLabels:
      app: evolution-go
  template:
    metadata:
      labels:
        app: evolution-go
    spec:
      containers:
      - name: evolution-go
        image: evoapicloud/evolution-go:latest
        ports:
        - containerPort: 4000
        env:
        - name: SERVER_PORT
          valueFrom:
            configMapKeyRef:
              name: evolution-config
              key: SERVER_PORT
        - name: GLOBAL_API_KEY
          valueFrom:
            secretKeyRef:
              name: evolution-secrets
              key: GLOBAL_API_KEY
        resources:
          requests:
            memory: "512Mi"
            cpu: "500m"
          limits:
            memory: "2Gi"
            cpu: "2000m"
        livenessProbe:
          httpGet:
            path: /server/ok
            port: 4000
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /server/ok
            port: 4000
          initialDelaySeconds: 10
          periodSeconds: 5
        volumeMounts:
        - name: evolution-data
          mountPath: /app/dbdata
        - name: evolution-logs
          mountPath: /app/logs
      volumes:
      - name: evolution-data
        persistentVolumeClaim:
          claimName: evolution-data-pvc
      - name: evolution-logs
        persistentVolumeClaim:
          claimName: evolution-logs-pvc
```

#### Service

```yaml
# service.yaml
apiVersion: v1
kind: Service
metadata:
  name: evolution-go-service
  namespace: evolution-go
spec:
  type: LoadBalancer
  selector:
    app: evolution-go
  ports:
  - port: 4000
    targetPort: 4000
```

#### Ingress

```yaml
# ingress.yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: evolution-ingress
  namespace: evolution-go
  annotations:
    kubernetes.io/ingress.class: "nginx"
    cert-manager.io/cluster-issuer: "letsencrypt-prod"
spec:
  tls:
  - hosts:
    - evolution.domain.com
    secretName: evolution-tls
  rules:
  - host: evolution.domain.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: evolution-go-service
            port:
              number: 4000
```

#### HorizontalPodAutoscaler

```yaml
# hpa.yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: evolution-hpa
  namespace: evolution-go
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: evolution-go
  minReplicas: 3
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
  - type: Resource
    resource:
      name: memory
      target:
        type: Utilization
        averageUtilization: 80
```

### Deploy Kubernetes

```bash
# Aplicar manifests
kubectl apply -f namespace.yaml
kubectl apply -f configmap.yaml
kubectl apply -f secrets.yaml
kubectl apply -f deployment.yaml
kubectl apply -f service.yaml
kubectl apply -f ingress.yaml
kubectl apply -f hpa.yaml

# Verificar
kubectl get all -n evolution-go
kubectl get pods -n evolution-go

# Logs
kubectl logs -f deployment/evolution-go -n evolution-go

# Escalar
kubectl scale deployment evolution-go --replicas=5 -n evolution-go

# Atualizar
kubectl set image deployment/evolution-go \
  evolution-go=evoapicloud/evolution-go:v1.2.0 \
  -n evolution-go

# Rollback
kubectl rollout undo deployment/evolution-go -n evolution-go
```

---

## Otimização e Gestão

### Gestão de Volumes

#### Backup

```bash
# Backup volume
docker run --rm \
  -v evolution_data:/data \
  -v $(pwd):/backup \
  alpine tar czf /backup/backup-$(date +%Y%m%d).tar.gz -C /data .

# Restaurar
docker run --rm \
  -v evolution_data:/data \
  -v $(pwd):/backup \
  alpine tar xzf /backup/backup-20250111.tar.gz -C /data
```

### Logging

```yaml
services:
  evolution-go:
logging:
  driver: "json-file"
  options:
    max-size: "10m"
    max-file: "3"
```

### Boas Práticas

**Segurança:**
- Não executar containers como root
- Usar secrets para credenciais
- Habilitar HTTPS em produção
- Configurar resource limits
- Implementar health checks

**Performance:**
- Definir resource requests/limits
- Usar health checks
- Implementar HPA (Kubernetes)
- Configurar connection pooling

**Monitoramento:**
- Coletar logs centralizados
- Implementar métricas (Prometheus)
- Configurar alertas
- Dashboard de visualização (Grafana)

---

## Troubleshooting

### Container Reiniciando

```bash
# Ver logs
docker-compose logs evolution-go

# Verificar variáveis obrigatórias
# - GLOBAL_API_KEY
# - POSTGRES_*_DB
```

### Conectividade PostgreSQL

```bash
# Testar conexão
docker-compose exec evolution-go ping postgres

# Verificar porta
docker-compose exec evolution-go nc -zv postgres 5432

# Inspecionar rede
docker network inspect evolution_network
```

### Sem Espaço em Disco

```bash
# Ver uso
docker system df

# Limpar
docker container prune
docker image prune
docker volume prune  # CUIDADO: apaga volumes não utilizados
docker system prune -a
```

### OOM (Out of Memory)

```bash
# Ver eventos OOM
docker events --filter 'event=oom'

# Ver uso de memória
docker stats evolution-go

# Aumentar limite
deploy:
  resources:
    limits:
      memory: 4G
```

---

## Comandos Úteis

### Docker Compose

```bash
docker-compose up -d          # Iniciar
docker-compose ps             # Status
docker-compose logs -f        # Logs
docker-compose stop           # Parar
docker-compose restart        # Reiniciar
docker-compose down           # Remover
docker-compose pull           # Atualizar imagens
```

### Docker Swarm

```bash
docker stack deploy -c file.yml name    # Deploy
docker stack ls                          # Listar stacks
docker service ls                        # Listar serviços
docker service logs name -f              # Logs
docker service scale name=N              # Escalar
docker stack rm name                     # Remover
```

### Kubernetes

```bash
kubectl apply -f file.yaml              # Aplicar
kubectl get all -n namespace            # Listar recursos
kubectl logs -f deployment/name         # Logs
kubectl scale deployment name --replicas=N  # Escalar
kubectl delete -f file.yaml             # Deletar
```

---

**Documentação Evolution GO v1.0**
