<h1 align="center">Evolution Go</h1>

<div align="center">

[![Docker Image](https://img.shields.io/badge/Docker-image-blue)](https://hub.docker.com/r/evoapicloud/evolution-go)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue)](./LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?logo=go)](https://golang.org/)
[![GitHub Stars](https://img.shields.io/github/stars/EvolutionAPI/evolution-go)](https://github.com/EvolutionAPI/evolution-go/stargazers)
[![Documentation](https://img.shields.io/badge/Documentation-Official-green)](https://docs.evolutionfoundation.com.br)

</div>

<div align="center"><img src="./public/images/cover.png" width="400"></div>

## About

Evolution Go is a high-performance WhatsApp API built in Go, part of the [Evolution](https://evolutionfoundation.com.br/) ecosystem. It provides a robust, lightweight solution for WhatsApp integration using the [whatsmeow](https://github.com/tulir/whatsmeow) library.

## Features

- **High Performance** — Built with Go for minimal resource usage
- **RESTful API** — Clean, well-documented REST endpoints with Swagger
- **Real-time Events** — WebSocket, Webhook, AMQP/RabbitMQ and NATS support
- **Media Support** — Images, videos, audio, documents with MinIO/S3 storage
- **Message Storage** — Optional PostgreSQL persistence
- **QR Code Pairing** — Built-in QR code generation for device linking
- **License Management** — Built-in licensing with registration, activation, and heartbeat
- **Docker Ready** — Production-ready Docker configuration

## Quick Start

### Docker (Recommended)

```bash
git clone https://github.com/EvolutionAPI/evolution-go.git
cd evolution-go
make docker-build
make docker-run
```

### Local Development

```bash
git clone https://github.com/EvolutionAPI/evolution-go.git
cd evolution-go

# Clone whatsmeow dependency
git clone git@github.com:EvolutionAPI/whatsmeow.git whatsmeow-lib

# Setup, configure and run
make setup
cp .env.example .env
make dev
```

> Run `make help` to see all available commands. See [COMMANDS.md](./COMMANDS.md) for detailed workflows.

## Configuration

Create a `.env` file:

```env
# Server
SERVER_PORT=8080
CLIENT_NAME=evolution

# Security
GLOBAL_API_KEY=your-secure-api-key-here

# Database
POSTGRES_AUTH_DB=postgresql://postgres:password@localhost:5432/evogo_auth?sslmode=disable
POSTGRES_USERS_DB=postgresql://postgres:password@localhost:5432/evogo_users?sslmode=disable
DATABASE_SAVE_MESSAGES=false

# Logging
WADEBUG=DEBUG
LOGTYPE=console

# Optional
# AMQP_URL=amqp://guest:guest@localhost:5672/
# NATS_URL=nats://localhost:4222
# WEBHOOK_URL=https://your-webhook-url.com/webhook
# MINIO_ENABLED=true
# MINIO_ENDPOINT=localhost:9000
# MINIO_ACCESS_KEY=minioadmin
# MINIO_SECRET_KEY=minioadmin
```

| Variable | Description | Default |
|----------|-------------|---------|
| `SERVER_PORT` | Server port | `8080` |
| `CLIENT_NAME` | Client identifier | `evolution` |
| `GLOBAL_API_KEY` | API authentication key | **Required** |
| `DATABASE_SAVE_MESSAGES` | Enable message storage | `false` |
| `WADEBUG` | WhatsApp debug level | `INFO` |

## License Activation

Evolution Go requires a license to operate. On first run:

1. Start the server — API endpoints return `503` until activated
2. Open the **Manager** at `http://localhost:8080/manager/login`
3. Enter your API URL and `GLOBAL_API_KEY`
4. Complete the license registration flow
5. Once activated, the API is fully operational

The license status persists in the database (`runtime_configs` table). Heartbeats are sent periodically to maintain activation.

## API Documentation

Swagger UI available at:

```
http://localhost:8080/swagger/index.html
```

### Key Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/instance/create` | Create WhatsApp instance |
| `GET` | `/instance/{name}/qrcode` | Get QR code for pairing |
| `POST` | `/message/sendText` | Send text message |
| `POST` | `/message/sendMedia` | Send media message |
| `GET` | `/instance/{name}/status` | Get instance status |
| `DELETE` | `/instance/{name}` | Delete instance |

## Project Structure

```
evolution-go/
├── cmd/evolution-go/     # Application entry point
├── pkg/
│   ├── core/            # License management & middleware
│   ├── instance/        # Instance management
│   ├── message/         # Message handling
│   ├── sendMessage/     # Message sending
│   ├── routes/          # HTTP routes
│   ├── middleware/       # Auth & validation middleware
│   ├── config/          # Configuration
│   ├── events/          # Event producers (AMQP, NATS, Webhook, WS)
│   └── storage/         # Media storage (MinIO)
├── whatsmeow-lib/       # WhatsApp protocol library
├── docs/                # Swagger documentation
├── Dockerfile
├── Makefile
└── VERSION
```

## Technology Stack

| Component | Technology |
|-----------|-----------|
| Language | Go 1.24+ |
| HTTP Framework | Gin |
| WhatsApp | [whatsmeow](https://github.com/tulir/whatsmeow) |
| Database | PostgreSQL |
| ORM | GORM |
| Message Queue | RabbitMQ, NATS |
| Object Storage | MinIO/S3 |
| Documentation | Swagger/OpenAPI |
| Container | Docker |

## Documentation & Support

| Resource | Link |
|----------|------|
| Website | [evolutionfoundation.com.br](https://evolutionfoundation.com.br/) |
| Documentation | [docs.evolutionfoundation.com.br](https://docs.evolutionfoundation.com.br/) |
| Community | [evolutionfoundation.com.br/community](https://evolutionfoundation.com.br/community) |
| WhatsApp Support | [+55 31 9621-9989](https://wa.me/553196219989) |
| GitHub Issues | [evolution-go/issues](https://github.com/EvolutionAPI/evolution-go/issues) |

## Hosting

Deploy Evolution Go with optimized infrastructure:

| Product | Link |
|---------|------|
| Evolution Go VPS | [Hostgator - Evo Go](https://www.hostgator.com.br/52579-144-3-55.html) |
| Evolution API VPS | [Hostgator - Evo API](https://www.hostgator.com.br/servidor-vps/hospedagem-evo-api/lp-afiliado) |

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## Security

For security concerns, please email: contato@evolution-api.com

## License

Evolution Go is licensed under the Apache License 2.0, with the following additional conditions:

1. **Logo and copyright**: You may not remove or modify the logo or copyright information in the Evolution console or applications when using frontend components.

2. **Usage notification**: If Evolution Go is used as part of any project (including closed-source), a clear notification that Evolution Go is being utilized must be visible to system administrators.

Please contact contato@evolution-api.com for licensing inquiries. Full license details at [apache.org/licenses/LICENSE-2.0](http://www.apache.org/licenses/LICENSE-2.0).

## Acknowledgments

- [whatsmeow](https://github.com/tulir/whatsmeow) by [tulir](https://github.com/tulir)
- [Evolution API](https://github.com/EvolutionAPI/evolution-api)

## Telemetry

Evolution Go collects anonymous telemetry data (routes used, API version) to improve the service. No sensitive or personal data is collected.

---

<div align="center">

**Evolution Go** — High-Performance WhatsApp API

Made with ❤️ by the [Evolution Team](https://evolutionfoundation.com.br/)

© 2025 Evolution Foundation

</div>
