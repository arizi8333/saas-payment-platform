# Dokumen Desain: SaaS Payment Platform Setup

## Overview

SaaS Payment Platform adalah platform pembayaran terinspirasi dari Stripe dan Xendit yang menyediakan API untuk developer melakukan integrasi pembayaran. Platform ini mencakup autentikasi user berbasis JWT, manajemen API key, simulasi payment gateway, webhook notification dengan retry mechanism, subscription billing, invoice management, rate limiting berbasis Redis, dan admin dashboard untuk monitoring.

Arsitektur menggunakan pendekatan clean architecture dengan monorepo structure: backend Go + Fiber framework dan frontend Next.js + TypeScript. Database utama PostgreSQL untuk persistent storage dan Redis untuk caching serta rate limiting. Sistem dirancang untuk menangani 10K-100K user dengan availability 99.9% dan mematuhi standar PCI-DSS untuk keamanan data pembayaran.

Platform dibangun dalam 4 fase: MVP (auth, API key, transaksi, invoice dasar), Enhancement (webhook, subscription, tracking), Scale & Optimize (rate limiter, monitoring, dashboard, Docker/CI-CD), dan Advanced (idempotency, event-driven, audit log).

## Architecture

### System Architecture Overview

```mermaid
graph TD
    subgraph Client Layer
        FE[Next.js Frontend]
        EXT[External Developer API Client]
    end

    subgraph API Gateway Layer
        RL[Rate Limiter Middleware]
        AUTH[JWT Auth Middleware]
        APIKEY[API Key Validator Middleware]
    end

    subgraph Application Layer
        UH[User Handler]
        AKH[API Key Handler]
        TH[Transaction Handler]
        WH[Webhook Handler]
        SH[Subscription Handler]
        IH[Invoice Handler]
        ADH[Admin Handler]
    end

    subgraph Service Layer
        US[User Service]
        AKS[API Key Service]
        TS[Transaction Service]
        WS[Webhook Service]
        SS[Subscription Service]
        IS[Invoice Service]
        ADS[Admin Service]
        TXM[Transaction Manager]
    end

    subgraph Repository Layer
        UR[User Repository]
        AKR[API Key Repository]
        TR[Transaction Repository]
        WR[Webhook Repository]
        SR[Subscription Repository]
        IR[Invoice Repository]
    end

    subgraph Data Layer
        PG[(PostgreSQL)]
        RD[(Redis)]
    end

    subgraph Background Workers
        WW[Webhook Worker]
        SW[Subscription Worker]
    end

    FE --> RL
    EXT --> RL
    RL --> AUTH
    RL --> APIKEY
    AUTH --> UH & AKH & SH & IH & ADH
    APIKEY --> TH & WH

    UH --> US
    AKH --> AKS
    TH --> TS
    WH --> WS
    SH --> SS
    IH --> IS
    ADH --> ADS

    US & AKS & TS & WS & SS & IS --> TXM
    US --> UR
    AKS --> AKR
    TS --> TR
    WS --> WR
    SS --> SR
    IS --> IR

    UR & AKR & TR & WR & SR & IR --> PG
    RL & AKS --> RD
    WW --> WS
    SW --> SS
```


### Request Flow Sequence

```mermaid
sequenceDiagram
    participant C as Client
    participant RL as Rate Limiter
    participant MW as Auth Middleware
    participant H as Handler
    participant S as Service
    participant TX as TxManager
    participant R as Repository
    participant DB as PostgreSQL
    participant RD as Redis

    C->>RL: HTTP Request
    RL->>RD: Check rate limit
    RD-->>RL: Allowed/Denied

    alt Rate Limit Exceeded
        RL-->>C: 429 Too Many Requests
    end

    RL->>MW: Forward request
    MW->>MW: Validate JWT / API Key

    alt Auth Failed
        MW-->>C: 401 Unauthorized
    end

    MW->>H: Authenticated request
    H->>H: Validate & parse DTO
    H->>S: Call service method(ctx, dto)
    S->>TX: WithTransaction(ctx, fn)
    TX->>DB: BEGIN
    S->>R: Repository operation(txCtx)
    R->>DB: SQL Query
    DB-->>R: Result
    S->>R: Additional operations
    R->>DB: SQL Query
    DB-->>R: Result
    TX->>DB: COMMIT
    S-->>H: Response DTO
    H-->>C: HTTP Response
```

