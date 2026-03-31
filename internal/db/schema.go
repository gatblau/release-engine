// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

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
DO $$ BEGIN
    CREATE TYPE job_state AS ENUM ('queued','running','succeeded','failed','jobs_exhausted','canceled');
EXCEPTION
    WHEN duplicate_object THEN NULL;
END $$;`

	// SchemaOutboxKind creates the outbox_kind enum type.
	SchemaOutboxKind = `
DO $$ BEGIN
    CREATE TYPE outbox_kind AS ENUM ('webhook','event','internal');
EXCEPTION
    WHEN duplicate_object THEN NULL;
END $$;`

	// SchemaEffectKind creates the effect_kind enum type.
	SchemaEffectKind = `
DO $$ BEGIN
    CREATE TYPE effect_kind AS ENUM ('connector_call');
EXCEPTION
    WHEN duplicate_object THEN NULL;
END $$;`

	// SchemaEffectStatus creates the effect_status enum type.
	SchemaEffectStatus = `
DO $$ BEGIN
    CREATE TYPE effect_status AS ENUM (
	'pending',
	'reserved',
	'in_flight',
	'succeeded',
	'failed',
	'canceled',
	'unknown_outcome',
	'dlq'
    );
EXCEPTION
    WHEN duplicate_object THEN NULL;
END $$;`

	// SchemaStepStatus creates the step_status enum type.
	SchemaStepStatus = `
DO $$ BEGIN
    CREATE TYPE step_status AS ENUM ('ok', 'error', 'skipped', 'waiting_approval');
EXCEPTION
    WHEN duplicate_object THEN NULL;
END $$;`

	// SchemaApprovalDecision creates the approval_decision enum type.
	SchemaApprovalDecision = `
DO $$ BEGIN
    CREATE TYPE approval_decision AS ENUM ('approved', 'rejected', 'expired');
EXCEPTION
    WHEN duplicate_object THEN NULL;
END $$;`

	// SchemaJobs creates the jobs table.
	// Based on design spec §12.1
	SchemaJobs = `
