# Retrieve a report by its ID
data "doit_report" "example" {
  id = "your-report-id"
}

# Output report details
output "report_name" {
  value = data.doit_report.example.name
}

output "report_description" {
  value = data.doit_report.example.description
}

output "report_metrics" {
  value = data.doit_report.example.config.metrics
}

output "report_time_range" {
  value = data.doit_report.example.config.time_range
}

# Output label IDs assigned to this report
output "report_labels" {
  value = data.doit_report.example.labels
}