### Payment Transaction Flow

```mermaid
sequenceDiagram
    participant DEV as Developer App
    participant API as Payment API
    participant TS as Transaction Service
    participant PS as Payment Simulator
    participant WS as Webhook Service
    participant WW as Webhook Worker
    participant DB as PostgreSQL

    DEV->>API: POST /api/v1/transactions (API Key)
    API->>TS: CreateTransaction(ctx, req)
    TS->>DB: Insert transaction (status: pending)
    TS-->>API: Transaction created
    API-->>DEV: 201 Created

    Note over PS: Async payment simulation
    PS->>DB: Update status (success/failed)
    PS->>WS: TriggerWebhook(transaction_id)
    WS->>DB: Create webhook delivery record
    WS->>WW: Enqueue webhook delivery

    WW->>DEV: POST webhook_url with payload

    alt Webhook Success
        DEV-->>WW: 200 OK
        WW->>DB: Update delivery (delivered)
    else Webhook Failed
        DEV-->>WW: Error / Timeout
        WW->>DB: Update delivery (failed, retry_count++)
        Note over WW: Retry with exponential backoff
        WW->>DEV: Retry POST webhook_url
    end
```

### Subscription Billing Flow

```mermaid
sequenceDiagram
    participant EU as End User
    participant API as Subscription API
    participant SS as Subscription Service
    participant TS as Transaction Service
    participant IS as Invoice Service
    participant DB as PostgreSQL

    EU->>API: POST /api/v1/subscriptions
    API->>SS: CreateSubscription(ctx, req)
    SS->>DB: Insert subscription (pending_payment)
    SS->>TS: CreateTransaction for subscription
    TS->>DB: Insert transaction (pending)
    SS->>IS: CreateInvoice for subscription
    IS->>DB: Insert invoice (unpaid)
    SS-->>API: Subscription created
    API-->>EU: 201 Created

    Note over TS: Payment simulation completes
    TS->>SS: NotifyPaymentComplete(transaction_id)
    SS->>DB: Update subscription (active)
    SS->>IS: UpdateInvoiceStatus(paid)
    IS->>DB: Update invoice (paid)
```


## Components and Interfaces

### Struktur Proyek Monorepo

