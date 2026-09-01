---
page_title: "doit_anomalies Data Source - terraform-provider-doit"
subcategory: ""
description: |-
  Monitor cost spikes in your cloud environment.
---

# doit_anomalies (Data Source)

Monitor cost spikes in your cloud environment.

## Example Usage

```terraform
# Retrieve all anomalies
data "doit_anomalies" "all" {}

# Filter by severity and sort by cost descending
data "doit_anomalies" "critical_by_cost" {
  filter     = "severityLevel:critical"
  sort_by    = "costOfAnomaly"
  sort_order = "desc"
}

# Filter by usage start time window (epoch milliseconds)
data "doit_anomalies" "recent" {
  min_creation_time = 1704067200000 # 2024-01-01T00:00:00Z
  max_creation_time = 1735689600000 # 2025-01-01T00:00:00Z
}

# Include notification events for each anomaly
data "doit_anomalies" "with_notifications" {
  include_notifications = true
}

output "notification_audit" {
  value = [for a in data.doit_anomalies.with_notifications.anomalies : {
    id       = a.id
    service  = a.service_name
    severity = a.severity_level
    notifications = [for n in a.notifications : {
      channel   = n.channel
      timestamp = n.timestamp
    }]
  }]
}

# Output anomaly counts and aggregate summary
output "total_anomalies" {
  value = data.doit_anomalies.all.row_count
}

output "total_count" {
  value = data.doit_anomalies.all.total_count
}

output "anomaly_summary_metrics" {
  value = {
    total_cost        = data.doit_anomalies.all.anomaly_summary.total_cost_of_anomaly
    critical_count    = data.doit_anomalies.all.anomaly_summary.count_by_severity.critical
    warning_count     = data.doit_anomalies.all.anomaly_summary.count_by_severity.warning
    information_count = data.doit_anomalies.all.anomaly_summary.count_by_severity.information
  }
}

output "anomaly_summary" {
  value = [for a in data.doit_anomalies.all.anomalies : {
    id                  = a.id
    service             = a.service_name
    cost_impact         = a.cost_of_anomaly
    severity            = a.severity_level
    monitor_level       = a.monitor_level
    status              = a.status
    deactivation_reason = a.deactivation_reason
  }]
}

# ─────────────────────────────────────────────────────────────────────────────
# Combining data sources: enrich anomalies with product and platform info
# ─────────────────────────────────────────────────────────────────────────────

# Use doit_platforms to discover available platforms
data "doit_platforms" "all" {}

# Group anomalies by platform
output "anomalies_by_platform" {
  description = "Cost anomalies grouped by cloud platform"
  value = {
    for platform, anomalies in { for a in data.doit_anomalies.all.anomalies : a.platform => a... } : platform => [
      for a in anomalies : {
        id          = a.id
        service     = a.service_name
        cost_impact = a.cost_of_anomaly
        severity    = a.severity_level
      }
    ] if contains([for p in data.doit_platforms.all.platforms : p.display_name], platform)
  }
}

# Use doit_products to cross-reference anomaly services with known products
data "doit_products" "all" {}

locals {
  all_product_names = [for p in data.doit_products.all.products : p.display_name]
}

output "anomalies_with_product_match" {
  description = "Anomalies enriched with whether their service matches a known product"
  value = [for a in data.doit_anomalies.all.anomalies : {
    id            = a.id
    service       = a.service_name
    platform      = a.platform
    cost_impact   = a.cost_of_anomaly
    severity      = a.severity_level
    known_product = contains(local.all_product_names, a.service_name)
  }]
}

# ─────────────────────────────────────────────────────────────────────────────
# Track unacknowledged anomalies and export to CSV
# ─────────────────────────────────────────────────────────────────────────────

# Filter anomalies that haven't been reviewed yet
locals {
  unacknowledged = [
    for a in data.doit_anomalies.all.anomalies : a
    if a.acknowledged != true
  ]
}

output "unacknowledged_count" {
  description = "Number of anomalies still awaiting review"
  value       = length(local.unacknowledged)
}

output "unacknowledged_summary" {
  description = "Quick overview of unreviewed anomalies"
  value = [for a in local.unacknowledged : {
    id          = a.id
    service     = a.service_name
    platform    = a.platform
    cost_impact = a.cost_of_anomaly
    severity    = a.severity_level
  }]
}

# Write unacknowledged anomalies to a CSV for incident tracking
resource "local_file" "unacknowledged_csv" {
  filename = "unacknowledged_anomalies.csv"
  content = join("\n", concat(
    ["id,platform,service,severity,cost_impact,status"],
    [for a in local.unacknowledged : join(",", [
      a.id,
      a.platform,
      a.service_name,
      a.severity_level,
      tostring(a.cost_of_anomaly),
      a.status != null ? a.status : "",
    ])]
  ))
}

# ─────────────────────────────────────────────────────────────────────────────
# Cross-reference anomalies with Cloud Analytics cost data
# ─────────────────────────────────────────────────────────────────────────────

# For each unacknowledged anomaly, query the cost breakdown for the affected
# billing account and service during the anomaly time window. This lets you see
# exactly which resources drove the unexpected spend.

data "doit_report_query" "anomaly_cost_detail" {
  count = length(local.unacknowledged)

  config = {
    metrics       = [{ type = "basic", value = "cost" }]
    currency      = "USD"
    time_interval = "day"
    custom_time_range = {
      # start_time / end_time are epoch milliseconds — divide by 1000 for seconds
      from = formatdate("YYYY-MM-DD'T'hh:mm:ss'Z'", timeadd("1970-01-01T00:00:00Z", "${floor(local.unacknowledged[count.index].start_time / 1000)}s"))
      to   = formatdate("YYYY-MM-DD'T'hh:mm:ss'Z'", timeadd("1970-01-01T00:00:00Z", "${floor((local.unacknowledged[count.index].end_time != null ? local.unacknowledged[count.index].end_time : local.unacknowledged[count.index].start_time + 86400000) / 1000)}s"))
    }
    time_range = {
      mode = "custom"
    }
    group = [
      { id = "sku_description", type = "fixed" }
    ]
    filters = [
      {
        id     = "billing_account_id"
        type   = "fixed"
        mode   = "is"
        values = [local.unacknowledged[count.index].billing_account]
      },
      {
        id     = "service_description"
        type   = "fixed"
        mode   = "is"
        values = [local.unacknowledged[count.index].service_name]
      },
    ]
  }
}

# Parse query results and export a combined CSV with per-SKU cost breakdown
locals {
  cost_details = [
    for idx, q in data.doit_report_query.anomaly_cost_detail : {
      anomaly_id = local.unacknowledged[idx].id
      service    = local.unacknowledged[idx].service_name
      platform   = local.unacknowledged[idx].platform
      rows       = try(jsondecode(q.result_json).rows, [])
      schema     = try(jsondecode(q.result_json).schema, [])
    }
  ]
}

resource "local_file" "anomaly_cost_breakdown_csv" {
  filename = "anomaly_cost_breakdown.csv"
  content = join("\n", concat(
    ["anomaly_id,platform,service,sku,cost_usd"],
    flatten([
      for detail in local.cost_details : [
        for row in detail.rows : join(",", [
          detail.anomaly_id,
          detail.platform,
          detail.service,
          row[0] != null ? row[0] : "",
          row[1] != null ? tostring(row[1]) : "0",
        ])
      ]
    ])
  ))
}

# ─────────────────────────────────────────────────────────────────────────────
# Acknowledgment audit trail
# ─────────────────────────────────────────────────────────────────────────────

# For compliance, generate a summary of who acknowledged which anomalies
locals {
  acknowledged = [
    for a in data.doit_anomalies.all.anomalies : a
    if a.acknowledged == true
  ]
}

resource "local_file" "acknowledgment_audit_csv" {
  filename = "anomaly_acknowledgment_audit.csv"
  content = join("\n", concat(
    ["anomaly_id,service,platform,cost_impact,acknowledged_by,acknowledged_at"],
    [for a in local.acknowledged : join(",", [
      a.id,
      a.service_name,
      a.platform,
      tostring(a.cost_of_anomaly),
      a.acknowledged_by != null ? a.acknowledged_by : "",
      a.acknowledged_at != null ? a.acknowledged_at : "",
    ])]
  ))
}

output "acknowledgment_audit" {
  description = "Who acknowledged which anomalies and when"
  value = [for a in local.acknowledged : {
    anomaly_id      = a.id
    service         = a.service_name
    cost_impact     = a.cost_of_anomaly
    acknowledged_by = a.acknowledged_by
    acknowledged_at = a.acknowledged_at
  }]
}
```

