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

# Look up which objects (reports, budgets, etc.) are assigned to a label
resource "doit_label" "team_reports" {
  name  = "team-reports"
  color = "blue"
}

data "doit_label_assignments" "team_reports" {
  id = doit_label.team_reports.id
}

output "assigned_objects" {
  description = "Objects assigned to the 'team-reports' label"
  value       = data.doit_label_assignments.team_reports.assignments
}

output "assigned_report_ids" {
  description = "IDs of reports assigned to the label"
  value = [
    for a in data.doit_label_assignments.team_reports.assignments :
    a.object_id if a.object_type == "report"
  ]
}

# Export labeled reports to CSV via doit_report_result
# This iterates over all report assignments and renders each result as a CSV file.
data "doit_report_result" "labeled" {
  for_each = toset([
    for a in data.doit_label_assignments.team_reports.assignments :
    a.object_id if a.object_type == "report"
  ])

  id          = each.value
  file_output = "csv"
}

data "http" "report_csvs" {
  for_each = data.doit_report_result.labeled

  url = each.value.file_output_url
}

resource "local_sensitive_file" "report_csvs" {
  for_each = data.http.report_csvs

  filename       = "${path.module}/${each.key}.csv"
  content_base64 = each.value.response_body_base64
}
