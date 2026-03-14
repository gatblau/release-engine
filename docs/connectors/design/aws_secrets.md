# AWS Secrets Connector — Pseudo Code

The connector encapsulates all interactions with the AWS API surface. The primary focus is Secrets Manager and KMS operations that back the Volta VaultManager, but the connector is general enough to support IAM, STS, S3, and SSM operations required by other workflows. All operations are idempotent by contract and return one of `Success`, `RetryableError`, or `TerminalError`.

---

## Interface

```
CONNECTOR: AWSConnector
implements Connector interface

registered name: "aws"

func Call(ctx, op, params, credential) -> ConnectorResult:
  client = resolve_client(credential)
  // credential: {
  //   provider:        "aws"
  //   auth_method:     "iam_role" | "access_key" | "web_identity"
  //   role_arn:        string (if iam_role or web_identity)
  //   access_key_id:   string (if access_key)
  //   secret_key:      string (if access_key) — guarded memory only
  //   region:          string
  //   endpoint:        string (optional — for localstack or custom endpoints)
  // }

  switch op:

    // ── Secrets Manager ───────────────────────────────────────────
    case "sm_create_secret":            return sm_create_secret(ctx, client, params)
    case "sm_get_secret":               return sm_get_secret(ctx, client, params)
    case "sm_put_secret_value":         return sm_put_secret_value(ctx, client, params)
    case "sm_delete_secret":            return sm_delete_secret(ctx, client, params)
    case "sm_restore_secret":           return sm_restore_secret(ctx, client, params)
    case "sm_list_secrets":             return sm_list_secrets(ctx, client, params)
    case "sm_describe_secret":          return sm_describe_secret(ctx, client, params)
    case "sm_list_secret_versions":     return sm_list_secret_versions(ctx, client, params)
    case "sm_get_secret_version":       return sm_get_secret_version(ctx, client, params)
    case "sm_rotate_secret":            return sm_rotate_secret(ctx, client, params)
    case "sm_cancel_rotation":          return sm_cancel_rotation(ctx, client, params)
    case "sm_tag_secret":               return sm_tag_secret(ctx, client, params)
    case "sm_untag_secret":             return sm_untag_secret(ctx, client, params)
    case "sm_get_resource_policy":      return sm_get_resource_policy(ctx, client, params)
    case "sm_put_resource_policy":      return sm_put_resource_policy(ctx, client, params)
    case "sm_delete_resource_policy":   return sm_delete_resource_policy(ctx, client, params)

    // ── KMS ───────────────────────────────────────────────────────
    case "kms_create_key":              return kms_create_key(ctx, client, params)
    case "kms_describe_key":            return kms_describe_key(ctx, client, params)
    case "kms_list_keys":               return kms_list_keys(ctx, client, params)
    case "kms_schedule_key_deletion":   return kms_schedule_key_deletion(ctx, client, params)
    case "kms_cancel_key_deletion":     return kms_cancel_key_deletion(ctx, client, params)
    case "kms_enable_key":              return kms_enable_key(ctx, client, params)
    case "kms_disable_key":             return kms_disable_key(ctx, client, params)
    case "kms_generate_data_key":       return kms_generate_data_key(ctx, client, params)
    case "kms_generate_data_key_without_plaintext": return kms_generate_data_key_without_plaintext(ctx, client, params)
    case "kms_encrypt":                 return kms_encrypt(ctx, client, params)
    case "kms_decrypt":                 return kms_decrypt(ctx, client, params)
    case "kms_re_encrypt":              return kms_re_encrypt(ctx, client, params)
    case "kms_create_alias":            return kms_create_alias(ctx, client, params)
    case "kms_delete_alias":            return kms_delete_alias(ctx, client, params)
    case "kms_list_aliases":            return kms_list_aliases(ctx, client, params)
    case "kms_create_grant":            return kms_create_grant(ctx, client, params)
    case "kms_revoke_grant":            return kms_revoke_grant(ctx, client, params)
    case "kms_list_grants":             return kms_list_grants(ctx, client, params)
    case "kms_get_key_policy":          return kms_get_key_policy(ctx, client, params)
    case "kms_put_key_policy":          return kms_put_key_policy(ctx, client, params)
    case "kms_enable_key_rotation":     return kms_enable_key_rotation(ctx, client, params)
    case "kms_disable_key_rotation":    return kms_disable_key_rotation(ctx, client, params)
    case "kms_get_key_rotation_status": return kms_get_key_rotation_status(ctx, client, params)
    case "kms_tag_resource":            return kms_tag_resource(ctx, client, params)
    case "kms_untag_resource":          return kms_untag_resource(ctx, client, params)
    case "kms_sign":                    return kms_sign(ctx, client, params)
    case "kms_verify":                  return kms_verify(ctx, client, params)

    // ── STS ───────────────────────────────────────────────────────
    case "sts_assume_role":             return sts_assume_role(ctx, client, params)
    case "sts_assume_role_with_web_identity": return sts_assume_role_with_web_identity(ctx, client, params)
    case "sts_get_caller_identity":     return sts_get_caller_identity(ctx, client, params)
    case "sts_get_session_token":       return sts_get_session_token(ctx, client, params)

    // ── IAM ───────────────────────────────────────────────────────
    case "iam_create_role":             return iam_create_role(ctx, client, params)
    case "iam_delete_role":             return iam_delete_role(ctx, client, params)
    case "iam_get_role":                return iam_get_role(ctx, client, params)
    case "iam_attach_role_policy":      return iam_attach_role_policy(ctx, client, params)
    case "iam_detach_role_policy":      return iam_detach_role_policy(ctx, client, params)
    case "iam_list_attached_role_policies": return iam_list_attached_role_policies(ctx, client, params)
    case "iam_create_policy":           return iam_create_policy(ctx, client, params)
    case "iam_delete_policy":           return iam_delete_policy(ctx, client, params)
    case "iam_get_policy":              return iam_get_policy(ctx, client, params)
    case "iam_create_policy_version":   return iam_create_policy_version(ctx, client, params)
    case "iam_list_policy_versions":    return iam_list_policy_versions(ctx, client, params)
    case "iam_tag_role":                return iam_tag_role(ctx, client, params)
    case "iam_untag_role":              return iam_untag_role(ctx, client, params)

    // ── SSM Parameter Store ───────────────────────────────────────
    case "ssm_put_parameter":           return ssm_put_parameter(ctx, client, params)
    case "ssm_get_parameter":           return ssm_get_parameter(ctx, client, params)
    case "ssm_get_parameters_by_path":  return ssm_get_parameters_by_path(ctx, client, params)
    case "ssm_delete_parameter":        return ssm_delete_parameter(ctx, client, params)
    case "ssm_describe_parameters":     return ssm_describe_parameters(ctx, client, params)
    case "ssm_add_tags_to_resource":    return ssm_add_tags_to_resource(ctx, client, params)

    // ── S3 ────────────────────────────────────────────────────────
    case "s3_put_object":               return s3_put_object(ctx, client, params)
    case "s3_get_object":               return s3_get_object(ctx, client, params)
    case "s3_delete_object":            return s3_delete_object(ctx, client, params)
    case "s3_head_object":              return s3_head_object(ctx, client, params)
    case "s3_list_objects":             return s3_list_objects(ctx, client, params)
    case "s3_copy_object":              return s3_copy_object(ctx, client, params)
    case "s3_create_bucket":            return s3_create_bucket(ctx, client, params)
    case "s3_delete_bucket":            return s3_delete_bucket(ctx, client, params)
    case "s3_put_bucket_policy":        return s3_put_bucket_policy(ctx, client, params)
    case "s3_get_bucket_policy":        return s3_get_bucket_policy(ctx, client, params)
    case "s3_put_bucket_versioning":    return s3_put_bucket_versioning(ctx, client, params)
    case "s3_put_bucket_encryption":    return s3_put_bucket_encryption(ctx, client, params)
    case "s3_generate_presigned_url":   return s3_generate_presigned_url(ctx, client, params)

    default:
      return TerminalError("unknown operation: " + op)
```

