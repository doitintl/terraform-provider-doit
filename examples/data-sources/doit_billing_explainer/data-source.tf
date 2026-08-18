# Retrieve the month-over-month billing explainer for every payer in the tenant
data "doit_billing_explainer" "example" {
  invoice_month = "2025-12"
}

# Output tenant-level totals
output "billing_explainer_doit_credits" {
  value = data.doit_billing_explainer.example.doit_credits
}

output "billing_explainer_invoice_adjustments" {
  value = data.doit_billing_explainer.example.invoice_adjustments
}

# Output the invoiced cost total for one payer, split by comparison basis
output "billing_explainer_payer_totals" {
  value = {
    for payer_id, payer in data.doit_billing_explainer.example.payers :
    payer_id => {
      friendly_name = payer.friendly_name
      aws_total     = payer.summary.aws.total
      doit_total    = payer.summary.doit.total
      no_doit_total = payer.summary.aws_without_doit.total
    }
  }
}
