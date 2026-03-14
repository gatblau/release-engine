# AWS Connector — Pseudo Code

The connector encapsulates all AWS API interactions required for operational management and troubleshooting of EKS
clusters. Provider authentication is resolved via the credential's
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

    // ── EKS ────────────────────────────────────────────────────
    case "eks_get_cluster":                   return eks_get_cluster(ctx, client, params)
    case "eks_list_clusters":                 return eks_list_clusters(ctx, client, params)
    case "eks_describe_nodegroup":            return eks_describe_nodegroup(ctx, client, params)
    case "eks_list_nodegroups":               return eks_list_nodegroups(ctx, client, params)
    case "eks_update_nodegroup_config":       return eks_update_nodegroup_config(ctx, client, params)
    case "eks_scale_nodegroup":               return eks_scale_nodegroup(ctx, client, params)
    case "eks_get_nodegroup_scaling":         return eks_get_nodegroup_scaling(ctx, client, params)
    case "eks_cordon_nodegroup":              return eks_cordon_nodegroup(ctx, client, params)
    case "eks_drain_nodegroup":               return eks_drain_nodegroup(ctx, client, params)
    case "eks_update_cluster_version":        return eks_update_cluster_version(ctx, client, params)
    case "eks_get_upgrade_insights":          return eks_get_upgrade_insights(ctx, client, params)
    case "eks_list_updates":                  return eks_list_updates(ctx, client, params)
    case "eks_get_update":                    return eks_get_update(ctx, client, params)
    case "eks_list_addons":                   return eks_list_addons(ctx, client, params)
    case "eks_describe_addon":                return eks_describe_addon(ctx, client, params)
    case "eks_update_addon":                  return eks_update_addon(ctx, client, params)
    case "eks_create_addon":                  return eks_create_addon(ctx, client, params)
    case "eks_delete_addon":                  return eks_delete_addon(ctx, client, params)
    case "eks_list_fargate_profiles":         return eks_list_fargate_profiles(ctx, client, params)
    case "eks_describe_fargate_profile":      return eks_describe_fargate_profile(ctx, client, params)
    case "eks_get_cluster_logging":           return eks_get_cluster_logging(ctx, client, params)
    case "eks_update_cluster_logging":        return eks_update_cluster_logging(ctx, client, params)
    case "eks_generate_kubeconfig":           return eks_generate_kubeconfig(ctx, client, params)
    case "eks_get_access_entry":              return eks_get_access_entry(ctx, client, params)
    case "eks_list_access_entries":           return eks_list_access_entries(ctx, client, params)
    case "eks_create_access_entry":           return eks_create_access_entry(ctx, client, params)
    case "eks_delete_access_entry":           return eks_delete_access_entry(ctx, client, params)
    case "eks_associate_access_policy":       return eks_associate_access_policy(ctx, client, params)
    case "eks_disassociate_access_policy":    return eks_disassociate_access_policy(ctx, client, params)
    case "eks_list_insights":                 return eks_list_insights(ctx, client, params)
    case "eks_get_insight":                   return eks_get_insight(ctx, client, params)

    default:
      return TerminalError("unknown operation: " + op)
```

---

## Supported Operations

---

## EKS — Cluster Inspection

---

#### `eks_get_cluster`

> Fetches full metadata for an EKS cluster including endpoint, Kubernetes version, OIDC config, networking, and status.
> Used as the initial inspection step in any cluster-scoped troubleshooting workflow.

```
func eks_get_cluster(ctx, client, params):
  // params: cluster_name

  resp = client.eks.DescribeCluster({ Name: params.cluster_name })

  if resp.error.code == "ResourceNotFoundException":
    return TerminalError("cluster_not_found")

  if is_retryable(resp.error):
    return RetryableError(resp.error)

  if resp.error:
    return TerminalError(resp.error)

  c = resp.cluster
  return Success({
    name:              c.Name,
    status:            c.Status,           // CREATING|ACTIVE|DELETING|FAILED|UPDATING
    kubernetes_version: c.Version,
    endpoint:          c.Endpoint,
    role_arn:          c.RoleArn,
    oidc_issuer:       c.Identity.Oidc.Issuer,
    platform_version:  c.PlatformVersion,
    logging:           c.Logging,
    networking: {
      vpc_id:          c.ResourcesVpcConfig.VpcId,
      subnet_ids:      c.ResourcesVpcConfig.SubnetIds,
      security_group_ids: c.ResourcesVpcConfig.SecurityGroupIds,
      endpoint_public_access:  c.ResourcesVpcConfig.EndpointPublicAccess,
      endpoint_private_access: c.ResourcesVpcConfig.EndpointPrivateAccess,
    },
    tags:              c.Tags,
    created_at:        c.CreatedAt,
  })
