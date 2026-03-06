package db

// Schema contains SQL statements for creating the database schema.
// These statements are used by integration tests to set up the database.

const (
	// SchemaMetricsJobEvents creates the metrics_job_events table for TimescaleDB.
	// This schema matches the INSERT statement in metrics_sql_writer.go
	SchemaMetricsJobEvents = `
CREATE TABLE IF NOT EXISTS metrics_job_events (
	id SERIAL PRIMARY KEY,
	tenant_id text NOT NULL,
	job_id text NOT NULL,
	run_id text NOT NULL,
	event_type text NOT NULL,
	timestamp timestamptz NOT NULL,
	state text,
	duration_ms bigint,
	error_code text,
	error_message text,
	metadata text,
	created_at timestamptz NOT NULL DEFAULT now()
);`

	// SchemaJobs creates the jobs table.
	SchemaJobs = `
CREATE TABLE IF NOT EXISTS jobs (
	id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
	tenant_id text NOT NULL,
	path_key text NOT NULL,
	idempotency_key text NOT NULL,
	params_json jsonb NOT NULL,
	state text NOT NULL DEFAULT 'queued',
	attempt int NOT NULL DEFAULT 0,
	max_attempts int NOT NULL DEFAULT 3,
	backoff_policy jsonb NOT NULL DEFAULT '{}'::jsonb,
	owner_id text,
	run_id uuid NOT NULL DEFAULT gen_random_uuid(),
	lease_expires_at timestamptz,
	next_run_at timestamptz,
	accepted_at timestamptz NOT NULL DEFAULT now(),
	cancel_requested_at timestamptz,
	cancel_reason text,
	last_error_code text,
	last_error_message text,
	created_by text,
	created_at timestamptz NOT NULL DEFAULT now(),
	started_at timestamptz,
	finished_at timestamptz,
	updated_at timestamptz NOT NULL DEFAULT now()
);`

	// SchemaOutbox creates the outbox table.
	SchemaOutbox = `
CREATE TABLE IF NOT EXISTS outbox (
	id bigserial PRIMARY KEY,
	tenant_id text NOT NULL,
	job_id uuid,
	kind text NOT NULL,
	dedupe_key text,
	payload_json jsonb NOT NULL,
	delivery_state text NOT NULL DEFAULT 'pending',
	attempt int NOT NULL DEFAULT 0,
	max_attempts int NOT NULL DEFAULT 12,
	next_run_at timestamptz,
	last_error text,
	created_at timestamptz NOT NULL DEFAULT now(),
	updated_at timestamptz NOT NULL DEFAULT now()
);`

	// SchemaAuditLog creates the audit_log table.
	SchemaAuditLog = `
CREATE TABLE IF NOT EXISTS audit_log (
	id bigserial PRIMARY KEY,
	ts timestamptz NOT NULL DEFAULT now(),
	tenant_id text NOT NULL,
	principal text NOT NULL,
	action text NOT NULL,
	target text NOT NULL,
	result text NOT NULL,
	details jsonb DEFAULT '{}'::jsonb,
	ip_address text,
	user_agent text,
	created_at timestamptz NOT NULL DEFAULT now()
);`

	// SchemaMigrations creates the schema_migrations table.
	SchemaMigrations = `
CREATE TABLE IF NOT EXISTS schema_migrations (
	version text PRIMARY KEY,
	applied_at timestamptz NOT NULL DEFAULT now()
);`
)

// SchemaAll returns all schema statements needed for testing.
var SchemaAll = []string{
	SchemaMigrations,
	SchemaJobs,
	SchemaOutbox,
	SchemaMetricsJobEvents,
	SchemaAuditLog,
}
