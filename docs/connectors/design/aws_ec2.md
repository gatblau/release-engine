# AWS EC2 Connector — Pseudo Code

The connector encapsulates all AWS API interactions required for operational management and troubleshooting of EC2 compute resources. Provider authentication is resolved via the credential's
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

    // ── EC2 ─────────────────────────────────────────────────────
    case "ec2_get_instance":                  return ec2_get_instance(ctx, client, params)
    case "ec2_list_instances":                return ec2_list_instances(ctx, client, params)
    case "ec2_start_instance":                return ec2_start_instance(ctx, client, params)
    case "ec2_stop_instance":                 return ec2_stop_instance(ctx, client, params)
    case "ec2_reboot_instance":               return ec2_reboot_instance(ctx, client, params)
    case "ec2_terminate_instance":            return ec2_terminate_instance(ctx, client, params)
    case "ec2_get_instance_status":           return ec2_get_instance_status(ctx, client, params)
    case "ec2_get_system_status":             return ec2_get_system_status(ctx, client, params)
    case "ec2_get_console_output":            return ec2_get_console_output(ctx, client, params)
    case "ec2_get_console_screenshot":        return ec2_get_console_screenshot(ctx, client, params)
    case "ec2_send_ssm_command":              return ec2_send_ssm_command(ctx, client, params)
    case "ec2_get_ssm_command_result":        return ec2_get_ssm_command_result(ctx, client, params)
    case "ec2_modify_instance_type":          return ec2_modify_instance_type(ctx, client, params)
    case "ec2_modify_instance_attribute":     return ec2_modify_instance_attribute(ctx, client, params)
    case "ec2_get_instance_metrics":          return ec2_get_instance_metrics(ctx, client, params)
    case "ec2_create_snapshot":               return ec2_create_snapshot(ctx, client, params)
    case "ec2_get_snapshot":                  return ec2_get_snapshot(ctx, client, params)
    case "ec2_list_snapshots":                return ec2_list_snapshots(ctx, client, params)
    case "ec2_delete_snapshot":               return ec2_delete_snapshot(ctx, client, params)
    case "ec2_create_ami":                    return ec2_create_ami(ctx, client, params)
    case "ec2_deregister_ami":                return ec2_deregister_ami(ctx, client, params)
    case "ec2_list_amis":                     return ec2_list_amis(ctx, client, params)
    case "ec2_modify_security_group":         return ec2_modify_security_group(ctx, client, params)
    case "ec2_get_security_group":            return ec2_get_security_group(ctx, client, params)
    case "ec2_list_security_groups":          return ec2_list_security_groups(ctx, client, params)
    case "ec2_add_tags":                      return ec2_add_tags(ctx, client, params)
    case "ec2_remove_tags":                   return ec2_remove_tags(ctx, client, params)
    case "ec2_get_volumes":                   return ec2_get_volumes(ctx, client, params)
    case "ec2_modify_volume":                 return ec2_modify_volume(ctx, client, params)
    case "ec2_attach_volume":                 return ec2_attach_volume(ctx, client, params)
    case "ec2_detach_volume":                 return ec2_detach_volume(ctx, client, params)
    case "ec2_get_reserved_instances":        return ec2_get_reserved_instances(ctx, client, params)
    case "ec2_list_spot_requests":            return ec2_list_spot_requests(ctx, client, params)
    case "ec2_cancel_spot_request":           return ec2_cancel_spot_request(ctx, client, params)
    case "ec2_describe_vpc":                  return ec2_describe_vpc(ctx, client, params)
    case "ec2_list_subnets":                  return ec2_list_subnets(ctx, client, params)
    case "ec2_get_route_table":               return ec2_get_route_table(ctx, client, params)

    default:
      return TerminalError("unknown operation: " + op)
```

---

## Supported Operations

---

## EC2 — Instance Inspection

---

#### `ec2_get_instance`

> Fetches full metadata for an EC2 instance including state, instance type, networking, IAM profile, placement, and
> attached volumes. Used as the entry point for all instance-level troubleshooting workflows.

```
func ec2_get_instance(ctx, client, params):
// params: instance_id

resp = client.ec2.DescribeInstances({
InstanceIds: [params.instance_id],
})

if resp.Reservations is empty:
return TerminalError("ec2_instance_not_found")

