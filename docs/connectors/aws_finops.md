# AWS FinOps Connector — Pseudo Code

---

## Interface

```
CONNECTOR: AWSConnector
implements Connector interface

registered name: "aws"

func Call(ctx, op, params, credential) -> ConnectorResult:
  client = resolve_client(credential)
  // credential: {
  //   auth_type:         "role_arn" | "access_key" | "instance_profile"
  //   role_arn:          (if auth_type == role_arn) cross-account role
  //   external_id:       (if auth_type == role_arn) STS ExternalId condition
  //   access_key_id:     (if auth_type == access_key)
  //   secret_access_key: (if auth_type == access_key)
  //   region:            default region
  //   session_name:      STS session label for audit trails
  // }

  switch op:

    // ── Cost Explorer ─────────────────────────────────────────────
    case "get_cost_and_usage":              return get_cost_and_usage(ctx, client, params)
    case "get_cost_forecast":               return get_cost_forecast(ctx, client, params)
    case "get_cost_comparison":             return get_cost_comparison(ctx, client, params)
    case "get_dimension_values":            return get_dimension_values(ctx, client, params)
    case "get_tags":                        return get_tags(ctx, client, params)
    case "get_cost_categories":             return get_cost_categories(ctx, client, params)
    case "create_cost_category":            return create_cost_category(ctx, client, params)
    case "update_cost_category":            return update_cost_category(ctx, client, params)
    case "delete_cost_category":            return delete_cost_category(ctx, client, params)

    // ── Budgets ───────────────────────────────────────────────────
    case "create_budget":                   return create_budget(ctx, client, params)
    case "update_budget":                   return update_budget(ctx, client, params)
    case "delete_budget":                   return delete_budget(ctx, client, params)
    case "get_budget":                      return get_budget(ctx, client, params)
    case "list_budgets":                    return list_budgets(ctx, client, params)
    case "create_budget_action":            return create_budget_action(ctx, client, params)
    case "delete_budget_action":            return delete_budget_action(ctx, client, params)
    case "execute_budget_action":           return execute_budget_action(ctx, client, params)
    case "list_budget_actions":             return list_budget_actions(ctx, client, params)

    // ── Anomaly Detection ─────────────────────────────────────────
    case "create_anomaly_monitor":          return create_anomaly_monitor(ctx, client, params)
    case "delete_anomaly_monitor":          return delete_anomaly_monitor(ctx, client, params)
    case "list_anomaly_monitors":           return list_anomaly_monitors(ctx, client, params)
    case "create_anomaly_subscription":     return create_anomaly_subscription(ctx, client, params)
    case "delete_anomaly_subscription":     return delete_anomaly_subscription(ctx, client, params)
    case "list_anomaly_subscriptions":      return list_anomaly_subscriptions(ctx, client, params)
    case "get_anomalies":                   return get_anomalies(ctx, client, params)

    // ── Savings Plans ─────────────────────────────────────────────
    case "describe_savings_plans":          return describe_savings_plans(ctx, client, params)
    case "get_savings_plans_coverage":      return get_savings_plans_coverage(ctx, client, params)
    case "get_savings_plans_utilization":   return get_savings_plans_utilization(ctx, client, params)
    case "get_savings_plans_purchase_recommendation": return get_savings_plans_purchase_recommendation(ctx, client, params)

    // ── Reserved Instances ────────────────────────────────────────
    case "describe_reserved_instances":     return describe_reserved_instances(ctx, client, params)
    case "get_ri_coverage":                 return get_ri_coverage(ctx, client, params)
    case "get_ri_utilization":              return get_ri_utilization(ctx, client, params)
    case "get_ri_purchase_recommendation":  return get_ri_purchase_recommendation(ctx, client, params)
    case "modify_reserved_instances":       return modify_reserved_instances(ctx, client, params)

    // ── Rightsizing ───────────────────────────────────────────────
    case "get_rightsizing_recommendation":  return get_rightsizing_recommendation(ctx, client, params)
    case "get_resource_utilization":        return get_resource_utilization(ctx, client, params)

    // ── Tagging ───────────────────────────────────────────────────
    case "tag_resources":                   return tag_resources(ctx, client, params)
    case "untag_resources":                 return untag_resources(ctx, client, params)
    case "get_resources_by_tags":           return get_resources_by_tags(ctx, client, params)
    case "get_tag_keys":                    return get_tag_keys(ctx, client, params)
    case "get_tag_values":                  return get_tag_values(ctx, client, params)
    case "describe_tag_policies":           return describe_tag_policies(ctx, client, params)
    case "get_tag_compliance_report":       return get_tag_compliance_report(ctx, client, params)

    // ── AWS Organizations & Accounts ──────────────────────────────
    case "list_accounts":                   return list_accounts(ctx, client, params)
    case "describe_account":               return describe_account(ctx, client, params)
    case "create_account":                  return create_account(ctx, client, params)
    case "close_account":                   return close_account(ctx, client, params)
    case "move_account":                    return move_account(ctx, client, params)
    case "list_organizational_units":       return list_organizational_units(ctx, client, params)
    case "list_policies":                   return list_policies(ctx, client, params)
    case "attach_policy":                   return attach_policy(ctx, client, params)
    case "detach_policy":                   return detach_policy(ctx, client, params)

    // ── Service Quotas ────────────────────────────────────────────
    case "list_service_quotas":             return list_service_quotas(ctx, client, params)
    case "get_service_quota":               return get_service_quota(ctx, client, params)
    case "request_quota_increase":          return request_quota_increase(ctx, client, params)
    case "list_quota_increase_requests":    return list_quota_increase_requests(ctx, client, params)

    // ── Compute Optimizer ─────────────────────────────────────────
    case "get_ec2_recommendations":         return get_ec2_recommendations(ctx, client, params)
    case "get_ecs_recommendations":         return get_ecs_recommendations(ctx, client, params)
    case "get_lambda_recommendations":      return get_lambda_recommendations(ctx, client, params)
    case "get_ebs_recommendations":         return get_ebs_recommendations(ctx, client, params)
    case "get_rds_recommendations":         return get_rds_recommendations(ctx, client, params)
    case "export_recommendations":          return export_recommendations(ctx, client, params)

    // ── S3 Storage Lens ───────────────────────────────────────────
    case "list_storage_lens_configurations": return list_storage_lens_configurations(ctx, client, params)
    case "get_storage_lens_configuration":  return get_storage_lens_configuration(ctx, client, params)
    case "get_storage_lens_dashboard":      return get_storage_lens_dashboard(ctx, client, params)

    // ── CloudWatch (Cost-Relevant Metrics) ────────────────────────
    case "get_metric_statistics":           return get_metric_statistics(ctx, client, params)
    case "list_metrics":                    return list_metrics(ctx, client, params)
    case "put_metric_alarm":                return put_metric_alarm(ctx, client, params)
    case "delete_metric_alarm":             return delete_metric_alarm(ctx, client, params)
    case "describe_alarms":                 return describe_alarms(ctx, client, params)

    // ── Trusted Advisor ───────────────────────────────────────────
    case "list_trusted_advisor_checks":     return list_trusted_advisor_checks(ctx, client, params)
    case "get_trusted_advisor_check_result": return get_trusted_advisor_check_result(ctx, client, params)
    case "refresh_trusted_advisor_check":   return refresh_trusted_advisor_check(ctx, client, params)

    default:
      return TerminalError("unknown operation: " + op)
```

