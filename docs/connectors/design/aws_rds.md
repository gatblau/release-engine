# AWS Connector — Pseudo Code

The connector encapsulates all AWS API interactions required for operational management and troubleshooting of RDS PostgreSQL instances. Provider authentication is resolved via the credential's
`auth_type` field. All operations are idempotent by contract and return one of `Success`, `RetryableError`, or
`TerminalError`.

---

## Interface

```
CONNECTOR: AWSConnector
implements Connector interface

registered name: "aws"

func Call(ctx, op, params, credential) -> ConnectorResult:
  client = resolve_client(credential)
  // credential: {
  //   auth_type:       "iam_role" | "access_key" | "web_identity",
  //   region:          "us-east-1",
  //   role_arn:        (if auth_type=iam_role),
  //   access_key_id:   (if auth_type=access_key),
  //   secret_key:      (if auth_type=access_key),
  //   web_token:       (if auth_type=web_identity),
  //   session_name:    (optional — for audit trail in CloudTrail),
  // }

  switch op:

    // ── RDS / PostgreSQL ────────────────────────────────────────
    case "rds_get_instance":                  return rds_get_instance(ctx, client, params)
    case "rds_list_instances":                return rds_list_instances(ctx, client, params)
    case "rds_get_cluster":                   return rds_get_cluster(ctx, client, params)
    case "rds_list_clusters":                 return rds_list_clusters(ctx, client, params)
    case "rds_start_instance":                return rds_start_instance(ctx, client, params)
    case "rds_stop_instance":                 return rds_stop_instance(ctx, client, params)
    case "rds_reboot_instance":               return rds_reboot_instance(ctx, client, params)
    case "rds_failover_cluster":              return rds_failover_cluster(ctx, client, params)
    case "rds_modify_instance":               return rds_modify_instance(ctx, client, params)
    case "rds_modify_cluster":                return rds_modify_cluster(ctx, client, params)
    case "rds_get_pending_maintenance":       return rds_get_pending_maintenance(ctx, client, params)
    case "rds_apply_pending_maintenance":     return rds_apply_pending_maintenance(ctx, client, params)
    case "rds_create_snapshot":               return rds_create_snapshot(ctx, client, params)
    case "rds_get_snapshot":                  return rds_get_snapshot(ctx, client, params)
    case "rds_list_snapshots":                return rds_list_snapshots(ctx, client, params)
    case "rds_delete_snapshot":               return rds_delete_snapshot(ctx, client, params)
    case "rds_restore_from_snapshot":         return rds_restore_from_snapshot(ctx, client, params)
    case "rds_restore_to_point_in_time":      return rds_restore_to_point_in_time(ctx, client, params)
    case "rds_get_parameter_group":           return rds_get_parameter_group(ctx, client, params)
    case "rds_list_parameter_groups":         return rds_list_parameter_groups(ctx, client, params)
    case "rds_modify_parameter_group":        return rds_modify_parameter_group(ctx, client, params)
    case "rds_reset_parameter_group":         return rds_reset_parameter_group(ctx, client, params)
    case "rds_get_log_files":                 return rds_get_log_files(ctx, client, params)
    case "rds_download_log_file":             return rds_download_log_file(ctx, client, params)
    case "rds_get_events":                    return rds_get_events(ctx, client, params)
    case "rds_subscribe_event":               return rds_subscribe_event(ctx, client, params)
    case "rds_get_metrics":                   return rds_get_metrics(ctx, client, params)
    case "rds_enable_performance_insights":   return rds_enable_performance_insights(ctx, client, params)
    case "rds_get_performance_insights":      return rds_get_performance_insights(ctx, client, params)
    case "rds_list_recommendations":          return rds_list_recommendations(ctx, client, params)
    case "rds_add_tags":                      return rds_add_tags(ctx, client, params)
    case "rds_remove_tags":                   return rds_remove_tags(ctx, client, params)
    case "rds_rotate_master_password":        return rds_rotate_master_password(ctx, client, params)

    default:
      return TerminalError("unknown operation: " + op)
```

---

## Supported Operations

---

## RDS — Instance & Cluster Inspection

---

#### `rds_get_instance`

