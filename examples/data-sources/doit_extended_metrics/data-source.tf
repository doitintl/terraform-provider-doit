terraform {
  required_providers {
    doit = {
      source = "doitintl/doit"
    }
  }
}

# List every extended metric available to the customer. All pages are fetched
# unless max_results is set. A key appears once per data source, so the same
# key can be listed more than once.
data "doit_extended_metrics" "all" {}

output "extended_metric_count" {
  value = data.doit_extended_metrics.all.row_count
}

# A custom metric variable accepts only billing and billing-datahub keys.
data "doit_extended_metrics" "custom_metric_variables" {
  filter = "dataSource:billing,billing-datahub"
}

output "custom_metric_variable_keys" {
  value = [for m in data.doit_extended_metrics.custom_metric_variables.items : m.key]
}

# Look up a key by its label. Use it as the value of
# metric = { type = "extended", value = <key> } in a doit_custom_metric variable,
# a report or an alert.
locals {
  keys_by_label = {
    for m in data.doit_extended_metrics.custom_metric_variables.items : m.label => m.key...
  }
}

output "keys_by_label" {
  value = local.keys_by_label
}

# Read a single page of up to 10 metrics; page_token holds the token of the
# next page. The token is bound to the filter it was returned for.
data "doit_extended_metrics" "first_page" {
  max_results = 10
}

output "next_page_token" {
  value = data.doit_extended_metrics.first_page.page_token
}
