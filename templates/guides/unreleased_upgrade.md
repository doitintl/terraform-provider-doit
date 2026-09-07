---
page_title: "Changes after v1.7.0"
subcategory: ""
description: |-
  Prepare for the unreleased report layout and Insights changes after v1.7.0.
---

# Changes after v1.7.0

These changes are on the development branch and are not part of v1.7.0. Review them before upgrading to a release that includes them. See the [changelog](https://github.com/doitintl/terraform-provider-doit/blob/main/CHANGELOG.md) for the complete release notes, including timeout changes.

## Report layouts

The `config.layout` attribute on `doit_report` and `doit_report_query` now accepts `sankey_chart`, `column_and_line_chart`, and `trend_board`. Other report configuration constraints still apply to these layouts.

The public API no longer accepts `csv_export` or `sheets_export` as report layouts. Configurations using either value must select a visualization layout, such as `table`, before upgrading. Selecting `table` does not perform an export; keep export operations separate from the report's visualization configuration.

## Insights removal

The upstream Insights service was deprecated, and these provider types have been removed:

- Resources: `doit_insight`, `doit_insight_resource_results`.
- Data sources: `doit_insight`, `doit_insights`, `doit_insight_resource_results`.

The public API reference may still list legacy Insights operations. Their presence does not mean the development provider supports them. A replacement API is being developed; this provider does not yet expose a replacement resource or data source.

Before upgrading, while the old provider version is still selected:

1. Use `terraform state list` to identify the affected resource and data-source addresses, including module and instance keys.
2. Remove references to these types and their outputs from your configuration. To stop managing existing insights without attempting to delete them remotely, remove the matching addresses from state with `terraform state rm`. For example, `terraform state rm 'doit_insight.example'`. This forgets Terraform's record; it does not delete the remote object. Do not run an apply between removing the state entries and removing their configuration.
3. Upgrade the provider and run `terraform plan` to check the remaining configuration. The new provider cannot refresh, destroy, or import the removed types.

If remote deletion is required, arrange it before upgrading through a still-supported upstream mechanism. Staying on v1.7.0 preserves the old Terraform schemas but does not guarantee that the deprecated service remains available.

## Dataset display names and query results

Dataset `display_name` changes the label shown to users; `name` remains the dataset's ingest identity. Removing `display_name` from a managed dataset's configuration clears the alias.

For `doit_report_query`, `jsondecode(result_json).details.valueAliases` contains optional display aliases keyed by dimension ID and stored value. Use the alias for presentation and retain the original row value for filters. The field may be absent when no aliases apply.

This alias contract currently applies to the ad-hoc query response (`RunReportResult`). The public saved-report response schema (`GetReportResponse`) does not declare `details.valueAliases`; the provider does not promise that field in `doit_report_result` until the upstream contract includes it.