> Fetches full configuration and status for a single RDS instance. Used as the initial inspection step in database
> troubleshooting workflows to determine instance health, class, storage, and multi-AZ state.

```
func rds_get_instance(ctx, client, params):
  // params: db_instance_identifier

  resp = client.rds.DescribeDBInstances({
    DBInstanceIdentifier: params.db_instance_identifier,
  })

  if resp.error.code == "DBInstanceNotFound":
    return TerminalError("rds_instance_not_found")

  if is_retryable(resp.error): return RetryableError(resp.error)
  if resp.error: return TerminalError(resp.error)

  i = resp.DBInstances[0]
  return Success({
    identifier:          i.DBInstanceIdentifier,
    status:              i.DBInstanceStatus,
    engine:              i.Engine,
    engine_version:      i.EngineVersion,
    instance_class:      i.DBInstanceClass,
    storage_type:        i.StorageType,
    allocated_storage_gb: i.AllocatedStorage,
    multi_az:            i.MultiAZ,
    availability_zone:   i.AvailabilityZone,
    secondary_az:        i.SecondaryAvailabilityZone,
    endpoint: {
      address: i.Endpoint.Address,
      port:    i.Endpoint.Port,
    },
    parameter_group:     i.DBParameterGroups[0].DBParameterGroupName,
    option_group:        i.OptionGroupMemberships[0].OptionGroupName,
    publicly_accessible: i.PubliclyAccessible,
    deletion_protection: i.DeletionProtection,
    ca_certificate:      i.CACertificateIdentifier,
    performance_insights_enabled: i.PerformanceInsightsEnabled,
    backup_retention_days: i.BackupRetentionPeriod,
    preferred_backup_window:      i.PreferredBackupWindow,
    preferred_maintenance_window: i.PreferredMaintenanceWindow,
    pending_modified_values: i.PendingModifiedValues,
    latest_restorable_time:  i.LatestRestorableTime,
    created_at:          i.InstanceCreateTime,
  })
```

---

#### `rds_get_cluster`

> Fetches configuration and status for an Aurora cluster or RDS multi-AZ cluster. Used in cluster-level troubleshooting
> workflows to identify the writer endpoint, reader endpoints, and member instances.

```
func rds_get_cluster(ctx, client, params):
  // params: db_cluster_identifier

  resp = client.rds.DescribeDBClusters({
    DBClusterIdentifier: params.db_cluster_identifier,
  })

  if resp.error.code == "DBClusterNotFoundFault":
    return TerminalError("rds_cluster_not_found")

  if is_retryable(resp.error): return RetryableError(resp.error)
  if resp.error: return TerminalError(resp.error)

  c = resp.DBClusters[0]
  return Success({
    identifier:        c.DBClusterIdentifier,
    status:            c.Status,
    engine:            c.Engine,
    engine_version:    c.EngineVersion,
    engine_mode:       c.EngineMode,       // provisioned | serverless | parallelquery
    multi_az:          c.MultiAZ,
    writer_endpoint:   c.Endpoint,
    reader_endpoint:   c.ReaderEndpoint,
    port:              c.Port,
    members: c.DBClusterMembers.map(m => ({
      identifier:    m.DBInstanceIdentifier,
      is_writer:     m.IsClusterWriter,
      promotion_tier: m.PromotionTier,
    })),
    parameter_group:        c.DBClusterParameterGroup,
    backup_retention_days:  c.BackupRetentionPeriod,
    deletion_protection:    c.DeletionProtection,
    performance_insights_enabled: c.PerformanceInsightsEnabled,
    latest_restorable_time: c.EarliestRestorableTime,
    created_at:        c.ClusterCreateTime,
  })
```

---

## RDS — Instance Lifecycle Operations

---

#### `rds_start_instance`

> Starts a stopped RDS instance. Used in cost-management and environment wake-up workflows for non-production databases
> that are stopped outside business hours.

```
func rds_start_instance(ctx, client, params):
  // params: db_instance_identifier

  current = rds_get_instance(ctx, client, params)
  if current is error: return current

  if current.data.status == "available":
    return Success({ idempotent: true, status: "available" })

  if current.data.status not in ["stopped", "stopping"]:
    return TerminalError("instance_cannot_be_started_from_status: " + current.data.status)

  resp = client.rds.StartDBInstance({
    DBInstanceIdentifier: params.db_instance_identifier,
  })

  if is_retryable(resp.error): return RetryableError(resp.error)
  if resp.error: return TerminalError(resp.error)

  return Success({
    identifier: params.db_instance_identifier,
    status:     resp.DBInstance.DBInstanceStatus,
  })
```

