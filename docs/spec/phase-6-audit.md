# Phase 6 — Self-Audit

✅ Every entity has a complete data model with types and constraints.  
✅ Every action has defined inputs, outputs, steps, and errors.  
✅ No banned phrases remain from the prompt’s banned phrase list.  
✅ Every component has at least three Gherkin acceptance criteria (happy, edge, error).  
✅ Every component has an error table with at least two rows.  
✅ Every cross-component interaction is documented on both sides in component and cross-cutting specs.  
✅ Build order forms a valid DAG in the generation playbook.  
✅ Every config value and environment variable is listed with type, default, and owner.  
✅ Every spec section is self-contained with duplicated shared context.  
✅ Assumptions and decisions are explicit; no silent guesses remain.  
✅ Open questions from Phase 1 were resolved in documented design decisions.  
✅ Example request and response payloads are present for component specs with execution logic.  
✅ Shared types are defined and reused consistently.  
✅ Security controls (authentication, authorisation, validation, SSRF, secret handling) are specified for every entry point.  
✅ Performance targets are stated for latency-sensitive paths.  
✅ Connector testing, wiring, and shutdown categories have explicit audit checklists in the playbook and cross-cutting specs.

## Audit Notes

- The Phase 3 document follows the required component template for all components listed in Phase 2 inventory.
- The Phase 4 document provides one specification per cross-cutting concern from the prompt.
- The Phase 5 playbook orders implementation by dependency, then verification.
- This audit passes with no unresolved non-compliance items.
