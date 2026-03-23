# RFC-INFRA-008: Approval Context Normalization and Release Engine Enforcement

--

## 1. Summary

This RFC normalizes the approval vocabulary, execution state modeling, and architectural boundaries in the `INFRA` domain to formally align with the capabilities defined in `SCAFFOLD` (specifically **RFC-SCAFFOLD-005**).

While the `INFRA` provision intent specifies how metadata can be derived for approval considerations, it previously lacked a formally consistent vocabulary or lifecycle state model. This RFC serves as an addendum to the `INFRA` standard to provide **explicit structural, operational, and architectural isomorphism** between both domains.

---

## 2. Core Architectural Principle

Across both INFRA and SCAFFOLD architectures, the following absolute constraint applies:

> **Derived approval metadata MUST NOT be interpreted as an approval decision.**  
> **Only Release Engine may create, evaluate, record, or satisfy approval state for executable work.**

At the request-layer (MCP, compiler, agent):
- Systems **MAY** derive: approval required flags, risk metadata, approval reason codes, reviewer hints, and supporting human review context.
- This derived information is strictly **advisory input to execution governance**.

At the execution-layer (Release Engine):
- Release Engine **owns** placing work into `awaiting_approval`.
- Release Engine **validates** approver identities and authority.
- Release Engine **enforces** separation of duties.
- Release Engine **records** approval decisions.
- Release Engine **resumes** execution upon successful approval lifecycle event.

---

## 3. Normalized Approval Payload Component Schema

To eliminate the disjointed metadata (such as `require_approval`, `approval_ttl`, `approval_metadata`) in prior `INFRA` compile proposals, compiled requests **SHOULD** adopt a consolidated `approvalContext` wrapper.

### 3.1 Recommended Payload Inclusion

```yaml
approvalContext:
  required: true
  decisionBasis:
    policyOutcome: allow_with_approval
    reasonCodes:
      - elevated_privilege_template
      - high_blast_radius
      - production_target
  riskSummary:
    blastRadius: high
    environment: production
    estimatedCostBand: medium
    sensitiveCapabilities:
      - public_ingress
  reviewContext:
    requestedBy: user@acme.example
    targetScope: team-data/production
    changeSummary: "Provision highly-available PostgreSQL analytics cluster"
  suggestedApproverRoles:
    - techops-lead
    - security-reviewer
  ttl:
    expiresAt: 2026-03-31T12:00:00Z
```

This encapsulates all approval considerations for submission. It is **context**, not a definitive decision object.

---

## 4. Policy Outcome Normalization

`INFRA` policy evaluations via MCP tools (e.g., `validate_infra_request`) **MUST** use the harmonized outcome vocabulary:

- `allow`: The infrastructure request implies accepted risks and needs no further validation.
- `allow_with_approval`: The infrastructure request is structurally compliant but has elements that cross policy thresholds needing human authorization.
- `deny`: The infrastructure request fails compliance validations or requests inherently blocked components (submission to Release Engine MUST NOT proceed).

This replaces previous `manual_review_required` loose validation warnings with concrete boundaries.

---

## 5. Normalized Execution Lifecycle State Names

Both `SCAFFOLD` and `INFRA` **MUST** map status behaviors identically when integrating and presenting outputs from Release Engine.

The formal states are:
- `queued` / `pending_submission`
- `running`: Engine execution is actively ongoing.
- `awaiting_approval`: Engine recognizes an unsatisfied approval block. Execution is paused.
- `approved`: Validated positive approval recorded (system immediately prepares to resume `running`).
- `rejected`: Approval explicitly denied by authoritative party. Terminal.
- `expired`: Approval timeout exceeded limits. Terminal.
- `cancelling`
- `cancelled`
- `succeeded`
- `failed`

*Implementation Note:* There should be no explicit `paused` state loosely mapped; it must strictly be `awaiting_approval` when pausing for this governance.

---

## 6. Security and Governance Extensions

### 6.1 Separation of Duties (SoD)

Any infrastructure template mapping a high severity component requires explicit checking, governed primarily by Release Engine constraints:
- Requesters **MAY NOT** authorize their own high-risk infra configurations.
- Delegated approvals must map exactly to authorized identity contexts.
- Approval actors and their identities must be bound directly to the execution intent block that was presented to them (`approvalContext.reviewContext` and compiled `InfrastructureRequest` hash).

### 6.2 Pre-Calculation Integrity Constraints

Upon resumed execution following an `approved` state transition:
- Release Engine **resumes execution strictly using the originally bound, generated and sealed request context**.
- An `approval` **MUST NOT** trigger a silent recompile that might change the provision trajectory.
- The execution plan (`job.params` representing `infra/provision-crossplane` values) is hashed and protected before entering `awaiting_approval`.

---

## 7. Migration Notes

1. All prior INFRA specifications (RFC-INFRA-001 through 007) have been updated to natively use the `approvalContext` block and explicit `allow`, `allow_with_approval`, and `deny` vocabulary.
2. Code mapping states such as "timeout" for approval windows should map explicitly to `expired` for consistent monitoring.

## 8. See Also
- **RFC-SCAFFOLD-005**: Details the identical contract applied successfully across the `SCAFFOLD` domain model.