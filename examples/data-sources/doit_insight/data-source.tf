# Look up a specific insight by source and key
data "doit_insight" "example" {
  source_id   = "public-api"
  insight_key = "my-insight-key"
}

# Output insight details
output "insight_title" {
  value = data.doit_insight.example.title
}

output "insight_status" {
  value = data.doit_insight.example.display_status
}

output "insight_categories" {
  value = data.doit_insight.example.categories
}

output "insight_cloud_provider" {
  value = data.doit_insight.example.cloud_provider
}

# Access the aggregate summary
output "insight_potential_savings" {
  value = data.doit_insight.example.summary.potential_daily_savings
}

# ─────────────────────────────────────────────────────────────────────────────
# Use with a managed insight resource
# ─────────────────────────────────────────────────────────────────────────────

resource "doit_insight" "managed" {
  key               = "my-custom-insight"
  title             = "Custom Cost Savings"
  short_description = "Identifies cost optimization opportunities"
  cloud_provider    = "gcp"
  categories        = ["FinOps"]
}

# Read back the insight after creation
data "doit_insight" "managed" {
  source_id   = doit_insight.managed.source_id
  insight_key = doit_insight.managed.key
}