---

## Supported Operations

### Cost Explorer

---

#### `get_cost_and_usage`
> Queries AWS Cost Explorer for actual spend data across a time range with support for granularity, grouping by dimension or tag, and filter expressions. The primary operation for cost reporting, showback, and chargeback workflows. Supports multi-account aggregation when called from the management account.

```
func get_cost_and_usage(ctx, client, params):
  // params: time_period{ start, end (YYYY-MM-DD) },
  //         granularity (DAILY|MONTHLY|HOURLY),
  //         metrics[]   (UnblendedCost|BlendedCost|AmortizedCost|
  //                      NetAmortizedCost|NetUnblendedCost|
  //                      NormalizedUsageAmount|UsageQuantity),
  //         group_by[]  { type: DIMENSION|TAG|COST_CATEGORY, key },
  //         filter{}    (Expression tree — see AWS CE filter DSL),
  //         next_page_token

  results    = []
  page_token = params.next_page_token ?? null

  loop:
    resp = client.ce.GetCostAndUsage({
      TimePeriod:    { Start: params.time_period.start,
                       End:   params.time_period.end },
      Granularity:   params.granularity ?? "MONTHLY",
      Metrics:       params.metrics ?? ["UnblendedCost"],
      GroupBy:       params.group_by ?? [],
      Filter:        params.filter,
      NextPageToken: page_token,
    })

    if resp.error in [ThrottlingException, ServiceUnavailableException]:
      return RetryableError(resp.error)

    if resp.error:
      return TerminalError(resp.error)

    results.append_all(resp.ResultsByTime)
    page_token = resp.NextPageToken
    if page_token is null: break

  return Success({
    results:                results,
    dimension_values_total: aggregate_totals(results, params.metrics),
  })
```

---

#### `get_cost_forecast`
> Retrieves a cost forecast for a future time range based on historical spend patterns. Used in budget-projection and proactive overspend alerting workflows.

```
func get_cost_forecast(ctx, client, params):
  // params: time_period{ start, end }, metric, granularity,
  //         filter{}, prediction_interval_level (50–99, default 80)

  resp = client.ce.GetCostForecast({
    TimePeriod:              { Start: params.time_period.start,
                               End:   params.time_period.end },
    Metric:                  params.metric ?? "UNBLENDED_COST",
    Granularity:             params.granularity ?? "MONTHLY",
    Filter:                  params.filter,
    PredictionIntervalLevel: params.prediction_interval_level ?? 80,
  })

  if resp.error in [ThrottlingException, DataUnavailableException]:
    return RetryableError(resp.error)

  if resp.error:
    return TerminalError(resp.error)

  return Success({
    total: {
      amount:      resp.Total.Amount,
      unit:        resp.Total.Unit,
      lower_bound: resp.Total.PredictionIntervalLowerBound,
      upper_bound: resp.Total.PredictionIntervalUpperBound,
    },
    forecast_results: resp.ForecastResultsByTime.map(r => ({
      time_period:  r.TimePeriod,
      mean_value:   r.MeanValue,
      lower_bound:  r.PredictionIntervalLowerBound,
      upper_bound:  r.PredictionIntervalUpperBound,
    })),
  })
```

---

#### `get_cost_comparison`
> Compares cost and usage between two time periods for a given dimension or tag breakdown. Used in monthly business reviews and anomaly root-cause workflows to surface period-over-period spend deltas.

```
func get_cost_comparison(ctx, client, params):
  // params: baseline_period{ start, end }, comparison_period{ start, end },
  //         metrics[], group_by[], filter{}

  baseline = get_cost_and_usage(ctx, client, {
    time_period: params.baseline_period,
    metrics:     params.metrics,
    group_by:    params.group_by,
    filter:      params.filter,
    granularity: "MONTHLY",
  })
  if baseline is error: return baseline

  comparison = get_cost_and_usage(ctx, client, {
    time_period: params.comparison_period,
    metrics:     params.metrics,
    group_by:    params.group_by,
    filter:      params.filter,
    granularity: "MONTHLY",
  })
  if comparison is error: return comparison

  delta = compute_delta(baseline.data.results, comparison.data.results)

  return Success({
    baseline_period:   params.baseline_period,
    comparison_period: params.comparison_period,
    delta:             delta,
    // delta shape per group:
    // { key, baseline_amount, comparison_amount,
    //   absolute_change, percentage_change }
  })
```

---

#### `get_dimension_values`
> Retrieves valid values for a given Cost Explorer dimension within a time range. Used to populate filter dropdowns in reporting workflows and to validate dimension values before constructing CE filter expressions.

```
func get_dimension_values(ctx, client, params):
  // params: dimension (SERVICE|LINKED_ACCOUNT|REGION|USAGE_TYPE|
  //                    INSTANCE_TYPE|OPERATION|PLATFORM|TENANCY|
  //                    PURCHASE_TYPE|DEPLOYMENT_OPTION|DATABASE_ENGINE),
  //         time_period{ start, end }, search_string, context, next_page_token

  resp = client.ce.GetDimensionValues({
    Dimension:     params.dimension,
    TimePeriod:    { Start: params.time_period.start,
                     End:   params.time_period.end },
    SearchString:  params.search_string,
    Context:       params.context ?? "COST_AND_USAGE",
    NextPageToken: params.next_page_token,
  })

  if resp.error in [ThrottlingException]:
    return RetryableError(resp.error)

  if resp.error:
    return TerminalError(resp.error)

  return Success({
    dimension:       params.dimension,
    values:          resp.DimensionValues.map(v => ({
      value:      v.Value,
      attributes: v.Attributes,
    })),
    next_page_token: resp.NextPageToken,
    total_size:      resp.TotalSize,
  })
```

---

