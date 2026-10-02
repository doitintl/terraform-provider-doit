terraform {
  required_providers {
    doit = {
      source  = "doitintl/doit"
      version = "~> 1.0"
    }
    http = {
      source  = "hashicorp/http"
      version = "~> 3.4"
    }
    local = {
      source  = "hashicorp/local"
      version = "~> 2.5"
    }
  }
}

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
  # Extract column names (schema objects contain name, type, and optional unit, currency, aggregation, id)
  columns = [for s in local.query_result.schema : s.name]
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

# ─────────────────────────────────────────────────────────────────────────────
# Render an ad-hoc query as a PNG chart
# ─────────────────────────────────────────────────────────────────────────────
# Set file_output to "png" (or "pdf") to receive a signed download URL for the
# rendered chart, without saving a report first. The URL is sensitive and
# valid for up to seven days.

data "doit_report_query" "top_services_png" {
  file_output = "png"
  config = {
    metrics       = [{ type = "basic", value = "cost" }]
    aggregation   = "total"
    time_interval = "month"
    currency      = "USD"
    time_range = {
      mode            = "last"
      amount          = 3
      unit            = "month"
      include_current = false
    }
    dimensions = [
      { id = "year", type = "datetime" },
      { id = "month", type = "datetime" },
    ]
    # Top 5 services by cost
    group = [{
      id   = "service_description"
      type = "fixed"
      limit = {
        value  = 5
        sort   = "desc"
        metric = { type = "basic", value = "cost" }
      }
    }]
    layout          = "stacked_column_chart"
    sort_dimensions = "a_to_z" # chronological x-axis in the rendered chart
  }
}

data "http" "top_services_png" {
  url = data.doit_report_query.top_services_png.file_output_url
}

resource "local_sensitive_file" "top_services_png" {
  filename       = "${path.module}/top-services.png"
  content_base64 = data.http.top_services_png.response_body_base64
}
