# Variables measure single allocations.
resource "doit_allocation" "gcp" {
  name        = "Google Cloud"
  description = "All Google Cloud costs"
  rule = {
    formula = "A"
    components = [
      { key = "cloud_provider", mode = "is", type = "fixed", values = ["google-cloud"] },
    ]
  }
}

resource "doit_allocation" "aws" {
  name        = "AWS"
  description = "All AWS costs"
  rule = {
    formula = "A"
    components = [
      { key = "cloud_provider", mode = "is", type = "fixed", values = ["amazon-web-services"] },
    ]
  }
}

# The Google Cloud share of the combined Google Cloud and AWS cost. The order
# of variables sets their letters: the first is A, the second B.
resource "doit_custom_metric" "gcp_share" {
  name        = "Google Cloud share"
  description = "Google Cloud cost as a percentage of Google Cloud and AWS cost"
  variables = [
    {
      metric        = { type = "basic", value = "cost" }
      allocation_id = doit_allocation.gcp.id
    },
    {
      metric        = { type = "basic", value = "cost" }
      allocation_id = doit_allocation.aws.id
    },
  ]
  formula   = "A / (A + B) * 100"
  unit_type = "percentage"
  decimals  = { min = 0, max = 2 }
}

# A variable can also measure an extended metric of the billing data source,
# such as amortized cost.
resource "doit_custom_metric" "aws_amortized_ratio" {
  name = "AWS amortized to billed cost"
  variables = [
    {
      metric        = { type = "extended", value = "amortized_cost" }
      allocation_id = doit_allocation.aws.id
    },
    {
      metric        = { type = "basic", value = "cost" }
      allocation_id = doit_allocation.aws.id
    },
  ]
  formula = "A / B"
}

# Reports use a custom metric by its ID.
resource "doit_report" "gcp_share" {
  name = "Google Cloud share"
  config = {
    metrics       = [{ type = "custom", value = doit_custom_metric.gcp_share.id }]
    aggregation   = "total"
    time_interval = "month"
    data_source   = "billing"
    dimensions    = [{ id = "month", type = "datetime" }]
    time_range = {
      mode            = "last"
      amount          = 3
      include_current = true
      unit            = "month"
    }
  }
}

# Alerts use a custom metric by its ID too. This one fires when Google Cloud
# is more than 60% of the combined Google Cloud and AWS cost. Budgets can't
# use custom metrics: their metric is cost or amortized_cost.
data "doit_current_user" "me" {}

resource "doit_alert" "gcp_share" {
  name = "Google Cloud share above 60%"
  config = {
    metric        = { type = "custom", value = doit_custom_metric.gcp_share.id }
    time_interval = "month"
    condition     = "value"
    operator      = "gt"
    value         = 60
    currency      = "USD"
  }
  recipients = [data.doit_current_user.me.email]
}