#### `get_tags`
> Retrieves all tag keys and optional tag values seen by Cost Explorer within a time period. Used in tagging-compliance audits and to enumerate the active tag universe before building CE filter expressions.

```
func get_tags(ctx, client, params):
  // params: time_period{ start, end }, tag_key (optional),
  //         search_string, filter{}, next_page_token

  resp = client.ce.GetTags({
    TimePeriod:    { Start: params.time_period.start,
                     End:   params.time_period.end },
    TagKey:        params.tag_key,
    SearchString:  params.search_string,
    Filter:        params.filter,
    NextPageToken: params.next_page_token,
  })

  if resp.error in [ThrottlingException]:
    return RetryableError(resp.error)

  if resp.error:
    return TerminalError(resp.error)

  return Success({
    tags:            resp.Tags,
    next_page_token: resp.NextPageToken,
    total_size:      resp.TotalSize,
  })
```

---

#### `get_cost_categories`
> Lists all Cost Category definitions. Used in showback and chargeback workflows to verify that the required cost allocation categories exist before running grouped cost queries.

```
func get_cost_categories(ctx, client, params):
  // params: effective_on (ISO date), search_string, next_page_token

  resp = client.ce.ListCostCategoryDefinitions({
    EffectiveOn: params.effective_on,
    NextToken:   params.next_page_token,
  })

  if resp.error in [ThrottlingException]:
    return RetryableError(resp.error)

  if resp.error:
    return TerminalError(resp.error)

  return Success({
    cost_categories: resp.CostCategoryReferences.map(c => ({
      arn:             c.CostCategoryArn,
      name:            c.Name,
      effective_start: c.EffectiveStart,
      effective_end:   c.EffectiveEnd,
      number_of_rules: c.NumberOfRules,
    })),
    next_page_token: resp.NextToken,
  })
```

---

#### `create_cost_category`
> Creates a new Cost Category with named rules that map dimension or tag values to category values. Used in account-vending and team-onboarding workflows to establish cost allocation groupings before any spend is incurred.

```
func create_cost_category(ctx, client, params):
  // params: name, rule_version, rules[]{ value, rule{} },
  //         default_value, split_charge_rules[]

  existing = find_cost_category_by_name(client, params.name)
  if existing is not null:
    return Success({
      cost_category_arn: existing.arn,
      effective_start:   existing.effective_start,
      idempotent:        true,
    })

  resp = client.ce.CreateCostCategoryDefinition({
    Name:             params.name,
    RuleVersion:      params.rule_version ?? "CostCategoryExpression.v1",
    Rules:            params.rules,
    DefaultValue:     params.default_value,
    SplitChargeRules: params.split_charge_rules ?? [],
  })

  if resp.error in [ThrottlingException, ServiceQuotaExceededException]:
    return RetryableError(resp.error)

  if resp.error in [DuplicateRecordException]:
    return Success({ idempotent: true })

  if resp.error:
    return TerminalError(resp.error)

  return Success({
    cost_category_arn: resp.CostCategoryArn,
    effective_start:   resp.EffectiveStart,
  })
```

---

#### `update_cost_category`
> Updates the rules or default value of an existing Cost Category. Used when team ownership, product boundaries, or chargeback rules change and the cost allocation model must be updated.

```
func update_cost_category(ctx, client, params):
  // params: cost_category_arn, rule_version, rules[],
  //         default_value, split_charge_rules[]

  resp = client.ce.UpdateCostCategoryDefinition({
    CostCategoryArn:  params.cost_category_arn,
    RuleVersion:      params.rule_version ?? "CostCategoryExpression.v1",
    Rules:            params.rules,
    DefaultValue:     params.default_value,
    SplitChargeRules: params.split_charge_rules ?? [],
  })

  if resp.error in [ThrottlingException]:
    return RetryableError(resp.error)

  if resp.error in [ResourceNotFoundException]:
    return TerminalError("cost_category_not_found")

  if resp.error:
    return TerminalError(resp.error)

  return Success({
    cost_category_arn: params.cost_category_arn,
    effective_start:   resp.EffectiveStart,
  })
```

---

#### `delete_cost_category`
> Deletes a Cost Category definition. Used in account decommissioning workflows to remove stale cost allocation categories no longer backed by active accounts or teams.

```
func delete_cost_category(ctx, client, params):
  // params: cost_category_arn

  resp = client.ce.DeleteCostCategoryDefinition({
    CostCategoryArn: params.cost_category_arn,
  })

  if resp.error in [ResourceNotFoundException]:
    return Success({ idempotent: true })

  if resp.error in [ThrottlingException]:
    return RetryableError(resp.error)

  if resp.error:
    return TerminalError(resp.error)

  return Success({
    cost_category_arn: params.cost_category_arn,
    effective_end:     resp.EffectiveEnd,
  })
```

---

### Budgets

---

#### `create_budget`
> Creates an AWS Budget for a given account with spend, usage, RI coverage, RI utilisation, or Savings Plans thresholds. Used in account-vending workflows to apply financial guardrails at the moment a new account is provisioned.

```
func create_budget(ctx, client, params):
  // params: account_id, budget{ name, budget_type, budget_limit{ amount, unit },
  //         cost_filters{}, cost_types{}, time_unit (DAILY|MONTHLY|QUARTERLY|ANNUALLY),
  //         time_period{ start, end }, planned_budget_limits{} },
  //         notifications_with_subscribers[]

  resp = client.budgets.CreateBudget({
    AccountId:                   params.account_id,
    Budget:                      params.budget,
    NotificationsWithSubscribers: params.notifications_with_subscribers ?? [],
  })

  if resp.error in [DuplicateRecordException]:
    return Success({ idempotent: true, budget_name: params.budget.name })

  if resp.error in [ThrottlingException, ServiceUnavailableException]:
    return RetryableError(resp.error)

  if resp.error in [CreationLimitExceededException]:
    return TerminalError("budget_limit_exceeded_for_account")

  if resp.error:
    return TerminalError(resp.error)

  return Success({
    account_id:   params.account_id,
    budget_name:  params.budget.name,
  })
```

---

#### `update_budget`
> Updates an existing budget's limit, time period, or cost filters. Used when a team's approved spend envelope changes and the existing budget guardrail must be adjusted to reflect the new allocation.

```
func update_budget(ctx, client, params):
  // params: account_id, budget{ name, ... updated fields ... }

  resp = client.budgets.UpdateBudget({
    AccountId: params.account_id,
    NewBudget: params.budget,
  })

  if resp.error in [NotFoundException]:
    return TerminalError("budget_not_found")

  if resp.error in [ThrottlingException]:
    return RetryableError(resp.error)

  if resp.error:
    return TerminalError(resp.error)

  return Success({
    account_id:  params.account_id,
    budget_name: params.budget.name,
  })
```