if is_retryable(resp.error): return RetryableError(resp.error)
if resp.error: return TerminalError(resp.error)

i = resp.Reservations[0].Instances[0]
return Success({
instance_id:        i.InstanceId,
state:              i.State.Name,  // pending|running|stopping|stopped|terminated
instance_type:      i.InstanceType,
architecture:       i.Architecture,
platform:           i.Platform ?? "linux",
ami_id:             i.ImageId,
key_name:           i.KeyName,
iam_instance_profile: i.IamInstanceProfile.Arn,
placement: {
availability_zone: i.Placement.AvailabilityZone,
tenancy:           i.Placement.Tenancy,
},
networking: {
vpc_id:            i.VpcId,
subnet_id:         i.SubnetId,
private_ip:        i.PrivateIpAddress,
public_ip:         i.PublicIpAddress,
private_dns:       i.PrivateDnsName,
public_dns:        i.PublicDnsName,
security_groups:   i.SecurityGroups.map(sg => ({
id:   sg.GroupId,
name: sg.GroupName,
})),
network_interfaces: i.NetworkInterfaces.map(ni => ({
interface_id:  ni.NetworkInterfaceId,
private_ip:    ni.PrivateIpAddress,
mac_address:   ni.MacAddress,
status:        ni.Status,
})),
},
volumes: i.BlockDeviceMappings.map(b => ({
device_name: b.DeviceName,
volume_id:   b.Ebs.VolumeId,
status:      b.Ebs.Status,
delete_on_termination: b.Ebs.DeleteOnTermination,
})),
tags:               to_tag_map(i.Tags),
launch_time:        i.LaunchTime,
})
```

---

#### `ec2_get_instance_status`

> Returns the instance status checks for an EC2 instance — both system-level and instance-level health checks. Used in
> automated recovery workflows to detect and respond to impaired instances before a user reports an outage.

```
func ec2_get_instance_status(ctx, client, params):
// params: instance_id, include_all_instances (bool)

resp = client.ec2.DescribeInstanceStatus({
InstanceIds:         [params.instance_id],
IncludeAllInstances: params.include_all_instances ?? true,
})

if is_retryable(resp.error): return RetryableError(resp.error)
if resp.error: return TerminalError(resp.error)

if resp.InstanceStatuses is empty:
return TerminalError("ec2_instance_not_found")

s = resp.InstanceStatuses[0]
return Success({
instance_id:    s.InstanceId,
state:          s.InstanceState.Name,
system_status: {
status:  s.SystemStatus.Status,   // ok | impaired | insufficient-data | not-applicable
details: s.SystemStatus.Details,
},
instance_status: {
status:  s.InstanceStatus.Status,
details: s.InstanceStatus.Details,
},
events: s.Events.map(e => ({
code:             e.Code,
description:      e.Description,
not_before:       e.NotBefore,
not_after:        e.NotAfter,
})),
})
```

---

#### `ec2_get_console_output`

> Retrieves the most recent console output for an EC2 instance. Used in instance boot failure and kernel panic
> troubleshooting workflows where SSH access is unavailable.

```
func ec2_get_console_output(ctx, client, params):
// params: instance_id, latest (bool — true returns most recent buffered output)

resp = client.ec2.GetConsoleOutput({
InstanceId: params.instance_id,
Latest:     params.latest ?? true,
})

if is_retryable(resp.error): return RetryableError(resp.error)
if resp.error: return TerminalError(resp.error)

return Success({
instance_id: params.instance_id,
output:      base64_decode(resp.Output),
timestamp:   resp.Timestamp,
})
```

---

#### `ec2_get_console_screenshot`

> Captures a screenshot of the EC2 instance console as a base64-encoded PNG. Used in incident workflows involving
> Windows instances or graphical boot failures where console text output is insufficient.

```
func ec2_get_console_screenshot(ctx, client, params):
// params: instance_id, wake_up (bool — sends a keystroke to wake display)

resp = client.ec2.GetConsoleScreenshot({
InstanceId: params.instance_id,
WakeUp:     params.wake_up ?? false,
})

if is_retryable(resp.error): return RetryableError(resp.error)
if resp.error: return TerminalError(resp.error)

