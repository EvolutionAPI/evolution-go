# evolution-go fork — Solar Teles bug fixes

**Path no repo:** `evolution-go/`
**Imagem resultante:** `registry.gitlab.com/douglasanpa/nextbotsdr/evolution-go:0.7.2-solar-fixes`

## Por que esse fork existe

4 bugs **críticos para o Solar Teles** estão reportados a **4+ semanas** no upstream
e os PRs com fix ainda não foram mergeados (ou os autores abandonaram). Cada bug,
sozinho, é capaz de derrubar a integração WhatsApp do Solar Teles em produção:

| # | PR | Sintoma | Detecção Solar Teles |
|---|---|---|---|
| #136 | cesar-carlos | Webhook/config são apagados ao recriar instance com mesmo nome | Webhook inbound quebrado silenciosamente |
| #178 | wilsonborba | Postgres connection pool leak — satura `max_connections` em ~6 dias | QR pairing para de funcionar; SendText vira 500 |
| #149 | iagocotta | `GET /instance/qr` derruba sessão ativa ao tentar regerar | "Por que minha sessão caiu sozinha?" |
| #163 | FlavioPulli | `/user/avatar` dá `info query timed out` + revoke/edit ignorados (`+` JID) | Avatar 500; apagar/editar mensagem silenciosamente ignorados |

Construir essa imagem localmente antes do merge upstream elimina os 3 sem
precisar esperar o release da v0.7.3.

## Como usar

```yaml
# docker-compose.yml — substitui o serviço evolution-go padrão
services:
  evolution-go:
    image: registry.gitlab.com/douglasanpa/nextbotsdr/evolution-go:0.7.2-solar-fixes
    container_name: evolution-go
    environment:
      - SERVER_TYPE=http
      - SERVER_PORT=8080
      - AUTHENTICATION_TYPE=apikey
      - AUTHENTICATION_API_KEY=${EVOLUTION_API_KEY}
      - AUTHENTICATION_EXPOSE_IN_FETCH_INSTANCES=true
      - DATABASE_ENABLED=true
      - DATABASE_PROVIDER=postgresql
      - DATABASE_CONNECTION_URI=postgresql://evogo:${DB_PASSWORD}@evogo-db:5432/evogo?schema=public
      - POSTGRES_ENABLED=true
      - POSTGRES_CONNECTION_URI=postgresql://evogo:${DB_PASSWORD}@evogo-db:5432/evogo_auth
      - POSTGRES_AUTH_DB=postgresql://evogo:${DB_PASSWORD}@evogo-db:5432/evogo_auth
      - WEBHOOK_GLOBAL_URL=${DOMAIN}/webhook/whatsapp
      - WEBHOOK_GLOBAL_ENABLED=true
    restart: unless-stopped
    ports:
      - "8080:8080"
```

Substituir a imagem padrão `evoapicloud/evolution-go:latest` por esta no
`docker-compose.yml` do Solar Teles.

## Como buildar localmente (testes / dev)

```bash
cd evolution-go/
docker build -t evolution-go:0.7.2-solar-fixes .
```

Build multi-stage: ~3min na primeira vez (download deps Go), ~30s nas
seguintes (cache de camadas).

## Validação automática

Cada patch é aplicado pelo `Dockerfile` via `git apply --reject` durante o
build. Camadas:

```
1. git clone --depth 1 --branch 0.7.2  →  base
2. apply pr-136.patch                  →  webhook fix
3. apply pr-178.patch                  →  postgres leak fix
4. apply pr-149.patch                  →  QR-no-disconnect fix
5. apply pr-163-avatar-canonical.patch →  avatar/revoke/edit JID fix (CanonicalJID)
5. go build ./cmd/evolution-go          →  binary
6. COPY binary to alpine                →  runtime
```

Se algum patch falhar, o `RUN` quebra o build com exit code != 0 — fork
não-builda = fork não-sobe em prod.

## Verificação de impacto em produção

### Antes de subir o fork

```bash
# Contar conexões abertas na DB auth do evolution
docker compose exec -T evogo-db psql -U evogo -d evogo_auth -c \
  "SELECT count(*), state FROM pg_stat_activity GROUP BY state ORDER BY 1 DESC;"

# Em prod Solar Teles, esperar ver ~15-30 conexões idle crescentes
# (sinal de leak do PR #178)
```

### Depois (com fork aplicado)

```bash
# Mesmo comando — conexões idle devem ficar planas (~2-4) mesmo após
# várias recriações de instance / reconnects
```

Critério de aceite: conexão idle plana por ≥24h mesmo com
`POST /instance/reconnect` acionado 20+ vezes.

## Quando descartar o fork

Quando:

1. **PR #178** (ou similar) for mergeado em `evolution-foundation/evolution-go`
2. **PR #149** for mergeado (idem)
3. **PR #136** já está aplicado no seu fork anterior — manter até confirmar merge upstream
4. Cortada nova tag ≥ v0.7.3 incluindo esses fixes

Aí trocar a imagem do `docker-compose.yml` de volta para
`evoapicloud/evolution-go:latest` (ou v0.7.3+ fixa).

## Arquivos do fork

```
evolution-go/
├── Dockerfile            # Multi-stage com aplicação de 3 patches
├── README.md             # Este arquivo
└── patch/
    ├── pr-136.patch                 # webhook wiping fix
    ├── pr-178.patch                 # postgres connection leak
    ├── pr-149.patch                 # QR disconnect active session
    └── pr-163-avatar-canonical.patch # avatar + revoke/edit JID (+ prefix)
```

## Pipeline CI/CD

Adicionado stage `build-evolution-go` em `.gitlab-ci.yml` no projeto
nextbotsdr. Dispara no push da branch `dev` ou via trigger manual.

A imagem é publicada no GitLab Container Registry do projeto e consumida
pelo `docker-compose.yml` do Solar Teles na VPS.