```

---

#### `eks_list_clusters`

> Lists all EKS cluster names in the account and region. Used in multi-cluster audit and fleet-wide health check
> workflows.

```
func eks_list_clusters(ctx, client, params):
  // params: max_results, next_token

  clusters = []
  next_token = params.next_token

  loop:
    resp = client.eks.ListClusters({
      MaxResults: params.max_results ?? 100,
      NextToken:  next_token,
    })

    if is_retryable(resp.error): return RetryableError(resp.error)
    if resp.error: return TerminalError(resp.error)

    clusters += resp.Clusters
    next_token = resp.NextToken
    if next_token is null or params.max_results is set: break

  return Success({
    clusters:   clusters,
    next_token: next_token,
  })
```

---

#### `eks_get_upgrade_insights`

> Returns upgrade readiness insights for a cluster — deprecation warnings, API version conflicts, and addon
> compatibility. Used as a mandatory pre-flight check before initiating a Kubernetes version upgrade workflow.

```
func eks_get_upgrade_insights(ctx, client, params):
  // params: cluster_name, kubernetes_version (target)

  resp = client.eks.ListInsights({
    ClusterName: params.cluster_name,
    Filter: {
      Category:            ["UPGRADE_READINESS"],
      KubernetesVersions:  [params.kubernetes_version],
    },
  })

  if is_retryable(resp.error): return RetryableError(resp.error)
  if resp.error: return TerminalError(resp.error)

  return Success({
    cluster_name:       params.cluster_name,
    target_version:     params.kubernetes_version,
    insights: resp.Insights.map(i => {
      id:               i.Id,
      name:             i.Name,
      status:           i.InsightStatus.Status,    // PASSING|WARNING|ERROR|UNKNOWN
      severity:         i.InsightStatus.Reason,
      description:      i.Description,
      recommendation:   i.Recommendation,
      resources:        i.Resources,
    }),
    blocking: resp.Insights.filter(i => i.InsightStatus.Status == "ERROR"),
  })
```

---

#### `eks_list_updates`

> Lists all in-progress and historical updates for an EKS cluster or nodegroup. Used to determine whether an update is
> already in flight before submitting a new one, preventing conflicting concurrent mutations.

```
func eks_list_updates(ctx, client, params):
  // params: cluster_name, nodegroup_name (optional), addon_name (optional),
  //         max_results, next_token

  resp = client.eks.ListUpdates({
    Name:          params.cluster_name,
    NodegroupName: params.nodegroup_name,
    AddonName:     params.addon_name,
    MaxResults:    params.max_results ?? 100,
    NextToken:     params.next_token,
  })

  if is_retryable(resp.error): return RetryableError(resp.error)
  if resp.error: return TerminalError(resp.error)

  return Success({
    update_ids: resp.UpdateIds,
    next_token: resp.NextToken,
  })
```

---

#### `eks_get_update`

> Fetches the status and error detail for a specific EKS update operation. Used in polling loops to gate on update
> completion before proceeding to the next workflow step.

```
func eks_get_update(ctx, client, params):
  // params: cluster_name, update_id, nodegroup_name (optional)

  resp = client.eks.DescribeUpdate({
    Name:          params.cluster_name,
    UpdateId:      params.update_id,
    NodegroupName: params.nodegroup_name,
  })

  if is_retryable(resp.error): return RetryableError(resp.error)
  if resp.error: return TerminalError(resp.error)

  u = resp.Update
  return Success({
    update_id:   u.Id,
    status:      u.Status,           // InProgress|Failed|Cancelled|Successful
    type:        u.Type,
    params:      u.Params,
    errors:      u.Errors,
    created_at:  u.CreatedAt,
  })
