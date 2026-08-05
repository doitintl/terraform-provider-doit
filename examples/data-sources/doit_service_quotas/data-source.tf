# List all service quotas (auto-paginates to fetch everything)
data "doit_service_quotas" "all" {}

output "total_service_quotas" {
  value = data.doit_service_quotas.all.row_count
}

# ─────────────────────────────────────────────────────────────────────────────
# Filter by cloud provider and minimum utilization
# ─────────────────────────────────────────────────────────────────────────────

data "doit_service_quotas" "aws_near_limit" {
  cloud_provider          = "aws"
  min_utilization_percent = 80
}

output "aws_quotas_near_limit" {
  value = [for q in data.doit_service_quotas.aws_near_limit.items : "${q.service}/${q.quota}"]
}

# ─────────────────────────────────────────────────────────────────────────────
# Manual pagination with max_results
# ─────────────────────────────────────────────────────────────────────────────

data "doit_service_quotas" "first_page" {
  max_results = 10
}

output "first_page_count" {
  value = length(data.doit_service_quotas.first_page.items)
}

output "next_page_token" {
  value = data.doit_service_quotas.first_page.page_token
}

# ─────────────────────────────────────────────────────────────────────────────
# Inspect exceeded quotas
# ─────────────────────────────────────────────────────────────────────────────

data "doit_service_quotas" "exceeded" {
}

output "exceeded_quotas" {
  value = [
    for q in data.doit_service_quotas.exceeded.items : {
      provider = q.cloud_provider
      service  = q.service
      quota    = q.quota
      region   = q.region
      usage    = q.usage
      limit    = q.limit
    }
    if q.status == "exceeded"
  ]
}