---

#### `rds_stop_instance`

> Stops a running RDS instance. Optionally creates a snapshot before stopping. Used in cost-management workflows for
> non-production environments.

```
func rds_stop_instance(ctx, client, params):
  // params: db_instance_identifier, snapshot_identifier (optional)

  current = rds_get_instance(ctx, client, params)
  if current is error: return current

  if current.data.status == "stopped":
    return Success({ idempotent: true, status: "stopped" })

  if current.data.status != "available":
    return TerminalError("instance_cannot_be_stopped_from_status: " + current.data.status)

  resp = client.rds.StopDBInstance({
    DBInstanceIdentifier: params.db_instance_identifier,
    DBSnapshotIdentifier: params.snapshot_identifier,
  })

  if is_retryable(resp.error): return RetryableError(resp.error)
  if resp.error: return TerminalError(resp.error)

  return Success({
    identifier: params.db_instance_identifier,
    status:     resp.DBInstance.DBInstanceStatus,
  })
```

---

#### `rds_reboot_instance`

> Reboots an RDS instance, optionally forcing a failover to a secondary replica. Used in incident response workflows
> when an instance becomes unresponsive or parameter group changes require a reboot to take effect.

```
func rds_reboot_instance(ctx, client, params):
  // params: db_instance_identifier, force_failover (bool)

  resp = client.rds.RebootDBInstance({
    DBInstanceIdentifier: params.db_instance_identifier,
    ForceFailover:        params.force_failover ?? false,
  })

  if is_retryable(resp.error): return RetryableError(resp.error)
  if resp.error: return TerminalError(resp.error)

  return Success({
    identifier: params.db_instance_identifier,
    status:     resp.DBInstance.DBInstanceStatus,
  })
```

---

#### `rds_failover_cluster`

> Initiates a manual failover for an Aurora or multi-AZ cluster, promoting a reader instance to writer. Used in DR drill
> and incident response workflows to test or execute a regional failover.

```
func rds_failover_cluster(ctx, client, params):
  // params: db_cluster_identifier, target_db_instance_identifier (optional)

  resp = client.rds.FailoverDBCluster({
    DBClusterIdentifier:         params.db_cluster_identifier,
    TargetDBInstanceIdentifier:  params.target_db_instance_identifier,
  })

  if is_retryable(resp.error): return RetryableError(resp.error)
  if resp.error: return TerminalError(resp.error)

  return Success({
    cluster_identifier: params.db_cluster_identifier,
    status:             resp.DBCluster.Status,
  })
```

---

#### `rds_modify_instance`

> Modifies configuration of an RDS instance — instance class, storage, parameter group, backup window, or maintenance
> window. Used in right-sizing and compliance remediation workflows.

```
func rds_modify_instance(ctx, client, params):
  // params: db_instance_identifier,
  //         db_instance_class, allocated_storage_gb,
  //         db_parameter_group_name, backup_retention_period,
  //         preferred_backup_window, preferred_maintenance_window,
  //         multi_az, deletion_protection,
  //         ca_certificate_identifier,
  //         apply_immediately (bool — false = next maintenance window)

  resp = client.rds.ModifyDBInstance({
    DBInstanceIdentifier:     params.db_instance_identifier,
    DBInstanceClass:          params.db_instance_class,
    AllocatedStorage:         params.allocated_storage_gb,
    DBParameterGroupName:     params.db_parameter_group_name,
    BackupRetentionPeriod:    params.backup_retention_period,
    PreferredBackupWindow:    params.preferred_backup_window,
    PreferredMaintenanceWindow: params.preferred_maintenance_window,
    MultiAZ:                  params.multi_az,
    DeletionProtection:       params.deletion_protection,
    CACertificateIdentifier:  params.ca_certificate_identifier,
    ApplyImmediately:         params.apply_immediately ?? false,
  })

  if is_retryable(resp.error): return RetryableError(resp.error)
  if resp.error: return TerminalError(resp.error)

  return Success({
    identifier:              params.db_instance_identifier,
    status:                  resp.DBInstance.DBInstanceStatus,
    pending_modified_values: resp.DBInstance.PendingModifiedValues,
  })
```