---

#### `delete_budget`
> Deletes a budget and all its associated notifications and actions. Used in account decommissioning workflows after spend visibility is no longer required for the account.

```
func delete_budget(ctx, client, params):
  // params: account_id, budget_name

  resp = client.budgets.DeleteBudget({
    AccountId:  params.account_id,
    BudgetName: params.budget_name,
  })

  if resp.error in [NotFoundException]:
    return Success({ idempotent: true })

  if resp.error in [ThrottlingException]:
    return RetryableError(resp.error)

  if resp.error:
    return TerminalError(resp.error)

  return Success({
    account_id:  params.account_id,
    budget_name: params.budget_name,
  })
```

---

#### `get_budget`
> Retrieves a single budget and its current spend figures. Used in spend-monitoring and approval workflows to check the current state of a budget before deciding whether to execute an action.

```
func get_budget(ctx, client, params):
  // params: account_id, budget_name

  resp = client.budgets.DescribeBudget({
    AccountId:  params.account_id,
    BudgetName: params.budget_name,
  })

  if resp.error in [NotFoundException]:
    return TerminalError("budget_not_found")

  if resp.error in [ThrottlingException]:
    return RetryableError(resp.error)

  if resp.error:
    return TerminalError(resp.error)

  return Success({
    budget: {
      name:               resp.Budget.BudgetName,
      budget_type:        resp.Budget.BudgetType,
      budget_limit:       resp.Budget.BudgetLimit,
      calculated_spend:   resp.Budget.CalculatedSpend,
      time_unit:          resp.Budget.TimeUnit,
      time_period:        resp.Budget.TimePeriod,
      last_updated:       resp.Budget.LastUpdatedTime,
    },
  })
```

---

#### `list_budgets`
> Lists all budgets for an account. Used in governance reporting workflows to audit the presence and configuration of financial guardrails across the organisation.

```
func list_budgets(ctx, client, params):
  // params: account_id, max_results, next_token

  budgets    = []
  next_token = params.next_token ?? null

  loop:
    resp = client.budgets.DescribeBudgets({
      AccountId:  params.account_id,
      MaxResults: params.max_results ?? 100,
      NextToken:  next_token,
    })

    if resp.error in [ThrottlingException]:
      return RetryableError(resp.error)

    if resp.error in [NotFoundException]:
      return Success({ budgets: [], account_id: params.account_id })

    if resp.error:
      return TerminalError(resp.error)

    budgets.append_all(resp.Budgets)
    next_token = resp.NextToken
    if next_token is null: break

  return Success({
    account_id: params.account_id,
    budgets:    budgets,
    count:      budgets.length,
  })
```

---

#### `create_budget_action`
> Creates an automated action attached to a budget notification threshold — for example, applying an IAM or SCP policy, or stopping EC2/RDS instances when spend reaches a defined percentage. Used in hard-limit enforcement workflows to automatically constrain spend without human intervention.

```
func create_budget_action(ctx, client, params):
  // params: account_id, budget_name, notification_type (ACTUAL|FORECASTED),
  //         action_type (APPLY_IAM_POLICY|APPLY_SCP_POLICY|RUN_SSM_DOCUMENTS),
  //         action_threshold{ value, type (PERCENTAGE|ABSOLUTE_VALUE) },
  //         definition{ iam_action_definition | scp_action_definition |
  //                     ssm_action_definition },
  //         execution_role_arn,
  //         approval_model (AUTOMATIC|MANUAL),
  //         subscribers[]

  resp = client.budgets.CreateBudgetAction({
    AccountId:        params.account_id,
    BudgetName:       params.budget_name,
    NotificationType: params.notification_type,
    ActionType:       params.action_type,
    ActionThreshold:  params.action_threshold,
    Definition:       params.definition,
    ExecutionRoleArn: params.execution_role_arn,
    ApprovalModel:    params.approval_model ?? "AUTOMATIC",
    Subscribers:      params.subscribers ?? [],
  })

  if resp.error in [DuplicateRecordException]:
    return Success({ idempotent: true })

  if resp.error in [ThrottlingException]:
    return RetryableError(resp.error)

  if resp.error in [NotFoundException]:
    return TerminalError("budget_not_found")

  if resp.error:
    return TerminalError(resp.error)

  return Success({
    account_id:  params.account_id,
    budget_name: params.budget_name,
    action_id:   resp.ActionId,
  })
```

---

#### `delete_budget_action`
> Removes an automated action from a budget. Used when enforcement policy is relaxed or when the action is superseded by a revised action definition.

```
func delete_budget_action(ctx, client, params):
  // params: account_id, budget_name, action_id

  resp = client.budgets.DeleteBudgetAction({
    AccountId:  params.account_id,
    BudgetName: params.budget_name,
    ActionId:   params.action_id,
  })

  if resp.error in [NotFoundException]:
    return Success({ idempotent: true })

  if resp.error in [ThrottlingException]:
    return RetryableError(resp.error)

  if resp.error:
    return TerminalError(resp.error)

  return Success({
    account_id:  params.account_id,
    budget_name: params.budget_name,
    action_id:   params.action_id,
  })
```

---

#### `execute_budget_action`
> Manually triggers or reverses a budget action regardless of the current threshold state. Used in incident-response workflows where spend must be immediately curtailed without waiting for a threshold to be breached.

```
func execute_budget_action(ctx, client, params):
  // params: account_id, budget_name, action_id,
  //         execution_type (APPROVE_BUDGET_ACTION|RETRY_BUDGET_ACTION|
  //                         REVERSE_BUDGET_ACTION|RESET_BUDGET_ACTION)

  resp = client.budgets.ExecuteBudgetAction({
    AccountId:     params.account_id,
    BudgetName:    params.budget_name,
    ActionId:      params.action_id,
    ExecutionType: params.execution_type,
  })

  if resp.error in [ThrottlingException]:
    return RetryableError(resp.error)

  if resp.error in [NotFoundException]:
    return TerminalError("budget_action_not_found")

  if resp.error in [ResourceLockedException]:
    return RetryableError("action_locked_retry")

  if resp.error:
    return TerminalError(resp.error)

  return Success({
    account_id:     resp.AccountId,
    budget_name:    resp.BudgetName,
    action_id:      resp.ActionId,
    execution_type: params.execution_type,
  })
```

---