---

## Supported Operations

### Secrets Manager Operations

---

#### `sm_create_secret`
> Creates a new secret in AWS Secrets Manager with an optional initial value and KMS key association. Used by Volta during tenant onboarding to create the encrypted vault that will hold the tenant's KEK metadata and connector credentials.

```
func sm_create_secret(ctx, client, params):
  // params: name, description, secret_string (guarded), secret_binary (guarded),
  //         kms_key_id, tags{}, recovery_window_days, client_request_token

  // client_request_token provides idempotency at the AWS layer
  token = params.client_request_token ?? deterministic_uuid(params.name)

  resp = client.SecretsManager.CreateSecret({
    Name:                        params.name,
    Description:                 params.description,
    SecretString:                params.secret_string,    // plaintext — guarded scope only
    SecretBinary:                params.secret_binary,    // binary — guarded scope only
    KmsKeyId:                    params.kms_key_id,
    Tags:                        to_aws_tags(params.tags),
    ClientRequestToken:          token,
  })

  // scrub plaintext immediately after SDK call returns
  scrub(params.secret_string)
  scrub(params.secret_binary)

  if resp.error.code == "ResourceExistsException":
    existing = sm_describe_secret(ctx, client, { secret_id: params.name })
    if existing is error: return existing
    return Success({
      arn:        existing.data.arn,
      name:       existing.data.name,
      version_id: existing.data.current_version_id,
      idempotent: true,
    })

  if resp.error.code in ["ThrottlingException", "InternalServiceError"]:
    return RetryableError(resp.error)

  if resp.error is not null:
    return TerminalError(resp.error)

  return Success({
    arn:        resp.ARN,
    name:       resp.Name,
    version_id: resp.VersionId,
  })
```

---

#### `sm_get_secret`
> Retrieves the current plaintext or binary value of a secret. This is the primary operation Volta uses at startup to fetch the master passphrase and derive the root key material that protects all tenant KEKs.

```
func sm_get_secret(ctx, client, params):
  // params: secret_id (name or ARN), version_id, version_stage
  // SECURITY: return value must be placed in guarded memory by caller.
  //           It is never written to logs, database, or unguarded heap.

  resp = client.SecretsManager.GetSecretValue({
    SecretId:     params.secret_id,
    VersionId:    params.version_id,
    VersionStage: params.version_stage ?? "AWSCURRENT",
  })

  if resp.error.code == "ResourceNotFoundException":
    return TerminalError("secret_not_found")

  if resp.error.code == "InvalidRequestException":
    // secret is pending deletion
    return TerminalError("secret_pending_deletion")

  if resp.error.code in [
    "ThrottlingException",
    "InternalServiceError",
    "ServiceUnavailableException"
  ]:
    return RetryableError(resp.error)

  if resp.error is not null:
    return TerminalError(resp.error)

  // NOTE: SecretString / SecretBinary are sensitive.
  // Caller MUST load these into memguard-protected memory
  // and scrub them from this response object immediately.
  return Success({
    arn:            resp.ARN,
    name:           resp.Name,
    version_id:     resp.VersionId,
    version_stages: resp.VersionStages,
    created_at:     resp.CreatedDate,
    // SENSITIVE — must be consumed into guarded scope and scrubbed
    secret_string:  resp.SecretString,
    secret_binary:  resp.SecretBinary,
  })
```

---

#### `sm_put_secret_value`
> Writes a new version of a secret's value. Used by Volta during key rotation to persist updated KEK metadata or a rotated master passphrase. The new version is staged as `AWSCURRENT`; the previous version is automatically demoted to `AWSPREVIOUS`.

```
func sm_put_secret_value(ctx, client, params):
  // params: secret_id, secret_string (guarded), secret_binary (guarded),
  //         version_stages[], client_request_token

  token = params.client_request_token ?? deterministic_uuid(
    params.secret_id + ":" + current_timestamp_truncated_to_minute()
  )

  resp = client.SecretsManager.PutSecretValue({
    SecretId:           params.secret_id,
    SecretString:       params.secret_string,
    SecretBinary:       params.secret_binary,
    VersionStages:      params.version_stages ?? ["AWSCURRENT"],
    ClientRequestToken: token,
  })

  scrub(params.secret_string)
  scrub(params.secret_binary)

  if resp.error.code == "ResourceExistsException":
    // idempotency token matched an already-written version — safe to treat as success
    return Success({ idempotent: true, version_id: resp.VersionId })

  if resp.error.code == "ResourceNotFoundException":
    return TerminalError("secret_not_found")

  if resp.error.code in ["ThrottlingException", "InternalServiceError"]:
    return RetryableError(resp.error)

  if resp.error is not null:
    return TerminalError(resp.error)

  return Success({
    arn:            resp.ARN,
    name:           resp.Name,
    version_id:     resp.VersionId,
    version_stages: resp.VersionStages,
  })
```