---

## RDS — Maintenance & Patching

---

#### `rds_get_pending_maintenance`

> Returns pending maintenance actions for an RDS instance or cluster — engine upgrades, OS patches, and certificate
> rotations. Used in maintenance planning workflows to determine what will be applied during the next maintenance window.

```
func rds_get_pending_maintenance(ctx, client, params):
  // params: db_instance_identifier (optional — omit to list all resources)

  filters = params.db_instance_identifier
    ? [{ Name: "db-instance-id", Values: [params.db_instance_identifier] }]
    : []

  resp = client.rds.DescribePendingMaintenanceActions({
    Filters: filters,
  })

  if is_retryable(resp.error): return RetryableError(resp.error)
  if resp.error: return TerminalError(resp.error)

  return Success({
    pending_actions: resp.PendingMaintenanceActions.map(r => ({
      resource_identifier: r.ResourceIdentifier,
      actions: r.PendingMaintenanceActionDetails.map(a => ({
        action:            a.Action,
        description:       a.Description,
        auto_applied_after: a.AutoAppliedAfterDate,
        forced_apply_date:  a.ForcedApplyDate,
        current_apply_date: a.CurrentApplyDate,
      })),
    })),
  })
```

---

#### `rds_apply_pending_maintenance`

> Immediately applies a pending maintenance action rather than waiting for the scheduled maintenance window. Used in
> patch compliance workflows where a CVE requires an out-of-band engine patch.

```
func rds_apply_pending_maintenance(ctx, client, params):
  // params: resource_identifier (ARN), apply_action (string), opt_in_type
  // opt_in_type: "immediate" | "next-maintenance" | "undo-opt-in"

  resp = client.rds.ApplyPendingMaintenanceAction({
    ResourceIdentifier: params.resource_identifier,
    ApplyAction:        params.apply_action,
    OptInType:          params.opt_in_type ?? "immediate",
  })

  if is_retryable(resp.error): return RetryableError(resp.error)
  if resp.error: return TerminalError(resp.error)

  return Success({
    resource_identifier: params.resource_identifier,
    action:              params.apply_action,
    result:              resp.ResourcePendingMaintenanceActions,
  })
```

---

## RDS — Snapshot & Recovery

---

#### `rds_create_snapshot`

> Creates a manual snapshot of an RDS instance or cluster. Used in pre-maintenance backup workflows and as a safety gate
> before applying destructive schema migrations.

```
func rds_create_snapshot(ctx, client, params):
  // params: db_instance_identifier OR db_cluster_identifier,
  //         snapshot_identifier, tags{}

  if params.db_cluster_identifier is set:
    resp = client.rds.CreateDBClusterSnapshot({
      DBClusterIdentifier:         params.db_cluster_identifier,
      DBClusterSnapshotIdentifier: params.snapshot_identifier,
      Tags: to_tag_list(params.tags),
    })
    key = "DBClusterSnapshot"
  else:
    resp = client.rds.CreateDBSnapshot({
      DBInstanceIdentifier: params.db_instance_identifier,
      DBSnapshotIdentifier: params.snapshot_identifier,
      Tags: to_tag_list(params.tags),
    })
    key = "DBSnapshot"

  if resp.error.code in ["DBSnapshotAlreadyExists", "DBClusterSnapshotAlreadyExistsFault"]:
    return Success({ idempotent: true, snapshot_identifier: params.snapshot_identifier })

  if is_retryable(resp.error): return RetryableError(resp.error)
  if resp.error: return TerminalError(resp.error)

  snap = resp[key]
  return Success({
    snapshot_identifier: snap.DBSnapshotIdentifier ?? snap.DBClusterSnapshotIdentifier,
    status:              snap.Status,
    engine:              snap.Engine,
    allocated_storage_gb: snap.AllocatedStorage,
    created_at:          snap.SnapshotCreateTime,
  })
```

---

#### `rds_restore_to_point_in_time`