CREATE TABLE IF NOT EXISTS jobs (
	id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
	tenant_id text NOT NULL,
	path_key text NOT NULL,
	idempotency_key text NOT NULL,
	params_json jsonb NOT NULL,
	schedule text,
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
	schedule text,
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

	// SchemaDoraEvents creates the dora_events table.
	// Based on design spec d10.md §44.
	SchemaDoraEvents = `
CREATE TABLE IF NOT EXISTS dora_events (
	id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
	tenant_id text NOT NULL,
	event_type text NOT NULL,
	event_source text NOT NULL,
	service_ref text,
	environment text,
	correlation_key text,
	source_event_id text,
	event_timestamp timestamptz NOT NULL,
	ingested_at timestamptz NOT NULL DEFAULT now(),
	payload jsonb NOT NULL DEFAULT '{}'::jsonb,
	CONSTRAINT dora_events_type_check CHECK (
		event_type IN (
			'deployment.succeeded',
			'deployment.failed',
			'commit.pushed',
			'commit.merged',
			'incident.opened',
			'incident.resolved',
			'rollback.completed'
		)
	)
);`

	SchemaDoraEventsTenantServiceIdx = `
CREATE INDEX IF NOT EXISTS idx_dora_events_tenant_service
ON dora_events (tenant_id, service_ref, event_timestamp DESC);`

	SchemaDoraEventsCorrelationIdx = `
CREATE INDEX IF NOT EXISTS idx_dora_events_correlation
ON dora_events (tenant_id, correlation_key)
WHERE correlation_key IS NOT NULL;`

	SchemaDoraEventsSourceEventUidx = `
CREATE UNIQUE INDEX IF NOT EXISTS uq_dora_events_source_event
ON dora_events (tenant_id, event_source, source_event_id)
WHERE source_event_id IS NOT NULL;`

	SchemaDoraEventsDeployDedupeUidx = `
CREATE UNIQUE INDEX IF NOT EXISTS uq_dora_events_deploy_dedupe
ON dora_events (tenant_id, service_ref, environment, event_type, correlation_key)
WHERE correlation_key IS NOT NULL
AND event_type IN ('deployment.succeeded', 'deployment.failed');`

	// SchemaDoraCommitDeploymentLinks creates the commit-deployment links table.
	SchemaDoraCommitDeploymentLinks = `
CREATE TABLE IF NOT EXISTS dora_commit_deployment_links (
	tenant_id text NOT NULL,
	service_ref text NOT NULL,
	commit_sha text NOT NULL,
	deployment_id uuid NOT NULL,
	deployment_outcome text NOT NULL,
	deployment_time timestamptz NOT NULL,
	created_at timestamptz NOT NULL DEFAULT now(),
	CONSTRAINT chk_dora_links_outcome CHECK (deployment_outcome IN ('succeeded', 'failed')),
	CONSTRAINT fk_dora_links_deployment FOREIGN KEY (deployment_id) REFERENCES dora_events(id) ON DELETE CASCADE,
	PRIMARY KEY (tenant_id, service_ref, commit_sha, deployment_id)
);`

	SchemaDoraCommitDeploymentLinksDeploymentTimeIdx = `
CREATE INDEX IF NOT EXISTS idx_dora_links_deployment_time
ON dora_commit_deployment_links (tenant_id, service_ref, deployment_time DESC, deployment_outcome);`

	SchemaDoraDeploymentFrequencyDailyMV = `
CREATE MATERIALIZED VIEW IF NOT EXISTS dora_deployment_frequency_daily AS
SELECT
	tenant_id,
	service_ref,
	environment,
	date_trunc('day', event_timestamp) AS bucket,
	count(*) FILTER (WHERE event_type = 'deployment.succeeded') AS success_count,
	count(*) FILTER (WHERE event_type = 'deployment.failed') AS failure_count
FROM dora_events
WHERE event_type IN ('deployment.succeeded', 'deployment.failed')
GROUP BY tenant_id, service_ref, environment, bucket;`

	SchemaDoraDeploymentFrequencyDailyMVKeyIdx = `
CREATE UNIQUE INDEX IF NOT EXISTS idx_dora_deploy_freq_daily_key
ON dora_deployment_frequency_daily (tenant_id, service_ref, environment, bucket);`

	SchemaDoraIncidentOpenedDailyMV = `
CREATE MATERIALIZED VIEW IF NOT EXISTS dora_incident_opened_daily AS
SELECT
	tenant_id,
	service_ref,
	date_trunc('day', event_timestamp) AS bucket,
	count(*) AS opened_incident_count
FROM dora_events
WHERE event_type = 'incident.opened'
GROUP BY tenant_id, service_ref, bucket;`

	SchemaDoraIncidentOpenedDailyMVKeyIdx = `
CREATE UNIQUE INDEX IF NOT EXISTS idx_dora_incident_opened_daily_key
ON dora_incident_opened_daily (tenant_id, service_ref, bucket);`

	SchemaDoraIncidentsDailyMV = `
CREATE MATERIALIZED VIEW IF NOT EXISTS dora_incidents_daily AS
SELECT
	o.tenant_id,
	o.service_ref,
	date_trunc('day', o.event_timestamp) AS bucket,
	count(*) AS resolved_incident_count,
	avg(EXTRACT(EPOCH FROM (r.event_timestamp - o.event_timestamp))) AS avg_restore_seconds,
	percentile_cont(0.5) WITHIN GROUP (
		ORDER BY EXTRACT(EPOCH FROM (r.event_timestamp - o.event_timestamp))
	) AS p50_restore_seconds
FROM dora_events o
JOIN dora_events r
	ON  r.tenant_id = o.tenant_id
	AND r.correlation_key = o.correlation_key
	AND r.event_type = 'incident.resolved'
WHERE o.event_type = 'incident.opened'
GROUP BY o.tenant_id, o.service_ref, bucket;`

	SchemaDoraIncidentsDailyMVKeyIdx = `
CREATE UNIQUE INDEX IF NOT EXISTS idx_dora_incidents_daily_key
ON dora_incidents_daily (tenant_id, service_ref, bucket);`

	SchemaDoraWebhookDeadLetter = `
CREATE TABLE IF NOT EXISTS dora_webhook_dead_letter (
	id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
	tenant_id text NOT NULL,
	provider text NOT NULL,
	source_event_id text,
	headers jsonb NOT NULL,
	body bytea NOT NULL,
	failure_reason text NOT NULL,
	created_at timestamptz NOT NULL DEFAULT now(),
	replayed_at timestamptz,
	replay_job_id uuid
);`

	SchemaDoraWebhookDeadLetterTenantCreatedIdx = `
CREATE INDEX IF NOT EXISTS idx_dora_webhook_dlq_tenant_created
ON dora_webhook_dead_letter (tenant_id, created_at DESC);`

	SchemaDoraGroupBrandMap = `
CREATE TABLE IF NOT EXISTS dora_group_brand_map (
	group_id text NOT NULL,
	brand_id text NOT NULL,
	tenant_id text NOT NULL,
	classification_version text NOT NULL DEFAULT 'dora-2023-default+gates-included',
	last_synced_at timestamptz NOT NULL DEFAULT now(),
	PRIMARY KEY (group_id, brand_id, tenant_id)
);`

	SchemaDoraGroupBrandMapGroupIdx = `
CREATE INDEX IF NOT EXISTS idx_dora_group_brand_map_group
ON dora_group_brand_map (tenant_id, group_id, last_synced_at DESC);`

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
	SchemaDoraEvents,
	SchemaDoraCommitDeploymentLinks,
	SchemaDoraWebhookDeadLetter,
	SchemaDoraGroupBrandMap,
	SchemaDoraDeploymentFrequencyDailyMV,
	SchemaDoraIncidentOpenedDailyMV,
	SchemaDoraIncidentsDailyMV,
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
	SchemaDoraEventsTenantServiceIdx,
	SchemaDoraEventsCorrelationIdx,
	SchemaDoraEventsSourceEventUidx,
	SchemaDoraEventsDeployDedupeUidx,
	SchemaDoraCommitDeploymentLinksDeploymentTimeIdx,
	SchemaDoraWebhookDeadLetterTenantCreatedIdx,
	SchemaDoraGroupBrandMapGroupIdx,
	SchemaDoraDeploymentFrequencyDailyMVKeyIdx,
	SchemaDoraIncidentOpenedDailyMVKeyIdx,
	SchemaDoraIncidentsDailyMVKeyIdx,
	// Helper functions
	SchemaBackoffInterval,
	SchemaEffectLease,
	SchemaEffectBackoff,
	SchemaEffectEscalateDlq,
}
