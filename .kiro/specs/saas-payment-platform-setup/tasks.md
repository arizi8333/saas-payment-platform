# Implementation Plan: SaaS Payment Platform Setup

## Overview

Implementasi SaaS Payment Platform menggunakan Go + Fiber (backend) dan Next.js + TypeScript (frontend) dalam struktur monorepo. Mengikuti clean architecture (handler → service → repository) dengan PostgreSQL sebagai database utama dan Redis untuk caching/rate limiting. Implementasi dilakukan secara bertahap sesuai roadmap: Phase 1 (MVP), Phase 2 (Enhancement), Phase 3 (Scale & Optimize), Phase 4 (Advanced).

## Tasks

- [x] 1. Project scaffolding dan infrastruktur dasar
  - [x] 1.1 Inisialisasi monorepo structure dan Go module
    - Buat direktori `backend/` dengan `cmd/server/main.go`, `internal/`, `pkg/`, `migrations/`
    - Inisialisasi `go.mod` dengan module name
    - Buat `Makefile` dengan target: `build`, `run`, `test`, `test-coverage`, `swagger`, `migrate-up`, `migrate-down`
    - Buat `backend/Dockerfile` untuk Go application (multi-stage build, non-root user, alpine base)
    - Buat root `.gitignore` — ignore: build artifacts, node_modules, vendor/, .env, IDE configs (.idea/, .vscode/), OS files (.DS_Store, Thumbs.db), reports/, compiled binaries
    - _Requirements: 12.1_

  - [x] 1.2 Setup konfigurasi aplikasi dan environment
    - Buat `internal/config/config.go` dengan struct untuk database, Redis, JWT, server, dan rate limit config
    - Load config dari environment variables dengan default values
    - Buat `.env.example` sebagai template
    - _Requirements: 12.1_

  - [x] 1.3 Setup koneksi PostgreSQL dan Redis
    - Buat `internal/pkg/database/postgres.go` dengan connection pooling (max_open, max_idle, max_lifetime dari config)
    - Buat `internal/pkg/database/redis.go` dengan Redis client setup
    - Implementasi health check untuk kedua koneksi
    - _Requirements: 12.1_

  - [x] 1.4 Implementasi Transaction Manager
    - Buat `internal/pkg/database/txmanager.go` dengan interface `TransactionManager`
    - Implementasi `WithTransaction(ctx, fn)` dengan commit/rollback/panic recovery
    - Implementasi nested transaction detection dan context propagation
    - Helper function untuk repository mengekstrak transaction dari context
    - _Requirements: 10.1, 10.2, 10.3, 10.4, 10.5_

  - [x] 1.5 Setup shared packages (logger, response, validator, auth, hash)
    - Buat `internal/pkg/logger/logger.go` — structured JSON logging dengan slog, termasuk request_id dan trace_id
    - Buat `internal/pkg/response/response.go` — standardized API response format (success/error)
    - Buat `internal/pkg/validator/validator.go` — input validation wrapper
    - Buat `internal/pkg/auth/jwt.go` — JWT token generation dan validation (access token dengan user_id, role)
    - Buat `internal/pkg/hash/bcrypt.go` — bcrypt hashing dan comparison
    - Buat `pkg/apierror/apierror.go` — custom API error types
    - _Requirements: 11.1, 11.2, 11.5, 12.4_

  - [x] 1.6 Setup Fiber app, router, dan base middlewares
    - Buat `cmd/server/main.go` dengan Fiber app initialization, graceful shutdown (SIGTERM/SIGINT)
    - Buat `internal/middleware/cors_middleware.go`
    - Buat `internal/middleware/logger_middleware.go` — structured request logging
    - Buat `internal/middleware/requestid_middleware.go` — generate/propagate X-Request-ID
    - Setup route groups: `/api/v1/`, `/health`, `/ready`, `/metrics`, `/swagger/`
    - `/health` — liveness check (return 200 if process alive), include version info (version number, build timestamp, git commit hash)
    - `/ready` — readiness check (cek PostgreSQL + Redis connectivity, return 503 jika unavailable)
    - `/metrics` — Prometheus-compatible metrics endpoint
    - Health endpoints TIDAK memerlukan authentication
    - _Requirements: 12.1, 12.4_

  - [x] 1.7 Write unit tests for Transaction Manager dan shared packages
    - Test WithTransaction success, rollback on error, rollback on panic, nested detection
    - Test JWT generation/validation, bcrypt hash/compare, validator
    - _Requirements: 10.1, 10.2, 10.3, 10.4_