> Restores an RDS instance to a specific point in time within the backup retention window. Used in data recovery
> workflows following accidental data deletion or corruption events.

```
func rds_restore_to_point_in_time(ctx, client, params):
  // params: source_db_instance_identifier,
  //         target_db_instance_identifier,
  //         restore_time (ISO8601) OR use_latest_restorable_time (bool),
  //         db_instance_class, multi_az, publicly_accessible,
  //         db_subnet_group_name, vpc_security_group_ids[]

  resp = client.rds.RestoreDBInstanceToPointInTime({
    SourceDBInstanceIdentifier: params.source_db_instance_identifier,
    TargetDBInstanceIdentifier: params.target_db_instance_identifier,
    RestoreTime:                params.restore_time,
    UseLatestRestorableTime:    params.use_latest_restorable_time ?? false,
    DBInstanceClass:            params.db_instance_class,
    MultiAZ:                    params.multi_az ?? false,
    PubliclyAccessible:         params.publicly_accessible ?? false,
    DBSubnetGroupName:          params.db_subnet_group_name,
    VpcSecurityGroupIds:        params.vpc_security_group_ids,
  })

  if is_retryable(resp.error): return RetryableError(resp.error)
  if resp.error: return TerminalError(resp.error)

  return Success({
    identifier: resp.DBInstance.DBInstanceIdentifier,
    status:     resp.DBInstance.DBInstanceStatus,
    endpoint:   resp.DBInstance.Endpoint,
  })
```

---

## RDS — Parameter Groups

---

#### `rds_modify_parameter_group`

> Updates one or more parameters within an RDS parameter group. Used in performance tuning and compliance workflows to
> apply PostgreSQL configuration changes such as `work_mem`, `max_connections`, or `log_min_duration_statement`.

```
func rds_modify_parameter_group(ctx, client, params):
  // params: db_parameter_group_name,
  //         parameters[]: { name, value, apply_method (immediate|pending-reboot) }

  resp = client.rds.ModifyDBParameterGroup({
    DBParameterGroupName: params.db_parameter_group_name,
    Parameters: params.parameters.map(p => ({
      ParameterName:  p.name,
      ParameterValue: p.value,
      ApplyMethod:    p.apply_method ?? "pending-reboot",
    })),
  })

  if is_retryable(resp.error): return RetryableError(resp.error)
  if resp.error: return TerminalError(resp.error)

  return Success({
    db_parameter_group_name: resp.DBParameterGroupName,
    parameters_applied:      params.parameters,
  })
```

---

#### `rds_reset_parameter_group`

> Resets one or more parameters in a parameter group back to engine defaults. Used in incident response workflows when a
> misconfigured parameter is causing instability.

```
func rds_reset_parameter_group(ctx, client, params):
  // params: db_parameter_group_name,
  //         parameters[]: { name } OR reset_all_parameters (bool)

  resp = client.rds.ResetDBParameterGroup({
    DBParameterGroupName:  params.db_parameter_group_name,
    ResetAllParameters:    params.reset_all_parameters ?? false,
    Parameters: (params.parameters ?? []).map(p => ({
      ParameterName: p.name,
      ApplyMethod:   "pending-reboot",
    })),
  })

  if is_retryable(resp.error): return RetryableError(resp.error)
  if resp.error: return TerminalError(resp.error)

  return Success({ db_parameter_group_name: resp.DBParameterGroupName })
```

---

## RDS — Logs, Events & Observability

---

#### `rds_get_log_files`

> Lists available log files for an RDS instance. Used as the discovery step before downloading specific logs for
> forensic analysis in incident response workflows.

```
func rds_get_log_files(ctx, client, params):
  // params: db_instance_identifier,
  //         filename_contains (optional filter),
  //         file_last_written (optional Unix ms timestamp filter),
  //         max_records

  resp = client.rds.DescribeDBLogFiles({
    DBInstanceIdentifier: params.db_instance_identifier,
    FilenameContains:     params.filename_contains,
    FileLastWritten:      params.file_last_written,
    MaxRecords:           params.max_records ?? 100,
})

if is_retryable(resp.error): return RetryableError(resp.error)
if resp.error: return TerminalError(resp.error)

return Success({
db_instance_identifier: params.db_instance_identifier,
log_files: resp.DescribeDBLogFiles.map(f => ({
filename:       f.LogFileName,
size_bytes:     f.Size,
last_written:   f.LastWritten,
})),
})
```

