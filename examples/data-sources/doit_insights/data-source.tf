# List all insights (no filters)
data "doit_insights" "all" {
}

# List insights filtered by category and display status
data "doit_insights" "actionable_finops" {
  display_status = ["actionable"]
  category       = ["FinOps"]
}

# List insights filtered by cloud provider
data "doit_insights" "gcp_insights" {
  provider = "gcp"
}

# Output the number of insights found
output "total_insights" {
  value = length(data.doit_insights.all.results)
}

output "actionable_finops_count" {
  value = length(data.doit_insights.actionable_finops.results)
}
