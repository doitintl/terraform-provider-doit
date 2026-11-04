terraform {
  required_providers {
    doit = {
      source  = "doitintl/doit"
      version = "~> 1.0"
    }
  }
}

resource "doit_report" "monthly_costs" {
  name = "Monthly costs"
  config = {
    metrics = [{
      type  = "basic"
      value = "cost"
    }]
    aggregation    = "total"
    time_interval  = "month"
    data_source    = "billing"
    display_values = "actuals_only"
    currency       = "USD"
    layout         = "table"
  }
}

data "doit_commitments" "all" {}

# Email the report every Monday at 08:00 Berlin time, with a PDF and a CSV
# attached, and send the first commitment once a month. All entries share the
# recipients. Both recipient lists are required; use [] for one you do not use.
resource "doit_customer_subscription" "finops" {
  recipients = {
    emails         = ["finops@example.com"]
    slack_channels = []
  }

  subscriptions = [
    {
      type   = "report-subscription"
      source = { id = doit_report.monthly_costs.id }
      schedule = {
        time_zone = "Europe/Berlin"
        # The start day is the calendar date of this instant in time_zone.
        start_date = "2026-11-02T00:00:00+01:00"
        # Only the hour and minute, read in time_zone, are used.
        time      = "2026-11-02T08:00:00+01:00"
        frequency = "Weekly"
      }
      attachments = ["pdf", "csv"]
      pdf_settings = {
        show_filters        = true
        include_table       = true
        include_annotations = false
      }
    },
    {
      type   = "commitment-manager"
      source = { id = data.doit_commitments.all.commitments[0].id }
      schedule = {
        time_zone  = "Europe/Berlin"
        start_date = "2026-11-01T00:00:00+01:00"
        time       = "2026-11-01T09:00:00+01:00"
        frequency  = "Monthly"
      }
    },
  ]
}

# Every two weeks, using a custom frequency.
resource "doit_customer_subscription" "biweekly" {
  recipients = {
    emails         = ["finops@example.com"]
    slack_channels = []
  }

  subscriptions = [{
    type   = "report-subscription"
    source = { id = doit_report.monthly_costs.id }
    schedule = {
      time_zone            = "UTC"
      start_date           = "2026-11-02T00:00:00Z"
      time                 = "2026-11-02T07:00:00Z"
      frequency            = "Custom"
      custom_period_amount = 2
      custom_period_unit   = "Week"
    }
  }]
}