---

#### `rds_download_log_file`

> Downloads the contents of a specific RDS log file in paginated chunks. Used in incident response and performance
> investigation workflows to retrieve PostgreSQL logs — including slow query logs, error logs, and autovacuum logs — for
> analysis.

```
func rds_download_log_file(ctx, client, params):
// params: db_instance_identifier, log_filename,
//         marker (pagination token), number_of_lines

resp = client.rds.DownloadDBLogFilePortion({
DBInstanceIdentifier: params.db_instance_identifier,
LogFileName:          params.log_filename,
Marker:               params.marker ?? "0",
NumberOfLines:        params.number_of_lines ?? 1000,
})

if is_retryable(resp.error): return RetryableError(resp.error)
if resp.error: return TerminalError(resp.error)

return Success({
db_instance_identifier: params.db_instance_identifier,
log_filename:    params.log_filename,
log_contents:    resp.LogFileData,
marker:          resp.Marker,
additional_data_pending: resp.AdditionalDataPending,
})
```

---

#### `rds_get_events`

> Retrieves RDS events for an instance, cluster, parameter group, or snapshot within a time range. Used in root cause
> analysis workflows to reconstruct what AWS-level events occurred around the time of an incident.

```
func rds_get_events(ctx, client, params):
// params: source_identifier (optional),
//         source_type: "db-instance" | "db-cluster" | "db-parameter-group" |
//                      "db-snapshot" | "db-cluster-snapshot",
//         start_time (ISO8601), end_time (ISO8601),
//         event_categories[], max_records, marker

resp = client.rds.DescribeEvents({
SourceIdentifier:  params.source_identifier,
SourceType:        params.source_type,
StartTime:         params.start_time,
EndTime:           params.end_time,
EventCategories:   params.event_categories ?? [],
MaxRecords:        params.max_records ?? 100,
Marker:            params.marker,
})

if is_retryable(resp.error): return RetryableError(resp.error)
if resp.error: return TerminalError(resp.error)

return Success({
events: resp.Events.map(e => ({
source_identifier: e.SourceIdentifier,
source_type:       e.SourceType,
message:           e.Message,
event_categories:  e.EventCategories,
date:              e.Date,
})),
marker: resp.Marker,
})
```

---

#### `rds_get_metrics`

> Fetches CloudWatch metrics for an RDS instance or cluster over a specified time range. Used in performance
> investigation and capacity planning workflows to retrieve key PostgreSQL indicators such as `CPUUtilization`,
`DatabaseConnections`, `FreeStorageSpace`, `ReadLatency`, `WriteLatency`, and `DiskQueueDepth`.

```
func rds_get_metrics(ctx, client, params):
// params: db_identifier,
//         metric_names[],
//         start_time (ISO8601), end_time (ISO8601),
//         period_seconds,
//         stat: "Average" | "Maximum" | "Minimum" | "Sum" | "SampleCount"

results = {}

for metric_name in params.metric_names:
resp = client.cloudwatch.GetMetricStatistics({
Namespace:  "AWS/RDS",
MetricName: metric_name,
Dimensions: [{ Name: "DBInstanceIdentifier", Value: params.db_identifier }],
StartTime:  params.start_time,
EndTime:    params.end_time,
Period:     params.period_seconds ?? 60,
Statistics: [params.stat ?? "Average"],
})

    if is_retryable(resp.error): return RetryableError(resp.error)
    if resp.error: return TerminalError(resp.error)

    results[metric_name] = resp.Datapoints
      .sort_by(d => d.Timestamp)
      .map(d => ({
        timestamp: d.Timestamp,
        value:     d[params.stat ?? "Average"],
        unit:      d.Unit,
      }))

return Success({
db_identifier: params.db_identifier,
period_seconds: params.period_seconds ?? 60,
metrics: results,
})
```

---

#### `rds_get_performance_insights`

> Queries Performance Insights for an RDS instance to return top SQL statements, wait events, and database load over a
> time range. Used in active performance investigation workflows to identify the root cause of query latency or connection
> saturation.

