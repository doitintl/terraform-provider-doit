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
# This iterates over all report assignments and fetches their results.
data "doit_report_result" "labeled" {
  for_each = toset([
    for a in data.doit_label_assignments.team_reports.assignments :
    a.object_id if a.object_type == "report"
  ])

  id = each.value
}

resource "local_file" "report_csvs" {
  for_each = data.doit_report_result.labeled

  filename = "${each.key}.csv"
  content = join("\n", concat(
    [join(",", [for s in jsondecode(each.value.result_json).schema : s.name])],
    [for row in jsondecode(each.value.result_json).rows :
      join(",", [for cell in row : cell == null ? "" : tostring(cell)])
    ]
  ))
}