---

#### `sm_delete_secret`
> Marks a secret for deletion with a configurable recovery window. Used during tenant offboarding to schedule removal of vault data. Recovery window defaults to 30 days to prevent accidental permanent loss.

```
func sm_delete_secret(ctx, client, params):
  // params: secret_id, recovery_window_days (7–30), force_delete_without_recovery

  if params.force_delete_without_recovery == true:
    // requires explicit opt-in — safety guard against accidental permanent deletion
    assert params.force_delete_ack == "i-understand-this-is-irreversible"

  resp = client.SecretsManager.DeleteSecret({
    SecretId:                   params.secret_id,
    RecoveryWindowInDays:       params.recovery_window_days ?? 30,
    ForceDeleteWithoutRecovery: params.force_delete_without_recovery ?? false,
  })

  if resp.error.code == "ResourceNotFoundException":
    return Success({ idempotent: true })

  if resp.error.code == "InvalidRequestException":
    // secret is already scheduled for deletion — idempotent
    if "already scheduled" in resp.error.message:
      return Success({ idempotent: true, deletion_date: resp.DeletionDate })
    return TerminalError(resp.error)

  if resp.error.code in ["ThrottlingException", "InternalServiceError"]:
    return RetryableError(resp.error)

  if resp.error is not null:
    return TerminalError(resp.error)

  return Success({
    arn:           resp.ARN,
    name:          resp.Name,
    deletion_date: resp.DeletionDate,
  })
```

---

#### `sm_restore_secret`
> Cancels a pending deletion and restores a secret to active state. Used in tenant offboarding rollback scenarios where deletion was triggered prematurely and the recovery window has not yet elapsed.

```
func sm_restore_secret(ctx, client, params):
  // params: secret_id

  resp = client.SecretsManager.RestoreSecret({
    SecretId: params.secret_id,
  })

  if resp.error.code == "ResourceNotFoundException":
    return TerminalError("secret_not_found_or_recovery_window_elapsed")

  if resp.error.code == "InvalidRequestException":
    // secret is not in a deleted state — idempotent
    return Success({ idempotent: true })

  if resp.error.code in ["ThrottlingException", "InternalServiceError"]:
    return RetryableError(resp.error)

  if resp.error is not null:
    return TerminalError(resp.error)

  return Success({
    arn:  resp.ARN,
    name: resp.Name,
  })
```

---

#### `sm_list_secrets`
> Lists secrets with optional name prefix and tag filters. Used in vault audit and housekeeping workflows to enumerate all tenant vault secrets and detect orphaned entries.

```
func sm_list_secrets(ctx, client, params):
  // params: filters[]{key, values[]}, sort_order, max_results, next_token
  // filter key examples: name | tag-key | tag-value | primary-region | owning-service

  resp = client.SecretsManager.ListSecrets({
    Filters:    params.filters.map(f => { Key: f.key, Values: f.values }),
    SortOrder:  params.sort_order ?? "asc",
    MaxResults: params.max_results ?? 100,
    NextToken:  params.next_token,
  })

  if resp.error.code in ["ThrottlingException", "InternalServiceError"]:
    return RetryableError(resp.error)

  if resp.error is not null:
    return TerminalError(resp.error)

  return Success({
    secrets: resp.SecretList.map(s => {
      arn:              s.ARN,
      name:             s.Name,
      description:      s.Description,
      kms_key_id:       s.KmsKeyId,
      rotation_enabled: s.RotationEnabled,
      last_rotated_at:  s.LastRotatedDate,
      last_changed_at:  s.LastChangedDate,
      last_accessed_at: s.LastAccessedDate,
      tags:             from_aws_tags(s.Tags),
      deleted_at:       s.DeletedDate,
    }),
    next_token: resp.NextToken,
  })
```

---

#### `sm_describe_secret`
> Fetches metadata for a secret without returning its value. Used to inspect rotation configuration, version staging, and tag state without exposing sensitive data.

```
func sm_describe_secret(ctx, client, params):
  // params: secret_id

  resp = client.SecretsManager.DescribeSecret({
    SecretId: params.secret_id,
  })

  if resp.error.code == "ResourceNotFoundException":
    return TerminalError("secret_not_found")

  if resp.error.code in ["ThrottlingException", "InternalServiceError"]:
    return RetryableError(resp.error)

  if resp.error is not null:
    return TerminalError(resp.error)

  return Success({
    arn:                     resp.ARN,
    name:                    resp.Name,
    description:             resp.Description,
    kms_key_id:              resp.KmsKeyId,
    rotation_enabled:        resp.RotationEnabled,
    rotation_lambda_arn:     resp.RotationLambdaARN,
    rotation_rules:          resp.RotationRules,
    last_rotated_at:         resp.LastRotatedDate,
    last_changed_at:         resp.LastChangedDate,
    last_accessed_at:        resp.LastAccessedDate,
    deleted_at:              resp.DeletedDate,
    tags:                    from_aws_tags(resp.Tags),
    current_version_id:      find_version_by_stage(resp.VersionIdsToStages, "AWSCURRENT"),
    previous_version_id:     find_version_by_stage(resp.VersionIdsToStages, "AWSPREVIOUS"),
    version_ids_to_stages:   resp.VersionIdsToStages,
  })
```

---

#### `sm_list_secret_versions`
> Lists all stored versions of a secret with their staging labels. Used during key rotation audit and version cleanup workflows to identify stale versions that should be pruned after a successful rotation.

```
func sm_list_secret_versions(ctx, client, params):
  // params: secret_id, include_deprecated, max_results, next_token

  resp = client.SecretsManager.ListSecretVersionIds({
    SecretId:          params.secret_id,
    IncludeDeprecated: params.include_deprecated ?? false,
    MaxResults:        params.max_results ?? 100,
    NextToken:         params.next_token,
  })

  if resp.error.code == "ResourceNotFoundException":
    return TerminalError("secret_not_found")

  if resp.error.code in ["ThrottlingException", "InternalServiceError"]:
    return RetryableError(resp.error)

  if resp.error is not null:
    return TerminalError(resp.error)

  return Success({
    arn:  resp.ARN,
    name: resp.Name,
    versions: resp.Versions.map(v => {
      version_id:     v.VersionId,
      version_stages: v.VersionStages,
      created_at:     v.CreatedDate,
      last_accessed:  v.LastAccessedDate,
    }),
    next_token: resp.NextToken,
  })
```

---

