# Notification Service

Event-Driven Notification System for sending notifications through multiple channels (SMS, Email, Push).

## Features

- **Multi-Channel Support**: Send notifications via SMS, Email, and Push
- **Batch Processing**: Create up to 1000 notifications in a single request
- **Priority Queue**: Support for high, normal, and low priority messages
- **Scheduled Notifications**: Schedule notifications for future delivery
- **Template System**: Message templates with variable substitution
- **Rate Limiting**: Configurable rate limits per channel (default: 100 msg/sec)
- **Retry Logic**: Exponential backoff retry for failed deliveries
- **Idempotency**: Prevent duplicate sends with idempotency keys
- **Real-time Updates**: WebSocket support for status notifications
- **Observability**: Prometheus metrics, structured logging, health checks

## Tech Stack

| Category | Technology | Version | Purpose |
|----------|------------|---------|---------|
| **Language** | Go | 1.22+ | High-performance, concurrent backend |
| **HTTP Router** | Chi | v5.1 | Lightweight, idiomatic Go router |
| **Database** | PostgreSQL | 16 | Persistent notification state storage |
| **Cache/Queue** | Redis | 7 | Message queue & rate limiting |
| **Validation** | go-playground/validator | v10 | Request validation |
| **WebSocket** | Gorilla WebSocket | v1.5 | Real-time status updates |
| **Metrics** | Prometheus Client | v1.19 | Application observability |
| **Migrations** | golang-migrate | v4.17 | Database schema versioning |
| **API Docs** | Swaggo | v1.16 | OpenAPI documentation generation |
| **Container** | Docker | - | Containerization & deployment |

## Architecture

### Clean Architecture

This project follows Robert C. Martin's (Uncle Bob) **Clean Architecture** principles, ensuring testability, maintainability, and independence of the codebase.

#### Layer Structure

```
┌──────────────────────────────────────────────────────────────┐
│                     External (Infrastructure)                 │
│  ┌─────────────┐  ┌──────────────┐  ┌─────────────────────┐  │
│  │  Handler    │  │  Repository  │  │     Provider        │  │
│  │  (HTTP/WS)  │  │  (Postgres)  │  │     (Webhook)       │  │
│  └──────┬──────┘  └──────┬───────┘  └──────────┬──────────┘  │
├─────────┼────────────────┼─────────────────────┼─────────────┤
│         │     Application Layer (Service)      │             │
│         │  ┌───────────────────────────────┐   │             │
│         └─▶│     NotificationService       │◀──┘             │
│            │     TemplateService           │                 │
│            │     SchedulerService          │                 │
│            └───────────────┬───────────────┘                 │
├────────────────────────────┼─────────────────────────────────┤
│                   Domain Layer (Core)                        │
│  ┌─────────────────────────┴─────────────────────────────┐   │
│  │  Entities: Notification, Template, QueueItem          │   │
│  │  Interfaces: NotificationRepository, Queue, Provider  │   │
│  │  Value Objects: Channel, Priority, Status             │   │
│  └───────────────────────────────────────────────────────┘   │
└──────────────────────────────────────────────────────────────┘
```

### System Architecture

```
┌─────────────┐     ┌──────────────┐     ┌─────────────┐
│   REST API  │────▶│   Service    │────▶│  PostgreSQL │
│  WebSocket  │     │    Layer     │     │   (State)   │
└─────────────┘     └──────────────┘     └─────────────┘
                          │
                          ▼
                   ┌──────────────┐     ┌─────────────┐
                   │    Redis     │────▶│   Workers   │
                   │   (Queue)    │     │ (Per-channel)│
                   └──────────────┘     └─────────────┘
                                              │
                                              ▼
                                       ┌─────────────┐
                                       │   Webhook   │
                                       │  Provider   │
                                       └─────────────┘
```

### Design Patterns & Principles

| Pattern/Principle | Location | Description |
|-------------------|----------|-------------|
| **Repository Pattern** | `internal/repository/` | Abstracts data access logic, isolates domain from database |
| **Dependency Injection** | `cmd/server/main.go` | Dependencies injected externally, improves testability |
| **Interface Segregation** | `internal/domain/` | Small, focused interfaces (NotificationRepository, Queue, Provider) |
| **Factory Pattern** | `domain.NewNotification()` | Encapsulates entity creation |
| **Observer Pattern** | `WebSocketHub` | Pub/sub mechanism for real-time status updates |
| **Strategy Pattern** | `NotificationProvider` | Different providers (webhook, SMTP, etc.) with same interface |
| **Worker Pool Pattern** | `internal/worker/` | Concurrent processing per channel |

### Technology Choices

#### Go (Golang)
- **Why**: High concurrency support (goroutines), low memory footprint, fast compilation, static typing
- **Usage**: All backend services

#### PostgreSQL
- **Why**: ACID compliant, JSON/JSONB support, powerful indexing, mature ecosystem
- **Usage**: Notification and template state storage, idempotency key checking

#### Redis
- **Why**: In-memory speed, sorted sets for priority queue, Lua scripting for atomic operations
- **Usage**: Message queue (per channel), rate limiting (sliding window), distributed locks

