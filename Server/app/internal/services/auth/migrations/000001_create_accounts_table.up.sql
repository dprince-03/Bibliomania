-- auth-service's copy of identity data: exactly what login needs (email,
-- password hash, role, is_active) plus the display name returned in the
-- auth response. Deliberately duplicated from user-service's `users` table
-- rather than looked up there on every login — login must not depend on
-- another service being up (Server/app/docs/plan.md, "Microservices split").
--
-- Single writer per column: email/password/names are written here (at
-- registration) and replicated to user-service via auth.user_registered;
-- is_active is written by user-service (admin action) and replicated here
-- via user.status_changed.
CREATE TABLE IF NOT EXISTS accounts (
    id         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    first_name VARCHAR(225)    NOT NULL,
    last_name  VARCHAR(225)    NOT NULL,
    email      VARCHAR(225)    NOT NULL UNIQUE,
    password   VARCHAR(225)    NOT NULL,
    role       ENUM('admin', 'librarian', 'member') NOT NULL DEFAULT 'member',
    is_active  BOOLEAN         NOT NULL DEFAULT TRUE,
    created_at DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_accounts_role (role)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
