-- Recoverable business commands keep their immutable request server-side so
-- browser storage only needs the opaque command id. The body is limited by the
-- API middleware and is never returned until a stale reservation is eligible
-- for an authenticated takeover.
ALTER TABLE idempotency_record
ADD COLUMN request_body BYTEA;

COMMENT ON COLUMN idempotency_record.request_body IS
'Immutable JSON request for closed recoverable commands; NULL for ordinary idempotent routes.';
