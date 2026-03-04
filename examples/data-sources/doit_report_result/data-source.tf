# Fetch the results of an existing report
data "doit_report_result" "example" {
  id = "your-report-id"
}

# Parse the JSON results
locals {
  result  = jsondecode(data.doit_report_result.example.result_json)
  columns = [for s in local.result.schema : s.name]
}

# Write results to a CSV file
resource "local_file" "report_csv" {
  filename = "report.csv"
  content = join("\n", concat(
    [join(",", local.columns)],
    [for row in local.result.rows : join(",", row)]
  ))
}

# Fetch results with a custom date range
data "doit_report_result" "custom_range" {
  id         = "your-report-id"
  start_date = "2026-01-01"
  end_date   = "2026-01-31"
}

# Fetch results with an ISO 8601 duration
data "doit_report_result" "last_week" {
  id         = "your-report-id"
  time_range = "P7D"
}
