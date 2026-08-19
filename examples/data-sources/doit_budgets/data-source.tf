# List all budgets
data "doit_budgets" "all" {}

# Filter budgets by name substring
data "doit_budgets" "production" {
  name_contains = "production"
}

# Filter budgets by risk status (atRisk, onTrack, unknown)
data "doit_budgets" "at_risk" {
  filter = "riskStatus:atRisk"
}

# Output total number of budgets
output "total_budgets" {
  value = data.doit_budgets.all.row_count
}

# Output risk aggregations across all budgets
output "risk_summary" {
  value = data.doit_budgets.all.risk_aggregations
}

# Output budget names and risk status
output "budget_risks" {
  value = {
    for b in data.doit_budgets.all.budgets : b.budget_name => b.risk_status
  }
}
