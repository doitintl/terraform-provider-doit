---
page_title: "Attribute attributions and legacy attribution dimensions"
description: |-
  How DoiT Attribute attributions differ from the deprecated attribution dimension types, and how to query them with doit_report_query
---

# Attribute attributions and legacy attribution dimensions

The word "attribution" means two different things in the DoiT API, and so in this
provider:

| Value | What it is | Status |
| ----- | ---------- | ------ |
| Dimension types `attribution` and `attribution_group` | Old names of allocation rules and allocations (`allocation_rule` and `allocation`). | Deprecated. Use `allocation_rule` and `allocation`, and manage allocations with `doit_allocation`. |
| Report data source `attribution` | Cost data of a DoiT Attribute attribution: an Attribution Center set that splits cost into teams, products or customers. | Read-only: query with `doit_report_query`; reports that use it cannot be managed with the `doit_report` resource. |
| Dimension types `attribution_tier_1`, `attribution_tier_2`, `attribution_tier_3` | The tier 1, 2 and 3 values of an Attribute attribution. | `doit_report_query` only, together with `data_source = "attribution"`. |

The legacy dimension types and the Attribute values are unrelated: an allocation rule
ID is never a valid tier dimension ID, and an Attribute attribution is not an
allocation. The deprecated `attribution` attribute of `doit_anomalies` is also
unrelated to Attribute: it is the name of the anomaly's primary allocation.

## Querying an Attribute attribution

Attribute attributions can only be read, with the
[`doit_report_query`](../data-sources/report_query) data source. A query must:

- set `data_source = "attribution"`;
- contain at least one `attribution_tier_1`, `attribution_tier_2` or
  `attribution_tier_3` dimension in `group`, `dimensions` or `filters`; and
- use one attribution only: every tier dimension needs the same `id`.

The provider checks these rules at plan time. Querying requires an Attribute
subscription; without one the API rejects the query with `entitlement_not_enabled`.

```terraform
variable "attribution_set_id" {
  type        = string
  description = "ID of the Attribute attribution set to query"
}

data "doit_report_query" "team_costs" {
  config = {
    data_source   = "attribution"
    time_interval = "day"
    time_range = {
      mode            = "last"
      amount          = 30
      unit            = "day"
      include_current = true
    }
    group = [
      {
        # The tier dimension ID is the set ID with "-" replaced by "_"
        id   = replace(var.attribution_set_id, "-", "_")
        type = "attribution_tier_1"
      }
    ]
  }
}

output "team_costs" {
  value = jsondecode(data.doit_report_query.team_costs.result_json).rows
}
```

Cost that no rule of the attribution attributed has a `null` tier value in the rows.

### Finding the tier dimension ID

A tier dimension's `id` is the attribution set ID with every `-` replaced by `_`. The
[`doit_dimensions`](../data-sources/dimensions) data source lists the tier dimensions
of every attribution that has data:

```terraform
data "doit_dimensions" "attribution_tiers" {
  filter = "type:attribution_tier_1"
}
```

Attribution set IDs are also returned by the DoiT API's
[List attribution sets](https://developer.doit.com/reference/listattributeattributionsets)
operation; convert one with `replace(set_id, "-", "_")` as in the example above.

## Where the Attribute values are rejected

The DoiT API cannot use the Attribute values outside an ad-hoc query, so the
provider rejects them at plan time:

| Resource | Rejected values | Why |
| -------- | --------------- | --- |
| `doit_report` | `config.data_source = "attribution"` | The API cannot create or update reports that use the `attribution` data source. |
| `doit_report` | `attribution_tier_*` in `config.dimensions`, `config.filters` and `config.group` | Tier dimensions need the `attribution` data source. |
| `doit_alert` | `attribution_tier_*` in `config.scopes` | Alert scopes do not support tier dimensions. |
| `doit_budget` | `attribution_tier_*` in `scopes` | Budgets cannot use the `attribution` data source that tier dimensions need. |

A saved report that uses the `attribution` data source, for example one created in
the DoiT console, can be read with the [`doit_report`](../data-sources/report) data
source and run with [`doit_report_result`](../data-sources/report_result). Do not
import it into a `doit_report` resource: the API rejects an update that leaves the
report on the `attribution` data source, so the provider fails any plan that would
change such a report.