- [x] 2. Database migrations dan domain models
  - [x] 2.1 Buat SQL migration files untuk semua tabel
    - Format naming: `YYYYMMDDHHMMSS_description.up.sql` / `YYYYMMDDHHMMSS_description.down.sql`
    - `20260410120000_create_users_table.up.sql` / `20260410120000_create_users_table.down.sql`
    - `20260410120001_create_api_keys_table.up.sql` / `20260410120001_create_api_keys_table.down.sql`
    - `20260410120002_create_transactions_table.up.sql` / `20260410120002_create_transactions_table.down.sql`
    - `20260410120003_create_webhook_endpoints_table.up.sql` / `20260410120003_create_webhook_endpoints_table.down.sql`
    - `20260410120004_create_webhook_deliveries_table.up.sql` / `20260410120004_create_webhook_deliveries_table.down.sql`
    - `20260410120005_create_products_table.up.sql` / `20260410120005_create_products_table.down.sql`
    - `20260410120006_create_plans_table.up.sql` / `20260410120006_create_plans_table.down.sql`
    - `20260410120007_create_subscriptions_table.up.sql` / `20260410120007_create_subscriptions_table.down.sql`
    - `20260410120008_create_invoices_table.up.sql` / `20260410120008_create_invoices_table.down.sql`
    - Semua tabel harus memiliki: primary key UUID, created_at, updated_at, deleted_at (soft delete)
    - Buat index pada semua foreign key columns dan kolom yang sering di-query
    - Buat unique constraints sesuai desain (email, key_hash, external_id, invoice_number)
    - Buat Go migration functions (`func Migrate{Timestamp}_{Description}(db *gorm.DB)`) sebagai pendamping SQL files
    - _Requirements: 12.5_

  - [x] 2.2 Implementasi migration runner
    - Buat Go migration runner yang menjalankan SQL files secara berurutan
    - Track migration history di tabel `schema_migrations`
    - Support `migrate up` dan `migrate down` commands
    - Integrasikan ke Makefile targets
    - _Requirements: 12.5_

  - [x] 2.3 Buat domain models (GORM structs)
    - Buat `internal/model/user.go` — User struct dengan roles (admin, developer, end_user)
    - Buat `internal/model/apikey.go` — APIKey struct dengan key_hash, key_prefix, rate_limit
    - Buat `internal/model/transaction.go` — Transaction struct dengan status enum, payment method enum
    - Buat `internal/model/webhook.go` — WebhookEndpoint dan WebhookDelivery structs
    - Buat `internal/model/product.go` — Product struct
    - Buat `internal/model/subscription.go` — Plan dan Subscription structs dengan billing interval
    - Buat `internal/model/invoice.go` — Invoice struct dengan invoice_number format
    - _Requirements: 12.2, 12.3_

  - [x] 2.4 Buat DTO structs untuk request/response
    - Buat `internal/dto/common_dto.go` — PaginationRequest, PaginationResponse, ErrorResponse, SuccessResponse
    - Buat `internal/dto/user_dto.go` — RegisterRequest, LoginRequest, LoginResponse, UserProfileResponse
    - Buat `internal/dto/apikey_dto.go` — CreateAPIKeyRequest, APIKeyResponse, APIKeyListResponse
    - Buat `internal/dto/transaction_dto.go` — CreateTransactionRequest, TransactionResponse, TransactionListResponse
    - Buat `internal/dto/webhook_dto.go` — CreateWebhookRequest, WebhookResponse, WebhookDeliveryResponse
    - Buat `internal/dto/subscription_dto.go` — CreateProductRequest, CreatePlanRequest, CreateSubscriptionRequest, dan response DTOs
    - Buat `internal/dto/invoice_dto.go` — InvoiceResponse, InvoiceListResponse
    - Semua DTO harus memiliki validation tags dan TIDAK mengekspos field sensitif (password_hash, key_hash)
    - _Requirements: 12.3, 11.2, 11.3_

