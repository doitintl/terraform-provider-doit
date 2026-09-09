---
page_title: "Removal of deprecated attributes in 1.8.0"
description: |-
  Migrating configurations off the four deprecated attributes removed in DoiT Provider v1.8.0
---

# Removal of deprecated attributes in 1.8.0

Version 1.8.0 removes four attributes that were deprecated in v1.0.0. The DoiT API
stopped publishing them in its OpenAPI specification
([omni#63754](https://github.com/doiteng/omni/pull/63754)), so the provider no longer
generates them.

| Removed               | Use instead      | Affects                                                              |
| --------------------- | ---------------- | -------------------------------------------------------------------- |
| `config.attributions` | `config.scopes`  | `doit_alert` (resource and data source), `doit_alerts`               |
| `inverse_selection`   | `inverse`        | `doit_allocation` (resource and data source)                         |
| `scope`               | `scopes`         | `doit_budget` (resource and data source), `doit_budgets`             |
| `config.metric`       | `config.metrics` | `doit_report` (resource and data source), `doit_report_query`        |

A configuration that still sets one of them fails with `Unsupported argument`. State
written by an earlier provider version needs no migration: the removed attributes are
dropped from state automatically on the first read.

## `doit_budget`: `scope` → `scopes`

`scope` took a list of allocation rule IDs. `scopes` takes filter objects, so the same
selection becomes a single filter whose `values` list carries every ID — the values of
one filter are OR-ed, which is what `scope` always meant.

**Before:**

```hcl
resource "doit_budget" "example" {
  name  = "Monthly cloud spend"
  scope = [doit_allocation.team_a.id, doit_allocation.team_b.id]
  # ...
}
```

**After:**

```hcl
resource "doit_budget" "example" {
  name = "Monthly cloud spend"
  scopes = [
    {
      type   = "allocation_rule"
      id     = "allocation_rule"
      mode   = "is"
      values = [doit_allocation.team_a.id, doit_allocation.team_b.id]
    }
  ]
  # ...
}
```

`scopes` is now required. It was previously enforced only indirectly, through a
validator that accepted either attribute.

## `doit_alert`: `config.attributions` → `config.scopes`

The same shape applies. `attributions` was a union of attribution IDs, so it becomes one
filter with several values.

**Before:**

```hcl
resource "doit_alert" "example" {
  name = "Spend spike"
  config = {
    attributions = [doit_allocation.team_a.id]
    # ...
  }
}
```

**After:**

```hcl
resource "doit_alert" "example" {
  name = "Spend spike"
  config = {
    scopes = [
      {
        type   = "allocation_rule"
        id     = "allocation_rule"
        mode   = "is"
        values = [doit_allocation.team_a.id]
      }
    ]
    # ...
  }
}
```

~> Alerts apply only the **first** entry of `scopes`. Express a multi-attribution alert
as one filter with several `values`, not as several filters.

## `doit_report`: `config.metric` → `config.metrics`

`metric` was a single object; `metrics` is a list that accepts up to four.

**Before:**

```hcl
config = {
  metric = {
    type  = "basic"
    value = "cost"
  }
}
```

**After:**

```hcl
config = {
  metrics = [{
    type  = "basic"
    value = "cost"
  }]
}
```

Note that `metric_filter.metric`, `limit_by_change.metric` and `group[*].limit.metric`
are unaffected — they are separate attributes and keep their existing shape.

## `doit_allocation`: `inverse_selection` → `inverse`

Drop the attribute and set `inverse` instead. The two were never meant to coexist; the
provider previously rejected setting both to `true`.

**Before:**

```hcl
components = [
  {
    key               = "country"
    type              = "fixed"
    mode              = "is"
    values            = ["JP"]
    inverse_selection = true
  }
]
```

**After:**

```hcl
components = [
  {
    key     = "country"
    type    = "fixed"
    mode    = "is"
    values  = ["JP"]
    inverse = true
  }
]
```

~> Until this release, a component whose state carried `inverse_selection = true` had
its `inverse` value read from state rather than from the API, to keep the deprecated
workflow stable. That masking is gone, so a genuine out-of-band change to `inverse` now
shows up as drift.

## Budgets and alerts created before the migration

Records that still hold the legacy scope server-side keep working. The API reports their
scope through `scopes` as well, so the provider reads them normally, and the first apply
after you migrate the configuration converts them.

For budgets this is fully automatic. For alerts, an apply that sends `scopes` while the
alert still holds legacy attribution references clears those references — but only when
the request does not also carry `attributions`, which the provider no longer sends.