```
project-root/
├── backend/
│   ├── cmd/server/main.go                  # Entry point aplikasi
│   ├── internal/
│   │   ├── config/config.go                # Konfigurasi aplikasi
│   │   ├── handler/                        # HTTP handlers (Fiber)
│   │   │   ├── user_handler.go
│   │   │   ├── apikey_handler.go
│   │   │   ├── transaction_handler.go
│   │   │   ├── webhook_handler.go
│   │   │   ├── subscription_handler.go
│   │   │   ├── invoice_handler.go
│   │   │   └── admin_handler.go
│   │   ├── service/                        # Business logic
│   │   │   ├── user_service.go
│   │   │   ├── apikey_service.go
│   │   │   ├── transaction_service.go
│   │   │   ├── webhook_service.go
│   │   │   ├── subscription_service.go
│   │   │   ├── invoice_service.go
│   │   │   └── admin_service.go
│   │   ├── repository/                     # Database operations
│   │   │   ├── user_repository.go
│   │   │   ├── apikey_repository.go
│   │   │   ├── transaction_repository.go
│   │   │   ├── webhook_repository.go
│   │   │   ├── subscription_repository.go
│   │   │   └── invoice_repository.go
│   │   ├── model/                          # Domain models
│   │   │   ├── user.go
│   │   │   ├── apikey.go
│   │   │   ├── transaction.go
│   │   │   ├── webhook.go
│   │   │   ├── subscription.go
│   │   │   ├── invoice.go
│   │   │   └── product.go
│   │   ├── dto/                            # Request/Response DTOs
│   │   │   ├── user_dto.go
│   │   │   ├── apikey_dto.go
│   │   │   ├── transaction_dto.go
│   │   │   ├── webhook_dto.go
│   │   │   ├── subscription_dto.go
│   │   │   ├── invoice_dto.go
│   │   │   └── common_dto.go
│   │   ├── middleware/                     # Fiber middlewares
│   │   │   ├── auth_middleware.go
│   │   │   ├── apikey_middleware.go
│   │   │   ├── ratelimit_middleware.go
│   │   │   ├── cors_middleware.go
│   │   │   ├── logger_middleware.go
│   │   │   └── requestid_middleware.go
│   │   ├── worker/                         # Background workers
│   │   │   ├── webhook_worker.go
│   │   │   └── subscription_worker.go
│   │   └── pkg/                            # Internal shared packages
│   │       ├── database/postgres.go
│   │       ├── database/redis.go
│   │       ├── database/txmanager.go
│   │       ├── auth/jwt.go
│   │       ├── hash/bcrypt.go
│   │       ├── response/response.go
│   │       ├── validator/validator.go
│   │       └── logger/logger.go
│   ├── pkg/                                # Public shared packages
│   │   └── apierror/apierror.go
│   ├── migrations/                         # SQL migration files
│   ├── go.mod
│   ├── go.sum
│   ├── Makefile
│   └── Dockerfile
├── frontend/
│   ├── src/
│   │   ├── app/                            # Next.js App Router
│   │   │   ├── (auth)/login/page.tsx
│   │   │   ├── (auth)/register/page.tsx
│   │   │   ├── dashboard/page.tsx
│   │   │   ├── api-keys/page.tsx
│   │   │   ├── transactions/page.tsx
│   │   │   ├── webhooks/page.tsx
│   │   │   ├── subscriptions/page.tsx
│   │   │   ├── invoices/page.tsx
│   │   │   └── admin/page.tsx
│   │   ├── components/                     # Reusable UI components
│   │   ├── services/                       # API service layer
│   │   ├── types/                          # TypeScript type definitions
│   │   ├── utils/                          # Utility functions
│   │   └── hooks/                          # Custom React hooks
│   ├── package.json
│   ├── tsconfig.json
│   └── Dockerfile
├── reports/                                # Gitignored - auto-generated
│   ├── backend/
│   └── frontend/
├── docker-compose.yml
├── .github/workflows/ci.yml
├── kiro/
├── README.md
└── project_intake.yaml
```

### Komponen 1: Auth Middleware

**Tujuan**: Memvalidasi JWT token dan API key pada setiap request yang membutuhkan autentikasi.

```go
// internal/middleware/auth_middleware.go

type AuthMiddleware struct {
    jwtService auth.JWTService
    userRepo   repository.UserRepository
}

// JWTProtected memvalidasi Bearer token dari header Authorization
func (m *AuthMiddleware) JWTProtected() fiber.Handler

// AdminOnly memvalidasi bahwa user memiliki role admin
func (m *AuthMiddleware) AdminOnly() fiber.Handler
```

```go
// internal/middleware/apikey_middleware.go

type APIKeyMiddleware struct {
    apiKeyRepo repository.APIKeyRepository
    redisCache *redis.Client
}

// ValidateAPIKey memvalidasi X-API-Key header dan mencatat usage
func (m *APIKeyMiddleware) ValidateAPIKey() fiber.Handler
```

**Tanggung Jawab**:
- Ekstraksi dan validasi JWT token dari header Authorization
- Ekstraksi dan validasi API key dari header X-API-Key
- Injeksi user context (user_id, role) ke request context
- Pencatatan API key usage ke Redis untuk rate limiting
- Penolakan request yang tidak terautentikasi dengan 401

### Komponen 2: Transaction Manager

**Tujuan**: Mengelola database transaction di service layer sesuai clean architecture.

```go
// internal/pkg/database/txmanager.go

type TransactionManager interface {
    WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}

type txManager struct {
    db *gorm.DB
}

func NewTransactionManager(db *gorm.DB) TransactionManager
```

