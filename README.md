# SaaS Payment Platform

Platform pembayaran SaaS terinspirasi dari Stripe dan Xendit. Menyediakan API untuk developer melakukan integrasi pembayaran, termasuk API key management, simulasi payment gateway, webhook notification, subscription billing, invoice management, rate limiting, dan admin dashboard.

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Backend | Go 1.22 + Fiber |
| Frontend | Next.js 14 + TypeScript + Tailwind CSS |
| Database | PostgreSQL 16 |
| Cache | Redis 7 |
| Container | Docker + Docker Compose |
| Orchestration | Kubernetes |
| CI/CD | GitHub Actions |

## Architecture

Clean architecture monorepo dengan layering: Handler → Service → Repository.

```
project-root/
├── backend/           # Go + Fiber API server
│   ├── cmd/           # Entry points (server, migrate, seed)
│   ├── internal/      # Private application code
│   │   ├── handler/   # HTTP handlers (request/response only)
│   │   ├── service/   # Business logic + transaction management
│   │   ├── repository/# Database operations (GORM)
│   │   ├── model/     # Domain models
│   │   ├── dto/       # Request/Response DTOs
│   │   ├── middleware/ # Auth, API key, rate limiting, CORS
│   │   ├── worker/    # Background workers (webhook delivery)
│   │   └── pkg/       # Shared packages (auth, hash, logger, audit)
│   ├── migrations/    # SQL migration files + seed data
│   ├── docs/          # Swagger generated docs
│   └── Makefile
├── frontend/          # Next.js dashboard
│   ├── src/app/       # App Router pages
│   ├── src/services/  # API service layer
│   ├── src/types/     # TypeScript interfaces
│   └── src/components/# Reusable UI components
├── k8s/               # Kubernetes manifests
├── docker-compose.yml
└── .github/workflows/ # CI/CD pipeline
```

## Quick Start

### Prerequisites

- Go 1.22+
- PostgreSQL 16
- Redis 7
- Node.js 20+ (for frontend)

### Backend Setup

```bash
cd backend
cp .env.example .env
# Edit .env with your database credentials

make migrate-up    # Apply database migrations
make seed          # Seed development data (idempotent)
make run           # Start server at :8080
```

### Frontend Setup

```bash
cd frontend
npm install
npm run dev        # Start at :3000
```

### Docker Compose (Full Stack)

```bash
docker compose up -d
curl http://localhost:8080/health
```

## API Endpoints

### Auth (Public)
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/auth/register` | Register new user |
| POST | `/api/v1/auth/login` | Login, returns JWT |

### Users (JWT Protected)
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/users/profile` | Get user profile |

### API Keys (JWT Protected)
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/api-keys` | Create API key |
| GET | `/api/v1/api-keys` | List API keys |
| DELETE | `/api/v1/api-keys/:id` | Revoke API key |

### Transactions (API Key Protected)
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/transactions` | Create transaction |
| GET | `/api/v1/transactions` | List transactions |
| GET | `/api/v1/transactions/:id` | Get transaction detail |

### Invoices (JWT Protected)
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/invoices` | List invoices |
| GET | `/api/v1/invoices/:id` | Get invoice detail |

### Webhooks (JWT Protected)
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/webhooks/endpoints` | Register webhook endpoint |
| GET | `/api/v1/webhooks/endpoints` | List webhook endpoints |
| DELETE | `/api/v1/webhooks/endpoints/:id` | Delete webhook endpoint |
| GET | `/api/v1/webhooks/deliveries` | List webhook deliveries |

### Products & Subscriptions (JWT Protected)
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/products` | Create product |
| GET | `/api/v1/products` | List products |
| POST | `/api/v1/products/:id/plans` | Create plan for product |
| POST | `/api/v1/subscriptions` | Create subscription |
| GET | `/api/v1/subscriptions` | List subscriptions |
| PATCH | `/api/v1/subscriptions/:id/cancel` | Cancel subscription |

### Admin (JWT + Admin Role)
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/admin/stats` | Dashboard statistics |

### Operational (No Auth)
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/health` | Liveness check |
| GET | `/ready` | Readiness check (DB + Redis) |
| GET | `/metrics` | Prometheus metrics |
| GET | `/swagger/*` | Swagger UI |

## Authentication

Two authentication methods:

1. **JWT Token** — for dashboard/management endpoints
   ```
   Authorization: Bearer <jwt_token>
   ```

2. **API Key** — for payment/transaction endpoints
   ```
   X-API-Key: sk_test_xxxxx
   ```

## Development

### Makefile Commands

```bash
make build          # Build binary
make run            # Build and run
make test           # Run tests with race detection + coverage
make test-coverage  # Generate HTML coverage report
make swagger        # Generate Swagger docs
make migrate-up     # Apply database migrations
make migrate-down   # Rollback last migration
make seed           # Seed development data
make clean          # Clean build artifacts
```

### Development Seed Data

After running `make seed`, the following dev-only accounts are available:

| Role | Email | Password |
|------|-------|----------|
| Admin | `admin@dev.local` | `admin1234dev` |
| Developer | `developer@dev.local` | `dev1234pass` |

> These are development-only credentials. Do NOT use in production.

### Running Tests

```bash
cd backend
make test           # 383 tests, race detection, coverage
make test-coverage  # HTML coverage report
```

Test stats: 383 tests, 0 failures, 81.1% coverage, zero race conditions.

## Key Features

- **Clean Architecture** — handler/service/repository layering with interfaces
- **Transaction Manager** — all DB writes wrapped in transactions via service layer
- **Circuit Breaker** — per-URL circuit breaker for webhook delivery (closed/open/half-open)
- **Exponential Backoff** — webhook retry: 1s, 2s, 4s, 8s, 16s, max 5 retries
- **Rate Limiting** — Redis sliding window counter per API key
- **HMAC Signatures** — webhook payloads signed with SHA-256
- **Audit Logging** — separate structured JSON audit log for security operations
- **Graceful Shutdown** — SIGTERM/SIGINT, drains in-flight requests and workers
- **Swagger Docs** — auto-generated OpenAPI at `/swagger/`

## Infrastructure

### Kubernetes

```bash
kubectl apply -f k8s/namespace.yaml
kubectl apply -f k8s/
```

Includes: Deployment (2 replicas, resource limits, startup/liveness/readiness probes), Service, Ingress, PDB, PostgreSQL StatefulSet, Redis Deployment.

### CI/CD Pipeline

7-stage GitHub Actions: Lint → Test → Build → Security Scan (govulncheck + Trivy) → Deploy Staging → Integration Test → Deploy Production.

## License

MIT