- [x] 3. Checkpoint — Pastikan project structure, migrations, dan models sudah benar
  - Ensure all tests pass, ask the user if questions arise.

- [x] 4. User authentication (Phase 1 MVP)
  - [x] 4.1 Implementasi User Repository
    - Buat `internal/repository/user_repository.go` dengan interface dan implementasi
    - Methods: Create, FindByEmail, FindByID, Update
    - Gunakan context-aware queries (`db.WithContext(ctx)`)
    - Gunakan transaction dari context jika tersedia
    - _Requirements: 1.1, 1.2, 1.6_

  - [x] 4.2 Implementasi User Service
    - Buat `internal/service/user_service.go` dengan interface dan implementasi
    - Register: validasi email unik, password min 8 char, bcrypt hash, default role "developer"
    - Login: validasi kredensial, generate JWT token dengan user_id dan role
    - GetProfile: return user data tanpa password_hash
    - Gunakan TransactionManager untuk operasi write
    - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5, 1.6_

  - [x] 4.3 Implementasi Auth Middleware (JWT)
    - Buat `internal/middleware/auth_middleware.go`
    - Ekstrak Bearer token dari header Authorization
    - Validasi JWT token, cek expiry
    - Inject user_id dan role ke request context
    - Return 401 untuk token invalid/expired/missing
    - Implementasi AdminOnly middleware — return 403 jika role bukan admin
    - _Requirements: 2.1, 2.2, 2.3, 2.4_

  - [x] 4.4 Implementasi User Handler dan routes
    - Buat `internal/handler/user_handler.go`
    - POST `/api/v1/auth/register` — registrasi user baru
    - POST `/api/v1/auth/login` — login dan return JWT
    - GET `/api/v1/users/profile` — get profil user (protected)
    - Validasi input DTO, panggil service, return standardized response
    - Tambahkan Swagger annotations pada setiap handler
    - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5, 1.6_

  - [x] 4.5 Write unit tests for User Service
    - Test register success, duplicate email, short password
    - Test login success, invalid credentials
    - Test get profile success
    - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5, 1.6_

- [x] 5. API Key Management (Phase 1 MVP)
  - [x] 5.1 Implementasi APIKey Repository
    - Buat `internal/repository/apikey_repository.go` dengan interface dan implementasi
    - Methods: Create, FindByKeyHash, FindByUserID (paginated), FindByID, Update, UpdateLastUsed
    - _Requirements: 3.1, 3.2, 3.4, 3.5_

  - [x] 5.2 Implementasi APIKey Service
    - Buat `internal/service/apikey_service.go` dengan interface dan implementasi
    - CreateKey: generate unique key (sk_test_xxx format), hash sebelum simpan, return full key sekali
    - ListKeys: return keys milik user dengan pagination (tanpa full key)
    - RevokeKey: set is_active = false
    - ValidateKey: cari by hash, cek is_active dan expiry
    - Default rate limit 1000 req/hour per key
    - _Requirements: 3.1, 3.2, 3.5, 3.6_

  - [x] 5.3 Implementasi API Key Middleware
    - Buat `internal/middleware/apikey_middleware.go`
    - Ekstrak X-API-Key dari header
    - Validasi key: hash dan lookup, cek active/expired
    - Catat usage ke Redis (untuk rate limiting)
    - Inject user context dari API key owner
    - Return 401 untuk key invalid/inactive/expired
    - _Requirements: 3.3, 3.4_

  - [x] 5.4 Implementasi APIKey Handler dan routes
    - Buat `internal/handler/apikey_handler.go`
    - POST `/api/v1/api-keys` — buat API key baru (JWT protected)
    - GET `/api/v1/api-keys` — list API keys milik user (JWT protected)
    - DELETE `/api/v1/api-keys/:id` — revoke API key (JWT protected)
    - Tambahkan Swagger annotations
    - _Requirements: 3.1, 3.2, 3.5_

  - [x] 5.5 Write unit tests for APIKey Service
    - Test create key, validate key, revoke key, list keys
    - Test expired key rejection, inactive key rejection
    - _Requirements: 3.1, 3.2, 3.4, 3.5, 3.6_

