# Assign a report to a label
resource "doit_label" "finance" {
  name  = "finance-reports"
  color = "skyBlue"
}

resource "doit_report" "monthly_costs" {
  name        = "Monthly Cost Overview"
  description = "Monthly overview of all cloud costs"
  # NOTE: Do NOT set the `labels` attribute here if you are using
  # doit_label_assignments to manage labels for this report.
  # Using both with DIFFERENT labels causes perpetual Terraform drift.
  # See the "Conflict with report labels" section below.
  config = {
    metric = {
      type  = "basic"
      value = "cost"
    }
    aggregation    = "total"
    time_interval  = "month"
    data_source    = "billing"
    display_values = "actuals_only"
    currency       = "USD"
    layout         = "table"
  }
}

resource "doit_label_assignments" "finance" {
  label_id = doit_label.finance.id
  assignments = [
    {
      object_id   = doit_report.monthly_costs.id
      object_type = "report"
    },
  ]
}

# Assign an alert to a label
resource "doit_label" "alerts" {
  name  = "critical-alerts"
  color = "rosePink"
}

resource "doit_alert" "cost_spike" {
  name = "Cost Spike Alert"
  config = {
    metric = {
      type  = "basic"
      value = "cost"
    }
    time_interval = "month"
    value         = 1000
    currency      = "USD"
    condition     = "value"
    operator      = "gt"
  }
}

resource "doit_label_assignments" "alerts" {
  label_id = doit_label.alerts.id
  assignments = [
    {
      object_id   = doit_alert.cost_spike.id
      object_type = "alert"
    },
  ]
}

# Assign mixed object types to a single label
resource "doit_label" "team_a" {
  name  = "team-a"
  color = "lavender"
}

resource "doit_label_assignments" "team_a" {
  label_id = doit_label.team_a.id
  assignments = [
    {
      object_id   = doit_report.monthly_costs.id
      object_type = "report"
    },
    {
      object_id   = doit_alert.cost_spike.id
      object_type = "alert"
    },
  ]
}

# Verify the assignments via data source
data "doit_label_assignments" "team_a" {
  id         = doit_label.team_a.id
  depends_on = [doit_label_assignments.team_a]
}

output "team_a_assignments" {
  value = data.doit_label_assignments.team_a.assignments
}

# ─── Conflict with report labels ───────────────────────────────────────
#
# The doit_report resource has its own `labels` attribute that can also
# assign labels. If you use BOTH mechanisms on the same report, the
# following rules apply:
#
#   ✅ OK:  Use doit_label_assignments only (omit `labels` on the report)
#   ✅ OK:  Use the same label in both `labels` and doit_label_assignments
#   ❌ BAD: Use different labels in `labels` vs doit_label_assignments
#           → causes perpetual Terraform drift
#
# The recommended approach is to choose ONE mechanism per report:
# - Use `labels` on the report for simple, direct label assignment
# - Use doit_label_assignments for cross-resource label management