#### `list_budget_actions`
> Lists all actions attached to a given budget. Used in governance audits to verify that every active budget has the required automated enforcement actions in place.

```
func list_budget_actions(ctx, client, params):
  // params: account_id, budget_name, max_results, next_token

  actions    = []
  next_token = params.next_token ?? null

  loop:
    resp = client.budgets.DescribeBudgetActionsForBudget({
      AccountId:  params.account_id,
      BudgetName: params.budget_name,
      MaxResults: params.max_results ?? 100,
      NextToken:  next_token,
    })

    if resp.error in [ThrottlingException]:
      return RetryableError(resp.error)

    if resp.error in [NotFoundException]:
      return Success({ actions: [], budget_name: params.budget_name })

    if resp.error:
      return TerminalError(resp.error)

    actions.append_all(resp.Actions)
    next_token = resp.NextToken
    if next_token is null: break

  return Success({
    account_id:  params.account_id,
    budget_name: params.budget_name,
    actions:     actions,
  })
```

---

### Anomaly Detection

---

#### `create_anomaly_monitor`
> Creates a cost anomaly monitor scoped to an AWS service, linked account, cost category, or custom tag. Used in platform-onboarding workflows to ensure every team's cost dimension is covered by anomaly detection from the day the account is provisioned.

```
func create_anomaly_monitor(ctx, client, params):
  // params: monitor_name,
  //         monitor_type (DIMENSIONAL|CUSTOM),
  //         monitor_dimension (SERVICE — if DIMENSIONAL),
  //         monitor_specification (Expression — if CUSTOM),
  //         resource_tags[]

  resp = client.ce.CreateAnomalyMonitor({
    AnomalyMonitor: {
      MonitorName:          params.monitor_name,
      MonitorType:          params.monitor_type,
      MonitorDimension:     params.monitor_dimension,
      MonitorSpecification: params.monitor_specification,
    },
    ResourceTags: params.resource_tags ?? [],
  })

  if resp.error in [ThrottlingException, LimitExceededException]:
    return RetryableError(resp.error)

  if resp.error:
    return TerminalError(resp.error)

  return Success({ monitor_arn: resp.MonitorArn })
```

---

#### `delete_anomaly_monitor`
> Deletes a cost anomaly monitor and its associated subscriptions. Used in account decommissioning workflows to clean up monitoring infrastructure after a team or product is retired.

```
func delete_anomaly_monitor(ctx, client, params):
  // params: monitor_arn

  resp = client.ce.DeleteAnomalyMonitor({
    MonitorArn: params.monitor_arn,
  })

  if resp.error in [UnknownMonitorException]:
    return Success({ idempotent: true })

  if resp.error in [ThrottlingException]:
    return RetryableError(resp.error)

  if resp.error:
    return TerminalError(resp.error)

  return Success({ monitor_arn: params.monitor_arn })
```

---

#### `list_anomaly_monitors`
> Lists all anomaly monitors in the account. Used in coverage-verification workflows to confirm that all required cost dimensions have active monitors.

```
func list_anomaly_monitors(ctx, client, params):
  // params: monitor_arns[], next_page_token, max_results

  monitors   = []
  page_token = params.next_page_token ?? null

  loop:
    resp = client.ce.GetAnomalyMonitors({
      MonitorArnList: params.monitor_arns ?? [],
      NextPageToken:  page_token,
      MaxResults:     params.max_results ?? 100,
    })

    if resp.error in [ThrottlingException]:
      return RetryableError(resp.error)

    if resp.error:
      return TerminalError(resp.error)

    monitors.append_all(resp.AnomalyMonitors)
    page_token = resp.NextPageToken
    if page_token is null: break

  return Success({ monitors: monitors, count: monitors.length })
```

---

#### `create_anomaly_subscription`
> Creates a subscription that routes anomaly alerts above a defined threshold to SNS topics or email addresses. Used in platform-onboarding workflows to wire anomaly notifications to team channels at provisioning time.

```
func create_anomaly_subscription(ctx, client, params):
  // params: subscription_name,
  //         monitor_arns[],
  //         subscribers[]{ address, type (EMAIL|SNS) },
  //         threshold_expression (or legacy: threshold as float),
  //         frequency (DAILY|IMMEDIATE|WEEKLY),
  //         resource_tags[]

  resp = client.ce.CreateAnomalySubscription({
    AnomalySubscription: {
      SubscriptionName:    params.subscription_name,
      MonitorArnList:      params.monitor_arns,
      Subscribers:         params.subscribers,
      ThresholdExpression: params.threshold_expression,
      Frequency:           params.frequency ?? "DAILY",
    },
    ResourceTags: params.resource_tags ?? [],
  })

  if resp.error in [ThrottlingException, LimitExceededException]:
    return RetryableError(resp.error)

  if resp.error:
    return TerminalError(resp.error)

  return Success({ subscription_arn: resp.SubscriptionArn })
```

---

#### `delete_anomaly_subscription`
> Deletes an anomaly subscription. Used when alert routing is changed or when a team's notification channel is decommissioned.

```
func delete_anomaly_subscription(ctx, client, params):
  // params: subscription_arn

  resp = client.ce.DeleteAnomalySubscription({
    SubscriptionArn: params.subscription_arn,
  })

  if resp.error in [UnknownSubscriptionException]:
    return Success({ idempotent: true })

  if resp.error in [ThrottlingException]:
    return RetryableError(resp.error)

  if resp.error:
    return TerminalError(resp.error)

  return Success({ subscription_arn: params.subscription_arn })
```

---

#### `list_anomaly_subscriptions`
> Lists all anomaly subscriptions. Used in governance audits to verify that every anomaly monitor has at least one active subscription with a valid notification target.

```
func list_anomaly_subscriptions(ctx, client, params):
  // params: subscription_arns[], monitor_arn, next_page_token, max_results

  subscriptions = []
  page_token    = params.next_page_token ?? null

  loop:
    resp = client.ce.GetAnomalySubscriptions({
      SubscriptionArnList: params.subscription_arns ?? [],
      MonitorArn:          params.monitor_arn,
      NextPageToken:       page_token,
      MaxResults:          params.max_results ?? 100,
    })

    if resp.error in [ThrottlingException]:
      return RetryableError(resp.error)

    if resp.error:
      return TerminalError(resp.error)

    subscriptions.append_all(resp.AnomalySubscriptions)
    page_token = resp.NextPageToken
    if page_token is null: break

  return Success({ subscriptions: subscriptions, count: subscriptions.length })
```

---

#### `get_anomalies`
> Retrieves detected cost anomalies within a date range, filterable by monitor, feedback status, and minimum impact. Used in daily FinOps review workflows and automated triage pipelines to surface unacknowledged spend spikes.

