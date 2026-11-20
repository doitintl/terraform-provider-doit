---
page_title: "Pagination Guide"
description: |-
  How auto and manual pagination works with the DoiT provider data sources.
---

# Pagination Guide

Many DoiT data sources return paginated results from the API. The provider supports two pagination modes: **automatic** (default) and **manual**. This guide explains how each mode works and when to use them.

## Auto-Pagination (Default)

When you omit the `max_results` attribute, the provider automatically fetches **all pages** and returns the complete result set. This is the simplest way to retrieve data:

```hcl
# Fetch all budgets automatically — no pagination needed
data "doit_budgets" "all" {}

output "total_budgets" {
  value = data.doit_budgets.all.row_count
}
```

With auto-pagination:

- All pages are fetched transparently
- `page_token` is set to `null` in the result (there's no "next page")
- If you set `page_token`, it is where the read starts: every page from that token on is fetched, and `page_token` is still `null` in the result
- `row_count` reflects the total number of items (see [Row Count](#row-count))

~> **Note:** For data sources with potentially large result sets, auto-pagination may take longer. Consider using manual pagination or filters to limit results.

## Manual Pagination

To manually control pagination, set `max_results` to limit the number of results per page. The provider returns a single page along with a `page_token` you can use to fetch the next page:

```hcl
# Fetch only the first 5 invoices
data "doit_invoices" "page1" {
  max_results = 5
}

# Fetch the next page, but only if there is one
data "doit_invoices" "page2" {
  count = data.doit_invoices.page1.page_token != null ? 1 : 0

  max_results = 5
  page_token  = data.doit_invoices.page1.page_token
}

output "second_page_token" {
  value = one(data.doit_invoices.page2[*].page_token)
}
```

With manual pagination:

- Only the requested number of results are fetched per call
- `page_token` contains the token for the next page (or `null` if no more pages)
- You can chain multiple data source declarations to iterate through pages

~> **Note:** Guard each chained read with `count` as shown above. A `null` `page_token` together with `max_results` reads the **first** page, so an unguarded `page2` would start over at the beginning of the list once `page1` is the last page.

A page can hold fewer items than `max_results`, or even none, while `page_token` is set, for example when a filter removes most of the items the API scanned for that page. Only a `null` `page_token` marks the end of the list, so keep following it until it is `null`.

## Row Count

`row_count` has one meaning per data source, whatever `max_results` and `page_token` are:

- **Total across all pages**: the number of items as reported by the API. With `max_results` or `page_token` set it can be larger than the number of items returned. If the API reports no count, `row_count` is the number of items the read returned instead.
- **Items returned**: the number of items in the result. With `max_results` set that is one page; without it, every page fetched.

The table below lists which one each data source reports. A data source's `row_count` documentation says the same.

## Cloud Incidents

The `doit_cloud_incidents` data source **only supports manual pagination** — it does not auto-paginate even when `max_results` is omitted. This is because cloud incidents can have thousands of records and auto-fetching all of them would result in very long response times.

Always specify `max_results` when querying cloud incidents:

```hcl
data "doit_cloud_incidents" "recent" {
  max_results = 100
}
```

## Supported Data Sources

The following table shows which data sources support pagination:

| Data Source                       | Auto-Pagination | Manual Pagination | `row_count`         |
| --------------------------------- | --------------- | ----------------- | ------------------- |
| `doit_alert_slack_channels`       | ✓               | ✓                 | Total, when known   |
| `doit_alerts`                     | ✓               | ✓                 | Items returned      |
| `doit_allocations`                | ✓               | ✓                 | Items returned      |
| `doit_annotations`                | ✓               | ✓                 | Items returned      |
| `doit_anomalies`                  | ✓               | ✓                 | Items returned      |
| `doit_assets`                     | ✓               | ✓                 | Items returned      |
| `doit_budgets`                    | ✓               | ✓                 | Items returned      |
| `doit_cloud_diagrams_search`      | ✓               | ✓                 | —                   |
| `doit_cloud_incidents`            | ✗               | ✓                 | Items returned      |
| `doit_commitments`                | ✓               | ✓                 | Items returned      |
| `doit_composer_recipe_results`    | ✓               | ✓                 | Total               |
| `doit_composer_recipes`           | ✓               | ✓                 | Total               |
| `doit_custom_metrics`             | ✓               | ✓                 | Total               |
| `doit_customer_subscriptions`     | ✓               | ✓                 | Total               |
| `doit_dimensions`                 | ✓               | ✓                 | Items returned      |
| `doit_extended_metrics`           | ✓               | ✓                 | Total               |
| `doit_folders`                    | ✓               | ✓                 | Items returned      |
| `doit_invoices`                   | ✓               | ✓                 | Items returned      |
| `doit_labels`                     | ✓               | ✓                 | Items returned      |
| `doit_ps4c_aws_organizations`     | ✓               | ✓                 | Total               |
| `doit_ps4c_commitment_policies`   | ✓               | ✓                 | Total               |
| `doit_ps4c_gcp_billing_accounts`  | ✓               | ✓                 | Total               |
| `doit_ps4c_gcp_planned_purchases` | ✓               | ✓                 | Items returned      |
| `doit_ps4c_gcp_resource_cuds`     | ✓               | ✓                 | Total               |
| `doit_ps4c_gcp_settings`          | ✓               | ✓                 | Items returned      |
| `doit_ps4c_gcp_spend_cuds`        | ✓               | ✓                 | Total               |
| `doit_reports`                    | ✓               | ✓                 | Items returned      |
| `doit_service_quotas`             | ✓               | ✓                 | Total               |
| `doit_support_requests`           | ✓               | ✓                 | Items returned      |

`doit_cloud_diagrams_search` pages with `from` and `size` instead of `page_token` and `max_results`: setting `size` reads a single page starting at `from`.

## Filtering vs Pagination

Most paginated data sources also support a `filter` attribute that lets you reduce the result set server-side. Use filters when possible to minimize the number of API calls:

```hcl
# Fetch only critical anomalies — much faster than fetching all and filtering client-side
data "doit_anomalies" "critical" {
  filter = "severityLevel:critical"
}
```

See each data source's documentation for available filter keys.