```
func rds_get_performance_insights(ctx, client, params):
// params: db_instance_identifier,
//         start_time (ISO8601), end_time (ISO8601),
//         period_seconds,
//         metric: "db.load.avg" | "db.sampledload.avg",
//         group_by: "db.sql" | "db.wait_event" | "db.user" | "db.host",
//         limit

resource_id = rds_get_instance(ctx, client, {
db_instance_identifier: params.db_instance_identifier
}).data.dbi_resource_id

resp = client.pi.GetResourceMetrics({
ServiceType:  "RDS",
Identifier:   resource_id,
StartTime:    params.start_time,
EndTime:      params.end_time,
PeriodInSeconds: params.period_seconds ?? 60,
MetricQueries: [{
Metric: params.metric ?? "db.load.avg",
GroupBy: {
Group:  params.group_by ?? "db.wait_event",
Limit:  params.limit ?? 10,
},
}],
})

if is_retryable(resp.error): return RetryableError(resp.error)
if resp.error: return TerminalError(resp.error)

return Success({
db_instance_identifier: params.db_instance_identifier,
metric:     params.metric ?? "db.load.avg",
group_by:   params.group_by ?? "db.wait_event",
data_points: resp.MetricList[0].DataPoints.map(d => ({
timestamp: d.Timestamp,
value:     d.Value,
})),
top_items: resp.MetricList[0].Groups.map(g => ({
dimension:  g.Dimensions,
data_points: g.DataPoints,
})),
})
```

---

#### `rds_list_recommendations`

> Retrieves AWS-generated recommendations for RDS resources — covering performance, security, cost, and reliability.
> Used in proactive health review workflows to surface actionable findings without manual console inspection.

```
func rds_list_recommendations(ctx, client, params):
// params: db_instance_identifier (optional),
//         type_filter: "performance" | "security" | "reliability" | "cost",
//         status_filter: "active" | "dismissed" | "resolved"

resp = client.rds.DescribeRecommendations({
Filters: [
params.db_instance_identifier
? { Name: "dbi-resource-id", Values: [params.db_instance_identifier] }
: null,
params.type_filter
? { Name: "recommendation-type", Values: [params.type_filter] }
: null,
params.status_filter
? { Name: "status", Values: [params.status_filter] }
: null,
].filter(f => f is not null),
})

if is_retryable(resp.error): return RetryableError(resp.error)
if resp.error: return TerminalError(resp.error)

return Success({
recommendations: resp.DBRecommendations.map(r => ({
recommendation_id: r.RecommendationId,
type:              r.TypeId,
severity:          r.Severity,
status:            r.Status,
description:       r.Description,
reason:            r.Reason,
suggested_actions: r.RecommendedActions,
created_at:        r.CreatedTime,
updated_at:        r.UpdatedTime,
})),
})
```

---

#### `rds_rotate_master_password`

> Triggers rotation of the master user password for an RDS instance or cluster via AWS Secrets Manager. Used in security
> incident response and routine credential rotation workflows.

```
func rds_rotate_master_password(ctx, client, params):
// params: db_instance_identifier OR db_cluster_identifier,
//         master_user_secret_arn (must be managed by Secrets Manager)

if params.db_cluster_identifier is set:
resp = client.rds.ModifyDBCluster({
DBClusterIdentifier:       params.db_cluster_identifier,
RotateMasterUserPassword:  true,
MasterUserSecretKmsKeyId:  params.kms_key_id,
ApplyImmediately:          true,
})
else:
resp = client.rds.ModifyDBInstance({
DBInstanceIdentifier:      params.db_instance_identifier,
RotateMasterUserPassword:  true,
MasterUserSecretKmsKeyId:  params.kms_key_id,
ApplyImmediately:          true,
})

if is_retryable(resp.error): return RetryableError(resp.error)
if resp.error: return TerminalError(resp.error)

return Success({
identifier:             params.db_instance_identifier ?? params.db_cluster_identifier,
rotation_status:        "initiated",
master_user_secret_arn: params.master_user_secret_arn,
})
```

---


## Shared Helpers