#### `sm_get_secret_version`
> Retrieves the value of a specific historical version of a secret by version ID or staging label. Used during key rotation rollback to retrieve and restore a previous KEK version if the new version proves corrupt.

```
func sm_get_secret_version(ctx, client, params):
  // params: secret_id, version_id, version_stage
  // SECURITY: same guarded-memory contract as sm_get_secret applies.

  resp = client.SecretsManager.GetSecretValue({
    SecretId:     params.secret_id,
    VersionId:    params.version_id,
    VersionStage: params.version_stage,
  })

  if resp.error.code == "ResourceNotFoundException":
    return TerminalError("secret_or_version_not_found")

  if resp.error.code in ["ThrottlingException", "InternalServiceError"]:
    return RetryableError(resp.error)

  if resp.error is not null:
    return TerminalError(resp.error)

  return Success({
    arn:            resp.ARN,
    name:           resp.Name,
    version_id:     resp.VersionId,
    version_stages: resp.VersionStages,
    created_at:     resp.CreatedDate,
    // SENSITIVE — must be consumed into guarded scope and scrubbed
    secret_string:  resp.SecretString,
    secret_binary:  resp.SecretBinary,
  })
```

---

#### `sm_rotate_secret`
> Triggers an immediate rotation of a secret using its configured Lambda rotation function. Used to initiate a scheduled or emergency rotation of connector credentials stored in Volta-managed vaults.

```
func sm_rotate_secret(ctx, client, params):
  // params: secret_id, client_request_token, rotation_lambda_arn (optional override),
  //         rotation_rules{ automatically_after_days }

  token = params.client_request_token ?? deterministic_uuid(params.secret_id + ":rotate")

  resp = client.SecretsManager.RotateSecret({
    SecretId:           params.secret_id,
    ClientRequestToken: token,
    RotationLambdaARN:  params.rotation_lambda_arn,
    RotationRules:      params.rotation_rules,
  })

  if resp.error.code == "ResourceNotFoundException":
    return TerminalError("secret_not_found")

  if resp.error.code == "InvalidRequestException":
    if "rotation is not enabled" in resp.error.message:
      return TerminalError("rotation_not_configured_for_secret")
    return TerminalError(resp.error)

  if resp.error.code in ["ThrottlingException", "InternalServiceError"]:
    return RetryableError(resp.error)

  if resp.error is not null:
    return TerminalError(resp.error)

  return Success({
    arn:        resp.ARN,
    name:       resp.Name,
    version_id: resp.VersionId,
  })
```

---

#### `sm_cancel_rotation`
> Cancels an in-progress rotation. Used when a rotation Lambda is stuck or has produced a corrupt secret version and the rotation must be aborted before it is finalized.

```
func sm_cancel_rotation(ctx, client, params):
  // params: secret_id

  resp = client.SecretsManager.CancelRotateSecret({
    SecretId: params.secret_id,
  })

  if resp.error.code == "ResourceNotFoundException":
    return TerminalError("secret_not_found")

  if resp.error.code == "InvalidRequestException":
    // rotation is not in progress — idempotent
    return Success({ idempotent: true })

  if resp.error.code in ["ThrottlingException", "InternalServiceError"]:
    return RetryableError(resp.error)

  if resp.error is not null:
    return TerminalError(resp.error)

  return Success({
    arn:  resp.ARN,
    name: resp.Name,
  })
```

---

#### `sm_tag_secret`
> Adds or updates tags on a secret. Used during tenant provisioning to attach tenant_id, environment, and classification tags to vault secrets for cost attribution, access control, and audit filtering.

```
func sm_tag_secret(ctx, client, params):
  // params: secret_id, tags{}

  resp = client.SecretsManager.TagResource({
    SecretId: params.secret_id,
    Tags:     to_aws_tags(params.tags),
  })

  if resp.error.code == "ResourceNotFoundException":
    return TerminalError("secret_not_found")

  if resp.error.code in ["ThrottlingException", "InternalServiceError"]:
    return RetryableError(resp.error)

  if resp.error is not null:
    return TerminalError(resp.error)

  return Success({ secret_id: params.secret_id, tags: params.tags })
```

---

#### `sm_untag_secret`
> Removes specific tags from a secret by key. Used when tenant metadata changes and stale classification tags must be removed before updated ones are applied.

```
func sm_untag_secret(ctx, client, params):
  // params: secret_id, tag_keys[]

  resp = client.SecretsManager.UntagResource({
    SecretId: params.secret_id,
    TagKeys:  params.tag_keys,
  })

  if resp.error.code == "ResourceNotFoundException":
    return TerminalError("secret_not_found")

  if resp.error.code in ["ThrottlingException", "InternalServiceError"]:
    return RetryableError(resp.error)

  if resp.error is not null:
    return TerminalError(resp.error)

  return Success({ secret_id: params.secret_id, removed_keys: params.tag_keys })
```

---

#### `sm_get_resource_policy`
> Fetches the resource-based policy attached to a secret. Used in compliance audit workflows to verify that vault secrets are not accessible outside of the intended IAM principal set.

```
func sm_get_resource_policy(ctx, client, params):
  // params: secret_id

  resp = client.SecretsManager.GetResourcePolicy({
    SecretId: params.secret_id,
  })

  if resp.error.code == "ResourceNotFoundException":
    return TerminalError("secret_not_found")

  if resp.error.code in ["ThrottlingException", "InternalServiceError"]:
    return RetryableError(resp.error)

  if resp.error is not null:
    return TerminalError(resp.error)

  return Success({
    arn:             resp.ARN,
    name:            resp.Name,
    resource_policy: parse_json(resp.ResourcePolicy),   // raw policy document
  })
```

---

#### `sm_put_resource_policy`
> Attaches or replaces the resource-based policy on a secret. Used to lock vault secrets to a specific IAM role or principal boundary during tenant provisioning or security remediation workflows.

```
func sm_put_resource_policy(ctx, client, params):
  // params: secret_id, policy{} (IAM policy document), block_public_policy

  resp = client.SecretsManager.PutResourcePolicy({
    SecretId:          params.secret_id,
    ResourcePolicy:    to_json(params.policy),
    BlockPublicPolicy: params.block_public_policy ?? true,
  })

  if resp.error.code == "ResourceNotFoundException":
    return TerminalError("secret_not_found")

  if resp.error.code == "MalformedPolicyDocumentException":
    return TerminalError({ reason: "invalid_policy_document", detail: resp.error })

  if resp.error.code in ["ThrottlingException", "InternalServiceError"]:
    return RetryableError(resp.error)

  if resp.error is not null:
    return TerminalError(resp.error)

  return Success({
    arn:  resp.ARN,
    name: resp.Name,
  })
```

