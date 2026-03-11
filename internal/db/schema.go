package db

// Schema contains SQL statements for creating the database schema.
// These statements are used by integration tests to set up the database.
// Based on the design spec in docs/design/d04.md and docs/design/d05.md.

const (
	// SchemaMigrations creates the schema_migrations table.
	SchemaMigrations = `
CREATE TABLE IF NOT EXISTS schema_migrations (
	version text PRIMARY KEY,
	applied_at timestamptz NOT NULL DEFAULT now()
);`

	// SchemaJobState creates the job_state enum type.
	SchemaJobState = `
CREATE TYPE job_state AS ENUM ('queued','running','succeeded','failed','jobs_exhausted','canceled');`

	// SchemaOutboxKind creates the outbox_kind enum type.
	SchemaOutboxKind = `
CREATE TYPE outbox_kind AS ENUM ('webhook','event','internal');`

	// SchemaEffectKind creates the effect_kind enum type.
	SchemaEffectKind = `
CREATE TYPE effect_kind AS ENUM ('connector_call');`

	// SchemaEffectStatus creates the effect_status enum type.
	SchemaEffectStatus = `
CREATE TYPE effect_status AS ENUM (
	'pending',
	'reserved',
	'in_flight',
	'succeeded',
	'failed',
	'canceled',
	'unknown_outcome',
	'dlq'
);`

	// SchemaStepStatus creates the step_status enum type.
	SchemaStepStatus = `
CREATE TYPE step_status AS ENUM ('ok', 'error', 'skipped', 'waiting_approval');`

	// SchemaApprovalDecision creates the approval_decision enum type.
	SchemaApprovalDecision = `
CREATE TYPE approval_decision AS ENUM ('approved', 'rejected', 'expired');`

	// SchemaJobs creates the jobs table.
	// Based on design spec §12.1
	SchemaJobs = `
CREATE TABLE IF NOT EXISTS jobs (
	id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
	tenant_id text NOT NULL,
	path_key text NOT NULL,
	idempotency_key text NOT NULL,
	params_json jsonb NOT NULL,
	state job_state NOT NULL DEFAULT 'queued',
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
	updated_at timestamptz NOT NULL DEFAULT now(),
	CONSTRAINT jobs_attempts_ck CHECK (attempt >= 0 AND max_attempts >= 1),
	CONSTRAINT idem_key_format_ck CHECK (
		char_length(idempotency_key) BETWEEN 1 AND 128
		AND idempotency_key ~ '^[a-zA-Z0-9\-_.]+$'
	)
);`

	// SchemaJobsIdemUidx creates unique index on idempotency key.
	SchemaJobsIdemUidx = `
CREATE UNIQUE INDEX IF NOT EXISTS jobs_tenant_idem_uidx
ON jobs (tenant_id, idempotency_key);`

	// SchemaJobsDueIdx creates partial index for due jobs.
	SchemaJobsDueIdx = `
CREATE INDEX IF NOT EXISTS jobs_due_idx
ON jobs (tenant_id, next_run_at)
WHERE state = 'queued';`

	// SchemaJobsExpiredRunningIdx creates partial index for expired running jobs.
	SchemaJobsExpiredRunningIdx = `
CREATE INDEX IF NOT EXISTS jobs_expired_running_idx
ON jobs (tenant_id, lease_expires_at)
WHERE state = 'running';`

	// SchemaJobsTenantIdIdx creates index on tenant_id.
	SchemaJobsTenantIdIdx = `
CREATE INDEX IF NOT EXISTS jobs_tenant_id_idx ON jobs (tenant_id, id);`

	// SchemaJobsRead creates the jobs_read table (read-optimized projection).
	// Based on design spec §12.2
	SchemaJobsRead = `
CREATE TABLE IF NOT EXISTS jobs_read (
	id uuid PRIMARY KEY,
	tenant_id text NOT NULL,
	path_key text NOT NULL,
	state job_state NOT NULL,
	attempt int NOT NULL,
	max_attempts int NOT NULL,
	owner_id text,
	run_id uuid NOT NULL,
	lease_expires_at timestamptz,
	next_run_at timestamptz,
	accepted_at timestamptz,
	last_error_code text,
	last_error_message text,
	started_at timestamptz,
	finished_at timestamptz,
	created_at timestamptz NOT NULL,
	updated_at timestamptz NOT NULL
);`

	// SchemaJobsReadTenantIdx creates index on tenant_id.
	SchemaJobsReadTenantIdx = `
CREATE INDEX IF NOT EXISTS jobs_read_tenant_idx ON jobs_read (tenant_id, id);`

	// SchemaOutbox creates the outbox table.
	// Based on design spec §12.3
	SchemaOutbox = `
CREATE TABLE IF NOT EXISTS outbox (
	id bigserial PRIMARY KEY,
	tenant_id text NOT NULL,
	job_id uuid,
	kind outbox_kind NOT NULL,
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

	// SchemaOutboxDueIdx creates partial index for due outbox entries.
	SchemaOutboxDueIdx = `
CREATE INDEX IF NOT EXISTS outbox_due_idx ON outbox (tenant_id, next_run_at) WHERE delivery_state = 'pending';`

	// SchemaExternalEffects creates the external_effects table.
	// Based on design spec §12.5
	SchemaExternalEffects = `
CREATE TABLE IF NOT EXISTS external_effects (
	effect_id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
	kind effect_kind NOT NULL DEFAULT 'connector_call',
	job_id uuid NOT NULL,
	run_id uuid NOT NULL,
	step_key text NOT NULL,
	connector_key text,
	operation text NOT NULL,
	input_digest bytea NOT NULL,
	call_id text NOT NULL,
	status effect_status NOT NULL DEFAULT 'pending',
	attempt int NOT NULL DEFAULT 0,
	max_attempts int NOT NULL DEFAULT 3,
	reconcile_attempts int NOT NULL DEFAULT 0,
	max_reconcile_attempts int NOT NULL DEFAULT 5,
	owner_id text,
	lease_expires_at timestamptz,
	next_run_at timestamptz,
	provider_ref text,
	last_error_code text,
	last_error_message text,
	created_at timestamptz NOT NULL DEFAULT now(),
	updated_at timestamptz NOT NULL DEFAULT now(),
	UNIQUE (job_id, run_id, step_key, operation, input_digest),
	UNIQUE (call_id)
);`

	// SchemaExternalEffectsReservedIdx creates index for reserved/unknown_outcome effects.
	SchemaExternalEffectsReservedIdx = `
CREATE INDEX IF NOT EXISTS external_effects_reserved_idx
ON external_effects (status, next_run_at) WHERE status IN ('reserved','unknown_outcome');`

	// SchemaExternalEffectsInFlightIdx creates index for in_flight effects.
	SchemaExternalEffectsInFlightIdx = `
CREATE INDEX IF NOT EXISTS external_effects_in_flight_idx
ON external_effects (status, lease_expires_at) WHERE status = 'in_flight';`

	// SchemaExternalEffectsDlqIdx creates index for dlq effects (alert scanning).
	SchemaExternalEffectsDlqIdx = `
CREATE INDEX IF NOT EXISTS external_effects_dlq_idx ON external_effects (status) WHERE status = 'dlq';`

	// SchemaSteps creates the steps table.
	// Based on design spec §12.6
	SchemaSteps = `
CREATE TABLE IF NOT EXISTS steps (
	id bigserial PRIMARY KEY,
	job_id uuid NOT NULL,
	run_id uuid NOT NULL,
	attempt int NOT NULL,
	step_key text NOT NULL,
	status step_status NOT NULL,
	output_json jsonb,
	error_code text,
	error_message text,
	started_at timestamptz NOT NULL DEFAULT now(),
	finished_at timestamptz,
	duration_ms int,
	approval_request jsonb,
	approval_ttl interval,
	approval_expires_at timestamptz,
	UNIQUE (job_id, attempt, step_key)
);`

	// SchemaStepsJobAttemptIdx creates index for step resumption on retry.
	SchemaStepsJobAttemptIdx = `
CREATE INDEX IF NOT EXISTS steps_job_attempt_idx ON steps (job_id, attempt);`

	// SchemaApprovalDecisions creates the approval_decisions table.
	SchemaApprovalDecisions = `
CREATE TABLE IF NOT EXISTS approval_decisions (
	id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
	job_id uuid NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
	step_id bigint NOT NULL REFERENCES steps(id) ON DELETE CASCADE,
	run_id uuid NOT NULL,
	decision approval_decision NOT NULL,
	approver text NOT NULL,
	justification text,
	policy_snapshot jsonb NOT NULL,
	idempotency_key text NOT NULL,
	created_at timestamptz NOT NULL DEFAULT now(),
	CONSTRAINT uq_approval_per_approver UNIQUE (step_id, approver),
	CONSTRAINT uq_idempotency UNIQUE (idempotency_key)
);`

	SchemaApprovalDecisionsStepIdx = `CREATE INDEX IF NOT EXISTS idx_approval_decisions_step ON approval_decisions(step_id);`
	SchemaApprovalDecisionsJobIdx  = `CREATE INDEX IF NOT EXISTS idx_approval_decisions_job ON approval_decisions(job_id);`

	// SchemaJobsPendingApprovals creates the jobs_with_pending_approvals view.
	SchemaJobsWithPendingApprovals = `
CREATE OR REPLACE VIEW jobs_with_pending_approvals AS
SELECT DISTINCT j.*
FROM jobs j
JOIN steps s ON s.job_id = j.id
WHERE s.status = 'waiting_approval';`

	// SchemaJobContext creates the job_context table.
	// Based on design spec §12.7
	SchemaJobContext = `
CREATE TABLE IF NOT EXISTS job_context (
	job_id uuid NOT NULL,
	key text NOT NULL,
	value_json jsonb NOT NULL,
	run_id uuid NOT NULL,
	updated_at timestamptz NOT NULL DEFAULT now(),
	PRIMARY KEY (job_id, key)
);`

	// SchemaJobContextJobIdx creates index on job_id.
	SchemaJobContextJobIdx = `
CREATE INDEX IF NOT EXISTS job_context_job_idx ON job_context (job_id);`

	// SchemaConnectorCallLog creates the connector_call_log table.
	// Based on design spec §12.8
	SchemaConnectorCallLog = `
CREATE TABLE IF NOT EXISTS connector_call_log (
	id bigserial,
	ts timestamptz NOT NULL DEFAULT now(),
	effect_id uuid NOT NULL,
	call_id text NOT NULL,
	provider text NOT NULL,
	operation text NOT NULL,
	request_ms int,
	outcome text NOT NULL,
	http_status int,
	provider_ref text,
	error_code text,
	error_message text
);`

	// SchemaConnectorCallLogCallIdIdx creates index on call_id.
	SchemaConnectorCallLogCallIdIdx = `
CREATE INDEX IF NOT EXISTS connector_call_log_call_id_idx ON connector_call_log (call_id, ts);`

	// SchemaConnectorCallLogEffectIdIdx creates index on effect_id.
	SchemaConnectorCallLogEffectIdIdx = `
CREATE INDEX IF NOT EXISTS connector_call_log_effect_id_idx ON connector_call_log (effect_id, ts);`

	// SchemaIdempotencyKeys creates the idempotency_keys table.
	// Based on design spec §12.9
	SchemaIdempotencyKeys = `
CREATE TABLE IF NOT EXISTS idempotency_keys (
	id bigserial PRIMARY KEY,
	tenant_id text NOT NULL,
	path_key text NOT NULL,
	idempotency_key text NOT NULL,
	payload_fingerprint bytea NOT NULL,
	status_code int NOT NULL,
	response_body_json jsonb NOT NULL,
	job_id uuid NOT NULL,
	accepted_at timestamptz NOT NULL DEFAULT now(),
	expires_at timestamptz NOT NULL DEFAULT now() + interval '48 hours',
	created_at timestamptz NOT NULL DEFAULT now(),
	updated_at timestamptz NOT NULL DEFAULT now(),
	UNIQUE (tenant_id, path_key, idempotency_key),
	CONSTRAINT idem_key_format_ck CHECK (
		char_length(idempotency_key) BETWEEN 1 AND 128
		AND idempotency_key ~ '^[a-zA-Z0-9\-_.]+$'
	)
);`

	// SchemaIdempotencyKeysExpiresAtIdx creates index for cleanup.
	SchemaIdempotencyKeysExpiresAtIdx = `
CREATE INDEX IF NOT EXISTS idempotency_keys_expires_at_idx ON idempotency_keys (expires_at);`

	// SchemaMetricsJobEvents creates the metrics_job_events table for TimescaleDB.
	// Based on design spec §12.4
	SchemaMetricsJobEvents = `
CREATE TABLE IF NOT EXISTS metrics_job_events (
	ts timestamptz NOT NULL DEFAULT now(),
	tenant_id text NOT NULL,
	job_id uuid NOT NULL,
	path_key text NOT NULL,
	event text NOT NULL,
	attempt int NOT NULL,
	run_id uuid NOT NULL,
	code text,
	duration_ms int,
	labels jsonb DEFAULT '{}'::jsonb
);`

	// SchemaAuditLog creates the audit_log table.
	// Used by AuditService
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

	// SchemaBackoffInterval creates the backoff_interval helper function.
	SchemaBackoffInterval = `
CREATE OR REPLACE FUNCTION backoff_interval(attempt int, policy jsonb)
RETURNS interval LANGUAGE sql AS $$
	SELECT make_interval(secs => LEAST(
		coalesce((policy->>'base_seconds')::int, 2) * (2 ^ GREATEST(attempt-1,0)),
		coalesce((policy->>'max_seconds')::int, 300)))
$$;`

	// SchemaEffectLease creates the effect_lease helper function.
	SchemaEffectLease = `
CREATE OR REPLACE FUNCTION effect_lease(_effect_id uuid, _owner text, _ttl_secs int)
RETURNS boolean LANGUAGE plpgsql AS $$
DECLARE
	_updated int;
BEGIN
	UPDATE external_effects
		SET status = 'in_flight',
			owner_id = _owner,
			lease_expires_at = now() + make_interval(secs => _ttl_secs),
			updated_at = now()
		WHERE effect_id = _effect_id
			AND status = 'reserved';
	GET DIAGNOSTICS _updated = ROW_COUNT;
	RETURN _updated = 1;
END;
$$;`

	// SchemaEffectBackoff creates the effect_backoff helper function.
	SchemaEffectBackoff = `
CREATE OR REPLACE FUNCTION effect_backoff(_effect_id uuid, _policy jsonb, _code text, _msg text)
RETURNS void LANGUAGE plpgsql AS $$
BEGIN
	UPDATE external_effects
		SET status = 'reserved',
			attempt = attempt + 1,
			owner_id = NULL,
			lease_expires_at = NULL,
			next_run_at = now() + backoff_interval(attempt, _policy),
			last_error_code = _code,
			last_error_message = _msg,
			updated_at = now()
		WHERE effect_id = _effect_id
			AND status = 'in_flight';
END;
$$;`

	// SchemaEffectEscalateDlq creates the effect_escalate_dlq helper function.
	SchemaEffectEscalateDlq = `
CREATE OR REPLACE FUNCTION effect_escalate_dlq(_effect_id uuid)
RETURNS void LANGUAGE plpgsql AS $$
BEGIN
	UPDATE external_effects
		SET reconcile_attempts = reconcile_attempts + 1,
			status = CASE
						WHEN reconcile_attempts + 1 >= max_reconcile_attempts THEN 'dlq'
						ELSE 'unknown_outcome'
					END,
			next_run_at = CASE
							WHEN reconcile_attempts + 1 >= max_reconcile_attempts THEN NULL
							ELSE now() + interval '5 minutes'
						END,
			updated_at = now()
		WHERE effect_id = _effect_id
			AND status = 'unknown_outcome';
END;
$$;`
)

// SchemaAll returns all schema statements needed for testing.
// Order matters: types first, then tables, then indexes, then functions.
var SchemaAll = []string{
	// Types
	SchemaJobState,
	SchemaOutboxKind,
	SchemaEffectKind,
	SchemaEffectStatus,
	SchemaStepStatus,
	SchemaApprovalDecision,
	// Core tables
	SchemaMigrations,
	SchemaJobs,
	SchemaJobsRead,
	SchemaOutbox,
	SchemaExternalEffects,
	SchemaSteps,
	SchemaApprovalDecisions,
	SchemaJobsWithPendingApprovals,
	SchemaJobContext,
	SchemaConnectorCallLog,
	SchemaIdempotencyKeys,
	SchemaMetricsJobEvents,
	SchemaAuditLog,
	// Indexes
	SchemaJobsIdemUidx,
	SchemaJobsDueIdx,
	SchemaJobsExpiredRunningIdx,
	SchemaJobsTenantIdIdx,
	SchemaJobsReadTenantIdx,
	SchemaOutboxDueIdx,
	SchemaExternalEffectsReservedIdx,
	SchemaExternalEffectsInFlightIdx,
	SchemaExternalEffectsDlqIdx,
	SchemaStepsJobAttemptIdx,
	SchemaApprovalDecisionsStepIdx,
	SchemaApprovalDecisionsJobIdx,
	SchemaJobContextJobIdx,
	SchemaConnectorCallLogCallIdIdx,
	SchemaConnectorCallLogEffectIdIdx,
	SchemaIdempotencyKeysExpiresAtIdx,
	// Helper functions
	SchemaBackoffInterval,
	SchemaEffectLease,
	SchemaEffectBackoff,
	SchemaEffectEscalateDlq,
}