```
func get_anomalies(ctx, client, params):
  // params: date_interval{ start_date, end_date },
  //         monitor_arn, feedback (PLANNED_ACTIVITY|YES|NO),
  //         total_impact{ numeric_operator, start_value, end_value },
  //         next_page_token, max_results

  anomalies  = []
  page_token = params.next_page_token ?? null

  loop:
    resp = client.ce.GetAnomalies({
      DateInterval:   { StartDate: params.date_interval.start_date,
                        EndDate:   params.date_interval.end_date },
      MonitorArn:     params.monitor_arn,
      Feedback:       params.feedback,
      TotalImpact:    params.total_impact,
      NextPageToken:  page_token,
      MaxResults:     params.max_results ?? 100,
    })

    if resp.error in [ThrottlingException, InvalidNextTokenException]:
      return RetryableError(resp.error)

    if resp.error:
      return TerminalError(resp.error)

    anomalies.append_all(resp.Anomalies.map(a => ({
      anomaly_id:     a.AnomalyId,
      monitor_arn:    a.MonitorArn,
      anomaly_start:  a.AnomalyStartDate,
      anomaly_end:    a.AnomalyEndDate,
      dimension_value: a.DimensionValue,
      root_causes:    a.RootCauses,
      impact: {
        max_impact:       a.Impact.MaxImpact,
        total_impact:     a.Impact.TotalImpact,
        total_actual:     a.Impact.TotalActualSpend,
        total_expected:   a.Impact.TotalExpectedSpend,
      },
      feedback:       a.Feedback,
    })))

    page_token = resp.NextPageToken
    if page_token is null: break

  return Success({ anomalies: anomalies, count: anomalies.length })
```

---

### Savings Plans

---

#### `describe_savings_plans`
> Returns details of active, queued, or expired Savings Plans commitments. Used in commitment inventory workflows to maintain an accurate picture of contracted spend and upcoming expiry dates.

```
func describe_savings_plans(ctx, client, params):
  // params: savings_plan_arns[], savings_plan_ids[], next_token,
  //         max_results, states[], filters[]

  resp = client.savingsplans.DescribeSavingsPlans({
    SavingsPlanArns: params.savings_plan_arns ?? [],
    SavingsPlanIds:  params.savings_plan_ids ?? [],
    NextToken:       params.next_token,
    MaxResults:      params.max_results ?? 100,
    States:          params.states ?? [],
    Filters:         params.filters ?? [],
  })

  if resp.error in [ThrottlingException]:
    return RetryableError(resp.error)

  if resp.error:
    return TerminalError(resp.error)

  return Success({
    savings_plans:  resp.SavingsPlans.map(sp => ({
      savings_plan_id:   sp.SavingsPlanId,
      savings_plan_arn:  sp.SavingsPlanArn,
      state:             sp.State,
      savings_plan_type: sp.SavingsPlanType,
      payment_option:    sp.PaymentOption,
      term_duration:     sp.TermDurationInSeconds,
      commitment:        sp.Commitment,
      currency:          sp.Currency,
      start:             sp.Start,
      end:               sp.End,
    })),
    next_token: resp.NextToken,
  })
```

---

#### `get_savings_plans_coverage`
> Returns the percentage of eligible spend covered by Savings Plans for a time range. Used in commitment health dashboards to identify coverage gaps where on-demand spend is not being absorbed by existing commitments.

```
func get_savings_plans_coverage(ctx, client, params):
  // params: time_period{ start, end }, granularity, group_by[], filter{},
  //         metrics[], next_token, max_results

  resp = client.ce.GetSavingsPlansCoverage({
    TimePeriod:   { Start: params.time_period.start,
                    End:   params.time_period.end },
    Granularity:  params.granularity ?? "MONTHLY",
    GroupBy:      params.group_by ?? [],
    Filter:       params.filter,
    Metrics:      params.metrics ?? ["SpendCoveredBySavingsPlans"],
    NextToken:    params.next_token,
    MaxResults:   params.max_results ?? 100,
  })

  if resp.error in [ThrottlingException, DataUnavailableException]:
    return RetryableError(resp.error)

  if resp.error:
    return TerminalError(resp.error)

  return Success({
    coverages:  resp.SavingsPlansCoverages,
    total:      resp.Total,
    next_token: resp.NextToken,
  })
```

---

#### `get_savings_plans_utilization`
> Returns utilisation metrics showing how much of the Savings Plans commitment is being consumed. Used in commitment health monitoring workflows to detect over-commitment and surface underused plans for potential modification.

```
func get_savings_plans_utilization(ctx, client, params):
  // params: time_period{ start, end }, granularity, filter{},
  //         sort_by{ key, order }, next_token, max_results

  resp = client.ce.GetSavingsPlansUtilization({
    TimePeriod:  { Start: params.time_period.start,
                   End:   params.time_period.end },
    Granularity: params.granularity ?? "MONTHLY",
    Filter:      params.filter,
    SortBy:      params.sort_by,
  })

  if resp.error in [ThrottlingException, DataUnavailableException]:
    return RetryableError(resp.error)

  if resp.error:
    return TerminalError(resp.error)

  return Success({
    utilizations: resp.SavingsPlansUtilizationsByTime,
    total:        resp.Total,
  })
```

---

#### `get_savings_plans_purchase_recommendation`
> Returns AWS-generated purchase recommendations for Savings Plans based on historical usage. Used in monthly commitment review workflows to surface actionable purchase candidates with estimated savings and payback analysis.