```

---

## EKS — Cluster Operations

---

#### `eks_update_cluster_version`

> Initiates a Kubernetes control plane version upgrade for an EKS cluster. Must be preceded by
`eks_get_upgrade_insights` to assert no blocking insights exist. Returns an `update_id` for polling via
`eks_get_update`.

```
func eks_update_cluster_version(ctx, client, params):
  // params: cluster_name, kubernetes_version, client_request_token (idempotency)

  // Guard: reject if another update is already in progress
  updates = eks_list_updates(ctx, client, { cluster_name: params.cluster_name })
  if updates is error: return updates

  in_progress = updates.data.update_ids.filter(id =>
    eks_get_update(ctx, client, {
      cluster_name: params.cluster_name,
      update_id: id
    }).data.status == "InProgress"
  )
  if in_progress is not empty:
    return TerminalError("cluster_update_already_in_progress")

  resp = client.eks.UpdateClusterVersion({
    Name:               params.cluster_name,
    Version:            params.kubernetes_version,
    ClientRequestToken: params.client_request_token ?? new_idempotency_token(),
  })

  if is_retryable(resp.error): return RetryableError(resp.error)
  if resp.error: return TerminalError(resp.error)

  return Success({
    cluster_name: params.cluster_name,
    update_id:    resp.Update.Id,
    status:       resp.Update.Status,
  })
```

---

#### `eks_get_cluster_logging`

> Returns the current control plane logging configuration for an EKS cluster. Used in compliance and audit workflows to
> verify that required log types are enabled before flagging a cluster as production-ready.

```
func eks_get_cluster_logging(ctx, client, params):
  // params: cluster_name

  cluster = eks_get_cluster(ctx, client, { cluster_name: params.cluster_name })
  if cluster is error: return cluster

  return Success({
    cluster_name: params.cluster_name,
    logging:      cluster.data.logging,
    // enabled types: api | audit | authenticator | controllerManager | scheduler
  })
```

---

#### `eks_update_cluster_logging`

> Enables or disables specific control plane log types for an EKS cluster. Used in incident response workflows to
> temporarily enable verbose logging such as `audit` or `authenticator` for forensic investigation.

```
func eks_update_cluster_logging(ctx, client, params):
  // params: cluster_name, enable_types[], disable_types[]

  resp = client.eks.UpdateClusterConfig({
    Name: params.cluster_name,
    Logging: {
      ClusterLogging: [
        { Types: params.enable_types,  Enabled: true  },
        { Types: params.disable_types, Enabled: false },
      ],
    },
    ClientRequestToken: new_idempotency_token(),
  })

  if is_retryable(resp.error): return RetryableError(resp.error)
  if resp.error: return TerminalError(resp.error)

  return Success({
    cluster_name: params.cluster_name,
    update_id:    resp.Update.Id,
    status:       resp.Update.Status,
  })
```

---

#### `eks_generate_kubeconfig`

> Generates a short-lived kubeconfig token for API server access. Used in workflows that need to interact with the
> Kubernetes API directly — for example, verifying pod readiness after a node group scaling event.

```
func eks_generate_kubeconfig(ctx, client, params):
  // params: cluster_name, role_arn (optional — for assume-role auth)

  cluster = eks_get_cluster(ctx, client, { cluster_name: params.cluster_name })
  if cluster is error: return cluster

  token_resp = client.eks.GetToken({
    ClusterName: params.cluster_name,
    RoleArn:     params.role_arn,
  })

  if is_retryable(token_resp.error): return RetryableError(token_resp.error)
  if token_resp.error: return TerminalError(token_resp.error)

  return Success({
    cluster_name:       params.cluster_name,
    endpoint:           cluster.data.endpoint,
    ca_data:            cluster.data.certificate_authority,
    token:              token_resp.Token,
    token_expiry:       token_resp.TokenExpiration,   // short-lived; ~15 minutes
  })

  // Caller note: token is returned in guarded scope.
  // It must not be logged, persisted, or passed to non-trusted subsystems.