---

#### `sm_delete_resource_policy`
> Removes the resource-based policy from a secret, reverting access control to identity-based policies only. Used when a tenant vault is migrated to a new access model and the old boundary policy must be cleared.

```
func sm_delete_resource_policy(ctx, client, params):
  // params: secret_id

  resp = client.SecretsManager.DeleteResourcePolicy({
    SecretId: params.secret_id,
  })

  if resp.error.code == "ResourceNotFoundException":
    return Success({ idempotent: true })

  if resp.error.code in ["ThrottlingException", "InternalServiceError"]:
    return RetryableError(resp.error)

  if resp.error is not null:
    return TerminalError(resp.error)

  return Success({
    arn:  resp.ARN,
    name: resp.Name,
  })
```

---

### KMS Operations

---

#### `kms_create_key`
> Creates a new customer-managed KMS key. Used by Volta when provisioning a new tenant to create a dedicated CMK that will serve as the root of that tenant's KEK hierarchy, ensuring cryptographic isolation between tenants.

```
func kms_create_key(ctx, client, params):
  // params: description, key_usage (ENCRYPT_DECRYPT|SIGN_VERIFY|GENERATE_VERIFY_MAC),
  //         key_spec (SYMMETRIC_DEFAULT|RSA_*|ECC_*|HMAC_*),
  //         policy{}, tags{}, multi_region, enable_key_rotation

  resp = client.KMS.CreateKey({
    Description: params.description,
    KeyUsage:    params.key_usage ?? "ENCRYPT_DECRYPT",
    KeySpec:     params.key_spec  ?? "SYMMETRIC_DEFAULT",
    Policy:      params.policy ? to_json(params.policy) : null,
    Tags:        to_aws_tags(params.tags),
    MultiRegion: params.multi_region ?? false,
  })

  if resp.error.code == "AlreadyExistsException":
    return TerminalError("key_alias_already_exists")

  if resp.error.code in ["ThrottlingException", "KMSInternalException"]:
    return RetryableError(resp.error)

  if resp.error is not null:
    return TerminalError(resp.error)

  key_id = resp.KeyMetadata.KeyId

  // Optionally enable automatic annual key rotation
  if params.enable_key_rotation == true:
    rotation_resp = kms_enable_key_rotation(ctx, client, { key_id: key_id })
    if rotation_resp is error: return rotation_resp

  return Success({
    key_id:      key_id,
    key_arn:     resp.KeyMetadata.Arn,
    key_state:   resp.KeyMetadata.KeyState,
    key_usage:   resp.KeyMetadata.KeyUsage,
    key_spec:    resp.KeyMetadata.KeySpec,
    created_at:  resp.KeyMetadata.CreationDate,
    multi_region: resp.KeyMetadata.MultiRegion,
  })
```

---

#### `kms_describe_key`
> Fetches metadata for a KMS key by key ID or alias. Used to verify key state, rotation configuration, and policy before performing cryptographic operations that depend on the key being enabled and accessible.

```
func kms_describe_key(ctx, client, params):
  // params: key_id (key ID, ARN, or alias)

  resp = client.KMS.DescribeKey({
    KeyId: params.key_id,
  })

  if resp.error.code == "NotFoundException":
    return TerminalError("kms_key_not_found")

  if resp.error.code in ["ThrottlingException", "KMSInternalException"]:
    return RetryableError(resp.error)

  if resp.error is not null:
    return TerminalError(resp.error)

  return Success({
    key_id:            resp.KeyMetadata.KeyId,
    key_arn:           resp.KeyMetadata.Arn,
    key_state:         resp.KeyMetadata.KeyState,    // Enabled | Disabled | PendingDeletion | ...
    key_usage:         resp.KeyMetadata.KeyUsage,
    key_spec:          resp.KeyMetadata.KeySpec,
    enabled:           resp.KeyMetadata.Enabled,
    rotation_enabled:  resp.KeyMetadata.KeyRotationStatus,
    multi_region:      resp.KeyMetadata.MultiRegion,
    created_at:        resp.KeyMetadata.CreationDate,
    deletion_date:     resp.KeyMetadata.DeletionDate,
    origin:            resp.KeyMetadata.Origin,
    manager:           resp.KeyMetadata.KeyManager,  // AWS | CUSTOMER
  })
```

---

#### `kms_generate_data_key`
> Generates a data encryption key (DEK) under a specified CMK, returning both the plaintext DEK and the ciphertext DEK. This is the core Volta operation used to create per-secret DEKs. The plaintext DEK is loaded into guarded memory to encrypt the payload and then immediately scrubbed; only the ciphertext DEK is persisted.

```
func kms_generate_data_key(ctx, client, params):
  // params: key_id, key_spec (AES_128|AES_256), number_of_bytes (alt to key_spec),
  //         encryption_context{}, grant_tokens[]
  // SECURITY: plaintext_dek in the response must be loaded into guarded
  //           memory by the caller and scrubbed immediately after use.

  resp = client.KMS.GenerateDataKey({
    KeyId:             params.key_id,
    KeySpec:           params.key_spec ?? "AES_256",
    NumberOfBytes:     params.number_of_bytes,
    EncryptionContext: params.encryption_context ?? {},
    GrantTokens:       params.grant_tokens,
  })

  if resp.error.code == "NotFoundException":
    return TerminalError("kms_key_not_found")

  if resp.error.code == "DisabledException":
    return TerminalError("kms_key_disabled")

  if resp.error.code == "KeyUnavailableException":
    return RetryableError("kms_key_temporarily_unavailable")

  if resp.error.code in ["ThrottlingException", "KMSInternalException"]:
    return RetryableError(resp.error)

  if resp.error is not null:
    return TerminalError(resp.error)

  return Success({
    key_id:          resp.KeyId,
    key_arn:         resp.KeyId,          // AWS returns ARN here
    // SENSITIVE — caller must load into guarded scope and scrub immediately
    plaintext_dek:   resp.Plaintext,
    // Safe to persist — encrypted under the CMK
    ciphertext_dek:  resp.CiphertextBlob,
  })
```

---

#### `kms_generate_data_key_without_plaintext`
> Generates an encrypted DEK without returning the plaintext. Used when Volta needs to pre-generate a future DEK for a key rotation operation where the plaintext is not needed immediately and must only be decrypted at the point of use.

