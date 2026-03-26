# Ask Ava about your cloud spending
data "doit_ava" "cost_summary" {
  question = "What are my top 3 cloud services by cost this month?"
}

output "ava_response" {
  value = data.doit_ava.cost_summary.answer
}

# Summarize a specific report by referencing its ID
resource "doit_report" "monthly_costs" {
  name = "Monthly Cost by Service"
  # ... report configuration ...
}

data "doit_ava" "report_summary" {
  question = "Can you summarize report ${doit_report.monthly_costs.id} for me?"
}

output "report_summary" {
  value = data.doit_ava.report_summary.answer
}
