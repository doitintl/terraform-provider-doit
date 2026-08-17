# List all budgets
data "doit_budgets" "all" {}

# Filter budgets by name substring
data "doit_budgets" "production" {
  name_contains = "production"
}

# Output total number of budgets
output "total_budgets" {
  value = data.doit_budgets.all.row_count
}

# Output budget names
output "budget_names" {
  value = [for b in data.doit_budgets.all.budgets : b.budget_name]
}
