-- Plain DATETIME has 1-second resolution. The offline-sync/progress-update
-- endpoints (Server/internal/modules/reading) compare client_updated_at with
-- `>=` to decide which write wins — two updates landing in the same second
-- get equal, second-truncated timestamps, and the tie-break favors the OLD
-- row, silently dropping a legitimate newer update. DATETIME(6) (microsecond
-- precision) makes real collisions practically impossible for this use case.
ALTER TABLE reading_sessions
    MODIFY COLUMN client_updated_at DATETIME(6);