return Success({
instance_id:   params.instance_id,
image_data:    resp.ImageData,   // base64-encoded PNG
image_type:    "image/png",
})
```

---

## EC2 — Instance Lifecycle Operations

---

#### `ec2_start_instance`

> Starts a stopped EC2 instance. Used in environment wake-up and automated recovery workflows.

```
func ec2_start_instance(ctx, client, params):
// params: instance_id

current = ec2_get_instance(ctx, client, { instance_id: params.instance_id })
if current is error: return current

if current.data.state == "running":
return Success({ idempotent: true, state: "running" })

if current.data.state not in ["stopped"]:
return TerminalError("instance_cannot_be_started_from_state: " + current.data.state)

resp = client.ec2.StartInstances({ InstanceIds: [params.instance_id] })

if is_retryable(resp.error): return RetryableError(resp.error)
if resp.error: return TerminalError(resp.error)

return Success({
instance_id:    params.instance_id,
previous_state: resp.StartingInstances[0].PreviousState.Name,
current_state:  resp.StartingInstances[0].CurrentState.Name,
})
```

---

#### `ec2_stop_instance`

> Stops a running EC2 instance. Supports both graceful and forced stops. Used in maintenance, cost management, and
> incident containment workflows.

```
func ec2_stop_instance(ctx, client, params):
// params: instance_id, force (bool — bypass graceful shutdown)

current = ec2_get_instance(ctx, client, { instance_id: params.instance_id })
if current is error: return current

if current.data.state == "stopped":
return Success({ idempotent: true, state: "stopped" })

if current.data.state not in ["running", "pending"]:
return TerminalError("instance_cannot_be_stopped_from_state: " + current.data.state)

resp = client.ec2.StopInstances({
InstanceIds: [params.instance_id],
Force:       params.force ?? false,
})

if is_retryable(resp.error): return RetryableError(resp.error)
if resp.error: return TerminalError(resp.error)

return Success({
instance_id:    params.instance_id,
previous_state: resp.StoppingInstances[0].PreviousState.Name,
current_state:  resp.StoppingInstances[0].CurrentState.Name,
})
```

---

#### `ec2_reboot_instance`

> Reboots one or more EC2 instances. Used in incident response workflows when an instance is unresponsive but the
> underlying host is healthy enough to accept a reboot signal.

```
func ec2_reboot_instance(ctx, client, params):
// params: instance_id

resp = client.ec2.RebootInstances({ InstanceIds: [params.instance_id] })

if is_retryable(resp.error): return RetryableError(resp.error)
if resp.error: return TerminalError(resp.error)

return Success({
instance_id: params.instance_id,
rebooted:    true,
})
```

---

#### `ec2_terminate_instance`

> Permanently terminates an EC2 instance. Validates that termination protection is disabled before proceeding. Used in
> decommissioning and scale-in workflows.

```
func ec2_terminate_instance(ctx, client, params):
// params: instance_id, confirm (bool — must be explicitly true)

if params.confirm != true:
return TerminalError("termination_requires_explicit_confirm=true")

current = ec2_get_instance(ctx, client, { instance_id: params.instance_id })
if current is error: return current

if current.data.state == "terminated":
return Success({ idempotent: true, state: "terminated" })

// Check termination protection
attr_resp = client.ec2.DescribeInstanceAttribute({
InstanceId: params.instance_id,
Attribute:  "disableApiTermination",
})
if attr_resp.DisableApiTermination.Value == true:
return TerminalError("termination_protection_enabled")

resp = client.ec2.TerminateInstances({ InstanceIds: [params.instance_id] })

if is_retryable(resp.error): return RetryableError(resp.error)
if resp.error: return TerminalError(resp.error)

return Success({
instance_id:    params.instance_id,
previous_state: resp.TerminatingInstances[0].PreviousState.Name,
current_state:  resp.TerminatingInstances[0].CurrentState.Name,
})
```

---

#### `ec2_modify_instance_type`

> Changes the instance type of a stopped EC2 instance. Used in right-sizing and vertical scaling workflows. The calling
> module is responsible for stopping the instance and verifying it is stopped before calling this operation.

```
func ec2_modify_instance_type(ctx, client, params):
// params: instance_id, instance_type

current = ec2_get_instance(ctx, client, { instance_id: params.instance_id })
if current is error: return current