-> For details on auto and manual pagination, see the [Pagination Guide](../guides/pagination).

<!-- schema generated by tfplugindocs -->
## Schema

### Optional

- `filter` (String) An expression for filtering the results. The syntax is `key:value`. Multiple criteria can be combined using a pipe |. See [Filters](https://developer.doit.com/docs/filters).
Available filter keys: **serviceName**, **billingAccount**, **platform**, **severityLevel**. `severityLevel` values must be lowercase: `information`, `warning`, `critical`. An unrecognised key, or a segment that is not `key:value`, is rejected with `400` rather than ignored.
- `include_notifications` (Boolean) Include anomaly notifications from the subcollection. Defaults to false.
- `max_creation_time` (Number) Inclusive upper bound on the anomaly's usage start time, in milliseconds since the POSIX epoch. Despite the name, this filters the anomaly's usage start time, not the time the anomaly document was created.
- `max_results` (Number) The maximum number of results to return in a single page. If omitted, all anomalies matching the filters and time window are returned in a single page.
- `min_creation_time` (Number) Inclusive lower bound on the anomaly's usage start time, in milliseconds since the POSIX epoch. Despite the name, this filters the anomaly's usage start time, not the time the anomaly document was created.
- `page_token` (String) Page token, returned by a previous call, to request the next page of results
- `sort_by` (String) A field by which the results will be sorted. Defaults to `startTime`.
Possible values: `startTime`, `severityLevel`, `costOfAnomaly`
- `sort_order` (String) Specifies the sort direction; accepts asc for ascending (lowest to highest) or desc for descending (highest to lowest).
Possible values: `asc`, `desc`
- `timeouts` (Attributes) (see [below for nested schema](#nestedatt--timeouts))

### Read-Only

- `anomalies` (Attributes List) Anomalies in this page. Always an array; empty (`[]`) when there are no matching anomalies. (see [below for nested schema](#nestedatt--anomalies))
- `anomaly_summary` (Attributes) Anomaly-specific summary. `countBySeverity` and `totalCostOfAnomaly` cover the complete filtered result set across all pages, not the returned page. Anomalies whose severity cannot be determined (legacy documents) are counted in `totalCount` and `totalCostOfAnomaly` but in none of the three severity buckets in `countBySeverity`, so the three values can sum to less than `totalCount`. Computed as of this request; under concurrent writes, values may differ between pages fetched during the same pagination sequence. (see [below for nested schema](#nestedatt--anomaly_summary))
- `row_count` (Number) Number of items in this page (`anomalies.length`). This is not the total count across all pages; the total across the full filtered result set is `totalCount`.
- `total_count` (Number) Total number of anomalies matching the filters and time window, across all pages. This is a per-request snapshot: under concurrent writes, the value can differ between pages fetched during the same pagination sequence.
- `total_count_exact` (Boolean) Whether `totalCount` is exact. Always `true` for this operation, because `totalCount` is computed over the fully materialised filtered result set.
- `truncated` (Boolean) `true` when filtered anomalies remain beyond this page. Derived from the same "rows remain after the last returned row" check as `pageToken`, so the two are never contradictory: `pageToken` is present if and only if `truncated` is `true`.

<a id="nestedatt--timeouts"></a>
### Nested Schema for `timeouts`

Optional:

- `read` (String) A string that can be [parsed as a duration](https://pkg.go.dev/time#ParseDuration) consisting of numbers and unit suffixes, such as "30s" or "2h45m". Valid time units are "s" (seconds), "m" (minutes), "h" (hours).


<a id="nestedatt--anomalies"></a>
### Nested Schema for `anomalies`

Read-Only:

- `acknowledged` (Boolean) Has the anomaly been acknowledged
- `acknowledged_at` (String) When the anomaly was first acknowledged
- `acknowledged_by` (String) Email of the user who first acknowledged the anomaly
- `actual_cost` (Number) Observed (actual) cost of the anomaly.
- `attribution` (String) Attribution ID.
- `billing_account` (String) Billing account ID.
- `cost_of_anomaly` (Number) Excess cost over and above the expected normal cost.
- `deactivation_reason` (String) Why the anomaly stopped being active. `reverted` means the cost returned inside the expected normal range; `expired` means the anomaly was deactivated without the cost returning inside that range; `unknown` means the reason could not be determined. Null while the anomaly is still active.
- `end_time` (Number) End of the anomaly.
- `expected_max_cost` (Number) Maximum cost within the expected normal range.
- `id` (String)
- `monitor_level` (String) Whether the anomaly was detected on a single SKU (`sku`) or at the level of a whole service (`service`).
- `notifications` (Attributes List) Chronologically ordered notification dispatch events. (see [below for nested schema](#nestedatt--anomalies--notifications))
- `platform` (String) Cloud Provider name.
- `resource_data` (Attributes List) Array of resources contributing to an anomaly. (see [below for nested schema](#nestedatt--anomalies--resource_data))
- `scope` (String) Scope: Project or Account
- `service_name` (String) Service name.
- `severity_level` (String) Severity level: `information`, `warning`, or `critical`.
- `start_time` (Number) Usage start time of the anomaly.
- `status` (String)
- `time_frame` (String) Timeframe: Daily or Hourly
- `top3skus` (Attributes List) Array of SKU entries contributing to an anomaly. (see [below for nested schema](#nestedatt--anomalies--top3skus))

<a id="nestedatt--anomalies--notifications"></a>
### Nested Schema for `anomalies.notifications`

Read-Only:

- `channel` (String) Dispatch channel.
- `timestamp` (String) Dispatch timestamp in RFC3339 UTC.


<a id="nestedatt--anomalies--resource_data"></a>
### Nested Schema for `anomalies.resource_data`

Read-Only:

- `cost` (Number)
- `labels` (Attributes List) Labels (also known as cost-allocation tags) present on this resource during the anomaly; each entry reports the label's key, its value, and the resource's cost tagged with that key/value pair.
Cloud providers use different names for the same concept; GCP uses "labels", AWS uses "cost-allocation tags", and Azure uses "tags". We refer to all of these as labels. (see [below for nested schema](#nestedatt--anomalies--resource_data--labels))
- `operation` (String) For anomalies related to AWS S3
- `resource_id` (String)
- `sku_description` (String)

<a id="nestedatt--anomalies--resource_data--labels"></a>
### Nested Schema for `anomalies.resource_data.labels`

Read-Only:

- `cost` (Number) The resource's cost tagged with this key/value pair; typically equal to the resource's cost, since labels/tags usually cover all of its spend.
- `key` (String) The label/tag key.
- `value` (String) The label/tag value.



<a id="nestedatt--anomalies--top3skus"></a>
### Nested Schema for `anomalies.top3skus`

Read-Only:

- `cost` (Number)
- `name` (String)



<a id="nestedatt--anomaly_summary"></a>
### Nested Schema for `anomaly_summary`

Read-Only:

- `count_by_severity` (Attributes) Count of matching anomalies per severity level. All three keys are always present, with zero counts included. (see [below for nested schema](#nestedatt--anomaly_summary--count_by_severity))
- `total_cost_of_anomaly` (Number) Sum of `costOfAnomaly` across all matching anomalies, in USD, rounded to cents.

<a id="nestedatt--anomaly_summary--count_by_severity"></a>
### Nested Schema for `anomaly_summary.count_by_severity`

Read-Only:

- `critical` (Number)
- `information` (Number)
- `warning` (Number)
