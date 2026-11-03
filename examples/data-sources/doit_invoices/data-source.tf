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
