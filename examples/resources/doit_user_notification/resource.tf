terraform {
  required_providers {
    doit = {
      source  = "doitintl/doit"
      version = "~> 1.0"
    }
  }
}

# Look up the user the notifications belong to.
data "doit_users" "jane" {
  email = "jane.doe@example.com"
}

# Notify about every cost anomaly of at least warning severity whose excess
# cost is above $100. The email goes to the user's own address.
resource "doit_user_notification" "all_anomalies" {
  user_id = data.doit_users.jane.users[0].id
  name    = "All cost anomalies"

  settings = {
    cost_anomalies = {
      min_severity    = "warning"
      min_excess_cost = { amount = "100.00", currency = "USD" }
      allocations     = { mode = "all", ids = [] }
    }
  }
}

# Only critical anomalies of specific allocations. Allocation IDs must belong
# to custom single allocations.
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

resource "doit_user_notification" "production_critical" {
  user_id = data.doit_users.jane.users[0].id
  name    = "Critical production anomalies"

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
