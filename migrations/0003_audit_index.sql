-- migrations/0003_audit_index.sql
-- Index events.kind to support fast queries for audit-log lookups
-- ("http_request") and existing kind-filtered queries.

-- +goose Up
CREATE INDEX idx_events_kind ON events(kind);

-- +goose Down
DROP INDEX idx_events_kind;
