# Retrieve a single insight by its source ID and insight key
data "doit_insight" "example" {
  source_id   = "aws-trusted-advisor"
  insight_key = "underutilized-ec2-instances"
}

# Use insight attributes in other resources
output "insight_title" {
  value = data.doit_insight.example.title
}

output "insight_status" {
  value = data.doit_insight.example.display_status
}

output "insight_savings" {
  value = data.doit_insight.example.summary.potential_daily_savings
}