```
func is_retryable(error) -> bool:
if error is null: return false
retryable_codes = [
"RequestLimitExceeded", "Throttling", "ThrottlingException",
"RequestThrottledException", "TooManyRequestsException",
"ProvisionedThroughputExceededException", "TransactionInProgressException",
"ServiceUnavailable", "InternalError", "InternalServerError",
]
return error.code in retryable_codes or error.http_status in [429, 500, 502, 503]

func to_tag_map(tags[]) -> map:
return tags.reduce((acc, t) => acc[t.Key] = t.Value, {})

func to_tag_list(tags{}) -> list:
return tags.entries().map(([k, v]) => ({ Key: k, Value: v }))

func new_idempotency_token() -> string:
return uuid_v4()
```

---

## Error Classification Reference

| HTTP Status / Error Code                                  | Classification       | Engine Behaviour                          |
|-----------------------------------------------------------|----------------------|-------------------------------------------|
| `2xx`                                                     | Success              | Advance to next step                      |
| `ResourceNotFoundException` / `DBInstanceNotFound` on get | TerminalError        | Resource absent — fail the step           |
| `ResourceNotFoundException` on delete                     | Success (idempotent) | Already absent — treat as done            |
| `AlreadyExists` / `InvalidPermission.Duplicate`           | Success (idempotent) | Already present — return existing         |
| `InvalidPermission.NotFound` on revoke                    | Success (idempotent) | Already absent — treat as done            |
| `InvalidInstanceID.NotFound`                              | TerminalError        | Instance does not exist — fail the step   |
| `InvalidParameterValue` / `ValidationError`               | TerminalError        | Malformed request — fail the step         |
| `UnauthorizedOperation` / `AccessDenied`                  | TerminalError        | IAM permissions missing — fail the step   |
| `OperationNotPermitted`                                   | TerminalError        | State precondition failed — fail the step |
| `RequestLimitExceeded` / `Throttling`                     | RetryableError       | Re-enqueue with exponential backoff       |
| `InternalError` / `ServiceUnavailable`                    | RetryableError       | Re-enqueue with exponential backoff       |
| `429`                                                     | RetryableError       | Re-enqueue with backoff                   |
| `5xx`                                                     | RetryableError       | Re-enqueue with exponential backoff       |
| Network timeout                                           | RetryableError       | Re-enqueue with backoff                   |

---

## Notes

**Authentication.** The `auth_type` field in the credential controls how AWS SDK credentials are initialised. `iam_role`
triggers `AssumeRole` via STS before each invocation. `web_identity` exchanges the provided OIDC token for temporary STS
credentials. `access_key` uses the provided key pair directly. Temporary credentials are cached for their validity
period and refreshed automatically. Secret material is scrubbed from guarded memory immediately after the SDK session is
established.

**Regional Clients.** Each AWS service client is constructed with the `region` from the credential. Operations that are
global by nature — such as IAM — use `us-east-1` as the fixed region. Cross-region operations require a separate
connector invocation with the target region specified in the credential.

**State Guards.** Lifecycle operations (`start`, `stop`, `terminate`, `modify`) validate the current resource state
before issuing the mutating API call. This prevents wasted API calls and surfaces actionable errors to the module rather
than relying on AWS to return a state-conflict error.

**Polling Contract.** Operations that initiate asynchronous work — EKS version upgrades, RDS snapshot creation, EBS
volume modification — return an identifier (`update_id`, `snapshot_identifier`, `volume_id`) immediately. The calling
module is responsible for polling the corresponding `get` operation at an appropriate interval. The connector makes no
assumptions about polling frequency or timeout.

**CloudWatch Client.** Metric operations (`rds_get_metrics`, `ec2_get_instance_metrics`) use the CloudWatch service
client rather than the service-specific client. The same credential and region are used. Metric queries are issued
sequentially per metric name to keep response payloads bounded and avoid CloudWatch `GetMetricStatistics` multi-metric
limitations.

**SSM Prerequisite.** `ec2_send_ssm_command` requires the SSM Agent to be running on the target instance and the
instance's IAM role to carry the `AmazonSSMManagedInstanceCore` policy. The connector does not verify this
precondition — the calling module is expected to assert it via `ec2_get_instance` and a prior IAM check before invoking
the command operation.