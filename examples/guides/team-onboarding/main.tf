terraform {
  required_providers {
    doit = {
      source  = "doitintl/doit"
      version = "~> 1.13"
    }
  }
}

provider "doit" {}

# Onboard a team by adding one entry to this map. Every team gets its own
# allocation, a share-of-spend metric and report, a weekly report delivery and
# a notification for critical cost anomalies.
locals {
  teams = {
    payments = {
      label         = "payments"        # value of the "owner" label on the team's costs
      slack_channel = "payments-finops" # name of a Slack channel connected to DoiT
      lead          = "ana@example.com" # a user of your DoiT organization
    }
    search = {
      label         = "search"
      slack_channel = "search-finops"
      lead          = "ravi@example.com"
    }
  }
}

# ─────────────────────────────────────────────────────────────────────────────
# Look up what the teams refer to by name
# ─────────────────────────────────────────────────────────────────────────────

data "doit_alert_slack_channels" "all" {}

data "doit_users" "all" {}

locals {
  # A shared channel can be listed twice (as shared and as public) with the
  # same id, so group by name and take the first entry.
  slack_channel_ids = {
    for name, ids in {
      for c in data.doit_alert_slack_channels.all.items : c.name => c.id...
    } : name => ids[0]
  }

  user_emails = [for u in data.doit_users.all.users : lower(u.email)]
}

# ─────────────────────────────────────────────────────────────────────────────
# Allocations
# ─────────────────────────────────────────────────────────────────────────────

# One single allocation per team, with anomaly detection switched on.
resource "doit_allocation" "team" {
  for_each = local.teams

  name              = "Team: ${each.key}"
  description       = "All costs labeled owner=${each.value.label}"
  anomaly_detection = true
  rule = {
    formula = "A"
    components = [{
      type   = "label"
      key    = "owner"
      mode   = "is"
      values = [each.value.label]
    }]
  }
}

# All spend: "is not none" matches every cost row, because no row has "none"
# as its cloud provider. A custom metric variable needs a single allocation,
# so this is the denominator of every team's share.
resource "doit_allocation" "total" {
  name        = "All cloud spend"
  description = "Every cost row"
  rule = {
    formula = "A"
    components = [{
      type    = "fixed"
      key     = "cloud_provider"
      mode    = "is"
      inverse = true
      values  = ["none"]
    }]
  }
}

# ─────────────────────────────────────────────────────────────────────────────
# Share-of-spend metric and report (needs the calculated metrics entitlement)
# ─────────────────────────────────────────────────────────────────────────────

resource "doit_custom_metric" "team_share" {
  for_each = local.teams

  name = "Share of spend: ${each.key}"
  variables = [
    {
      metric        = { type = "basic", value = "cost" }
      allocation_id = doit_allocation.team[each.key].id
    },
    {
      metric        = { type = "basic", value = "cost" }
      allocation_id = doit_allocation.total.id
    },
  ]
  formula   = "A / B * 100"
  unit_type = "percentage"
  decimals  = { min = 0, max = 1 }
}

resource "doit_report" "team_share" {
  for_each = local.teams

  name = "Team ${each.key}: share of spend"
  config = {
    metrics         = [{ type = "custom", value = doit_custom_metric.team_share[each.key].id }]
    aggregation     = "total"
    time_interval   = "month"
    data_source     = "billing"
    currency        = "USD"
    layout          = "column_chart"
    sort_dimensions = "a_to_z"
    dimensions = [
      { id = "year", type = "datetime" },
      { id = "month", type = "datetime" },
    ]
    time_range = {
      mode            = "last"
      amount          = 6
      unit            = "month"
      include_current = true
    }
  }
}

# ─────────────────────────────────────────────────────────────────────────────
# Delivery: the weekly report and critical anomalies, to the team's channel
# ─────────────────────────────────────────────────────────────────────────────

# The report as a PDF every Monday at 08:00 Berlin time, by email and in Slack.
resource "doit_customer_subscription" "team_weekly" {
  for_each = local.teams

  recipients = {
    emails         = [each.value.lead]
    slack_channels = [local.slack_channel_ids[each.value.slack_channel]]
  }

  subscriptions = [{
    type   = "report-subscription"
    source = { id = doit_report.team_share[each.key].id }
    schedule = {
      time_zone  = "Europe/Berlin"
      start_date = "2026-11-30T00:00:00+01:00"
      time       = "2026-11-30T08:00:00+01:00"
      frequency  = "Weekly"
    }
    attachments = ["pdf"]
  }]

  # Fail the plan with a readable message instead of an API error at apply.
  lifecycle {
    precondition {
      condition     = contains(keys(local.slack_channel_ids), each.value.slack_channel)
      error_message = "Team ${each.key}: the Slack channel \"${each.value.slack_channel}\" is not connected to DoiT."
    }
    precondition {
      condition     = contains(local.user_emails, lower(each.value.lead))
      error_message = "Team ${each.key}: ${each.value.lead} is not a user of this DoiT organization."
    }
  }
}

# Critical anomalies of the team's allocation only. Allocation IDs of a
# notification must belong to custom single allocations, which the team
# allocations are.
resource "doit_customer_notification" "team_critical" {
  for_each = local.teams

  name = "Team ${each.key}: critical cost anomalies"

  destinations = {
    emails         = [each.value.lead]
    slack_channels = [local.slack_channel_ids[each.value.slack_channel]]
    teams_channels = []
  }

  settings = {
    cost_anomalies = {
      min_severity    = "critical"
      min_excess_cost = { amount = "100.00", currency = "USD" }
      allocations = {
        mode = "include"
        ids  = [doit_allocation.team[each.key].id]
      }
    }
  }

  lifecycle {
    precondition {
      condition     = contains(keys(local.slack_channel_ids), each.value.slack_channel)
      error_message = "Team ${each.key}: the Slack channel \"${each.value.slack_channel}\" is not connected to DoiT."
    }
    precondition {
      condition     = contains(local.user_emails, lower(each.value.lead))
      error_message = "Team ${each.key}: ${each.value.lead} is not a user of this DoiT organization."
    }
  }
}