- [x] 6. Payment Gateway Simulator (Phase 1 MVP)
  - [x] 6.1 Implementasi Transaction Repository
    - Buat `internal/repository/transaction_repository.go` dengan interface dan implementasi
    - Methods: Create, FindByID, FindByUserID (paginated), FindByExternalID, UpdateStatus
    - _Requirements: 4.1, 4.3, 4.6_

  - [x] 6.2 Implementasi Transaction Service
    - Buat `internal/service/transaction_service.go` dengan interface dan implementasi
    - CreateTransaction: validasi amount > 0, cek external_id unik, simpan amount dalam satuan terkecil, status "pending"
    - SimulatePayment: async update status ke success/failed, trigger webhook
    - GetTransaction: return detail transaksi
    - ListTransactions: paginated, filtered by user_id
    - Gunakan TransactionManager untuk operasi write
    - _Requirements: 4.1, 4.2, 4.3, 4.4, 4.5, 4.6, 4.7_

  - [x] 6.3 Implementasi Transaction Handler dan routes
    - Buat `internal/handler/transaction_handler.go`
    - POST `/api/v1/transactions` — buat transaksi (API Key protected)
    - GET `/api/v1/transactions` — list transaksi (API Key protected)
    - GET `/api/v1/transactions/:id` — detail transaksi (API Key protected)
    - Tambahkan Swagger annotations
    - _Requirements: 4.1, 4.6_

  - [x] 6.4 Write unit tests for Transaction Serviceok
    - Test create transaction success, invalid amount, duplicate external_id
    - Test simulate payment success/failed
    - Test list transactions with pagination
    - _Requirements: 4.1, 4.2, 4.3, 4.4, 4.6, 4.7_

- [x] 7. Invoice Management (Phase 1 MVP)
  - [x] 7.1 Implementasi Invoice Repository
    - Buat `internal/repository/invoice_repository.go` dengan interface dan implementasi
    - Methods: Create, FindByID, FindByUserID (paginated), UpdateStatus, GenerateInvoiceNumber
    - _Requirements: 7.1, 7.4_

  - [x] 7.2 Implementasi Invoice Service
    - Buat `internal/service/invoice_service.go` dengan interface dan implementasi
    - CreateInvoice: generate nomor unik "INV-YYYYMMDD-XXXXX", kaitkan dengan Transaction atau Subscription
    - UpdateStatus: unpaid → paid (catat paid_at) atau unpaid → void
    - ListInvoices: paginated, filtered by user_id
    - _Requirements: 7.1, 7.2, 7.3, 7.4, 7.5_

  - [x] 7.3 Implementasi Invoice Handler dan routes
    - Buat `internal/handler/invoice_handler.go`
    - GET `/api/v1/invoices` — list invoices (JWT protected)
    - GET `/api/v1/invoices/:id` — detail invoice (JWT protected)
    - Tambahkan Swagger annotations
    - _Requirements: 7.4_

  - [x] 7.4 Write unit tests for Invoice Service
    - Test create invoice with unique number format
    - Test status transitions (unpaid → paid, unpaid → void)
    - Test list invoices with pagination
    - _Requirements: 7.1, 7.2, 7.3, 7.4, 7.5_

