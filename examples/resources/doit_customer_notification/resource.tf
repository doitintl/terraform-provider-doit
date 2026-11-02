terraform {
  required_providers {
    doit = {
      source  = "doitintl/doit"
      version = "~> 1.0"
    }
  }
}

# Notify about every cost anomaly of at least warning severity whose excess
# cost is above $100. All three destination lists are required; use [] for a
# type you do not use. Email recipients must be users of your organization.
resource "doit_customer_notification" "all_anomalies" {
  name = "All cost anomalies"

  destinations = {
    emails         = ["finops@example.com"]
    slack_channels = []
    teams_channels = []
  }

  settings = {
    cost_anomalies = {
      min_severity    = "warning"
      min_excess_cost = { amount = "100.00", currency = "USD" }
      allocations     = { mode = "all", ids = [] }
    }
  }
}

# Only critical anomalies of specific allocations, delivered to Slack and
# Microsoft Teams as well as by email. Slack channel IDs and Teams conversation
# IDs are the ones the DoiT console shows for your connected channels.
# Allocation IDs must belong to custom single allocations.
resource "doit_allocation" "production" {
  name        = "Production"
  description = "Production workloads"
  rule = {
    formula = "A"
    components = [
      {
        key    = "project_id"
        mode   = "is"
        type   = "fixed"
        values = ["my-production-project"]
      }
    ]
  }
}

resource "doit_customer_notification" "production_critical" {
  name = "Critical production anomalies"

  destinations = {
    emails         = ["finops@example.com"]
    slack_channels = ["C0123ABCD"]
    teams_channels = ["19:abc@thread.tacv2"]
  }

  settings = {
    cost_anomalies = {
      min_severity    = "critical"
      min_excess_cost = { amount = "1000.00", currency = "USD" }
      allocations = {
        mode = "include"
        ids  = [doit_allocation.production.id]
      }

      # Resend while the anomaly is active: every 12 hours, and whenever its
      # cost grows by 20% or by $500 within 6 hours. Remove the block to turn
      # resending off.
      repeat = {
        every_hours = 12
        when_cost_grows = {
          match            = "or"
          percent_increase = 20
          min_increase     = { amount = "500.00", currency = "USD" }
          within_hours     = 6
        }
      }
    }
  }
}
