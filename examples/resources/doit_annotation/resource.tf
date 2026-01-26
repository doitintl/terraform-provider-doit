# Create a simple annotation for a specific date
resource "doit_annotation" "release" {
  content   = "v2.0.0 released - Major infrastructure update"
  timestamp = "2024-06-15T00:00:00Z"
}

# Create a report to associate with annotations
resource "doit_report" "monthly_costs" {
  name = "Monthly Cost Overview"
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

# Create an annotation associated with a report
resource "doit_annotation" "cost_spike" {
  content   = "AWS cost spike due to Black Friday traffic"
  timestamp = "2024-11-29T00:00:00Z"
  reports   = [doit_report.monthly_costs.id]
}

# Create labels for categorization
resource "doit_label" "infrastructure" {
  name  = "infrastructure"
  color = "blue"
}

resource "doit_label" "migration" {
  name  = "migration"
  color = "mint"
}

# Create an annotation with labels for categorization
resource "doit_annotation" "migration" {
  content   = "Started GCP to AWS migration for production workloads"
  timestamp = "2024-03-01T00:00:00Z"
  labels    = [doit_label.infrastructure.id, doit_label.migration.id]
}