```
func kms_generate_data_key_without_plaintext(ctx, client, params):
  // params: key_id, key_spec, number_of_bytes, encryption_context{}, grant_tokens[]

  resp = client.KMS.GenerateDataKeyWithoutPlaintext({
    KeyId:             params.key_id,
    KeySpec:           params.key_spec ?? "AES_256",
    NumberOfBytes:     params.number_of_bytes,
    EncryptionContext: params.encryption_context ?? {},
    GrantTokens:       params.grant_tokens,
  })

  if resp.error.code == "NotFoundException":
    return TerminalError("kms_key_not_found")

  if resp.error.code == "DisabledException":
    return TerminalError("kms_key_disabled")

  if resp.error.code == "KeyUnavailableException":
    return RetryableError("kms_key_temporarily_unavailable")

  if resp.error.code in ["ThrottlingException", "KMSInternalException"]:
    return RetryableError(resp.error)

  if resp.error is not null:
    return TerminalError(resp.error)

  return Success({
    key_id:         resp.KeyId,
    ciphertext_dek: resp.CiphertextBlob,    // Safe to persist
  })
```

---

#### `kms_encrypt`
> Encrypts a small plaintext payload (up to 4 KB) directly using a CMK. Used by Volta to encrypt KEK material itself — not for bulk data. For larger payloads, envelope encryption via `kms_generate_data_key` is used instead.

```
func kms_encrypt(ctx, client, params):
  // params: key_id, plaintext (guarded, max 4096 bytes),
  //         encryption_context{}, grant_tokens[]
  // SECURITY: plaintext is scrubbed immediately after SDK call.

  resp = client.KMS.Encrypt({
    KeyId:             params.key_id,
    Plaintext:         params.plaintext,
    EncryptionContext: params.encryption_context ?? {},
    GrantTokens:       params.grant_tokens,
  })

  scrub(params.plaintext)

  if resp.error.code == "NotFoundException":
    return TerminalError("kms_key_not_found")

  if resp.error.code == "DisabledException":
    return TerminalError("kms_key_disabled")

  if resp.error.code == "KeyUnavailableException":
    return RetryableError("kms_key_temporarily_unavailable")

  if resp.error.code in ["ThrottlingException", "KMSInternalException"]:
    return RetryableError(resp.error)

  if resp.error is not null:
    return TerminalError(resp.error)

  return Success({
    key_id:          resp.KeyId,
    ciphertext_blob: resp.CiphertextBlob,
    algorithm:       resp.EncryptionAlgorithm,
  })
```

---

#### `kms_decrypt`
> Decrypts a ciphertext blob produced by KMS. Used by Volta to decrypt a tenant's KEK when loading a vault session, enabling it to derive the DEK needed to decrypt individual connector credentials. The resulting plaintext is loaded into guarded memory.

```
func kms_decrypt(ctx, client, params):
  // params: ciphertext_blob, encryption_context{}, key_id (optional for CMK hint),
  //         grant_tokens[], encryption_algorithm
  // SECURITY: plaintext in the response must be loaded into guarded
  //           memory immediately and scrubbed after use.

  resp = client.KMS.Decrypt({
    CiphertextBlob:    params.ciphertext_blob,
    EncryptionContext: params.encryption_context ?? {},
    KeyId:             params.key_id,
    GrantTokens:       params.grant_tokens,
    EncryptionAlgorithm: params.encryption_algorithm,
  })

  if resp.error.code == "NotFoundException":
    return TerminalError("kms_key_not_found")

  if resp.error.code == "DisabledException":
    return TerminalError("kms_key_disabled")

  if resp.error.code == "InvalidCiphertextException":
    return TerminalError("ciphertext_corrupt_or_wrong_key")

  if resp.error.code == "KeyUnavailableException":
    return RetryableError("kms_key_temporarily_unavailable")

  if resp.error.code in ["ThrottlingException", "KMSInternalException"]:
    return RetryableError(resp.error)

  if resp.error is not null:
    return TerminalError(resp.error)

  return Success({
    key_id:    resp.KeyId,
    algorithm: resp.EncryptionAlgorithm,
    // SENSITIVE — caller must load into guarded scope and scrub immediately
    plaintext: resp.Plaintext,
  })
```

---

#### `kms_re_encrypt`
> Re-encrypts a ciphertext from one CMK to another without exposing the plaintext to the caller. Used during tenant CMK rotation to migrate all DEK ciphertexts from the old CMK to the new one without ever decrypting the underlying key material outside of KMS.

```
func kms_re_encrypt(ctx, client, params):
  // params: ciphertext_blob, source_key_id, destination_key_id,
  //         source_encryption_context{}, destination_encryption_context{},
  //         grant_tokens[]

  resp = client.KMS.ReEncrypt({
    CiphertextBlob:               params.ciphertext_blob,
    SourceKeyId:                  params.source_key_id,
    DestinationKeyId:             params.destination_key_id,
    SourceEncryptionContext:      params.source_encryption_context ?? {},
    DestinationEncryptionContext: params.destination_encryption_context ?? {},
    GrantTokens:                  params.grant_tokens,
  })

  if resp.error.code == "NotFoundException":
    return TerminalError("kms_key_not_found")

  if resp.error.code == "InvalidCiphertextException":
    return TerminalError("ciphertext_corrupt_or_wrong_source_key")

  if resp.error.code == "KeyUnavailableException":
    return RetryableError("kms_key_temporarily_unavailable")

  if resp.error.code in ["ThrottlingException", "KMSInternalException"]:
    return RetryableError(resp.error)

  if resp.error is not null:
    return TerminalError(resp.error)

  return Success({
    source_key_id:      resp.SourceKeyId,
    destination_key_id: resp.KeyId,
    ciphertext_blob:    resp.CiphertextBlob,    // now encrypted under destination CMK
    algorithm:          resp.DestinationEncryptionAlgorithm,
  })
```

---

#### `kms_schedule_key_deletion`
> Schedules a CMK for deletion after a waiting period. Used during tenant offboarding after all DEKs encrypted under the key have been either migrated or destroyed, rendering any residual ciphertext permanently unrecoverable.

