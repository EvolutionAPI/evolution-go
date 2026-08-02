FROM node:22-alpine AS manager-v2-build

WORKDIR /manager-v2
COPY manager-v2/package.json ./
RUN npm install --no-audit --no-fund
COPY manager-v2/ ./
RUN npm run typecheck && npm run build

FROM golang:1.25.0-alpine AS build

RUN apk update && apk add --no-cache git build-base libjpeg-turbo-dev libwebp-dev

WORKDIR /build

# Copiar apenas arquivos de dependências primeiro para cachear o download
COPY go.mod go.sum ./

# whatsmeow agora vem do proxy oficial (go.mau.fi/whatsmeow, sem replace local) —
# não há mais submódulo whatsmeow-lib para copiar.
RUN go mod download

# Copiar o restante do código
COPY . .

ARG VERSION=dev
# Mantém a imagem padrão sem tags. Para habilitar chamadas com Pion, use:
# docker build --build-arg GO_BUILD_TAGS=voip_pion ...
ARG GO_BUILD_TAGS=""
RUN CGO_ENABLED=1 go build -tags "${GO_BUILD_TAGS}" -ldflags "-X main.version=${VERSION}" -o server ./cmd/evolution-go

FROM alpine:3.19.1 AS final

# poppler-utils provides pdftoppm, used to rasterize PDF page 1 for /send/media document thumbnails
RUN apk update && apk add --no-cache tzdata ffmpeg libjpeg-turbo libwebp poppler-utils

WORKDIR /app

COPY --from=build /build/server .
COPY --from=build /build/manager/dist ./manager/dist
COPY --from=manager-v2-build /manager-v2/dist ./manager-v2/dist
COPY --from=build /build/VERSION ./VERSION

ENV TZ=America/Sao_Paulo

ENTRYPOINT ["/app/server"]
