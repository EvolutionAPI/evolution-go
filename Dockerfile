# Evolution GO fork — bug fixes para Solar Teles
#
# Base: evolution-foundation/evolution-go v0.7.2 (release 2026-07-03)
# Patches (todos upstream-pendentes, mantidos aqui porque os PRs ainda
# não foram mergeados no upstream):
#
#   * PR #136 (cesar-carlos, 2026-07-27) — webhook wiping fix
#       stop Connect/advanced-settings from wiping config (settings,
#       webhook, events). Recreate com nome já em uso passa a retornar
#       409 em vez de sobrescrever config existente.
#
#   * PR #178 (wilsonborba, 2026-08-19) — Postgres connection-pool leak
#       Reusa authDB *sql.DB já configurado em vez de abrir pool novo
#       a cada StartClient/ReconnectClient. Sem ele, ~15 reconnects
#       === 32 connections abertas, satura max_connections=100 em 6
#       dias, derruba QR pairing.
#
#   * PR #149 (iagocotta, 2026-07-31) — fix GetQr disconnecting active
#       session
#       GET /instance/qr não derruba sessão ativa quando está logado
#       apenas retorna "already logged in" em vez de matar o container.
#
# Resultado: imagem registry.gitlab.com/douglasanpa/nextbotsdr/evolution-go:0.7.2-solar-fixes
#
# Quando dropar o fork: quando qualquer um dos 3 PRs for mergeado no
# upstream + nova tag v0.7.3+ for cortada. Aí volta a usar
# evoapicloud/evolution-go:latest direto.
#
# ── Stage 1: build ───────────────────────────────────────────────
FROM golang:1.25.0-alpine AS builder

ARG VERSION=0.7.2
ARG REPO=https://github.com/evolution-foundation/evolution-go.git

RUN apk update && apk add --no-cache git build-base libjpeg-turbo-dev libwebp-dev ca-certificates tzdata

WORKDIR /src

# Clone the tag we want to fork from
RUN git clone --depth 1 --branch ${VERSION} ${REPO} . \
 && git log -1 --format='%H' > /tmp/base_commit

# Apply patches in order. -3 way-back merge + fuzz tolerance for slight
# context drift across upstream commits.
COPY patch/ /patches/
RUN for p in /patches/pr-*.patch; do \
      echo "==> applying $p"; \
      git apply --whitespace=fix --reject "$p" || \
      (echo "git apply failed for $p" && exit 1); \
    done

# Build the server binary — must match upstream: CGO_ENABLED=1 + libwebp
# (chai2010/webp needs CGO; CGO_ENABLED=0 quebra com undefined: webpGetInfo)
RUN CGO_ENABLED=1 GOOS=linux go build \
      -ldflags="-s -w -X main.version=${VERSION}" \
      -o /out/server ./cmd/evolution-go

# ── Stage 2: runtime ─────────────────────────────────────────────
FROM alpine:3.19.1 AS final

LABEL org.opencontainers.image.title="evolution-go (Solar Teles fork)"
LABEL org.opencontainers.image.description="Evolution GO v0.7.2 with PR #136, #178, #149 applied"
LABEL org.opencontainers.image.source="https://github.com/evolution-foundation/evolution-go"
LABEL org.opencontainers.image.licenses="MIT"
LABEL org.opencontainers.image.version="0.7.2-solar-fixes"

RUN apk update && apk add --no-cache tzdata ffmpeg libjpeg-turbo libwebp poppler-utils ca-certificates curl \
 && addgroup -S evolution && adduser -S evolution -G evolution

WORKDIR /app

COPY --from=builder /out/server /app/server
COPY --from=builder /src/manager/dist ./manager/dist
COPY --from=builder /src/VERSION ./VERSION

# ── Mobile UX fix (mantido no fork até upstream corrigir) ─────────
# Sidebar era hidden md:flex (some no celular) + action bar opacity-0
# group-hover (invisível sem hover). Injeta CSS+JS sem precisar do
# manager/src — dist é pré-buildado no upstream.
COPY manager-mobile-fix.css manager-mobile-fix.js ./manager/dist/assets/
RUN sed -i 's|</head>|<link rel="stylesheet" href="/assets/manager-mobile-fix.css"></head>|' ./manager/dist/index.html && \
    sed -i 's|</body>|<script src="/assets/manager-mobile-fix.js"></script></body>|' ./manager/dist/index.html

ENV TZ=America/Sao_Paulo

EXPOSE 4000

HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
  CMD sh -c 'curl -fsS http://localhost:${SERVER_PORT:-4000}/server/ok || exit 1'

ENTRYPOINT ["/app/server"]
