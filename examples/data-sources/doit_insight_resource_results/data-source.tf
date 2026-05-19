# Retrieve all resource results for an insight (auto-paginates)
data "doit_insight_resource_results" "all" {
  source_id   = "public-api"
  insight_key = "my-insight-key"
}

output "total_resources" {
  value = data.doit_insight_resource_results.all.row_count
}

output "resource_ids" {
  value = [for r in data.doit_insight_resource_results.all.resource_results : r.resource_id]
}

# ─────────────────────────────────────────────────────────────────────────────
# Manual pagination with max_results
# ─────────────────────────────────────────────────────────────────────────────

data "doit_insight_resource_results" "first_page" {
  source_id   = "public-api"
  insight_key = "my-insight-key"
  max_results = 10
}

output "first_page_resources" {
  value = length(data.doit_insight_resource_results.first_page.resource_results)
}

# ─────────────────────────────────────────────────────────────────────────────
# Use with a managed insight and resource results
# ─────────────────────────────────────────────────────────────────────────────

resource "doit_insight" "managed" {
  key               = "custom-savings-insight"
  title             = "Custom Savings Insight"
  short_description = "Identifies cost optimization opportunities"
  cloud_provider    = "gcp"
  categories        = ["FinOps"]
}

resource "doit_insight_resource_results" "managed" {
  source_id   = doit_insight.managed.source_id
  insight_key = doit_insight.managed.key
  resource_results = [
    {
      resource_id    = "my-vm-1"
      account        = "my-project-123"
      cloud_provider = "gcp"
      result_type    = "potential_daily_savings"
      result = {
        value = 5.50
      }
    },
  ]
}

# Read back the resource results
data "doit_insight_resource_results" "managed" {
  source_id   = doit_insight.managed.source_id
  insight_key = doit_insight.managed.key
  depends_on  = [doit_insight_resource_results.managed]
}

output "managed_savings" {
  value = [for r in data.doit_insight_resource_results.managed.resource_results : r.result.value]
}
