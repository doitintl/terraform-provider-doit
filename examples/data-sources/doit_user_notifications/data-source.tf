terraform {
  required_providers {
    doit = {
      source = "doitintl/doit"
    }
  }
}

variable "source_user_email" {
  type        = string
  description = "The user whose cost anomaly notifications are copied."
  default     = "jane.doe@example.com"
}

variable "target_user_emails" {
  type        = list(string)
  description = "The users that receive a copy of every notification of the source user."
  default     = ["john.doe@example.com", "mary.major@example.com"]
}

data "doit_users" "source" {
  email = var.source_user_email
}

data "doit_users" "target" {
  for_each = toset(var.target_user_emails)

  email = each.value
}

# List all personal notifications of the source user (not paginated).
data "doit_user_notifications" "source" {
  user_id = data.doit_users.source.users[0].id
}

locals {
  # One copy per target user and source notification. Notification names are
  # not unique, so the key uses the ID of the source notification.
  copies = {
    for pair in setproduct(var.target_user_emails, data.doit_user_notifications.source.items) :
    "${pair[0]}/${pair[1].id}" => {
      user_id  = data.doit_users.target[pair[0]].users[0].id
      name     = pair[1].name
      settings = pair[1].settings
    }
  }
}

# Copy the notifications to the other users. The email destination is always
# the address of the user the notification belongs to.
resource "doit_user_notification" "copy" {
  for_each = local.copies

  user_id  = each.value.user_id
  name     = each.value.name
  settings = each.value.settings
}

output "source_notification_names" {
  value = [for notification in data.doit_user_notifications.source.items : notification.name]
}
