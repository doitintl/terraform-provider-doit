data "doit_budget_suggestions" "all" {}

output "pending_suggestion_count" {
  value = data.doit_budget_suggestions.all.row_count
}

output "suggestion_summary" {
  value = [for s in data.doit_budget_suggestions.all.items : {
    id         = s.id
    name       = s.name
    confidence = s.confidence
    status     = s.status
    amount     = "${s.amount.amount} ${s.amount.currency}"
    scope      = [for c in s.scope_chips : "${c.key}: ${join(", ", c.values)}"]
  }]
}