**Tanggung Jawab**:
- Memulai database transaction
- Menyimpan transaction ke context untuk digunakan repository
- Commit jika semua operasi sukses
- Rollback jika ada error atau panic
- Deteksi nested transaction untuk mencegah double-begin

### Komponen 3: Webhook Worker

**Tujuan**: Background worker yang mengirimkan webhook notification secara asynchronous dengan retry mechanism.

```go
// internal/worker/webhook_worker.go

type WebhookWorker struct {
    webhookService service.WebhookService
    httpClient     *http.Client
    maxRetries     int
    logger         *slog.Logger
}

func NewWebhookWorker(ws service.WebhookService, maxRetries int) *WebhookWorker

// Start memulai worker loop yang memproses webhook delivery queue
func (w *WebhookWorker) Start(ctx context.Context) error

// Stop menghentikan worker secara graceful
func (w *WebhookWorker) Stop() error
```

**Tanggung Jawab**:
- Polling webhook delivery queue dari database
- Mengirim HTTP POST ke webhook URL milik developer
- Retry dengan exponential backoff (1s, 2s, 4s, 8s, 16s) jika gagal
- Update status delivery (delivered/failed) di database
- Graceful shutdown saat menerima signal

### Komponen 4: Rate Limiter

**Tujuan**: Membatasi jumlah request API berdasarkan API key menggunakan Redis sliding window.

```go
// internal/middleware/ratelimit_middleware.go

type RateLimiter struct {
    redisClient *redis.Client
    limit       int
    window      time.Duration
}

func NewRateLimiter(redisClient *redis.Client, limit int, window time.Duration) *RateLimiter

// Limit mengembalikan Fiber middleware yang menerapkan rate limiting
func (rl *RateLimiter) Limit() fiber.Handler
```

**Tanggung Jawab**:
- Implementasi sliding window counter menggunakan Redis MULTI/EXEC
- Identifikasi client berdasarkan API key atau IP address
- Return 429 Too Many Requests dengan header Retry-After jika limit terlampaui
- Menambahkan header X-RateLimit-Limit, X-RateLimit-Remaining, X-RateLimit-Reset


## Data Models

### Model 1: User

```go
// internal/model/user.go

type UserRole string

const (
    RoleAdmin     UserRole = "admin"
    RoleDeveloper UserRole = "developer"
    RoleEndUser   UserRole = "end_user"
)

type User struct {
    ID           uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
    Email        string         `gorm:"type:varchar(255);uniqueIndex;not null"`
    PasswordHash string         `gorm:"type:varchar(255);not null"`
    FullName     string         `gorm:"type:varchar(255);not null"`
    Role         UserRole       `gorm:"type:varchar(20);not null;default:'developer'"`
    IsActive     bool           `gorm:"not null;default:true"`
    CreatedAt    time.Time      `gorm:"not null;default:now()"`
    UpdatedAt    time.Time      `gorm:"not null;default:now()"`
    DeletedAt    gorm.DeletedAt `gorm:"index"`
}
```

**Aturan Validasi**:
- Email harus format valid dan unik
- Password minimal 8 karakter, harus di-hash dengan bcrypt
- Role harus salah satu dari: admin, developer, end_user
- Soft delete menggunakan deleted_at

### Model 2: APIKey

```go
// internal/model/apikey.go

type APIKey struct {
    ID         uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
    UserID     uuid.UUID      `gorm:"type:uuid;not null;index"`
    Name       string         `gorm:"type:varchar(100);not null"`
    KeyHash    string         `gorm:"type:varchar(255);not null;uniqueIndex"`
    KeyPrefix  string         `gorm:"type:varchar(12);not null"`
    IsActive   bool           `gorm:"not null;default:true"`
    LastUsedAt *time.Time
    ExpiresAt  *time.Time
    RateLimit  int            `gorm:"not null;default:1000"`
    CreatedAt  time.Time      `gorm:"not null;default:now()"`
    UpdatedAt  time.Time      `gorm:"not null;default:now()"`
    DeletedAt  gorm.DeletedAt `gorm:"index"`

    User User `gorm:"foreignKey:UserID"`
}
```

