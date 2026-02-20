# Retrieve all reports
data "doit_reports" "all" {}

# Use doit_current_user to filter reports owned by the current user
data "doit_current_user" "me" {}

data "doit_reports" "my_reports" {
  filter = "owner:[${data.doit_current_user.me.email}]"
}

# Filter reports by type
data "doit_reports" "custom_reports" {
  filter = "type:[custom]"
}

# Paginate through results
data "doit_reports" "paginated" {
  max_results = "10"
}

# Output the list of reports
output "report_names" {
  value = [for r in data.doit_reports.all.reports : r.report_name]
}

output "total_reports" {
  value = data.doit_reports.all.row_count
}

# Output all reports owned by the current user
output "my_report_names" {
  value = [for r in data.doit_reports.my_reports.reports : r.report_name]
}