if current.data.state != "stopped":
return TerminalError("instance_must_be_stopped_to_modify_type")

resp = client.ec2.ModifyInstanceAttribute({
InstanceId:   params.instance_id,
InstanceType: { Value: params.instance_type },
})

if is_retryable(resp.error): return RetryableError(resp.error)
if resp.error: return TerminalError(resp.error)

return Success({
instance_id:   params.instance_id,
instance_type: params.instance_type,
})
```

---

## EC2 — Remote Command Execution

---

#### `ec2_send_ssm_command`

> Sends a remote shell command to an EC2 instance via AWS Systems Manager Run Command without requiring SSH. Used in
> incident response workflows to collect diagnostics, restart services, or apply hotfixes on instances that are reachable
> but not accessible via bastion.

```
func ec2_send_ssm_command(ctx, client, params):
// params: instance_id,
//         commands[],
//         document_name: "AWS-RunShellScript" | "AWS-RunPowerShellScript",
//         timeout_seconds,
//         output_s3_bucket (optional), output_s3_key_prefix (optional)

resp = client.ssm.SendCommand({
InstanceIds:    [params.instance_id],
DocumentName:   params.document_name ?? "AWS-RunShellScript",
Parameters: {
commands:         params.commands,
executionTimeout: [str(params.timeout_seconds ?? 60)],
},
TimeoutSeconds:     params.timeout_seconds ?? 60,
OutputS3BucketName: params.output_s3_bucket,
OutputS3KeyPrefix:  params.output_s3_key_prefix,
})

if is_retryable(resp.error): return RetryableError(resp.error)
if resp.error: return TerminalError(resp.error)

return Success({
instance_id:  params.instance_id,
command_id:   resp.Command.CommandId,
status:       resp.Command.Status,
requested_at: resp.Command.RequestedDateTime,
})
```

---

#### `ec2_get_ssm_command_result`

> Fetches the execution status and output of a previously submitted SSM Run Command. Used in polling loops within
> incident response and diagnostic workflows to retrieve command stdout, stderr, and exit code.

```
func ec2_get_ssm_command_result(ctx, client, params):
// params: command_id, instance_id, plugin_name (optional — default "aws:runShellScript")

resp = client.ssm.GetCommandInvocation({
CommandId:  params.command_id,
InstanceId: params.instance_id,
PluginName: params.plugin_name ?? "aws:runShellScript",
})

if resp.error.code == "InvocationDoesNotExist":
return RetryableError("invocation_not_yet_available")

if is_retryable(resp.error): return RetryableError(resp.error)
if resp.error: return TerminalError(resp.error)

return Success({
command_id:    params.command_id,
instance_id:   params.instance_id,
status:        resp.Status,          // Pending|InProgress|Success|Failed|TimedOut|Cancelled
exit_code:     resp.ResponseCode,
stdout:        resp.StandardOutputContent,
stderr:        resp.StandardErrorContent,
executed_at:   resp.ExecutionStartDateTime,
completed_at:  resp.ExecutionEndDateTime,
})
```

---

## EC2 — Metrics & Observability

---

#### `ec2_get_instance_metrics`

> Fetches CloudWatch metrics for an EC2 instance over a specified time range. Used in performance investigation and
> capacity planning workflows. Key metrics include `CPUUtilization`, `NetworkIn`, `NetworkOut`, `DiskReadOps`,
`DiskWriteOps`, `StatusCheckFailed`, and `StatusCheckFailed_System`.

```
func ec2_get_instance_metrics(ctx, client, params):
// params: instance_id,
//         metric_names[],
//         start_time (ISO8601), end_time (ISO8601),
//         period_seconds,
//         stat: "Average" | "Maximum" | "Minimum" | "Sum" | "SampleCount"

results = {}

for metric_name in params.metric_names:
resp = client.cloudwatch.GetMetricStatistics({
Namespace:  "AWS/EC2",
MetricName: metric_name,
Dimensions: [{ Name: "InstanceId", Value: params.instance_id }],
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
instance_id:    params.instance_id,
period_seconds: params.period_seconds ?? 60,
metrics:        results,
})
```

---

## EC2 — Storage Operations

---

#### `ec2_get_volumes`

> Lists EBS volumes attached to an EC2 instance or filtered by state. Used in storage capacity and health investigation
> workflows to identify volumes that are full, degraded, or in an error state.

```
func ec2_get_volumes(ctx, client, params):
// params: instance_id (optional), volume_ids[] (optional),
//         state_filter: "available" | "in-use" | "error" | "creating" | "deleting"