```

---

## EKS — Nodegroup Operations

---

#### `eks_describe_nodegroup`

> Fetches full configuration and status for a managed nodegroup including instance types, AMI release version, scaling
> config, labels, and taints. Used as the initial inspection step in node-level troubleshooting workflows.

```
func eks_describe_nodegroup(ctx, client, params):
  // params: cluster_name, nodegroup_name

  resp = client.eks.DescribeNodegroup({
    ClusterName:   params.cluster_name,
    NodegroupName: params.nodegroup_name,
  })

  if resp.error.code == "ResourceNotFoundException":
    return TerminalError("nodegroup_not_found")

  if is_retryable(resp.error): return RetryableError(resp.error)
  if resp.error: return TerminalError(resp.error)

  ng = resp.Nodegroup
  return Success({
    nodegroup_name:   ng.NodegroupName,
    cluster_name:     ng.ClusterName,
    status:           ng.Status,     // CREATING|ACTIVE|UPDATING|DELETING|DEGRADED|DELETE_FAILED
    capacity_type:    ng.CapacityType,
    instance_types:   ng.InstanceTypes,
    ami_type:         ng.AmiType,
    release_version:  ng.ReleaseVersion,
    scaling_config:   ng.ScalingConfig,
    labels:           ng.Labels,
    taints:           ng.Taints,
    node_role:        ng.NodeRole,
    subnets:          ng.Subnets,
    health: {
      issues: ng.Health.Issues.map(i => ({
        code:     i.Code,
        message:  i.Message,
        resource_ids: i.ResourceIds,
      })),
    },
    created_at:       ng.CreatedAt,
    modified_at:      ng.ModifiedAt,
  })
```

---

#### `eks_list_nodegroups`

> Lists all nodegroups associated with a cluster. Used in fleet inspection and pre-upgrade workflows to enumerate all
> groups that require sequential version updates.

```
func eks_list_nodegroups(ctx, client, params):
  // params: cluster_name, max_results, next_token

  resp = client.eks.ListNodegroups({
    ClusterName: params.cluster_name,
    MaxResults:  params.max_results ?? 100,
    NextToken:   params.next_token,
  })

  if is_retryable(resp.error): return RetryableError(resp.error)
  if resp.error: return TerminalError(resp.error)

  return Success({
    cluster_name:    params.cluster_name,
    nodegroup_names: resp.Nodegroups,
    next_token:      resp.NextToken,
  })
```

---

#### `eks_scale_nodegroup`

> Sets the desired, minimum, and maximum node counts for a managed nodegroup. Used in capacity management workflows
> triggered by scheduled scaling events or alert-driven scale-out responses.

```
func eks_scale_nodegroup(ctx, client, params):
  // params: cluster_name, nodegroup_name,
  //         desired_size, min_size, max_size

  // Guard: fetch current config and skip if already at desired state
  current = eks_describe_nodegroup(ctx, client, {
    cluster_name:   params.cluster_name,
    nodegroup_name: params.nodegroup_name,
  })
  if current is error: return current

  sc = current.data.scaling_config
  if sc.DesiredSize == params.desired_size
  and sc.MinSize    == params.min_size
  and sc.MaxSize    == params.max_size:
    return Success({ idempotent: true, scaling_config: sc })

  resp = client.eks.UpdateNodegroupConfig({
    ClusterName:   params.cluster_name,
    NodegroupName: params.nodegroup_name,
    ScalingConfig: {
      DesiredSize: params.desired_size,
      MinSize:     params.min_size,
      MaxSize:     params.max_size,
    },
    ClientRequestToken: new_idempotency_token(),
  })

  if is_retryable(resp.error): return RetryableError(resp.error)
  if resp.error: return TerminalError(resp.error)

  return Success({
    nodegroup_name: params.nodegroup_name,
    update_id:      resp.Update.Id,
    status:         resp.Update.Status,
  })
```

---

#### `eks_get_nodegroup_scaling`

> Returns current scaling configuration and live node counts for a nodegroup. Used in monitoring and scaling-decision
> workflows to determine headroom before triggering a scale event.

```
func eks_get_nodegroup_scaling(ctx, client, params):
  // params: cluster_name, nodegroup_name

  ng = eks_describe_nodegroup(ctx, client, params)
  if ng is error: return ng

  return Success({
    nodegroup_name: params.nodegroup_name,
    desired_size:   ng.data.scaling_config.DesiredSize,
    min_size:       ng.data.scaling_config.MinSize,
    max_size:       ng.data.scaling_config.MaxSize,
    status:         ng.data.status,
    health_issues:  ng.data.health.issues,
  })
