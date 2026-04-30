# Look up sharing permissions for an existing report
data "doit_sharing" "report" {
  resource_type = "reports"
  resource_id   = "your-report-id"
}

# Use the permissions data in other resources or outputs
output "report_permissions" {
  value = data.doit_sharing.report.permissions
}

output "report_public_access" {
  value = data.doit_sharing.report.public
}