filters = []
if params.instance_id:
filters.append({ Name: "attachment.instance-id", Values: [params.instance_id] })
if params.state_filter:
filters.append({ Name: "status", Values: [params.state_filter] })

resp = client.ec2.DescribeVolumes({
VolumeIds: params.volume_ids ?? [],
Filters:   filters,
})

if is_retryable(resp.error): return RetryableError(resp.error)
if resp.error: return TerminalError(resp.error)

return Success({
volumes: resp.Volumes.map(v => ({
volume_id:        v.VolumeId,
state:            v.State,
volume_type:      v.VolumeType,
size_gb:          v.Size,
iops:             v.Iops,
throughput_mbps:  v.Throughput,
encrypted:        v.Encrypted,
kms_key_id:       v.KmsKeyId,
availability_zone: v.AvailabilityZone,
attachments: v.Attachments.map(a => ({
instance_id:  a.InstanceId,
device:       a.Device,
state:        a.State,
attached_at:  a.AttachTime,
delete_on_termination: a.DeleteOnTermination,
})),
tags: to_tag_map(v.Tags),
created_at: v.CreateTime,
})),
})
```

---

#### `ec2_modify_volume`

> Modifies an EBS volume's size, type, IOPS, or throughput without detaching it. Used in capacity expansion and
> performance tuning workflows. Volume modifications are applied online for supported instance and volume types.

```
func ec2_modify_volume(ctx, client, params):
// params: volume_id,
//         size_gb (optional), volume_type (optional),
//         iops (optional), throughput_mbps (optional)

resp = client.ec2.ModifyVolume({
VolumeId:   params.volume_id,
Size:       params.size_gb,
VolumeType: params.volume_type,
Iops:       params.iops,
Throughput: params.throughput_mbps,
})

if is_retryable(resp.error): return RetryableError(resp.error)
if resp.error: return TerminalError(resp.error)

m = resp.VolumeModification
return Success({
volume_id:              params.volume_id,
modification_state:     m.ModificationState,  // modifying|optimizing|completed|failed
target_size_gb:         m.TargetSize,
target_volume_type:     m.TargetVolumeType,
target_iops:            m.TargetIops,
target_throughput_mbps: m.TargetThroughput,
start_time:             m.StartTime,
})
```

---

#### `ec2_attach_volume`

> Attaches an available EBS volume to an EC2 instance at the specified device path. Used in data recovery and storage
> expansion workflows.

```
func ec2_attach_volume(ctx, client, params):
// params: volume_id, instance_id, device ("/dev/xvdf")

resp = client.ec2.AttachVolume({
VolumeId:   params.volume_id,
InstanceId: params.instance_id,
Device:     params.device,
})

if resp.error.code == "VolumeInUse":
return TerminalError("volume_already_attached")

if is_retryable(resp.error): return RetryableError(resp.error)
if resp.error: return TerminalError(resp.error)

return Success({
volume_id:   params.volume_id,
instance_id: params.instance_id,
device:      params.device,
state:       resp.State,
attached_at: resp.AttachTime,
})
```

---

#### `ec2_detach_volume`

> Detaches an EBS volume from an EC2 instance. Supports forced detach for unresponsive instances. Used in
> decommissioning and volume migration workflows.

```
func ec2_detach_volume(ctx, client, params):
// params: volume_id, instance_id (optional), device (optional),
//         force (bool — force detach from unresponsive instance)

resp = client.ec2.DetachVolume({
VolumeId:   params.volume_id,
InstanceId: params.instance_id,
Device:     params.device,
Force:      params.force ?? false,
})

if is_retryable(resp.error): return RetryableError(resp.error)
if resp.error: return TerminalError(resp.error)

return Success({
volume_id:   params.volume_id,
instance_id: params.instance_id,
state:       resp.State,
})
```

---

## EC2 — Security Groups

---

#### `ec2_get_security_group`

> Fetches full inbound and outbound rule details for a security group. Used in network access troubleshooting workflows
> to verify whether a required port is open between a source and destination.

```
func ec2_get_security_group(ctx, client, params):
// params: security_group_id

