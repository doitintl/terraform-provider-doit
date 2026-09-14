---
name: implement-validator
description: Add or modify Terraform Plugin Framework validators in this provider using the enforced canonical unknown-safe shapes and regression-test conventions.
---

# Implement Validator

Use this skill whenever adding or changing validation of Terraform configuration values.

## Choose the Validator Kind

- Use an attribute validator (`validator.String`, `validator.List`, and related interfaces) when validity depends only on that attribute and its nested values.
- Use a resource, data-source, or provider `ConfigValidator` when validity depends on multiple attributes.
- Put resource-specific validators in `<resource>_validator.go` or `<resource>_validators.go`. Shared resource/data-source helpers belong in the existing shared validator file.
- Add compile-time interface checks for configuration validators.

## Canonical Unknown-Safe Shapes

Validity that depends on an unknown value must be deferred. Keep state predicates direct; never store `IsUnknown()` or `IsNull()` in raw Boolean aliases.

Scalar or whole collection:

```go
if value.IsNull() || value.IsUnknown() {
    return
}
```

Multiple values:

```go
if left.IsUnknown() || right.IsUnknown() {
    return
}
```

Definite presence requires both predicates, in this order:

```go
if !value.IsUnknown() && !value.IsNull() {
    // value is definitely present
}
```

Partially unknown collections must scan every element. Record uncertainty and continue:

```go
unknownCount := 0
for _, element := range elements {
    if element.IsUnknown() {
        unknownCount++
        continue
    }
    // validate known element
}

if knownCount == 0 && unknownCount == 0 {
    // definite minimum-count failure
}
if knownCount > maximum {
    // definite upper-bound failures remain valid despite unknown elements
}
```

A Boolean `hasUnknown` flag may replace the counter. The uncertainty update and `continue` must be the only statements in the unknown-element guard.

## Ordering and Helpers

- Emit errors that are provably independent of an unknown value before its deferral guard.
- Do not mix business conditions with an unknown guard. Guard first, then evaluate known business values.
- Do not put Terraform state predicates in switches or closures. Business-only switches are allowed after the relevant values are known.
- Any helper that receives Terraform values and emits diagnostics must establish its own null/unknown guards. Never rely on a caller's facts.
- If one large validator checks independent attributes, extract self-guarding diagnostic helpers so one unknown attribute does not suppress unrelated definite errors.

`validatorshape` enforces these control-flow forms. `validatordefer` consumes the verified form and detects unsafe presence and collection-count diagnostics.

## Tests

- Store validator unit tests in `_internal_test.go` files.
- Construct generated nested values only with generated `NewXxxValue`, `NewXxxValueMust`, `NewXxxValueNull`, or `NewXxxValueUnknown` constructors.
- Cover the full null/unknown/known decision table, including unknown collection elements before and after known-invalid elements.
- Assert diagnostic count, error/warning severity, exact summary, and attribute path when present. Also preserve known-valid and known-invalid behavior.
- Add a plan-only acceptance regression when the bug requires Terraform evaluation to produce the unknown value.

## Verification

```bash
cd tools/linters && go test ./validatorshape/... ./validatordefer/... -v
make test
make lint
make lint-tools
```

Run the relevant plan-only acceptance test with `make testacc-run TEST=<name>`. Do not run code generation unless the schema itself changed.