#### Chi Router
- **Why**: Standard `net/http` compatible, minimal, middleware-friendly, zero dependencies
- **Usage**: HTTP routing, middleware chain

#### WebSocket (Gorilla)
- **Why**: Production-ready, ping/pong support, concurrent-safe connections
- **Usage**: Real-time status notifications

## Quick Start

### Prerequisites

- Docker & Docker Compose
- Go 1.22+ (for local development)
- Make (optional)

### One-Command Setup

```bash
# Clone the repository
git clone https://github.com/insider-one/notification-service.git
cd notification-service

# Start all services
docker compose up -d

# Run database migrations
docker compose --profile tools run --rm migrate
```

The service will be available at `http://localhost:8080`

### Configuration

Create a `.env` file based on `.env.example`:

```bash
cp .env.example .env
```

Configure your webhook URL (get one from [webhook.site](https://webhook.site)):

```env
WEBHOOK_URL=https://webhook.site/your-uuid-here
```

## API Documentation

### Base URL

```
http://localhost:8080
```

### OpenAPI Specification

The full API documentation is available at `api/openapi.yaml` or can be viewed using Swagger UI.

### Endpoints Overview

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/notifications` | Create notification |
| POST | `/api/v1/notifications/batch` | Create batch notifications |
| GET | `/api/v1/notifications` | List notifications |
| GET | `/api/v1/notifications/:id` | Get notification by ID |
| GET | `/api/v1/notifications/batch/:batchId` | Get batch by ID |
| DELETE | `/api/v1/notifications/:id` | Cancel notification |
| POST | `/api/v1/templates` | Create template |
| GET | `/api/v1/templates` | List templates |
| GET | `/api/v1/templates/:id` | Get template by ID |
| PUT | `/api/v1/templates/:id` | Update template |
| DELETE | `/api/v1/templates/:id` | Delete template |
| GET | `/health` | Health check |
| GET | `/metrics` | Prometheus metrics |
| GET | `/metrics/realtime` | Real-time queue metrics |
| WS | `/ws` | WebSocket for status updates |

## API Examples

### Create Notification

```bash
curl -X POST http://localhost:8080/api/v1/notifications \
  -H "Content-Type: application/json" \
  -d '{
    "recipient": "+905555555555",
    "channel": "sms",
    "content": "Your verification code is 123456",
    "priority": "high"
  }'
```

### Create Batch Notifications

```bash
curl -X POST http://localhost:8080/api/v1/notifications/batch \
  -H "Content-Type: application/json" \
  -d '{
    "notifications": [
      {
        "recipient": "+905555555555",
        "channel": "sms",
        "content": "Message 1"
      },
      {
        "recipient": "user@example.com",
        "channel": "email",
        "content": "Message 2"
      }
    ]
  }'
```

### Create Scheduled Notification

```bash
curl -X POST http://localhost:8080/api/v1/notifications \
  -H "Content-Type: application/json" \
  -d '{
    "recipient": "+905555555555",
    "channel": "sms",
    "content": "Your appointment is in 1 hour",
    "scheduled_at": "2026-01-28T10:00:00Z"
  }'
```

### Create Template

```bash
curl -X POST http://localhost:8080/api/v1/templates \
  -H "Content-Type: application/json" \
  -d '{
    "name": "welcome_sms",
    "channel": "sms",
    "content": "Hello {{name}}, welcome to {{company}}!"
  }'
```

### Use Template in Notification

```bash
curl -X POST http://localhost:8080/api/v1/notifications \
  -H "Content-Type: application/json" \
  -d '{
    "recipient": "+905555555555",
    "channel": "sms",
    "template_name": "welcome_sms",
    "template_vars": {
      "name": "John",
      "company": "Insider"
    }
  }'
```

### Idempotent Request

```bash
curl -X POST http://localhost:8080/api/v1/notifications \
  -H "Content-Type: application/json" \
  -d '{
    "recipient": "+905555555555",
    "channel": "sms",
    "content": "Your verification code is 123456",
    "idempotency_key": "unique-request-123"
  }'
```

### Query Notifications

```bash
# List all notifications
curl "http://localhost:8080/api/v1/notifications"

# Filter by status
curl "http://localhost:8080/api/v1/notifications?status=sent"

# Filter by channel with pagination
curl "http://localhost:8080/api/v1/notifications?channel=sms&page=1&page_size=10"

# Filter by date range
curl "http://localhost:8080/api/v1/notifications?start_date=2026-01-01T00:00:00Z&end_date=2026-01-31T23:59:59Z"
```

### WebSocket Connection

```javascript
const ws = new WebSocket('ws://localhost:8080/ws');

// Subscribe to specific notifications
ws.send(JSON.stringify({
  action: 'subscribe',
  filter: {
    channels: ['sms', 'email'],
    batch_ids: ['batch-uuid-here']
  }
}));

// Receive status updates
ws.onmessage = (event) => {
  const update = JSON.parse(event.data);
  console.log('Status update:', update);
};
```

## Development

### Local Setup

```bash
# Install dependencies
go mod download

