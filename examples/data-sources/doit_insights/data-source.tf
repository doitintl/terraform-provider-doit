# List all insights (auto-paginates to fetch everything)
data "doit_insights" "all" {
}

output "total_insights" {
  value = data.doit_insights.all.pagination.row_count
}

# ─────────────────────────────────────────────────────────────────────────────
# Filter by category and cloud provider
# ─────────────────────────────────────────────────────────────────────────────

data "doit_insights" "finops_gcp" {
  category       = "FinOps"
  cloud_provider = "gcp"
}

output "gcp_finops_count" {
  value = data.doit_insights.finops_gcp.pagination.row_count
}

# ─────────────────────────────────────────────────────────────────────────────
# Manual pagination with max_results
# ─────────────────────────────────────────────────────────────────────────────

data "doit_insights" "first_page" {
  max_results = 10
}

output "first_page_count" {
  value = length(data.doit_insights.first_page.results)
}

output "next_page_token" {
  value = data.doit_insights.first_page.pagination.page_token
}

# ─────────────────────────────────────────────────────────────────────────────
# Filter by display status and priority
# ─────────────────────────────────────────────────────────────────────────────

data "doit_insights" "actionable_high" {
  display_status = ["actionable"]
  priority       = ["High"]
}

# ─────────────────────────────────────────────────────────────────────────────
# Search by text
# ─────────────────────────────────────────────────────────────────────────────

data "doit_insights" "search" {
  search_term = "idle"
}
