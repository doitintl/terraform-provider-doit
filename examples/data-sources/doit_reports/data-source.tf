# Retrieve all reports
data "doit_reports" "all" {}

# Filter reports by owner
data "doit_reports" "my_reports" {
  filter = "owner:[user@example.com]"
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
