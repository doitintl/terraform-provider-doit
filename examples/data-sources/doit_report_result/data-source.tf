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

# Fetch the results of an existing report
data "doit_report_result" "example" {
  id = "your-report-id"
}

# Parse the JSON results
locals {
  result = jsondecode(data.doit_report_result.example.result_json)
  # Extract column names (schema objects contain name, type, and optional unit, currency, aggregation, id)
  columns = [for s in local.result.schema : s.name]
}

# Write results to a CSV file
resource "local_file" "report_csv" {
  filename = "report.csv"
  content = join("\n", concat(
    [join(",", local.columns)],
    [for row in local.result.rows : join(",", [for cell in row : cell == null ? "" : tostring(cell)])]
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

# Render the existing report result as a PDF and download the signed URL.
data "doit_report_result" "pdf" {
  id          = "your-report-id"
  file_output = "pdf"

  lifecycle {
    postcondition {
      condition     = self.file_output_url != null
      error_message = "The report result was available, but PDF rendering failed. Try applying again."
    }
  }
}

data "http" "report_pdf" {
  url = data.doit_report_result.pdf.file_output_url
}

resource "local_sensitive_file" "report_pdf" {
  filename       = "${path.module}/report.pdf"
  content_base64 = data.http.report_pdf.response_body_base64
}