- [x] 8. Checkpoint — Phase 1 MVP complete
  - Ensure all tests pass, ask the user if questions arise.
  - Verifikasi: user auth, API key, transactions, dan invoices berfungsi end-to-end

- [x] 9. Webhook Notification (Phase 2 Enhancement)
  - [x] 9.1 Implementasi Webhook Repository
    - Buat `internal/repository/webhook_repository.go` dengan interface dan implementasi
    - Methods untuk WebhookEndpoint: Create, FindByUserID, FindByID, Update, Delete
    - Methods untuk WebhookDelivery: Create, FindPendingDeliveries, UpdateStatus, IncrementRetry
    - _Requirements: 5.1, 5.2_

  - [x] 9.2 Implementasi Webhook Service
    - Buat `internal/service/webhook_service.go` dengan interface dan implementasi
    - RegisterEndpoint: simpan URL, secret (HMAC), event subscriptions
    - TriggerWebhook: buat delivery record (status pending), masukkan ke queue
    - GetDeliveries: list delivery history per endpoint
    - Generate HMAC signature untuk payload verification
    - _Requirements: 5.1, 5.2, 5.3, 11.4_

  - [x] 9.3 Implementasi Webhook Worker (background)
    - Buat `internal/worker/webhook_worker.go`
    - Polling pending deliveries dari database
    - HTTP POST ke webhook URL dengan payload + HMAC signature header
    - Implementasi circuit breaker untuk HTTP calls ke external webhook URLs (library: `sony/gobreaker` atau equivalent)
    - Circuit breaker states: closed (normal) → open (failing) → half-open (testing recovery)
    - Configurable thresholds: failure count, timeout duration, half-open max requests
    - HTTP client HARUS memiliki explicit timeout (default 30 seconds, configurable)
    - Success (2xx): update status "delivered", catat timestamp
    - Failure: increment retry_count, schedule next retry dengan exponential backoff (1s, 2s, 4s, 8s, 16s)
    - Max retries (5): update status "failed", stop retry
    - Graceful shutdown: selesaikan delivery yang sedang diproses
    - Implementasi Start(ctx) dan Stop() interface
    - Log circuit state changes (closed→open, open→half-open, half-open→closed)
    - _Requirements: 5.3, 5.4, 5.5, 5.6, 5.7_

  - [x] 9.4 Implementasi Webhook Handler dan routes
    - Buat `internal/handler/webhook_handler.go`
    - POST `/api/v1/webhooks/endpoints` — register webhook endpoint (JWT protected)
    - GET `/api/v1/webhooks/endpoints` — list endpoints (JWT protected)
    - DELETE `/api/v1/webhooks/endpoints/:id` — delete endpoint (JWT protected)
    - GET `/api/v1/webhooks/deliveries` — list deliveries (JWT protected)
    - Tambahkan Swagger annotations
    - _Requirements: 5.1_

  - [x] 9.5 Integrasi webhook trigger ke Transaction Service
    - Setelah simulasi payment selesai (status berubah), panggil WebhookService.TriggerWebhook
    - Kirim event type "transaction.success" atau "transaction.failed"
    - _Requirements: 4.5, 5.2_

  - [x] 9.6 Write unit tests for Webhook Service dan Worker
    - Test register endpoint, trigger webhook, HMAC signature generation
    - Test worker: delivery success, retry on failure, max retries reached
    - Test graceful shutdown
    - _Requirements: 5.1, 5.2, 5.3, 5.4, 5.5, 5.6, 5.7_

