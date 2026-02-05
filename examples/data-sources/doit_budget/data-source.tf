# Retrieve a budget by its ID
data "doit_budget" "example" {
  id = doit_budget.my_budget.id
}

# Output budget details
output "budget_name" {
  value = data.doit_budget.example.name
}

output "budget_amount" {
  value = data.doit_budget.example.amount
}

output "budget_currency" {
  value = data.doit_budget.example.currency
}

output "budget_time_period" {
  value = data.doit_budget.example.type
}
