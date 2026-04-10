# Dokumen Requirements: SaaS Payment Platform Setup

## Pendahuluan

Dokumen ini mendefinisikan requirements untuk SaaS Payment Platform, sebuah platform pembayaran terinspirasi dari Stripe dan Xendit. Platform menyediakan API key management, simulasi payment gateway, webhook notification, subscription billing, invoice management, rate limiting, dan admin dashboard. Requirements diturunkan dari dokumen desain teknis yang telah disetujui dan mengikuti standar EARS (Easy Approach to Requirements Syntax) serta INCOSE quality rules.

## Glossary

- **Platform**: SaaS Payment Platform secara keseluruhan
- **Auth_Middleware**: Komponen middleware yang memvalidasi JWT token dan API key pada setiap request
- **User_Service**: Komponen service layer yang mengelola business logic terkait user (registrasi, login, profil)
- **APIKey_Service**: Komponen service layer yang mengelola pembuatan, validasi, dan revokasi API key
- **Transaction_Service**: Komponen service layer yang mengelola pembuatan dan pemrosesan transaksi pembayaran
- **Webhook_Service**: Komponen service layer yang mengelola webhook endpoint dan delivery
- **Webhook_Worker**: Background worker yang mengirimkan webhook notification secara asynchronous
- **Subscription_Service**: Komponen service layer yang mengelola produk, plan, dan subscription billing
- **Invoice_Service**: Komponen service layer yang mengelola pembuatan dan status invoice
- **Rate_Limiter**: Komponen middleware yang membatasi jumlah request API menggunakan Redis sliding window
- **Admin_Service**: Komponen service layer yang menyediakan statistik dan monitoring untuk admin
- **Transaction_Manager**: Komponen yang mengelola database transaction di service layer
- **Developer**: User dengan role developer yang menggunakan API platform untuk integrasi pembayaran
- **End_User**: User akhir yang melakukan pembayaran atau berlangganan produk
- **Admin**: User dengan role admin yang memiliki akses penuh ke monitoring dan manajemen

## Requirements

### Requirement 1: Registrasi dan Autentikasi User

**User Story:** Sebagai developer, saya ingin mendaftar dan login ke platform, sehingga saya dapat mengakses layanan pembayaran melalui API.

#### Acceptance Criteria

1. WHEN seorang user mengirimkan request registrasi dengan email dan password yang valid, THE User_Service SHALL membuat akun baru dengan role default "developer" dan menyimpan password dalam bentuk bcrypt hash
2. WHEN seorang user mengirimkan request registrasi dengan email yang sudah terdaftar, THE User_Service SHALL menolak registrasi dan mengembalikan error yang deskriptif
3. WHEN seorang user mengirimkan request registrasi dengan password kurang dari 8 karakter, THE User_Service SHALL menolak registrasi dan mengembalikan error validasi
4. WHEN seorang user mengirimkan kredensial login yang valid, THE User_Service SHALL mengembalikan JWT token yang berisi user_id dan role
5. WHEN seorang user mengirimkan kredensial login yang tidak valid, THE User_Service SHALL menolak login dan mengembalikan response 401 Unauthorized
6. WHEN seorang user yang terautentikasi mengakses endpoint profil, THE User_Service SHALL mengembalikan data profil user tanpa mengekspos password_hash

### Requirement 2: Validasi JWT dan Otorisasi

**User Story:** Sebagai platform, saya ingin memvalidasi setiap request yang membutuhkan autentikasi, sehingga hanya user yang terautentikasi yang dapat mengakses resource yang dilindungi.

#### Acceptance Criteria

1. WHEN sebuah request masuk dengan Bearer token yang valid di header Authorization, THE Auth_Middleware SHALL mengekstrak user_id dan role lalu menyimpannya ke request context
2. WHEN sebuah request masuk tanpa token atau dengan token yang tidak valid, THE Auth_Middleware SHALL menolak request dengan response 401 Unauthorized
3. WHEN sebuah request masuk dengan token yang sudah expired, THE Auth_Middleware SHALL menolak request dengan response 401 Unauthorized
4. WHILE sebuah endpoint memerlukan role admin, THE Auth_Middleware SHALL memvalidasi bahwa user memiliki role admin dan menolak request dengan 403 Forbidden jika role tidak sesuai

### Requirement 3: Manajemen API Key

**User Story:** Sebagai developer, saya ingin membuat dan mengelola API key, sehingga saya dapat mengakses layanan payment gateway melalui API.

#### Acceptance Criteria

1. WHEN seorang developer membuat API key baru, THE APIKey_Service SHALL menghasilkan key unik, menyimpan hash-nya di database, dan mengembalikan full key hanya sekali saat pembuatan
2. WHEN seorang developer membuat API key, THE APIKey_Service SHALL menyimpan key prefix dengan format "sk_test_xxxx" untuk identifikasi tanpa mengekspos full key
3. WHEN sebuah request API masuk dengan header X-API-Key yang valid dan aktif, THE Auth_Middleware SHALL mengautentikasi request dan mencatat usage ke Redis
4. WHEN sebuah request API masuk dengan API key yang tidak valid, expired, atau non-aktif, THE Auth_Middleware SHALL menolak request dengan response 401 Unauthorized
5. WHEN seorang developer menonaktifkan API key, THE APIKey_Service SHALL mengubah status key menjadi non-aktif dan menolak semua request berikutnya yang menggunakan key tersebut
6. THE APIKey_Service SHALL menetapkan rate limit default 1000 requests per jam untuk setiap API key baru