```
func kms_schedule_key_deletion(ctx, client, params):
  // params: key_id, pending_window_days (7–30)
  // WARNING: this operation is irreversible after the waiting period.
  //          Caller must ensure all ciphertexts under this key are migrated.

  assert params.deletion_ack == "i-understand-data-will-be-unrecoverable"

  resp = client.KMS.ScheduleKeyDeletion({
    KeyId:               params.key_id,
    PendingWindowInDays: params.pending_window_days ?? 30,
  })

  if resp.error.code == "NotFoundException":
    return TerminalError("kms_key_not_found")

  if resp.error.code == "KMSInvalidStateException":
    if "pending deletion" in resp.error.message:
      return Success({ idempotent: true, deletion_date: resp.DeletionDate })
    return TerminalError(resp.error)

  if resp.error.code in ["ThrottlingException", "KMSInternalException"]:
    return RetryableError(resp.error)

  if resp.error is not null:
    return TerminalError(resp.error)

  return Success({
    key_id:        resp.KeyId,
    key_state:     resp.KeyState,
    deletion_date: resp.DeletionDate,
  })
```

---

#### `kms_enable_key_rotation`
> Enables automatic annual rotation of a symmetric CMK. Used after key creation to ensure that Volta's root tenant CMKs rotate without manual intervention, reducing the blast radius of long-lived key exposure.

```
func kms_enable_key_rotation(ctx, client, params):
  // params: key_id

  resp = client.KMS.EnableKeyRotation({
    KeyId: params.key_id,
  })

  if resp.error.code == "NotFoundException":
    return TerminalError("kms_key_not_found")

  if resp.error.code == "DisabledException":
    return TerminalError("kms_key_must_be_enabled_to_configure_rotation")

  if resp.error.code == "UnsupportedOperationException":
    return TerminalError("key_type_does_not_support_rotation")

  if resp.error.code in ["ThrottlingException", "KMSInternalException"]:
    return RetryableError(resp.error)

  if resp.error is not null:
    return TerminalError(resp.error)

  return Success({ key_id: params.key_id, rotation_enabled: true })
```

---

#### `kms_sign`
> Signs a message digest using an asymmetric KMS key. Used in workflows that require cryptographic attestation of job outputs or webhook payloads, where the signature must be verifiable by an external party using the corresponding public key.

```
func kms_sign(ctx, client, params):
  // params: key_id, message (raw bytes or digest), message_type (RAW|DIGEST),
  //         signing_algorithm, grant_tokens[]

  resp = client.KMS.Sign({
    KeyId:            params.key_id,
    Message:          params.message,
    MessageType:      params.message_type ?? "RAW",
    SigningAlgorithm: params.signing_algorithm,
    GrantTokens:      params.grant_tokens,
  })

  if resp.error.code == "NotFoundException":
    return TerminalError("kms_key_not_found")

  if resp.error.code == "DisabledException":
    return TerminalError("kms_key_disabled")

  if resp.error.code == "KeyUnavailableException":
    return RetryableError("kms_key_temporarily_unavailable")

  if resp.error.code in ["ThrottlingException", "KMSInternalException"]:
    return RetryableError(resp.error)

  if resp.error is not null:
    return TerminalError(resp.error)

  return Success({
    key_id:    resp.KeyId,
    signature: resp.Signature,
    algorithm: resp.SigningAlgorithm,
  })
```

---

#### `kms_verify`
> Verifies a signature produced by a KMS asymmetric signing key. Used to assert the integrity and origin of signed payloads before acting on them in security-sensitive orchestration steps.

```
func kms_verify(ctx, client, params):
  // params: key_id, message, message_type, signature, signing_algorithm, grant_tokens[]

  resp = client.KMS.Verify({
    KeyId:            params.key_id,
    Message:          params.message,
    MessageType:      params.message_type ?? "RAW",
    Signature:        params.signature,
    SigningAlgorithm: params.signing_algorithm,
    GrantTokens:      params.grant_tokens,
  })

  if resp.error.code == "NotFoundException":
    return TerminalError("kms_key_not_found")

  if resp.error.code == "KMSInvalidSignatureException":
    return Success({ valid: false, key_id: params.key_id })

  if resp.error.code in ["ThrottlingException", "KMSInternalException"]:
    return RetryableError(resp.error)

  if resp.error is not null:
    return TerminalError(resp.error)

  return Success({
    key_id:    resp.KeyId,
    valid:     resp.SignatureValid,
    algorithm: resp.SigningAlgorithm,
  })
```

---

### STS Operations

---

#### `sts_assume_role`
> Assumes an IAM role and returns short-lived credentials. Used by the connector itself when cross-account operations are required, and by provisioning workflows that need to act within a target account on behalf of the engine's service role.

```
func sts_assume_role(ctx, client, params):
  // params: role_arn, session_name, duration_seconds, policy{},
  //         external_id, mfa_serial, tags[]

  resp = client.STS.AssumeRole({
    RoleArn:         params.role_arn,
    RoleSessionName: params.session_name ?? "release-engine",
    DurationSeconds: params.duration_seconds ?? 3600,
    Policy:          params.policy ? to_json(params.policy) : null,
    ExternalId:      params.external_id,
    SerialNumber:    params.mfa_serial,
    Tags:            to_aws_session_tags(params.tags),
  })

  if resp.error.code == "AccessDenied":
    return TerminalError("sts_assume_role_access_denied")

  if resp.error.code == "RegionDisabledException":
    return TerminalError("sts_region_disabled")

  if resp.error.code in ["ThrottlingException", "IDPCommunicationError"]:
    return RetryableError(resp.error)

  if resp.error is not null:
    return TerminalError(resp.error)

  return Success({
    access_key_id:     resp.Credentials.AccessKeyId,
    // SENSITIVE — caller must load into guarded scope and scrub after use
    secret_access_key: resp.Credentials.SecretAccessKey,
    session_token:     resp.Credentials.SessionToken,
    expiration:        resp.Credentials.Expiration,
    assumed_role_id:   resp.AssumedRoleUser.AssumedRoleId,
    assumed_role_arn:  resp.AssumedRoleUser.Arn,
  })
```

---

#### `sts_get_caller_identity`
> Returns the AWS account ID, user ID, and ARN for the current credential. Used as a health check and to assert that the engine is operating under the expected identity before performing sensitive vault operations.

```
func sts_get_caller_identity(ctx, client, params):
  // params: (none)

  resp = client.STS.GetCallerIdentity({})

  if resp.error.code in ["ThrottlingException"]:
    return RetryableError(resp.error)

  if resp.error is not null:
    return TerminalError(resp.error)

  return Success({
    account_id: resp.Account,
    user_id:    resp.UserId,
    arn:        resp.Arn,
  })
```

---

### S3 Operations

---

