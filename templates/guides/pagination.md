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
- `row_count` reflects the total number of items

~> **Note:** For data sources with potentially large result sets, auto-pagination may take longer. Consider using manual pagination or filters to limit results.

## Manual Pagination

To manually control pagination, set `max_results` to limit the number of results per page. The provider returns a single page along with a `page_token` you can use to fetch the next page:

```hcl
# Fetch only the first 5 invoices
data "doit_invoices" "page1" {
  max_results = 5
}

# Use the page_token to fetch the next page
data "doit_invoices" "page2" {
  max_results = 5
  page_token  = data.doit_invoices.page1.page_token
}
```

With manual pagination:
- Only the requested number of results are fetched per call
- `page_token` contains the token for the next page (or `null` if no more pages)
- You can chain multiple data source declarations to iterate through pages

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

| Data Source | Auto-Pagination | Manual Pagination |
|-------------|:-:|:-:|
| `doit_alerts` | ✓ | ✓ |
| `doit_allocations` | ✓ | ✓ |
| `doit_annotations` | ✓ | ✓ |
| `doit_anomalies` | ✓ | ✓ |
| `doit_assets` | ✓ | ✓ |
| `doit_budgets` | ✓ | ✓ |
| `doit_cloud_incidents` | ✗ | ✓ |
| `doit_commitments` | ✓ | ✓ |
| `doit_dimensions` | ✓ | ✓ |
| `doit_invoices` | ✓ | ✓ |
| `doit_labels` | ✓ | ✓ |
| `doit_reports` | ✓ | ✓ |
| `doit_support_requests` | ✓ | ✓ |

## Filtering vs Pagination

Most paginated data sources also support a `filter` attribute that lets you reduce the result set server-side. Use filters when possible to minimize the number of API calls:

```hcl
# Fetch only critical anomalies — much faster than fetching all and filtering client-side
data "doit_anomalies" "critical" {
  filter = "severityLevel:critical"
}
```

See each data source's documentation for available filter keys.