**Aturan Validasi**:
- API key di-hash sebelum disimpan (hanya ditampilkan sekali saat pembuatan)
- KeyPrefix disimpan untuk identifikasi tanpa expose full key (format: sk_test_xxxx)
- Rate limit default 1000 requests/hour, configurable per key
- Foreign key ke users table

### Model 3: Transaction

```go
// internal/model/transaction.go

type TransactionStatus string

const (
    TxStatusPending TransactionStatus = "pending"
    TxStatusSuccess TransactionStatus = "success"
    TxStatusFailed  TransactionStatus = "failed"
    TxStatusExpired TransactionStatus = "expired"
)

type PaymentMethod string

const (
    PMBankTransfer PaymentMethod = "bank_transfer"
    PMCreditCard   PaymentMethod = "credit_card"
    PMEWallet      PaymentMethod = "e_wallet"
    PMQRCode       PaymentMethod = "qr_code"
)

type Transaction struct {
    ID             uuid.UUID         `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
    UserID         uuid.UUID         `gorm:"type:uuid;not null;index"`
    ExternalID     string            `gorm:"type:varchar(255);not null;uniqueIndex"`
    Amount         int64             `gorm:"not null"`
    Currency       string            `gorm:"type:varchar(3);not null;default:'IDR'"`
    Status         TransactionStatus `gorm:"type:varchar(20);not null;default:'pending'"`
    PaymentMethod  PaymentMethod     `gorm:"type:varchar(30);not null"`
    Description    string            `gorm:"type:text"`
    CustomerEmail  string            `gorm:"type:varchar(255)"`
    IdempotencyKey *string           `gorm:"type:varchar(255);uniqueIndex"`
    Metadata       datatypes.JSON    `gorm:"type:jsonb"`
    PaidAt         *time.Time
    ExpiredAt      *time.Time
    CreatedAt      time.Time         `gorm:"not null;default:now()"`
    UpdatedAt      time.Time         `gorm:"not null;default:now()"`
    DeletedAt      gorm.DeletedAt    `gorm:"index"`

    User User `gorm:"foreignKey:UserID"`
}
```

**Aturan Validasi**:
- Amount harus positif, disimpan dalam satuan terkecil (cents/rupiah)
- ExternalID unik per developer untuk referensi eksternal
- IdempotencyKey opsional, unik jika diisi (Phase 4)
- Status transition: pending → success/failed/expired
- Metadata berformat JSONB untuk data tambahan fleksibel

### Model 4: WebhookEndpoint & WebhookDelivery

```go
// internal/model/webhook.go

type WebhookEndpoint struct {
    ID        uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
    UserID    uuid.UUID      `gorm:"type:uuid;not null;index"`
    URL       string         `gorm:"type:varchar(500);not null"`
    Secret    string         `gorm:"type:varchar(255);not null"` // untuk HMAC signature verification
    Events    datatypes.JSON `gorm:"type:jsonb;not null"`
    IsActive  bool           `gorm:"not null;default:true"`
    CreatedAt time.Time      `gorm:"not null;default:now()"`
    UpdatedAt time.Time      `gorm:"not null;default:now()"`
    DeletedAt gorm.DeletedAt `gorm:"index"`

    User User `gorm:"foreignKey:UserID"`
}

type DeliveryStatus string

const (
    DeliveryPending   DeliveryStatus = "pending"
    DeliveryDelivered DeliveryStatus = "delivered"
    DeliveryFailed    DeliveryStatus = "failed"
)