resp = client.ec2.DescribeSecurityGroups({
GroupIds: [params.security_group_id],
})

if resp.SecurityGroups is empty:
return TerminalError("security_group_not_found")

if is_retryable(resp.error): return RetryableError(resp.error)
if resp.error: return TerminalError(resp.error)

sg = resp.SecurityGroups[0]
return Success({
security_group_id:   sg.GroupId,
name:                sg.GroupName,
description:         sg.Description,
vpc_id:              sg.VpcId,
inbound_rules: sg.IpPermissions.map(r => ({
protocol:    r.IpProtocol,
from_port:   r.FromPort,
to_port:     r.ToPort,
ipv4_ranges: r.IpRanges.map(x => ({ cidr: x.CidrIp, description: x.Description })),
ipv6_ranges: r.Ipv6Ranges.map(x => ({ cidr: x.CidrIpv6, description: x.Description })),
sg_references: r.UserIdGroupPairs.map(x => ({ sg_id: x.GroupId, vpc_id: x.VpcId })),
})),
outbound_rules: sg.IpPermissionsEgress.map(r => ({
protocol:    r.IpProtocol,
from_port:   r.FromPort,
to_port:     r.ToPort,
ipv4_ranges: r.IpRanges.map(x => ({ cidr: x.CidrIp, description: x.Description })),
})),
tags: to_tag_map(sg.Tags),
})
```

---

#### `ec2_modify_security_group`

> Adds or removes inbound or outbound rules from a security group. Used in incident response workflows to immediately
> block malicious traffic or open a port required for emergency access.

```
func ec2_modify_security_group(ctx, client, params):
// params: security_group_id,
//         add_inbound_rules[]  (optional),
//         remove_inbound_rules[] (optional),
//         add_outbound_rules[] (optional),
//         remove_outbound_rules[] (optional)
// rule shape: { protocol, from_port, to_port, cidr OR sg_id, description }

to_permission = (rule) => ({
IpProtocol: rule.protocol,
FromPort:   rule.from_port,
ToPort:     rule.to_port,
IpRanges:   rule.cidr ? [{ CidrIp: rule.cidr, Description: rule.description }] : [],
UserIdGroupPairs: rule.sg_id ? [{ GroupId: rule.sg_id }] : [],
})

if params.add_inbound_rules is not empty:
r = client.ec2.AuthorizeSecurityGroupIngress({
GroupId:       params.security_group_id,
IpPermissions: params.add_inbound_rules.map(to_permission),
})
if is_retryable(r.error): return RetryableError(r.error)
if r.error and r.error.code != "InvalidPermission.Duplicate":
return TerminalError(r.error)

if params.remove_inbound_rules is not empty:
r = client.ec2.RevokeSecurityGroupIngress({
GroupId:       params.security_group_id,
IpPermissions: params.remove_inbound_rules.map(to_permission),
})
if is_retryable(r.error): return RetryableError(r.error)
if r.error and r.error.code != "InvalidPermission.NotFound":
return TerminalError(r.error)

if params.add_outbound_rules is not empty:
r = client.ec2.AuthorizeSecurityGroupEgress({
GroupId:       params.security_group_id,
IpPermissions: params.add_outbound_rules.map(to_permission),
})
if is_retryable(r.error): return RetryableError(r.error)
if r.error and r.error.code != "InvalidPermission.Duplicate":
return TerminalError(r.error)

if params.remove_outbound_rules is not empty:
r = client.ec2.RevokeSecurityGroupEgress({
GroupId:       params.security_group_id,
IpPermissions: params.remove_outbound_rules.map(to_permission),
})
if is_retryable(r.error): return RetryableError(r.error)
if r.error and r.error.code != "InvalidPermission.NotFound":
return TerminalError(r.error)

return Success({
security_group_id:      params.security_group_id,
added_inbound_count:    (params.add_inbound_rules ?? []).length,
removed_inbound_count:  (params.remove_inbound_rules ?? []).length,
added_outbound_count:   (params.add_outbound_rules ?? []).length,
removed_outbound_count: (params.remove_outbound_rules ?? []).length,
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