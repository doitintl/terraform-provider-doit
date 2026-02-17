# Create a report with multiple metrics
resource "doit_report" "my_report" {
  name        = "Monthly Cost Report"
  description = "Tracks monthly costs and usage across cloud providers"
  config = {
    # Use metrics list (supports 1-4 metrics)
    metrics = [
      {
        type  = "basic"
        value = "cost"
      },
      {
        type  = "basic"
        value = "usage"
      }
    ]
    include_promotional_credits = false
    advanced_analysis = {
      trending_up   = false
      trending_down = false
      not_trending  = false
      forecast      = false
    }
    aggregation   = "total"
    time_interval = "month"
    data_source   = "billing"
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
    time_range = {
      mode            = "last"
      amount          = 12
      include_current = true
      unit            = "month"
    }
    # Compare against the same period last year
    secondary_time_range = {
      amount          = 1
      unit            = "year"
      include_current = false
    }
    filters = [
      {
        inverse = false
        id      = "cloud_provider"
        type    = "fixed"
        mode    = "is"
        values  = ["amazon-web-services", "google-cloud"]
      }
    ]
    group = [
      {
        id   = "cloud_provider"
        type = "fixed"
      }
    ]
    layout         = "table"
    display_values = "actuals_only"
    currency       = "USD"
  }
}