type WebhookDelivery struct {
    ID                uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
    WebhookEndpointID uuid.UUID      `gorm:"type:uuid;not null;index"`
    EventType         string         `gorm:"type:varchar(50);not null"`
    Payload           datatypes.JSON `gorm:"type:jsonb;not null"`
    Status            DeliveryStatus `gorm:"type:varchar(20);not null;default:'pending'"`
    ResponseCode      *int
    ResponseBody      *string        `gorm:"type:text"`
    RetryCount        int            `gorm:"not null;default:0"`
    MaxRetries        int            `gorm:"not null;default:5"`
    NextRetryAt       *time.Time     `gorm:"index"`
    DeliveredAt       *time.Time
    CreatedAt         time.Time      `gorm:"not null;default:now()"`
    UpdatedAt         time.Time      `gorm:"not null;default:now()"`

    WebhookEndpoint WebhookEndpoint `gorm:"foreignKey:WebhookEndpointID"`
}
```

### Model 5: Product, Plan & Subscription

```go
// internal/model/product.go

type Product struct {
    ID          uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
    UserID      uuid.UUID      `gorm:"type:uuid;not null;index"`
    Name        string         `gorm:"type:varchar(255);not null"`
    Description string         `gorm:"type:text"`
    IsActive    bool           `gorm:"not null;default:true"`
    CreatedAt   time.Time      `gorm:"not null;default:now()"`
    UpdatedAt   time.Time      `gorm:"not null;default:now()"`
    DeletedAt   gorm.DeletedAt `gorm:"index"`

    User  User   `gorm:"foreignKey:UserID"`
    Plans []Plan `gorm:"foreignKey:ProductID"`
}
```

```go
// internal/model/subscription.go

type BillingInterval string

const (
    BillingMonthly BillingInterval = "monthly"
    BillingYearly  BillingInterval = "yearly"
)

type Plan struct {
    ID              uuid.UUID       `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
    ProductID       uuid.UUID       `gorm:"type:uuid;not null;index"`
    Name            string          `gorm:"type:varchar(255);not null"`
    Amount          int64           `gorm:"not null"`
    Currency        string          `gorm:"type:varchar(3);not null;default:'IDR'"`
    BillingInterval BillingInterval `gorm:"type:varchar(20);not null"`
    IsActive        bool            `gorm:"not null;default:true"`
    CreatedAt       time.Time       `gorm:"not null;default:now()"`
    UpdatedAt       time.Time       `gorm:"not null;default:now()"`
    DeletedAt       gorm.DeletedAt  `gorm:"index"`

    Product Product `gorm:"foreignKey:ProductID"`
}

type SubscriptionStatus string

const (
    SubStatusPendingPayment SubscriptionStatus = "pending_payment"
    SubStatusActive         SubscriptionStatus = "active"
    SubStatusCancelled      SubscriptionStatus = "cancelled"
    SubStatusExpired        SubscriptionStatus = "expired"
)

type Subscription struct {
    ID                 uuid.UUID          `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
    UserID             uuid.UUID          `gorm:"type:uuid;not null;index"`
    PlanID             uuid.UUID          `gorm:"type:uuid;not null;index"`
    Status             SubscriptionStatus `gorm:"type:varchar(30);not null;default:'pending_payment'"`
    CurrentPeriodStart time.Time          `gorm:"not null"`
    CurrentPeriodEnd   time.Time          `gorm:"not null"`
    CancelledAt        *time.Time
    CreatedAt          time.Time          `gorm:"not null;default:now()"`
    UpdatedAt          time.Time          `gorm:"not null;default:now()"`
    DeletedAt          gorm.DeletedAt     `gorm:"index"`

    User User `gorm:"foreignKey:UserID"`
    Plan Plan `gorm:"foreignKey:PlanID"`
}
```

### Model 6: Invoice

```go
// internal/model/invoice.go

type InvoiceStatus string

const (
    InvoiceUnpaid InvoiceStatus = "unpaid"
    InvoicePaid   InvoiceStatus = "paid"
    InvoiceVoid   InvoiceStatus = "void"
)

