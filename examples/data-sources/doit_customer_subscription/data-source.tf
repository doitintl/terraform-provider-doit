terraform {
  required_providers {
    doit = {
      source = "doitintl/doit"
    }
  }
}

# Look up the ID of an existing subscription. Replace this with the ID of a
# doit_customer_subscription resource, or with a fixed ID.
data "doit_customer_subscriptions" "all" {}

# Read one subscription by ID.
data "doit_customer_subscription" "example" {
  id = data.doit_customer_subscriptions.all.items[0].id
}

output "subscription_name" {
  value = data.doit_customer_subscription.example.name
}

output "recipient_emails" {
  value = data.doit_customer_subscription.example.recipients.emails
}

output "next_deliveries" {
  value = [
    for e in data.doit_customer_subscription.example.subscriptions : e.schedule.next_at
    if e.schedule != null
  ]
}