### Requirement 4: Simulasi Payment Gateway

**User Story:** Sebagai developer, saya ingin membuat transaksi pembayaran melalui API, sehingga saya dapat mengintegrasikan simulasi pembayaran ke dalam aplikasi saya.

#### Acceptance Criteria

1. WHEN seorang developer mengirimkan request pembuatan transaksi dengan data yang valid (amount, payment_method, external_id), THE Transaction_Service SHALL membuat transaksi baru dengan status "pending" dan mengembalikan detail transaksi
2. WHEN seorang developer mengirimkan request pembuatan transaksi dengan amount yang tidak positif, THE Transaction_Service SHALL menolak request dan mengembalikan error validasi
3. WHEN seorang developer mengirimkan request pembuatan transaksi dengan external_id yang sudah ada, THE Transaction_Service SHALL menolak request dan mengembalikan error duplikasi
4. WHEN simulasi pembayaran selesai diproses, THE Transaction_Service SHALL memperbarui status transaksi menjadi "success" atau "failed" sesuai hasil simulasi
5. WHEN status transaksi berubah, THE Transaction_Service SHALL memicu pengiriman webhook notification ke endpoint yang terdaftar
6. WHEN seorang developer mengambil daftar transaksi, THE Transaction_Service SHALL mengembalikan hasil dengan pagination dan hanya menampilkan transaksi milik developer tersebut
7. THE Transaction_Service SHALL menyimpan amount dalam satuan terkecil mata uang (cents/rupiah) sebagai integer

### Requirement 5: Webhook Notification

**User Story:** Sebagai developer, saya ingin menerima notifikasi webhook ketika status transaksi berubah, sehingga aplikasi saya dapat merespons perubahan status secara real-time.

#### Acceptance Criteria

1. WHEN seorang developer mendaftarkan webhook endpoint, THE Webhook_Service SHALL menyimpan URL, secret untuk HMAC signature, dan daftar event yang di-subscribe
2. WHEN sebuah event terjadi (perubahan status transaksi), THE Webhook_Service SHALL membuat record webhook delivery dengan status "pending" dan memasukkannya ke delivery queue
3. WHEN Webhook_Worker memproses delivery, THE Webhook_Worker SHALL mengirimkan HTTP POST ke URL webhook dengan payload yang berisi detail event dan HMAC signature
4. WHEN webhook delivery berhasil (response 2xx), THE Webhook_Worker SHALL memperbarui status delivery menjadi "delivered" dan mencatat timestamp
5. IF webhook delivery gagal (response non-2xx atau timeout), THEN THE Webhook_Worker SHALL meningkatkan retry_count dan menjadwalkan retry dengan exponential backoff (1s, 2s, 4s, 8s, 16s)
6. IF webhook delivery telah mencapai max_retries (default 5), THEN THE Webhook_Worker SHALL memperbarui status delivery menjadi "failed" dan menghentikan retry
7. WHEN Webhook_Worker menerima signal shutdown, THE Webhook_Worker SHALL menyelesaikan delivery yang sedang diproses sebelum berhenti

### Requirement 6: Subscription Billing

**User Story:** Sebagai developer, saya ingin membuat produk dan subscription plan, sehingga end user dapat berlangganan layanan dengan billing otomatis.

#### Acceptance Criteria

1. WHEN seorang developer membuat produk baru, THE Subscription_Service SHALL menyimpan produk dengan nama, deskripsi, dan status aktif
2. WHEN seorang developer membuat plan untuk sebuah produk, THE Subscription_Service SHALL menyimpan plan dengan amount, currency, dan billing interval (monthly/yearly)
3. WHEN seorang end user membuat subscription baru, THE Subscription_Service SHALL membuat subscription dengan status "pending_payment", membuat transaksi pembayaran terkait, dan membuat invoice dengan status "unpaid"
4. WHEN pembayaran subscription berhasil, THE Subscription_Service SHALL memperbarui status subscription menjadi "active" dan memperbarui status invoice menjadi "paid"
5. WHEN seorang end user membatalkan subscription, THE Subscription_Service SHALL memperbarui status menjadi "cancelled" dan mencatat timestamp pembatalan
6. THE Subscription_Service SHALL menyimpan current_period_start dan current_period_end untuk setiap subscription aktif

### Requirement 7: Invoice Management

**User Story:** Sebagai developer, saya ingin melihat invoice untuk setiap transaksi dan subscription, sehingga saya dapat melacak riwayat pembayaran.

#### Acceptance Criteria