```

---

#### `eks_update_nodegroup_config`

> Updates labels, taints, or scaling configuration for a managed nodegroup. Used in operational workflows that need to
> apply scheduling constraints or workload isolation rules to a group of nodes.

```
func eks_update_nodegroup_config(ctx, client, params):
  // params: cluster_name, nodegroup_name,
  //         labels_to_add{}, labels_to_remove[],
  //         taints_to_add[], taints_to_remove[],
  //         scaling_config{}

  resp = client.eks.UpdateNodegroupConfig({
    ClusterName:   params.cluster_name,
    NodegroupName: params.nodegroup_name,
    Labels: {
      AddOrUpdateLabels: params.labels_to_add    ?? {},
      RemoveLabels:      params.labels_to_remove ?? [],
    },
    Taints: {
      AddOrUpdateTaints: params.taints_to_add    ?? [],
      RemoveTaints:      params.taints_to_remove ?? [],
    },
    ScalingConfig: params.scaling_config,
    ClientRequestToken: new_idempotency_token(),
  })

  if is_retryable(resp.error): return RetryableError(resp.error)
  if resp.error: return TerminalError(resp.error)

  return Success({
    nodegroup_name: params.nodegroup_name,
    update_id:      resp.Update.Id,
    status:         resp.Update.Status,
  })
```

---

#### `eks_cordon_nodegroup`

> Applies a `NoSchedule` taint to all nodes in a nodegroup to prevent new pods from being scheduled onto them. Used as
> the first step in a safe node drain sequence before patching or replacing nodes.

```
func eks_cordon_nodegroup(ctx, client, params):
  // params: cluster_name, nodegroup_name

  // Cordon is implemented by applying a well-known NoSchedule taint.
  // New workloads without a matching toleration will not be scheduled
  // onto these nodes. Existing pods are not evicted.

  return eks_update_nodegroup_config(ctx, client, {
    cluster_name:   params.cluster_name,
    nodegroup_name: params.nodegroup_name,
    taints_to_add: [{
      Key:    "release-engine.io/cordoned",
      Value:  "true",
      Effect: "NO_SCHEDULE",
    }],
  })
```

---

#### `eks_drain_nodegroup`

> Scales a nodegroup to zero desired nodes after cordoning, forcing the Kubernetes scheduler to reschedule pods onto
> other nodegroups. Used before nodegroup replacement or decommissioning workflows. Must be preceded by
`eks_cordon_nodegroup`.

```
func eks_drain_nodegroup(ctx, client, params):
  // params: cluster_name, nodegroup_name

  // Step 1: verify nodegroup is already cordoned
  ng = eks_describe_nodegroup(ctx, client, {
    cluster_name:   params.cluster_name,
    nodegroup_name: params.nodegroup_name,
  })
  if ng is error: return ng

  is_cordoned = ng.data.taints.any(t =>
    t.Key == "release-engine.io/cordoned" and t.Effect == "NO_SCHEDULE"
  )
  if not is_cordoned:
    return TerminalError("nodegroup_must_be_cordoned_before_drain")

  // Step 2: scale desired count to 0
  // min_size must also be set to 0 to allow the scale-down
  return eks_scale_nodegroup(ctx, client, {
    cluster_name:   params.cluster_name,
    nodegroup_name: params.nodegroup_name,
    desired_size:   0,
    min_size:       0,
    max_size:       ng.data.scaling_config.MaxSize,
  })
```

---

## EKS — Addon Operations

---

#### `eks_list_addons`

> Lists all managed addons installed on a cluster. Used in upgrade pre-flight workflows to enumerate addons that require
> version compatibility validation.

```
func eks_list_addons(ctx, client, params):
  // params: cluster_name

  resp = client.eks.ListAddons({ ClusterName: params.cluster_name })

  if is_retryable(resp.error): return RetryableError(resp.error)
  if resp.error: return TerminalError(resp.error)

  return Success({ cluster_name: params.cluster_name, addons: resp.Addons })