```
func get_savings_plans_purchase_recommendation(ctx, client, params):
  // params: savings_plans_type (COMPUTE_SP|EC2_INSTANCE_SP|SAGEMAKER_SP),
  //         term_in_years (ONE_YEAR|THREE_YEARS),
  //         payment_option (ALL_UPFRONT|PARTIAL_UPFRONT|NO_UPFRONT),
  //         lookback_period (SEVEN_DAYS|THIRTY_DAYS|SIXTY_DAYS),
  //         account_scope (PAYER|LINKED), filter{}, next_page_token, page_size

  resp = client.ce.GetSavingsPlansPurchaseRecommendation({
    SavingsPlansType:          params.savings_plans_type,
    TermInYears:               params.term_in_years ?? "ONE_YEAR",
    PaymentOption:             params.payment_option ?? "NO_UPFRONT",
    LookbackPeriodInDays:      params.lookback_period ?? "THIRTY_DAYS",
    AccountScope:              params.account_scope ?? "PAYER",
    Filter:                    params.filter,
    NextPageToken:             params.next_page_token,
    PageSize:                  params.page_size ?? 20,
  })

  if resp.error in [ThrottlingException]:
    return RetryableError(resp.error)

  if resp.error:
    return TerminalError(resp.error)

  return Success({
    metadata:        resp.Metadata,
    summary:         resp.SavingsPlansPurchaseRecommendationSummary,
    recommendations: resp.SavingsPlansPurchaseRecommendation
                        .SavingsPlansPurchaseRecommendationDetails.map(r => ({
      hourly_commitment:            r.HourlyCommitmentToPurchase,
      estimated_savings_amount:     r.EstimatedSavingsAmount,
      estimated_savings_percentage: r.EstimatedSavingsPercentage,
      estimated_monthly_savings:    r.EstimatedMonthlySavingsAmount,
      estimated_on_demand_cost:     r.EstimatedOnDemandCost,
      current_avg_hourly_od_spend:  r.CurrentAverageHourlyOnDemandSpend,
      upfront_cost:                 r.UpfrontCost,
      recurring_monthly_cost:       r.RecurringStandardMonthlyCost,
    })),
    next_page_token: resp.NextPageToken,
  })
```

---

### Reserved Instances

---

#### `describe_reserved_instances`
> Returns details of active, retired, or queued Reserved Instances. Used in commitment inventory workflows to track RI expiry dates and identify instances approaching end-of-term that require renewal or replacement decisions.

```
func describe_reserved_instances(ctx, client, params):
  // params: reserved_instances_ids[], filters[], offering_class,
  //         offering_type, include_marketplace

  resp = client.ec2.DescribeReservedInstances({
    ReservedInstancesIds: params.reserved_instances_ids ?? [],
    Filters:              params.filters ?? [],
    OfferingClass:        params.offering_class ?? "STANDARD",
    OfferingType:         params.offering_type,
  })

  if resp.error in [ThrottlingException]:
    return RetryableError(resp.error)

  if resp.error:
    return TerminalError(resp.error)

  return Success({
    reserved_instances: resp.ReservedInstances.map(ri => ({
      reserved_instances_id: ri.ReservedInstancesId,
      instance_type:         ri.InstanceType,
      instance_count:        ri.InstanceCount,
      availability_zone:     ri.AvailabilityZone,
      scope:                 ri.Scope,
      state:                 ri.State,
      start:                 ri.Start,
      end:                   ri.End,
      duration:              ri.Duration,
      fixed_price:           ri.FixedPrice,
      usage_price:           ri.UsagePrice,
      offering_class:        ri.OfferingClass,
      offering_type:         ri.OfferingType,
      product_description:   ri.ProductDescription,
    })),
  })
```

---

#### `get_ri_coverage`
> Returns the percentage of running instance hours covered by Reserved Instances. Used in commitment health dashboards to identify coverage gaps by service, region, or instance family.

```
func get_ri_coverage(ctx, client, params):
  // params: time_period{ start, end }, group_by[], granularity,
  //         filter{}, metrics[], next_page_token

  resp = client.ce.GetReservationCoverage({
    TimePeriod:    { Start: params.time_period.start,
                     End:   params.time_period.end },
    GroupBy:       params.group_by ?? [],
    Granularity:   params.granularity ?? "MONTHLY",
    Filter:        params.filter,
    Metrics:       params.metrics ?? ["CoverageHours"],
    NextPageToken: params.next_page_token,
  })

  if resp.error in [ThrottlingException, DataUnavailableException]:
    return RetryableError(resp.error)

  if resp.error:
    return TerminalError(resp.error)

  return Success({
    coverages:       resp.CoveragesByTime,
    total:           resp.Total,
    next_page_token: resp.NextPageToken,
  })
```

---

#### `get_ri_utilization`
> Returns RI utilisation metrics showing how much reserved capacity is being consumed. Used in commitment health monitoring to detect underutilised reservations that are candidates for modification or marketplace listing.

```
func get_ri_utilization(ctx, client, params):
  // params: time_period{ start, end }, granularity, group_by[], filter{},
  //         sort_by{ key, order }, next_page_token, max_results

  resp = client.ce.GetReservationUtilization({
    TimePeriod:    { Start: params.time_period.start,
                     End:   params.time_period.end },
    Granularity:   params.granularity ?? "MONTHLY",
    GroupBy:       params.group_by ?? [],
    Filter:        params.filter,
    SortBy:        params.sort_by,
    NextPageToken: params.next_page_token,
    MaxResults:    params.max_results ?? 100,
  })

  if resp.error in [ThrottlingException, DataUnavailableException]:
    return RetryableError(resp.error)

  if resp.error:
    return TerminalError(resp.error)

  return Success({
    utilizations:    resp.UtilizationsByTime,
    total:           resp.Total,
    next_page_token: resp.NextPageToken,
  })
```

---

#### `get_ri_purchase_recommendation`
> Returns AWS-generated RI purchase recommendations. Used in quarterly commitment review workflows to identify opportunities to convert on-demand spend to reserved capacity with projected break-even and savings figures.

```
func get_ri_purchase_recommendation(ctx, client, params):
  // params: service, account_id, account_scope, lookback_period,
  //         term_in_years, payment_option, filter{}, next_page_token, page_size

  resp = client.ce.GetReservationPurchaseRecommendation({
    Service:              params.service,
    AccountId:            params.account_id,
    AccountScope:         params.account_scope ?? "PAYER",
    LookbackPeriodInDays: params.lookback_period ?? "THIRTY_DAYS",
    TermInYears:          params.term_in_years ?? "ONE_YEAR",
    PaymentOption:        params.payment_option ?? "NO_UPFRONT",
    Filter:               params.filter,
    NextPageToken:        params.next_page_token,
    PageSize:             params.page_size ?? 20,
  })

  if resp.error in [ThrottlingException]:
    return RetryableError(resp.error)

  if resp.error:
    return TerminalError(resp.error)

  return Success({
    metadata:        resp.Metadata,
    summary:         resp.RecommendationSummaries,
    recommendations: resp.Recommendations.map(r => ({
      account_id:       r.AccountId,
      instance_details: r.RecommendationDetails.map(d => ({
        instance_type:               d.InstanceDetails,
        recommended_units:           d.RecommendedNumberOfInstancesToPurchase,
        expected_utilization:        d.ExpectedUtilization,
        estimated_break_even_months: d.EstimatedBreakEvenInMonths,
        estimated_monthly_savings:   d.EstimatedMonthlyOnDemandCost -
                                       d.EstimatedReservationCostForLookbackPeriod / 12,
        upfront_cost:                d.UpfrontCost,
        recurring_monthly_cost:      d.RecurringStandardMonthlyCost,
      })),
    })),
    next_page_token: resp.NextPageToken,
  })
```