- [x] 10. Subscription Billing (Phase 2 Enhancement)
  - [x] 10.1 Implementasi Subscription Repository
    - Buat `internal/repository/subscription_repository.go` dengan interface dan implementasi
    - Methods untuk Product: Create, FindByUserID, FindByID, Update
    - Methods untuk Plan: Create, FindByProductID, FindByID, Update
    - Methods untuk Subscription: Create, FindByUserID, FindByID, UpdateStatus
    - _Requirements: 6.1, 6.2, 6.3_

  - [x] 10.2 Implementasi Subscription Service
    - Buat `internal/service/subscription_service.go` dengan interface dan implementasi
    - CreateProduct: simpan produk dengan nama, deskripsi, status aktif
    - CreatePlan: simpan plan dengan amount, currency, billing interval (monthly/yearly)
    - CreateSubscription: status "pending_payment", buat transaksi terkait via TransactionService, buat invoice via InvoiceService
    - ActivateSubscription: dipanggil setelah payment success, update status "active", update invoice "paid"
    - CancelSubscription: update status "cancelled", catat cancelled_at
    - Set current_period_start dan current_period_end
    - Gunakan TransactionManager untuk operasi multi-write
    - _Requirements: 6.1, 6.2, 6.3, 6.4, 6.5, 6.6_

  - [x] 10.3 Implementasi Subscription Handler dan routes
    - Buat `internal/handler/subscription_handler.go`
    - POST `/api/v1/products` — buat produk (JWT protected)
    - GET `/api/v1/products` — list produk (JWT protected)
    - POST `/api/v1/products/:id/plans` — buat plan (JWT protected)
    - POST `/api/v1/subscriptions` — buat subscription (JWT protected)
    - GET `/api/v1/subscriptions` — list subscriptions (JWT protected)
    - PATCH `/api/v1/subscriptions/:id/cancel` — cancel subscription (JWT protected)
    - Tambahkan Swagger annotations
    - _Requirements: 6.1, 6.2, 6.3, 6.5_

  - [x] 10.4 Write unit tests for Subscription Service
    - Test create product, create plan, create subscription flow
    - Test activate subscription after payment
    - Test cancel subscription
    - _Requirements: 6.1, 6.2, 6.3, 6.4, 6.5, 6.6_

- [x] 11. Checkpoint — Phase 2 Enhancement complete
  - Ensure all tests pass, ask the user if questions arise.
  - Verifikasi: webhooks terkirim dengan retry, subscription billing flow berjalan end-to-end

- [x] 12. Rate Limiting (Phase 3 Scale & Optimize)
  - [x] 12.1 Implementasi Rate Limiter Middleware
    - Buat `internal/middleware/ratelimit_middleware.go`
    - Implementasi sliding window counter menggunakan Redis MULTI/EXEC
    - Identifikasi client berdasarkan API key (prioritas) atau IP address
    - Tambahkan response headers: X-RateLimit-Limit, X-RateLimit-Remaining, X-RateLimit-Reset
    - Return 429 Too Many Requests dengan header Retry-After jika limit terlampaui
    - Rate limit configurable per API key (default dari config)
    - _Requirements: 8.1, 8.2, 8.3, 8.4_

  - [x] 12.2 Integrasi Rate Limiter ke router
    - Terapkan rate limiter middleware pada route group API yang menggunakan API key
    - Pastikan rate limit per-key sesuai dengan konfigurasi di APIKey model
    - _Requirements: 8.1, 8.4_

  - [x] 12.3 Write unit tests for Rate Limiter
    - Test request allowed within limit
    - Test request rejected when limit exceeded (429 response)
    - Test rate limit headers present in response
    - Test sliding window reset
    - _Requirements: 8.1, 8.2, 8.3, 8.4_

