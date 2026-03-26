# Ask Ava about your cloud spending
data "doit_ava" "cost_summary" {
  question = "What are my top 3 cloud services by cost this month?"
}

output "ava_response" {
  value = data.doit_ava.cost_summary.answer
}
