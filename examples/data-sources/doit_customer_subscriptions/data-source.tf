terraform {
  required_providers {
    doit = {
      source = "doitintl/doit"
    }
  }
}

# List every subscription visible to the caller. All pages are fetched unless
# max_results is set. Alerts and budgets are listed too; their entries have
# type "alert" or "budget" and no schedule.
data "doit_customer_subscriptions" "all" {}

locals {
  # Subscriptions that deliver a report.
  report_subscriptions = [
    for s in data.doit_customer_subscriptions.all.items : s
    if contains([for e in s.subscriptions : e.type], "report-subscription")
  ]
}

output "subscription_names" {
  value = [for s in data.doit_customer_subscriptions.all.items : s.name]
}

output "report_subscription_ids" {
  value = [for s in local.report_subscriptions : s.id]
}

# Read a single page of up to 10 subscriptions; page_token holds the token of
# the next page.
data "doit_customer_subscriptions" "first_page" {
  max_results = 10
}

output "next_page_token" {
  value = data.doit_customer_subscriptions.first_page.page_token
}