- [x] 13. Admin Dashboard API (Phase 3 Scale & Optimize)
  - [x] 13.1 Implementasi Admin Service
    - Buat `internal/service/admin_service.go` dengan interface dan implementasi
    - GetDashboardStats: total transaksi (per status), jumlah user aktif, webhook delivery success rate
    - Aggregate data dari multiple repositories
    - _Requirements: 9.1, 9.2_

  - [x] 13.2 Implementasi Admin Handler dan routes
    - Buat `internal/handler/admin_handler.go`
    - GET `/api/v1/admin/stats` — dashboard statistics (JWT + AdminOnly protected)
    - Tambahkan Swagger annotations
    - _Requirements: 9.1, 9.2, 9.3_

  - [x] 13.3 Write unit tests for Admin Service
    - Test get dashboard stats with various data scenarios
    - Test admin-only access enforcement
    - _Requirements: 9.1, 9.2, 9.3_

- [x] 14. Docker dan CI/CD Setup (Phase 3 Scale & Optimize)
  - [x] 14.1 Buat docker-compose.yml
    - Service: backend (Go app), frontend (Next.js), postgres, redis
    - Volume mounts untuk persistent data
    - Environment variables dari .env file
    - Health checks untuk semua services
    - Network configuration
    - _Requirements: 12.1_

  - [x] 14.2 Buat Kubernetes deployment manifests
    - Buat `k8s/backend-deployment.yaml` — Deployment dengan resource limits (CPU, memory), liveness/readiness/startup probes
    - Buat `k8s/backend-service.yaml` — ClusterIP Service
    - Buat `k8s/backend-ingress.yaml` — Ingress configuration
    - Buat `k8s/postgres-statefulset.yaml` — PostgreSQL StatefulSet dengan persistent volume
    - Buat `k8s/redis-deployment.yaml` — Redis Deployment
    - Definisikan PodDisruptionBudget untuk backend
    - Gunakan namespaces per environment (dev, staging, prod)
    - Resource limits HARUS didefinisikan sesuai infra_contract.yaml
    - _Requirements: 12.1_

  - [x] 14.3 Buat GitHub Actions CI workflow
    - Buat `.github/workflows/ci.yml`
    - Pipeline stages sesuai infra_rules: lint → test → build → security scan → deploy-staging → integration-test → deploy-production
    - Security scan: dependency vulnerability scan (`govulncheck`), container image scan (`trivy`)
    - Test results dan coverage report sebagai artifacts
    - Fail pipeline jika critical/high vulnerability ditemukan
    - _Requirements: 12.1_

- [x] 15. Checkpoint — Phase 3 complete
  - Ensure all tests pass, ask the user if questions arise.
  - Verifikasi: rate limiting berfungsi, admin stats tersedia, Docker setup berjalan

- [x] 16. Seed data dan audit logging
  - [x] 16.1 Buat seed data per environment
    - Buat `backend/migrations/seed/` directory
    - Buat seed data untuk development: admin user, sample developer, sample API keys, sample transactions
    - Seed data HARUS idempotent (safe to run multiple times)
    - Seed data TIDAK BOLEH mengandung production data
    - Integrasikan ke Makefile target: `make seed`
    - _Requirements: 11.2_

  - [x] 16.2 Implementasi audit logging
    - Buat `internal/pkg/audit/audit.go` — audit log writer
    - Log security-sensitive operations: login, logout, API key creation/revocation, transaction status changes, admin actions
    - Audit log entry HARUS include: who (user_id), what (action), when (timestamp), where (endpoint/service), result (success/failure)
    - Audit logs HARUS disimpan terpisah dari application logs
    - _Requirements: 11.5_

