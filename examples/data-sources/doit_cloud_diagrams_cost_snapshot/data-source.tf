# Use case: Check last month's spend for a specific project's diagram.

# Step 1: Find the diagram by project name.
data "doit_cloud_diagrams_search" "project" {
  query = "my-gcp-project"
}

locals {
  layer_id = data.doit_cloud_diagrams_search.project.scheme[0].ss_id
}

# Step 2: Get the cost snapshot for the last 30 days.
# start_date and end_date accept ISO dates (YYYY-MM-DD), not timestamps.
data "doit_cloud_diagrams_cost_snapshot" "last_month" {
  id         = local.layer_id
  start_date = formatdate("YYYY-MM-DD", timeadd(plantimestamp(), "-720h"))
  end_date   = formatdate("YYYY-MM-DD", plantimestamp())
}

output "total_spend" {
  value = "${data.doit_cloud_diagrams_cost_snapshot.last_month.total} ${data.doit_cloud_diagrams_cost_snapshot.last_month.currency}"
}

output "trending" {
  value = data.doit_cloud_diagrams_cost_snapshot.last_month.trending_pct
}

# Top resources driving cost.
output "top_resources" {
  value = [
    for r in data.doit_cloud_diagrams_cost_snapshot.last_month.top_resources : {
      name   = r.name
      type   = r.type
      amount = r.amount
    }
  ]
}

# Cost breakdown by service.
output "by_service" {
  value = [
    for s in data.doit_cloud_diagrams_cost_snapshot.last_month.by_service : {
      service = s.service
      amount  = s.amount
    }
  ]
}

# Use case: Compare monthly costs across multiple diagrams.

variable "layer_ids" {
  description = "Layer IDs to compare costs across."
  type        = list(string)
  default     = ["layer-id-1", "layer-id-2"]
}

data "doit_cloud_diagrams_cost_snapshot" "compare" {
  for_each   = toset(var.layer_ids)
  id         = each.value
  start_date = "2026-06-01"
  end_date   = "2026-06-30"
  interval   = "week"
}

output "cost_comparison" {
  value = {
    for id, snapshot in data.doit_cloud_diagrams_cost_snapshot.compare : id => {
      total    = snapshot.total
      currency = snapshot.currency
      trend    = [for t in snapshot.trend : { week = t.bucket_start, amount = t.amount }]
    }
  }
}