# Install development tools
make tools

# Start dependencies (PostgreSQL, Redis)
docker compose up -d postgres redis

# Run migrations
make migrate-up

# Run the application
make run
```

### Running Tests

```bash
# Run all tests
make test

# Run tests with coverage
make test-coverage

# Run linter
make lint
```

### Project Structure (Clean Architecture)

```
.
├── cmd/
│   └── server/
│       └── main.go              # Entry point & dependency injection (Composition Root)
│
├── internal/                    # Private application code
│   │
│   ├── domain/                  # 🎯 DOMAIN LAYER (Core Business Logic)
│   │   ├── notification.go      #   - Notification entity & repository interface
│   │   ├── template.go          #   - Template entity & repository interface
│   │   ├── queue.go             #   - Queue interface
│   │   ├── provider.go          #   - Provider interface
│   │   └── errors.go            #   - Domain-specific errors
│   │
│   ├── service/                 # 📋 APPLICATION LAYER (Use Cases)
│   │   ├── notification.go      #   - Notification business logic
│   │   ├── template.go          #   - Template CRUD operations
│   │   └── scheduler.go         #   - Scheduled notification processing
│   │
│   ├── repository/              # 💾 INFRASTRUCTURE LAYER (Data Access)
│   │   ├── postgres/            #   - PostgreSQL implementations
│   │   │   ├── notification.go  #     implements domain.NotificationRepository
│   │   │   └── template.go      #     implements domain.TemplateRepository
│   │   └── redis/               #   - Redis implementations
│   │       ├── queue.go         #     implements domain.Queue
│   │       └── ratelimiter.go   #     rate limiting logic
│   │
│   ├── handler/                 # 🌐 INTERFACE LAYER (HTTP/WS Adapters)
│   │   ├── notification.go      #   - REST endpoints for notifications
│   │   ├── template.go          #   - REST endpoints for templates
│   │   ├── websocket.go         #   - WebSocket handler
│   │   ├── health.go            #   - Health check endpoints
│   │   └── metrics.go           #   - Prometheus metrics endpoint
│   │
│   ├── middleware/              # HTTP middleware (logging, recovery, correlation)
│   ├── worker/                  # Background workers (queue processors)
│   ├── provider/                # External service adapters (webhook provider)
│   └── config/                  # Configuration loading
│
├── migrations/                  # Database migrations (golang-migrate)
├── api/                         # OpenAPI/Swagger specification
├── Dockerfile                   # Multi-stage Docker build
├── docker-compose.yml           # Local development environment
└── README.md
```

#### Layer Responsibilities

| Layer | Directory | Responsibility | Dependencies |
|-------|-----------|----------------|--------------|
| **Domain** | `internal/domain/` | Entities, interfaces, business rules | No external dependencies |
| **Application** | `internal/service/` | Use cases, workflows, orchestration | Domain only |
| **Infrastructure** | `internal/repository/`, `internal/provider/` | Database, cache, external APIs | Domain + External libs |
| **Interface** | `internal/handler/` | HTTP/WS request handling, serialization | Application + HTTP libs |

## Configuration Reference

| Environment Variable | Description | Default |
|---------------------|-------------|---------|
| `APP_ENV` | Application environment | `development` |
| `SERVER_PORT` | HTTP server port | `8080` |
| `DATABASE_URL` | PostgreSQL connection string | - |
| `REDIS_URL` | Redis connection string | - |
| `WEBHOOK_URL` | External provider webhook URL | - |
| `LOG_LEVEL` | Log level (debug, info, warn, error) | `info` |
| `RATE_LIMIT_PER_CHANNEL` | Rate limit per channel (msg/sec) | `100` |
| `WORKER_COUNT_SMS` | SMS worker count | `5` |
| `WORKER_COUNT_EMAIL` | Email worker count | `5` |
| `WORKER_COUNT_PUSH` | Push worker count | `5` |
| `MAX_RETRY_COUNT` | Maximum retry attempts | `5` |
| `RETRY_BASE_DELAY` | Base delay for retry backoff | `1s` |

## Retry Logic

Failed notifications are retried with exponential backoff:

| Retry | Delay |
|-------|-------|
| 1 | 1s |
| 2 | 2s |
| 3 | 4s |
| 4 | 8s |
| 5 | 16s |

After 5 retries, notifications are marked as `failed`.

## Monitoring

### Health Check

```bash
curl http://localhost:8080/health
```

### Prometheus Metrics

```bash
curl http://localhost:8080/metrics
```

Available metrics:
- `http_requests_total` - Total HTTP requests
- `http_request_duration_seconds` - Request duration histogram
- `notifications_sent_total` - Successfully sent notifications
- `notifications_failed_total` - Failed notifications
- `notification_queue_depth` - Current queue depth per channel
- `notification_processing_latency_seconds` - End-to-end latency

### Real-time Queue Metrics

```bash
curl http://localhost:8080/metrics/realtime
```

## License

MIT License - see [LICENSE](LICENSE) for details.
