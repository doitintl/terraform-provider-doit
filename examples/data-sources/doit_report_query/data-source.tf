# Fetch the last 3 months of cost data grouped by cloud provider
data "doit_report_query" "cost_by_provider" {
  config = {
    metrics = [
      {
        type  = "basic"
        value = "cost"
      }
    ]
    aggregation   = "total"
    time_interval = "month"
    currency      = "USD"
    time_range = {
      mode            = "last"
      amount          = 3
      include_current = true
      unit            = "month"
    }
    dimensions = [
      {
        id   = "year"
        type = "datetime"
      },
      {
        id   = "month"
        type = "datetime"
      }
    ]
    group = [
      {
        id   = "cloud_provider"
        type = "fixed"
      }
    ]
  }
}

# Parse the JSON result
locals {
  query_result = jsondecode(data.doit_report_query.cost_by_provider.result_json)
  columns      = [for s in local.query_result.schema : s.name]
}

# Write results to a CSV file
resource "local_file" "query_csv" {
  filename = "cost_by_provider.csv"
  content = join("\n", concat(
    [join(",", local.columns)],
    [for row in local.query_result.rows : join(",", [for cell in row : cell == null ? "" : tostring(cell)])]
  ))
}

output "row_count" {
  value = data.doit_report_query.cost_by_provider.row_count
}
