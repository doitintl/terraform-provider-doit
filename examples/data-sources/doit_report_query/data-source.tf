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
}

# Output the rows
output "cost_rows" {
  value = local.query_result.rows
}

output "row_count" {
  value = data.doit_report_query.cost_by_provider.row_count
}