#### `s3_put_object`
> Uploads an object to an S3 bucket with optional SSE configuration. Used by Volta to persist encrypted vault blobs — the double-encryption layer where Volta's AES-256-GCM ciphertext is stored under S3 SSE-KMS.

```
func s3_put_object(ctx, client, params):
  // params: bucket, key, body (bytes), content_type, metadata{},
  //         sse_algorithm (aws:kms|AES256), kms_key_id,
  //         checksum_algorithm, tagging{}, if_none_match

  resp = client.S3.PutObject({
    Bucket:               params.bucket,
    Key:                  params.key,
    Body:                 params.body,
    ContentType:          params.content_type ?? "application/octet-stream",
    Metadata:             params.metadata ?? {},
    ServerSideEncryption: params.sse_algorithm ?? "aws:kms",
    SSEKMSKeyId:          params.kms_key_id,
    ChecksumAlgorithm:    params.checksum_algorithm ?? "SHA256",
    Tagging:              to_query_string_tags(params.tagging),
    IfNoneMatch:          params.if_none_match,    // "*" to prevent overwrite
  })

  if resp.error.code == "NoSuchBucket":
    return TerminalError("s3_bucket_not_found")

  if resp.error.code == "PreconditionFailed":
    return TerminalError("s3_object_already_exists")

  if resp.error.code in [
    "SlowDown",
    "RequestTimeout",
    "InternalError",
    "ServiceUnavailable"
  ]:
    return RetryableError(resp.error)

  if resp.error is not null:
    return TerminalError(resp.error)

  return Success({
    bucket:     params.bucket,
    key:        params.key,
    etag:       resp.ETag,
    version_id: resp.VersionId,
    checksum:   resp.ChecksumSHA256,
  })
```

---

#### `s3_get_object`
> Downloads an object from S3. Used by Volta to load encrypted vault blobs on demand per tenant session. The response body is read directly into guarded memory when it contains key material.

```
func s3_get_object(ctx, client, params):
  // params: bucket, key, version_id, range, if_none_match (etag),
  //         checksum_mode
  // SECURITY: when body contains key material, caller must load
  //           it into guarded memory and scrub after use.

  resp = client.S3.GetObject({
    Bucket:       params.bucket,
    Key:          params.key,
    VersionId:    params.version_id,
    Range:        params.range,
    IfNoneMatch:  params.if_none_match,
    ChecksumMode: params.checksum_mode ?? "ENABLED",
  })

  if resp.error.code == "NoSuchKey":
    return TerminalError("s3_object_not_found")

  if resp.error.code == "NoSuchBucket":
    return TerminalError("s3_bucket_not_found")

  if resp.error.code == "NotModified":
    return Success({ not_modified: true })

  if resp.error.code in ["SlowDown", "RequestTimeout", "InternalError"]:
    return RetryableError(resp.error)

  if resp.error is not null:
    return TerminalError(resp.error)

  return Success({
    bucket:        params.bucket,
    key:           params.key,
    etag:          resp.ETag,
    version_id:    resp.VersionId,
    content_type:  resp.ContentType,
    content_length: resp.ContentLength,
    metadata:      resp.Metadata,
    last_modified: resp.LastModified,
    checksum:      resp.ChecksumSHA256,
    // body may be SENSITIVE — see security note above
    body:          resp.Body,
  })
```

---

## Error Classification Reference

| Error Code | Service | Classification | Engine Behaviour |
|---|---|---|---|
| `ResourceExistsException` | SM | Success (idempotent) | Fetch existing and return |
| `ResourceNotFoundException` | SM / KMS | TerminalError | Resource absent — fail step |
| `InvalidRequestException` (pending deletion) | SM | TerminalError | Secret in terminal state |
| `InvalidCiphertextException` | KMS | TerminalError | Ciphertext corrupt or wrong key — do not retry |
| `KMSInvalidSignatureException` | KMS | Success `valid:false` | Return verification result to module |
| `DisabledException` | KMS | TerminalError | Key disabled — operator action required |
| `KeyUnavailableException` | KMS | RetryableError | Transient — re-enqueue with backoff |
| `UnsupportedOperationException` | KMS | TerminalError | Key type incompatible with operation |
| `NoSuchKey` / `NoSuchBucket` | S3 | TerminalError | Object or bucket absent — fail step |
| `PreconditionFailed` | S3 | TerminalError | Conditional write rejected — fail step |
| `AccessDenied` | STS / S3 / SM | TerminalError | IAM permission missing — operator action required |
| `ThrottlingException` | All | RetryableError | Re-enqueue with exponential backoff |
| `SlowDown` | S3 | RetryableError | Re-enqueue with exponential backoff |
| `InternalServiceError` / `KMSInternalException` / `InternalError` | All | RetryableError | Re-enqueue with backoff |
| Network timeout | All | RetryableError | Re-enqueue with backoff |

---

## Notes

**Authentication.** Credentials are resolved at call time. For `iam_role` and `web_identity` auth methods the connector calls `sts_assume_role` or `sts_assume_role_with_web_identity` internally before constructing the service client. The resulting short-lived credentials are held in guarded memory for the duration of the connector invocation and scrubbed immediately afterwards.

**Encryption Context.** All `kms_encrypt`, `kms_decrypt`, and `kms_generate_data_key` calls include an `encryption_context` map keyed by `tenant_id` and `purpose`. This binds the ciphertext to a specific tenant and operation category, ensuring that a DEK generated for one tenant cannot be decrypted in the context of another even if the ciphertext is misrouted.

**Envelope Encryption Pattern.** Volta never uses `kms_encrypt` directly on credential payloads. The canonical pattern is: `kms_generate_data_key` → encrypt payload locally with the plaintext DEK using AES-256-GCM → scrub plaintext DEK → persist ciphertext DEK alongside ciphertext payload → on load: `kms_decrypt` ciphertext DEK → decrypt payload → scrub plaintext DEK.

**S3 Double Encryption.** Vault blobs written via `s3_put_object` are already Volta-encrypted before they reach S3. The `sse_algorithm: aws:kms` parameter adds a second encryption layer at rest. The S3 SSE key is a separate AWS-managed key from the tenant CMK, so the two encryption layers are cryptographically independent.

**Idempotency Tokens.** `sm_create_secret` and `sm_put_secret_value` derive their `ClientRequestToken` deterministically from the secret name and a time-truncated window. This ensures that a re-enqueued job step cannot create duplicate secret versions if the original write succeeded but the response was lost before acknowledgement.