type Invoice struct {
    ID             uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
    UserID         uuid.UUID      `gorm:"type:uuid;not null;index"`
    TransactionID  *uuid.UUID     `gorm:"type:uuid;index"`
    SubscriptionID *uuid.UUID     `gorm:"type:uuid;index"`
    InvoiceNumber  string         `gorm:"type:varchar(50);not null;uniqueIndex"`
    Amount         int64          `gorm:"not null"`
    Currency       string         `gorm:"type:varchar(3);not null;default:'IDR'"`
    Status         InvoiceStatus  `gorm:"type:varchar(20);not null;default:'unpaid'"`
    DueDate        time.Time      `gorm:"not null"`
    PaidAt         *time.Time
    CreatedAt      time.Time      `gorm:"not null;default:now()"`
    UpdatedAt      time.Time      `gorm:"not null;default:now()"`
    DeletedAt      gorm.DeletedAt `gorm:"index"`

    User         User          `gorm:"foreignKey:UserID"`
    Transaction  *Transaction  `gorm:"foreignKey:TransactionID"`
    Subscription *Subscription `gorm:"foreignKey:SubscriptionID"`
}
```

**Aturan Validasi**:
- InvoiceNumber auto-generated dengan format: INV-YYYYMMDD-XXXXX
- Invoice bisa terkait dengan Transaction ATAU Subscription (salah satu)
- Status transition: unpaid → paid/void

### Database Schema Diagram

```mermaid
erDiagram
    users ||--o{ api_keys : "has many"
    users ||--o{ transactions : "has many"
    users ||--o{ webhook_endpoints : "has many"
    users ||--o{ products : "has many"
    users ||--o{ subscriptions : "subscribes"
    users ||--o{ invoices : "has many"

    products ||--o{ plans : "has many"
    plans ||--o{ subscriptions : "has many"

    webhook_endpoints ||--o{ webhook_deliveries : "has many"

    transactions ||--o| invoices : "has one"
    subscriptions ||--o{ invoices : "has many"

    users {
        uuid id PK
        varchar email UK
        varchar password_hash
        varchar full_name
        varchar role
        boolean is_active
        timestamp created_at
        timestamp updated_at
        timestamp deleted_at
    }

    api_keys {
        uuid id PK
        uuid user_id FK
        varchar name
        varchar key_hash UK
        varchar key_prefix
        boolean is_active
        timestamp last_used_at
        timestamp expires_at
        int rate_limit
        timestamp created_at
        timestamp updated_at
        timestamp deleted_at
    }

    transactions {
        uuid id PK
        uuid user_id FK
        varchar external_id UK
        bigint amount
        varchar currency
        varchar status
        varchar payment_method
        text description
        varchar customer_email
        varchar idempotency_key UK
        jsonb metadata
        timestamp paid_at
        timestamp expired_at
        timestamp created_at
        timestamp updated_at
        timestamp deleted_at
    }

    webhook_endpoints {
        uuid id PK
        uuid user_id FK
        varchar url
        varchar secret
        jsonb events
        boolean is_active
        timestamp created_at
        timestamp updated_at
        timestamp deleted_at
    }

    webhook_deliveries {
        uuid id PK
        uuid webhook_endpoint_id FK
        varchar event_type
        jsonb payload
        varchar status
        int response_code
        text response_body
        int retry_count
        int max_retries
        timestamp next_retry_at
        timestamp delivered_at
        timestamp created_at
        timestamp updated_at
    }

    products {
        uuid id PK
        uuid user_id FK
        varchar name
        text description
        boolean is_active
        timestamp created_at
        timestamp updated_at
        timestamp deleted_at
    }

    plans {
        uuid id PK
        uuid product_id FK
        varchar name
        bigint amount
        varchar currency
        varchar billing_interval
        boolean is_active
        timestamp created_at
        timestamp updated_at
        timestamp deleted_at
    }

    subscriptions {
        uuid id PK
        uuid user_id FK
        uuid plan_id FK
        varchar status
        timestamp current_period_start
        timestamp current_period_end
        timestamp cancelled_at
        timestamp created_at
        timestamp updated_at
        timestamp deleted_at
    }

    invoices {
        uuid id PK
        uuid user_id FK
        uuid transaction_id FK
        uuid subscription_id FK
        varchar invoice_number UK
        bigint amount
        varchar currency
        varchar status
        timestamp due_date
        timestamp paid_at
        timestamp created_at
        timestamp updated_at
        timestamp deleted_at
    }
```
