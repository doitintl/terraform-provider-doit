terraform {
  required_providers {
    doit = {
      source = "doitintl/doit"
    }
  }
}

variable "source_user_email" {
  type    = string
  default = "jane.doe@example.com"
}

variable "target_user_email" {
  type    = string
  default = "john.doe@example.com"
}

data "doit_users" "source" {
  email = var.source_user_email
}

data "doit_users" "target" {
  email = var.target_user_email
}

# A notification of the source user. Create it first, or use the ID of an
# existing one (see the doit_user_notifications data source to list them).
resource "doit_user_notification" "source" {
  user_id = data.doit_users.source.users[0].id
  name    = "Critical anomalies"

  settings = {
    cost_anomalies = {
      min_severity    = "critical"
      min_excess_cost = { amount = "500.00", currency = "USD" }
      allocations     = { mode = "all", ids = [] }
    }
  }
}

# Read one notification of a user by its ID.
data "doit_user_notification" "example" {
  user_id = doit_user_notification.source.user_id
  id      = doit_user_notification.source.id
}

# Give another user the same notification settings.
resource "doit_user_notification" "copy" {
  user_id  = data.doit_users.target.users[0].id
  name     = data.doit_user_notification.example.name
  settings = data.doit_user_notification.example.settings
}

output "notification_destinations" {
  value = data.doit_user_notification.example.destinations
}

output "copied_notification_id" {
  value = doit_user_notification.copy.id
}