---

#### `modify_reserved_instances`
> Modifies the configuration of existing RIs — changing instance type, scope, or AZ within the same family. Used in rightsizing workflows when RI inventory must be restructured to match a shifting compute footprint.

```
func modify_reserved_instances(ctx, client, params):
  // params: client_token, reserved_instances_ids[],
  //         target_configurations[]{ availability_zone, platform,
  //                                   instance_count, instance_type,
  //                                   scope, tenancy }

  resp = client.ec2.ModifyReservedInstances({
    ClientToken:           params.client_token,
    ReservedInstancesIds:  params.reserved_instances_ids,
    TargetConfigurations:  params.target_configurations,
  })

  if resp.error in [ThrottlingException]:
    return RetryableError(resp.error)

  if resp.error:
    return TerminalError(resp.error)

  return Success({
    modification_id: resp.ReservedInstancesModificationId,
  })
```

---

### Rightsizing

---

#### `get_rightsizing_recommendation`
> Returns Cost Explorer rightsizing recommendations for EC2 instances — terminate or downsize — with estimated savings. Used in automated cost optimisation workflows that surface recommendations to engineers via tickets or approval workflows before actioning.

```
func get_rightsizing_recommendation(ctx, client, params):
  // params: service, filter{},
  //         configuration{ recommendation_target, benefits_considered },
  //         page_size, next_page_token

  resp = client.ce.GetRightsizingRecommendation({
    Service: params.service ?? "AmazonEC2",
    Filter:  params.filter,
    Configuration: {
      RecommendationTarget: params.configuration.recommendation_target
                              ?? "SAME_INSTANCE_FAMILY",
      BenefitsConsidered:   params.configuration.benefits_considered ?? true,
    },
    PageSize:      params.page_size ?? 100,
    NextPageToken: params.next_page_token,
  })

  if resp.error in [ThrottlingException, InvalidNextTokenException]:
    return RetryableError(resp.error)

  if resp.error:
    return TerminalError(resp.error)

  return Success({
    summary:          resp.Summary,
    recommendations:  resp.RightsizingRecommendations.map(r => ({
      account_id:           r.AccountId,
      current_instance:     r.CurrentInstance,
      rightsizing_type:     r.RightsizingType,    // TERMINATE | MODIFY
      modification:         r.ModifyRecommendationDetail,
      termination:          r.TerminateRecommendationDetail,
      finding_reason_codes: r.FindingReasonCodes,
    })),
    next_page_token:  resp.NextPageToken,
    metadata:         resp.Metadata,
  })
```

---

#### `get_resource_utilization`
> Returns CPU, memory, storage, and network utilisation statistics for a set of resources. Used alongside rightsizing recommendations to validate that a resource is genuinely underutilised before proposing a downsize or termination.

```
func get_resource_utilization(ctx, client, params):
  // params: filter{}, next_page_token, page_size

  resp = client.ce.GetResourceOptimizationRecommendationStatistics({
    Filter:        params.filter,
    NextPageToken: params.next_page_token,
    PageSize:      params.page_size ?? 100,
  })

  if resp.error in [ThrottlingException]:
    return RetryableError(resp.error)

  if resp.error:
    return TerminalError(resp.error)

  return Success({
    statistics:      resp.ResourceOptimizationRecommendationStatistics,
    next_page_token: resp.NextPageToken,
  })
```

---

### Tagging

---

#### `tag_resources`
> Applies a set of tags to one or more AWS resources by ARN. Used in post-provisioning workflows to enforce required cost allocation tags — such as `team`, `environment`, `cost-centre`, and `product` — immediately after a resource is created.

```
func tag_resources(ctx, client, params):
  // params: resource_arns[], tags{}, region

  batches  = chunk(params.resource_arns, 20)
  failures = []

  for batch in batches:
    resp = client.tagging.TagResources({
      ResourceARNList: batch,
      Tags:            params.tags,
    })

    if resp.error in [ThrottlingException]:
      return RetryableError(resp.error)

    if resp.error:
      return TerminalError(resp.error)

    if resp.FailedResourcesMap is not empty:
      failures.append_all(resp.FailedResourcesMap)

  if failures is not empty:
    return TerminalError({ reason: "partial_tagging_failure", failures: failures })

  return Success({ tagged_count: params.resource_arns.length, tags: params.tags })
```

---

#### `untag_resources`
> Removes specific tag keys from one or more AWS resources. Used in tag-remediation workflows when incorrect cost allocation tags must be removed before correct tags are reapplied.

```
func untag_resources(ctx, client, params):
  // params: resource_arns[], tag_keys[], region

  batches  = chunk(params.resource_arns, 20)
  failures = []

  for batch in batches:
    resp = client.tagging.UntagResources({
      ResourceARNList: batch,
      TagKeys:         params.tag_keys,
    })

    if resp.error in [ThrottlingException]:
      return RetryableError(resp.error)

    if resp.error:
      return TerminalError(resp.error)

    if resp.FailedResourcesMap is not empty:
      failures.append_all(resp.FailedResourcesMap)

  if failures is not empty:
    return TerminalError({ reason: "partial_untag_failure", failures: failures })

  return Success({ untagged_count: params.resource_arns.length })
```

---

#### `get_resources_by_tags`
> Lists all AWS resources matching a given tag filter. Used in compliance workflows to enumerate resources that share a tag value before bulk-updating or removing them.

```
func get_resources_by_tags(ctx, client, params):
  // params: tag_filters[]{ key, values[] }, resource_type_filters[],
  //         resources_per_page, pagination_token, region,
  //         include_compliance_details, exclude_compliant_resources

  results    = []
  page_token = params.pagination_token ?? null

  loop:
    resp = client.tagging.GetResources({
      TagFilters:                params.tag_filters,
      ResourceTypeFilters:       params.resource_type_filters ?? [],
      ResourcesPerPage:          params.resources_per_page ?? 100,
      PaginationToken:           page_token,
      IncludeComplianceDetails:  params.include_compliance_details ?? false,
      ExcludeCompliantResources: params.exclude_compliant_resources ?? false,
    })

    if resp.error in [ThrottlingException]:
      return RetryableError(resp.error)

    if resp.error:
      return TerminalError(resp.error)

    results.append_all(resp.ResourceTagMappingList.map(r => ({
      resource_arn:       r.ResourceARN,
      tags:               r.Tags,