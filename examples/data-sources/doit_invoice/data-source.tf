# Retrieve an invoice by its invoice number
data "doit_invoice" "example" {
  id = "your-invoice-number"
}

# Output invoice details
output "invoice_total" {
  value = data.doit_invoice.example.total_amount
}

output "invoice_balance" {
  value = data.doit_invoice.example.balance_amount
}

output "invoice_currency" {
  value = data.doit_invoice.example.currency
}

output "invoice_status" {
  value = data.doit_invoice.example.status
}

output "invoice_url" {
  value = data.doit_invoice.example.url
}
