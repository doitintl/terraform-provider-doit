# Retrieve all invoices
data "doit_invoices" "all" {}

# Filter by status
data "doit_invoices" "open" {
  filter = "status:OPEN"
}

# Canceled invoices and credit memos are only returned when filtering for them
data "doit_invoices" "canceled" {
  filter = "status:CANCELED"
}

# Warn on every plan while an invoice is past due. A check block reports a
# warning and never blocks an apply. Only one status value is applied per
# request.
check "no_past_due_invoices" {
  data "doit_invoices" "past_due" {
    filter = "status:PAST DUE"
  }

  assert {
    condition     = data.doit_invoices.past_due.row_count == 0
    error_message = "Past due invoices: ${join(", ", [for inv in data.doit_invoices.past_due.invoices : inv.id])}"
  }
}

# Output invoice totals
output "total_invoices" {
  value = data.doit_invoices.all.row_count
}

output "invoice_summary" {
  value = [for inv in data.doit_invoices.all.invoices : {
    id       = inv.id
    amount   = inv.total_amount
    status   = inv.status
    due_date = inv.due_date
  }]
}
