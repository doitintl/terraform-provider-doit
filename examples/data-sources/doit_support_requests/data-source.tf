# List all support requests
data "doit_support_requests" "all" {
}

# Filter support requests by severity
data "doit_support_requests" "high_severity" {
  filter = "severity:high"
}

output "ticket_count" {
  value = data.doit_support_requests.all.row_count
}
