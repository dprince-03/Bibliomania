> Covers the env vars and schema as they exist today (the 10-table schema
> from Steps 1-20). New tables/modules planned under the platform-vision
> repositioning (`Library`/`Branch`, billing, curation, categories, etc.)
> aren't reflected below yet — see [`Server/app/docs/plan.md`](plan.md).

# .env :
```
# Server
APP_ENV=development
SERVER_PORT=8080
SERVER_READ_TIMEOUT=10
SERVER_WRITE_TIMEOUT=10

# Database (MySQL)
DB_HOST=localhost
DB_PORT=3306
DB_USER=root
DB_PASSWORD=yourpassword
DB_NAME=bibliomania

# Redis
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=
REDIS_DB=0

# JWT
JWT_SECRET=your-secret-here
JWT_ACCESS_TOKEN_TTL=15      # minutes
JWT_REFRESH_TOKEN_TTL=10080  # minutes (7 days)

# Storage (E-Library uploads)
STORAGE_PATH=./storage
MAX_UPLOAD_SIZE_MB=50

# Rate Limiting
RATE_LIMIT_RPS=100       # requests per second
RATE_LIMIT_BURST=200
```

# migration
## create migration:
- migrate create -ext sql -dir migrations -seq your_change_description
```
migrate create -ext sql -dir migrations -seq create_users_table
migrate create -ext sql -dir migrations -seq create_authors_table
migrate create -ext sql -dir migrations -seq create_books_table
migrate create -ext sql -dir migrations -seq create_refresh_tokens_table
migrate create -ext sql -dir migrations -seq create_users_profile_table
migrate create -ext sql -dir migrations -seq create_borrows_records_table
migrate create -ext sql -dir migrations -seq create_book_authors_table
migrate create -ext sql -dir migrations -seq create_reading_sessions_table
migrate create -ext sql -dir migrations -seq create_user_library_table
migrate create -ext sql -dir migrations -seq create_bookmarks_table
```

## structure (actual, as of 2026-08-23) :
migrations/
├── 000001_create_users_table.{up,down}.sql
├── 000002_create_authors_table.{up,down}.sql
├── 000003_create_books_table.{up,down}.sql
├── 000004_create_refresh_tokens_table.{up,down}.sql
├── 000005_create_users_profile_table.{up,down}.sql
├── 000006_create_borrows_records_table.{up,down}.sql
├── 000007_create_book_authors_table.{up,down}.sql
├── 000008_create_reading_sessions_table.{up,down}.sql
├── 000009_create_user_library_table.{up,down}.sql
└── 000010_create_bookmarks_table.{up,down}.sql

### Migration File Order
```
000001_create_users_table
000002_create_authors_table
000003_create_books_table             ← runs before profile/borrows, unlike the original plan below
000004_create_refresh_tokens_table
000005_create_users_profile_table
000006_create_borrows_records_table
000007_create_book_authors_table      (junction table)
000008_create_reading_sessions_table
000009_create_user_library_table
000010_create_bookmarks_table
```

> Note: an earlier draft of this doc numbered these 003=profile, 004=books,
> 005=borrows, 006=refresh_tokens. The files actually on disk use the order
> above — trust the filenames in `Server/app/migrations/`, not the numbers in
> old notes.

## Complete Database Schema (10 tables)
```
users
  └── users_profile        (1:1)
  └── refresh_tokens        (1:many)
  └── borrows_records       (1:many)
  └── reading_sessions      (1:many)
  └── user_library          (1:many)
  └── bookmarks             (1:many)

books
  └── book_authors          (many:many junction)
  └── borrows_records       (1:many)
  └── reading_sessions      (1:many)
  └── user_library          (1:many)
  └── bookmarks             (1:many)

authors
  └── book_authors          (many:many junction)
```

migrate -path ./migrations -database "mysql://user:pass@tcp(localhost:3306)/bibliomania" up
