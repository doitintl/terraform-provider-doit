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

# Filter by severity
data "doit_anomalies" "critical" {
  filter = "severityLevel:critical"
}

# Include notification events for each anomaly
data "doit_anomalies" "with_notifications" {
  include_notifications = true
}

output "notification_audit" {
  value = [for a in data.doit_anomalies.with_notifications.anomalies : {
    id           = a.id
    service      = a.service_name
    severity     = a.severity_level
    notifications = [for n in a.notifications : {
      channel   = n.channel
      timestamp = n.timestamp
    }]
  }]
}

# Output anomaly details
output "total_anomalies" {
  value = data.doit_anomalies.all.row_count
}

output "anomaly_summary" {
  value = [for a in data.doit_anomalies.all.anomalies : {
    id          = a.id
    service     = a.service_name
    cost_impact = a.cost_of_anomaly
    severity    = a.severity_level
    status      = a.status
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
    metrics = [{ type = "basic", value = "cost" }]
    currency      = "USD"
    time_interval = "day"
    time_range = {
      mode = "custom"
      unit = "day"
      custom_time_range = {
        # start_time / end_time are epoch milliseconds — divide by 1000 for seconds
        from = formatdate("YYYY-MM-DD'T'hh:mm:ss'Z'", timeadd("1970-01-01T00:00:00Z", "${floor(local.unacknowledged[count.index].start_time / 1000)}s"))
        to   = formatdate("YYYY-MM-DD'T'hh:mm:ss'Z'", timeadd("1970-01-01T00:00:00Z", "${floor((local.unacknowledged[count.index].end_time != null ? local.unacknowledged[count.index].end_time : local.unacknowledged[count.index].start_time + 86400000) / 1000)}s"))
      }
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

- `filter` (String) An expression for filtering the results of the request
- `include_notifications` (Boolean) Include anomaly notifications from the subcollection. Defaults to false.
- `max_creation_time` (String) Max value for the anomaly detection time
- `max_results` (Number) The maximum number of results to return in a single page
- `min_creation_time` (String) Min value for the anomaly detection time
- `page_token` (String) Page token, returned by a previous call, to request the next page of results
- `timeouts` (Attributes) (see [below for nested schema](#nestedatt--timeouts))

### Read-Only

- `anomalies` (Attributes List) (see [below for nested schema](#nestedatt--anomalies))
- `row_count` (Number)

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
- `attribution` (String) Attribution ID.
- `billing_account` (String) Billing account ID.
- `cost_of_anomaly` (Number) Excess cost over and above the expected normal cost.
- `end_time` (Number) End of the anomaly.
- `id` (String)
- `notifications` (Attributes List) Chronologically ordered notification dispatch events. (see [below for nested schema](#nestedatt--anomalies--notifications))
- `platform` (String) Cloud Provider name.
- `resource_data` (Attributes List) Array of resources contributing to an anomaly. (see [below for nested schema](#nestedatt--anomalies--resource_data))
- `scope` (String) Scope: Project or Account
- `service_name` (String) Service name.
- `severity_level` (String) Severity level: Information, Warning or Critical
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