1. WHEN sebuah transaksi atau subscription dibuat, THE Invoice_Service SHALL membuat invoice dengan nomor unik berformat "INV-YYYYMMDD-XXXXX"
2. WHEN pembayaran berhasil, THE Invoice_Service SHALL memperbarui status invoice dari "unpaid" menjadi "paid" dan mencatat timestamp pembayaran
3. WHEN sebuah invoice dibatalkan, THE Invoice_Service SHALL memperbarui status menjadi "void"
4. WHEN seorang developer mengambil daftar invoice, THE Invoice_Service SHALL mengembalikan hasil dengan pagination dan hanya menampilkan invoice milik developer tersebut
5. THE Invoice_Service SHALL mengaitkan setiap invoice dengan Transaction atau Subscription sebagai referensi sumber

### Requirement 8: Rate Limiting

**User Story:** Sebagai platform, saya ingin membatasi jumlah request API per API key, sehingga sistem terlindungi dari abuse dan performa tetap terjaga.

#### Acceptance Criteria

1. WHEN sebuah request API masuk, THE Rate_Limiter SHALL memeriksa jumlah request dalam sliding window menggunakan Redis
2. WHEN jumlah request belum mencapai limit, THE Rate_Limiter SHALL mengizinkan request dan menambahkan header X-RateLimit-Limit, X-RateLimit-Remaining, dan X-RateLimit-Reset ke response
3. WHEN jumlah request telah mencapai limit, THE Rate_Limiter SHALL menolak request dengan response 429 Too Many Requests dan menyertakan header Retry-After
4. THE Rate_Limiter SHALL mengidentifikasi client berdasarkan API key atau IP address

### Requirement 9: Admin Dashboard

**User Story:** Sebagai admin, saya ingin melihat statistik sistem melalui dashboard, sehingga saya dapat memonitor performa dan aktivitas platform.

#### Acceptance Criteria

1. WHILE seorang user memiliki role admin, THE Admin_Service SHALL menyediakan akses ke statistik jumlah transaksi, user aktif, dan performa webhook
2. WHEN seorang admin mengakses endpoint statistik, THE Admin_Service SHALL mengembalikan data agregat yang mencakup total transaksi, jumlah user aktif, dan tingkat keberhasilan webhook delivery
3. WHEN seorang user tanpa role admin mengakses endpoint admin, THE Auth_Middleware SHALL menolak request dengan response 403 Forbidden

### Requirement 10: Database Transaction Management

**User Story:** Sebagai platform, saya ingin memastikan konsistensi data pada operasi yang melibatkan multiple database write, sehingga integritas data tetap terjaga.

#### Acceptance Criteria

1. WHEN sebuah operasi service melibatkan multiple database write, THE Transaction_Manager SHALL menjalankan semua operasi dalam satu database transaction
2. IF salah satu operasi dalam transaction gagal, THEN THE Transaction_Manager SHALL melakukan rollback seluruh operasi dalam transaction tersebut
3. IF terjadi panic dalam transaction, THEN THE Transaction_Manager SHALL melakukan recover dan rollback transaction
4. THE Transaction_Manager SHALL mendeteksi nested transaction dan mencegah double-begin
5. THE Transaction_Manager SHALL menyimpan transaction ke context sehingga repository dapat menggunakannya secara implisit

### Requirement 11: Validasi Input dan Keamanan Data

**User Story:** Sebagai platform, saya ingin memvalidasi semua input dan melindungi data sensitif, sehingga sistem aman dari serangan dan mematuhi standar keamanan.

#### Acceptance Criteria

1. THE Platform SHALL memvalidasi semua input request sebelum memproses business logic
2. THE Platform SHALL menyimpan password dalam bentuk bcrypt hash dan tidak pernah mengembalikan password dalam response
3. THE Platform SHALL menyimpan API key dalam bentuk hash dan hanya menampilkan full key sekali saat pembuatan
4. THE Platform SHALL menggunakan HMAC signature untuk verifikasi webhook payload
5. THE Platform SHALL tidak mencatat data sensitif (password, full API key, JWT token) dalam log aplikasi
6. IF sebuah request mengandung input yang tidak valid, THEN THE Platform SHALL mengembalikan error response yang deskriptif tanpa mengekspos detail internal sistem

### Requirement 12: Struktur Monorepo dan Clean Architecture

**User Story:** Sebagai developer platform, saya ingin kode terorganisir dengan clean architecture dalam struktur monorepo, sehingga sistem mudah di-maintain dan di-extend.

#### Acceptance Criteria

1. THE Platform SHALL memisahkan backend (Go + Fiber) dan frontend (Next.js + TypeScript) dalam direktori terpisah di root project
2. THE Platform SHALL mengimplementasikan clean architecture dengan layer: handler (HTTP request/response), service (business logic), dan repository (database operations)
3. THE Platform SHALL menggunakan DTO untuk request dan response di handler layer dan tidak mengekspos domain model secara langsung ke client
4. THE Platform SHALL mempropagasi context dari handler ke service ke repository untuk request_id dan tracing data
5. THE Platform SHALL menyediakan database migration files dengan format SQL yang memiliki fungsi up dan down yang reversible