- [x] 17. Frontend setup (Next.js + TypeScript)
  - [x] 17.1 Inisialisasi Next.js project
    - Buat `frontend/` dengan Next.js App Router, TypeScript, Tailwind CSS
    - Setup `package.json`, `tsconfig.json`
    - Buat `frontend/Dockerfile` (multi-stage build, non-root user)
    - Buat `frontend/.gitignore`
    - Setup ESLint + Prettier
    - _Requirements: 12.1_

  - [x] 17.2 Buat API service layer dan types
    - Buat `frontend/src/services/api.ts` — base HTTP client dengan auth token handling
    - Buat `frontend/src/types/` — TypeScript interfaces matching backend DTOs
    - Buat service files: `auth.ts`, `apikeys.ts`, `transactions.ts`, `webhooks.ts`, `subscriptions.ts`, `invoices.ts`, `admin.ts`
    - _Requirements: 12.1_

  - [x] 17.3 Implementasi halaman auth (login/register)
    - Buat `frontend/src/app/(auth)/login/page.tsx`
    - Buat `frontend/src/app/(auth)/register/page.tsx`
    - Form validation, JWT token storage (secure cookie/httpOnly), redirect after login
    - Gunakan semantic HTML elements, ARIA attributes, keyboard navigation support
    - _Requirements: 1.1, 1.4_

  - [x] 17.4 Implementasi halaman dashboard dan fitur utama
    - Buat `frontend/src/app/dashboard/page.tsx` — overview stats
    - Buat `frontend/src/app/api-keys/page.tsx` — CRUD API keys
    - Buat `frontend/src/app/transactions/page.tsx` — list transaksi dengan pagination
    - Buat `frontend/src/app/webhooks/page.tsx` — manage webhook endpoints
    - Buat `frontend/src/app/subscriptions/page.tsx` — products, plans, subscriptions
    - Buat `frontend/src/app/invoices/page.tsx` — list invoices
    - Buat `frontend/src/app/admin/page.tsx` — admin dashboard (admin only)
    - Setiap route HARUS memiliki error boundary dengan fallback UI
    - Confirmation dialog untuk destructive actions (delete, revoke)
    - _Requirements: 3.1, 4.6, 5.1, 6.1, 7.4, 9.1_

- [x] 18. Swagger documentation generation
  - [x] 18.1 Setup Swagger dan generate docs
    - Install `swaggo/swag` dan `swaggo/fiber-swagger`
    - Tambahkan general API info annotations di `cmd/server/main.go`
    - Jalankan `swag init` untuk generate swagger docs
    - Expose Swagger UI di `/swagger/` endpoint
    - Tambahkan `make swagger` target di Makefile
    - _Requirements: 12.2_

- [x] 19. Wiring dan integrasi akhir
  - [x] 19.1 Wire semua komponen di main.go
    - Inisialisasi semua repositories, services, handlers dengan dependency injection
    - Register semua routes dengan middleware yang sesuai
    - Start webhook worker sebagai background goroutine
    - Setup graceful shutdown untuk server dan workers
    - _Requirements: 12.1, 12.2, 12.4_

  - [x] 19.2 Write integration tests untuk critical API flows
    - Test full auth flow: register → login → access protected endpoint
    - Test transaction flow: create API key → create transaction → simulate payment → check webhook delivery
    - Test subscription flow: create product → create plan → subscribe → payment → activate
    - _Requirements: 1.1, 1.4, 3.1, 3.3, 4.1, 4.4, 5.2, 5.3, 6.3, 6.4, 7.1_

- [x] 20. Final checkpoint — Semua fitur terintegrasi dan tests pass
  - Ensure all tests pass, ask the user if questions arise.
  - Verifikasi seluruh flow end-to-end berfungsi sesuai requirements

## Notes

- Tasks marked with `*` are optional and can be skipped for faster MVP
- Each task references specific requirements for traceability
- Checkpoints ensure incremental validation at each phase boundary
- Implementation follows the phased roadmap: Phase 1 (tasks 1-8), Phase 2 (tasks 9-11), Phase 3 (tasks 12-16), Frontend & Integration (tasks 17-20)
- All code must follow clean architecture rules: handler → service → repository
- All database writes must use Transaction Manager
- All handlers must have Swagger annotations
- Migration naming MUST follow format: YYYYMMDDHHMMSS_description