```

---

#### `eks_describe_addon`

> Returns full configuration, version, status, and health detail for a specific addon. Used to detect degraded addon
> state as part of cluster health check workflows.

```
func eks_describe_addon(ctx, client, params):
  // params: cluster_name, addon_name

  resp = client.eks.DescribeAddon({
    ClusterName: params.cluster_name,
    AddonName:   params.addon_name,
  })

  if resp.error.code == "ResourceNotFoundException":
    return TerminalError("addon_not_found")

  if is_retryable(resp.error): return RetryableError(resp.error)
  if resp.error: return TerminalError(resp.error)

  a = resp.Addon
  return Success({
    addon_name:       a.AddonName,
    addon_version:    a.AddonVersion,
    status:           a.Status,     // CREATING|ACTIVE|UPDATING|DELETING|DEGRADED|CREATE_FAILED
    service_account:  a.ServiceAccountRoleArn,
    health: {
      issues: a.Health.Issues.map(i => ({
        code:     i.Code,
        message:  i.Message,
        resource_ids: i.ResourceIds,
      })),
    },
    marketplace_version: a.MarketplaceVersion,
    configuration_schema: a.ConfigurationSchema,
    created_at:       a.CreatedAt,
    modified_at:      a.ModifiedAt,
  })
```

---

#### `eks_update_addon`

> Updates an addon to a new version or modifies its configuration. Used in cluster upgrade workflows to align addon
> versions with the target Kubernetes version after a control plane upgrade completes.

```
func eks_update_addon(ctx, client, params):
  // params: cluster_name, addon_name, addon_version,
  //         service_account_role_arn (optional),
  //         configuration_values (optional, JSON string),
  //         resolve_conflicts (OVERWRITE|PRESERVE|NONE)

  resp = client.eks.UpdateAddon({
    ClusterName:           params.cluster_name,
    AddonName:             params.addon_name,
    AddonVersion:          params.addon_version,
    ServiceAccountRoleArn: params.service_account_role_arn,
    ConfigurationValues:   params.configuration_values,
    ResolveConflicts:      params.resolve_conflicts ?? "OVERWRITE",
    ClientRequestToken:    new_idempotency_token(),
  })

  if is_retryable(resp.error): return RetryableError(resp.error)
  if resp.error: return TerminalError(resp.error)

  return Success({
    addon_name: params.addon_name,
    update_id:  resp.Update.Id,
    status:     resp.Update.Status,
  })
```

---

#### `eks_create_addon`

> Installs a managed addon on a cluster. Used in cluster bootstrapping and compliance remediation workflows to ensure
> required addons such as `vpc-cni`, `coredns`, and `kube-proxy` are present.

```
func eks_create_addon(ctx, client, params):
  // params: cluster_name, addon_name, addon_version,
  //         service_account_role_arn, configuration_values,
  //         resolve_conflicts, tags{}

  // Idempotency: if addon already exists, describe and return current state
  existing = eks_describe_addon(ctx, client, {
    cluster_name: params.cluster_name,
    addon_name:   params.addon_name,
  })
  if existing is Success:
    return Success({ idempotent: true, addon: existing.data })

  resp = client.eks.CreateAddon({
    ClusterName:           params.cluster_name,
    AddonName:             params.addon_name,
    AddonVersion:          params.addon_version,
    ServiceAccountRoleArn: params.service_account_role_arn,
    ConfigurationValues:   params.configuration_values,
    ResolveConflicts:      params.resolve_conflicts ?? "OVERWRITE",
    Tags:                  params.tags ?? {},
    ClientRequestToken:    new_idempotency_token(),
  })

  if is_retryable(resp.error): return RetryableError(resp.error)
  if resp.error: return TerminalError(resp.error)

  return Success({
    addon_name:    params.addon_name,
    addon_version: resp.Addon.AddonVersion,
    status:        resp.Addon.Status,
  })
```

---

#### `eks_delete_addon`

> Removes a managed addon from a cluster. Used in cluster cleanup and configuration reset workflows. Can preserve or
> delete addon-managed resources in the cluster on deletion.

```
func eks_delete_addon(ctx, client, params):
  // params: cluster_name, addon_name,
  //         preserve (bool — keep resources in cluster after addon removal)

  resp = client.eks.DeleteAddon({
    ClusterName: params.cluster_name,
    AddonName:   params.addon_name,
    Preserve:    params.preserve ?? false,
  })

  if resp.error.code == "ResourceNotFoundException":
    return Success({ idempotent: true })

  if is_retryable(resp.error): return RetryableError(resp.error)
  if resp.error: return TerminalError(resp.error)

  return Success({ addon_name: params.addon_name, status: resp.Addon.Status })
