# Bibliotheca

📚 Bibliotheca    
- A Library Management &amp; E-Library System    
- Built with Go, MySQL, Redis, Docker

```
Bibliotheca/
├── Frontend/                          # (later)
│
└── Backend/
    ├── cmd/
    │   ├── api/
    │   │   └── main.go                # ✅ Entry point (updated each step)
    │   └── keygen/
    │       └── main.go                # ✅ JWT secret generator CLI
    │
    ├── internal/
    │   ├── auth/                      # Step 7  — Register/Login/Logout flows
    │   ├── cache/
    │   │   ├── cache.go               # ✅ Cache interface
    │   │   ├── redis_cache.go         # ✅ Redis implementation
    │   │   ├── errors.go              # ✅ ErrCacheMiss
    │   │   └── keys.go                # ✅ Centralized cache keys & TTLs
    │   ├── config/
    │   │   └── config.go              # ✅ Env loading & validation
    │   ├── database/
    │   │   └── migrate.go             # ✅ Migration runner
    │   ├── dto/                       # Step 5  — Request/Response structs
    │   │   ├── auth.go
    │   │   ├── book.go
    │   │   ├── author.go
    │   │   ├── borrow.go
    │   │   ├── user.go
    │   │   └── reading.go
    │   ├── errors/                    # Step 5  — Custom error types
    │   │   └── errors.go
    │   ├── handlers/                  # Step 11+ — HTTP handlers
    │   │   ├── auth.go
    │   │   ├── book.go
    │   │   ├── author.go
    │   │   ├── borrow.go
    │   │   ├── user.go
    │   │   └── reading.go
    │   ├── middleware/                # Step 8-10 — All middleware
    │   │   ├── auth.go
    │   │   ├── cors.go
    │   │   ├── logger.go
    │   │   ├── rate_limiter.go
    │   │   ├── rbac.go
    │   │   └── recovery.go
    │   ├── models/                    # Step 5  — DB structs
    │   │   ├── user.go
    │   │   ├── author.go
    │   │   ├── book.go
    │   │   ├── borrow.go
    │   │   ├── reading.go
    │   │   └── token.go
    │   ├── repository/                # Step 6  — Raw DB queries
    │   │   ├── user.go
    │   │   ├── author.go
    │   │   ├── book.go
    │   │   ├── borrow.go
    │   │   ├── reading.go
    │   │   └── token.go
    │   ├── services/                  # Step 11+ — Business logic
    │   │   ├── auth.go
    │   │   ├── book.go
    │   │   ├── author.go
    │   │   ├── borrow.go
    │   │   ├── user.go
    │   │   └── reading.go
    │   └── utils/                     # Step 5  — Helpers
    │       ├── response.go
    │       ├── pagination.go
    │       ├── file.go
    │       └── hash.go
    │
    ├── pkg/
    │   ├── jwt/                       # Step 7  — JWT generate/parse/validate
    │   │   └── jwt.go
    │   ├── mysqlclient/
    │   │   └── mysqlclient.go         # ✅ MySQL connection + pooling
    │   ├── redisclient/
    │   │   └── redisclient.go         # ✅ Redis connection + pooling
    │   └── refreshtoken/              # Step 7  — Refresh token helpers
    │       └── refreshtoken.go
    │
    ├── migrations/
    │   ├── 000001_create_users_table.up.sql                  # ✅
    │   ├── 000001_create_users_table.down.sql                # ✅
    │   ├── 000002_create_authors_table.up.sql                # ✅
    │   ├── 000002_create_authors_table.down.sql              # ✅
    │   ├── 000003_create_users_profile_table.up.sql          # ✅ updated
    │   ├── 000003_create_users_profile_table.down.sql        # ✅
    │   ├── 000004_create_books_table.up.sql                  # ✅ updated
    │   ├── 000004_create_books_table.down.sql                # ✅
    │   ├── 000005_create_borrows_records_table.up.sql        # ✅
    │   ├── 000005_create_borrows_records_table.down.sql      # ✅
    │   ├── 000006_create_refresh_tokens_table.up.sql         # ✅ fixed
    │   ├── 000006_create_refresh_tokens_table.down.sql       # ✅
    │   ├── 000007_create_book_authors_table.up.sql           # ✅ NEW
    │   ├── 000007_create_book_authors_table.down.sql         # ✅ NEW
    │   ├── 000008_create_reading_sessions_table.up.sql       # ✅ NEW
    │   ├── 000008_create_reading_sessions_table.down.sql     # ✅ NEW
    │   ├── 000009_create_user_library_table.up.sql           # ✅ NEW
    │   ├── 000009_create_user_library_table.down.sql         # ✅ NEW
    │   ├── 000010_create_bookmarks_table.up.sql              # ✅ NEW
    │   └── 000010_create_bookmarks_table.down.sql            # ✅ NEW
    │
    ├── storage/
    │   └── books/                     # Step 13 — Uploaded PDFs/EPUBs
    │
    ├── docs/                          # Step 16 — Swagger generated files
    │
    ├── .air.toml                      # ✅ Hot reload config
    ├── .env                           # ✅ Local secrets (gitignored)
    ├── .env.example                   # ✅ Committed template
    ├── Dockerfile.dev                 # ✅
    ├── Dockerfile.prod                # ✅
    ├── docker-compose.yml             # ✅
    └── Makefile                       # Step 17
```