```

---

## EKS — Access Management

---

#### `eks_list_access_entries`

> Lists IAM principal ARNs that have been granted access to a cluster via the EKS access entry API. Used in security
> audit and access review workflows.

```
func eks_list_access_entries(ctx, client, params):
  // params: cluster_name, associated_policy_arn (optional filter),
  //         max_results, next_token

  resp = client.eks.ListAccessEntries({
    ClusterName:          params.cluster_name,
    AssociatedPolicyArn:  params.associated_policy_arn,
    MaxResults:           params.max_results ?? 100,
    NextToken:            params.next_token,
  })

  if is_retryable(resp.error): return RetryableError(resp.error)
  if resp.error: return TerminalError(resp.error)

  return Success({
    access_entries: resp.AccessEntries,
    next_token:     resp.NextToken,
  })
```

---

#### `eks_create_access_entry`

> Creates an IAM access entry for a principal on a cluster. Used in onboarding and break-glass access provisioning
> workflows.

```
func eks_create_access_entry(ctx, client, params):
  // params: cluster_name, principal_arn, type (STANDARD|FARGATE_LINUX|EC2_LINUX|EC2_WINDOWS),
  //         kubernetes_groups[], username, tags{}

  // Idempotency: return existing entry if principal already has access
  existing = eks_get_access_entry(ctx, client, {
    cluster_name:  params.cluster_name,
    principal_arn: params.principal_arn,
  })
  if existing is Success:
    return Success({ idempotent: true, access_entry: existing.data })

  resp = client.eks.CreateAccessEntry({
    ClusterName:       params.cluster_name,
    PrincipalArn:      params.principal_arn,
    Type:              params.type ?? "STANDARD",
    KubernetesGroups:  params.kubernetes_groups ?? [],
    Username:          params.username,
    Tags:              params.tags ?? {},
    ClientRequestToken: new_idempotency_token(),
  })

  if is_retryable(resp.error): return RetryableError(resp.error)
  if resp.error: return TerminalError(resp.error)

  return Success({
    principal_arn:     resp.AccessEntry.PrincipalArn,
    cluster_name:      resp.AccessEntry.ClusterName,
    type:              resp.AccessEntry.Type,
    kubernetes_groups: resp.AccessEntry.KubernetesGroups,
  })
```

---

#### `eks_delete_access_entry`

> Removes an IAM access entry from a cluster. Used in offboarding and access revocation workflows.

```
func eks_delete_access_entry(ctx, client, params):
  // params: cluster_name, principal_arn

  resp = client.eks.DeleteAccessEntry({
    ClusterName:  params.cluster_name,
    PrincipalArn: params.principal_arn,
  })

  if resp.error.code == "ResourceNotFoundException":
    return Success({ idempotent: true })

  if is_retryable(resp.error): return RetryableError(resp.error)
  if resp.error: return TerminalError(resp.error)

  return Success({
    cluster_name:  params.cluster_name,
    principal_arn: params.principal_arn,
    revoked: true,
  })
```

---

#### `eks_associate_access_policy`

> Associates an EKS access policy with an existing access entry, granting cluster or namespace-scoped Kubernetes RBAC
> permissions to an IAM principal. Used in role assignment and least-privilege provisioning workflows.

```
func eks_associate_access_policy(ctx, client, params):
  // params: cluster_name, principal_arn, policy_arn,
  //         access_scope: { type: "cluster"|"namespace", namespaces[] }

  resp = client.eks.AssociateAccessPolicy({
    ClusterName:  params.cluster_name,
    PrincipalArn: params.principal_arn,
    PolicyArn:    params.policy_arn,
    AccessScope: {
      Type:       params.access_scope.type,
      Namespaces: params.access_scope.namespaces ?? [],
    },
  })

  if is_retryable(resp.error): return RetryableError(resp.error)
  if resp.error: return TerminalError(resp.error)

  return Success({
    principal_arn: params.principal_arn,
    policy_arn:    params.policy_arn,
    access_scope:  params.access_scope,
    associated_at: resp.AssociatedAccessPolicy.AssociatedAt,
  })
```

---

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