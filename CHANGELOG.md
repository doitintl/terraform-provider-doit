# Changelog

## v1.1.0 (Unreleased)

### FEATURES

- **resource/doit_report**: Added `secondary_time_range` support for comparative reports, including custom date ranges ([#87](https://github.com/doitintl/terraform-provider-doit/pull/87))
- **data-source/doit_commitment**: New data source to get a single commitment by ID ([#89](https://github.com/doitintl/terraform-provider-doit/pull/89))
- **data-source/doit_commitments**: New data source to list commitments with filtering ([#89](https://github.com/doitintl/terraform-provider-doit/pull/89))
- **data-source/doit_platforms**: New data source to list available cloud platforms ([#91](https://github.com/doitintl/terraform-provider-doit/pull/91))
- **data-source/doit_products**: New data source to list available cloud products ([#91](https://github.com/doitintl/terraform-provider-doit/pull/91))

### BUG FIXES

- **data-sources**: Fixed auto-pagination to honor user-provided `page_token` instead of always starting from the beginning ([#90](https://github.com/doitintl/terraform-provider-doit/pull/90))
- **data-sources**: Replaced `types.ListNull` with empty lists for list attributes on data sources and added unknown input guards to all singular data sources ([#92](https://github.com/doitintl/terraform-provider-doit/pull/92))

### INTERNAL

- **spec**: Synced upstream rename of `DimensionsExternalAPIList` to `DimensionsExternalAPIListResponse` ([#88](https://github.com/doitintl/terraform-provider-doit/pull/88))
- **test**: Skipped support requests pagination test pending API fix (CMP-38591) ([#94](https://github.com/doitintl/terraform-provider-doit/pull/94))

## v1.0.1 (2026-02-13)

### BUG FIXES

- **resource/doit_report**: Corrected `data_source` enum values from underscores to hyphens (`billing_datahub` → `billing-datahub`, `kubernetes_utilization` → `kubernetes-utilization`) and added `IsUnknown` guards to prevent sending empty strings to the API ([#82](https://github.com/doitintl/terraform-provider-doit/pull/82))
- **resource/doit_budget**: Fixed handling of `BudgetCreateUpdateRequestMetric` enum type after upstream spec changes ([#83](https://github.com/doitintl/terraform-provider-doit/pull/83))
- **data-source/doit_dimensions**: Updated dimensions data source to align with latest API spec ([#81](https://github.com/doitintl/terraform-provider-doit/pull/81))

### FEATURES

- Provider now sends a custom `User-Agent` header (`Terraform/{version} terraform-provider-doit/{version}`) following HashiCorp conventions. Also respects `TF_APPEND_USER_AGENT` for custom identifiers ([#79](https://github.com/doitintl/terraform-provider-doit/pull/79))

### DOCUMENTATION

- Added group allocation example with `unallocated_costs` attribute ([#85](https://github.com/doitintl/terraform-provider-doit/pull/85))
- Clarified possible values for the `metric` field in the `budget` resource documentation ([#85](https://github.com/doitintl/terraform-provider-doit/pull/85))

### INTERNAL

- **ci**: Pre-install Terraform CLI via `hashicorp/setup-terraform` to prevent 502 download flakes during tests ([#80](https://github.com/doitintl/terraform-provider-doit/pull/80))
- **build**: Ignore Windows ARM builds and update archive format syntax in goreleaser config ([#78](https://github.com/doitintl/terraform-provider-doit/pull/78))
- **test**: Added acceptance test verifying report creation without `data_source` attribute ([#84](https://github.com/doitintl/terraform-provider-doit/pull/84))

## v1.0.0 (2026-02-10)

This is a major release with breaking changes. Please see the [v1.0.0 Upgrade Guide](docs/guides/version_1_upgrade.md) for migration instructions.

### BREAKING CHANGES

- **resource/doit_allocation_group**: Resource has been removed. Use `doit_allocation` with the `rules` attribute instead.
- **resource/doit_attribution**: Resource has been removed. Use Allocations instead (via DoiT Console or `doit_allocation` resource).
- **resource/doit_attribution_group**: Resource has been removed. Use Allocations instead.
- **resource/doit_allocation**: The `description` attribute is now required.
- **resource/doit_report**: The `mode` attribute is now required in filter configurations.

### DEPRECATIONS

- **resource/doit_budget**: The `scope` attribute is deprecated. Use `scopes` instead.
- **resource/doit_budget**: The `last_updated` attribute has been removed. Use `update_time` instead.
- **resource/doit_report**: The `config.metric` attribute is deprecated. Use `config.metrics` instead.
- **resource/doit_alert**: The `config.attributions` attribute is deprecated. Use `config.scopes` instead.

### NEW RESOURCES

- **resource/doit_alert**: Manage cost/usage alerts with threshold notifications
- **resource/doit_annotation**: Create custom notes on cost data for contextual information
- **resource/doit_label**: Create labels to categorize annotations

### NEW DATA SOURCES

- **data-source/doit_account_team**: Get account team information
- **data-source/doit_alert**: Get a single alert by ID
- **data-source/doit_alerts**: List alerts with filtering
- **data-source/doit_allocation**: Get a single allocation by ID
- **data-source/doit_allocations**: List allocations with filtering
- **data-source/doit_annotation**: Get a single annotation by ID
- **data-source/doit_annotations**: List annotations with filtering
- **data-source/doit_anomalies**: List cost anomalies with filtering
- **data-source/doit_anomaly**: Get a single anomaly by ID
- **data-source/doit_assets**: List cloud assets with filtering
- **data-source/doit_budget**: Get a single budget by ID
- **data-source/doit_budgets**: List budgets with filtering
- **data-source/doit_cloud_incident**: Get a single cloud incident by ID
- **data-source/doit_cloud_incidents**: List cloud incidents with filtering
- **data-source/doit_current_user**: Get current authenticated user information
- **data-source/doit_dimensions**: List available dimensions for reports
- **data-source/doit_invoice**: Get a single invoice by ID
- **data-source/doit_invoices**: List invoices with filtering
- **data-source/doit_label**: Get a single label by ID
- **data-source/doit_labels**: List labels with filtering
- **data-source/doit_report**: Get a single report by ID
- **data-source/doit_reports**: List reports with filtering
- **data-source/doit_roles**: List available roles
- **data-source/doit_support_requests**: List support requests
- **data-source/doit_users**: List users with filtering

### FEATURES

- **resource/doit_allocation**: New `rules` attribute for creating allocation groups within a single resource
- **resource/doit_allocation**: New `unallocated_costs` attribute for group-type allocations
- **resource/doit_budget**: New `scopes` attribute with flexible filtering options (type, mode, values)
- **resource/doit_budget**: New `seasonal_amounts` attribute for varying budget amounts per period
- **resource/doit_report**: New `config.metrics` attribute supporting up to 4 metrics per report
- **resource/doit_report**: New `config.custom_time_range` attribute for custom time range queries
- **resource/doit_alert**: New `config.scopes` attribute with flexible filtering options

### ENHANCEMENTS

- **resource/doit_allocation**: New computed attributes: `allocation_type`, `create_time`, `update_time`
- **resource/doit_budget**: New computed attributes: `create_time`, `update_time`, `current_utilization`, `forecasted_utilization`
- **resource/doit_budget**: Alert thresholds now include computed `forecasted_date` and `triggered` fields
- Provider now uses generated OpenAPI client for improved API compatibility
- Improved error handling with automatic retry on rate limits and transient errors
- OAuth2 token validation for API authentication
- Automatic state migration for `doit_budget` resources from v0 to v1 schema

### BUG FIXES

- Various schema validation improvements
- Fixed state inconsistencies after apply operations
- Empty list validation for `doit_budget.alerts`, `doit_allocation.rules`, and `doit_report.config.metrics`

## v0.26.0 (2025-12-04)

## What's Changed

- Branding and styling updates by @spark2ignite in https://github.com/doitintl/terraform-provider-doit/pull/15
- Cleanup by @hanneshayashi in https://github.com/doitintl/terraform-provider-doit/pull/27
- Bump golang.org/x/crypto from 0.41.0 to 0.45.0 by @dependabot[bot] in https://github.com/doitintl/terraform-provider-doit/pull/32
- feat: Add deprecation warnings to the allocation group resource schem… by @hanneshayashi in https://github.com/doitintl/terraform-provider-doit/pull/40
- add deprecation messages to attribution resources by @hanneshayashi in https://github.com/doitintl/terraform-provider-doit/pull/25
- Budget start period validation by @hanneshayashi in https://github.com/doitintl/terraform-provider-doit/pull/47
- More docs updates by @hanneshayashi in https://github.com/doitintl/terraform-provider-doit/pull/44
- Update doit provider version to 0.26.0 in documentation and examples. by @hanneshayashi in https://github.com/doitintl/terraform-provider-doit/pull/49

## New Contributors

- @spark2ignite made their first contribution in https://github.com/doitintl/terraform-provider-doit/pull/15

**Full Changelog**: https://github.com/doitintl/terraform-provider-doit/compare/v0.25.0...v0.26.0

## v0.25.0 (2025-09-10)

## What's Changed

- Support Allocations and Allocation Groups, using code generation from the DoiT OAS by @hanneshayashi in https://github.com/doitintl/terraform-provider-doit/pull/12
- Allocations and Allocation Groups user QoL and refactoring, added test suites for both state and plan lifecycles for all resources: by @oboukili in https://github.com/doitintl/terraform-provider-doit/pull/14

## New Contributors

- @hanneshayashi made their first contribution in https://github.com/doitintl/terraform-provider-doit/pull/12
- @oboukili made their first contribution in https://github.com/doitintl/terraform-provider-doit/pull/13

**Full Changelog**: https://github.com/doitintl/terraform-provider-doit/compare/v0.24.0...v0.25.0

## v0.24.0 (2025-03-11)

### Commits

- Modified DoiT API auth endpoint 8e4010d

## v0.23.0 (2024-09-25)

### Commits

- Add latest version docs 9dac675

## v0.22.0 (2024-09-25)

### Commits

- Updated .goreleaser.yml b547147
- made customer_context optional 4c52381

## v0.20.0 (2024-05-21)

### Commits

- Add retry in 502,503 and 520 ba62018

## v0.19.0 (2024-05-10)

### Commits

- define RecipientsSlackChannels outside the cycle so it is updated when is empty 69dc4a8
- Spelling/grammar for README.md bf4d1dd

## v0.18.0 (2024-04-02)

### Commits

- fix documentation version and add clarification about proof of concept be77b3d

## v0.17.0 (2024-02-27)

### Commits

- fixes #6 by removing the arguments forecasted_date and triggered from the alert, fixes #4 by adding a budget example" d5e7421

## v0.16.0 (2024-02-27)

### Commits

- fixes #7; if a resource is not found the terraform read signal remove the resource from the state f4e1d27

## v0.15.0 (2024-02-07)

### Commits

- Fixing bugs to validate empty response and remove "/" to avoid redirect request 72fd38b
- Add budgets 462e554

## v0.14.0 (2024-01-29)

### Commits

- add plan modifier to avoid replacement when there is an update 2c5ec41
- Bump github.com/cloudflare/circl from 1.3.3 to 1.3.7 c6f09e1
- Bump golang.org/x/crypto from 0.14.0 to 0.17.0 ccdd75e

## v0.13.0 (2024-01-25)

### Commits

- Update terraform plugin references to work with terraform 1.6.x fbc405b

## v0.12.0 (2024-01-19)

### Commits

- Adding properties sort_groups and sort_dimensions 92e6df5

## v0.11.0 (2024-01-05)

### Commits

- fixing the rate 35ee031

## v0.10.0 (2024-01-04)

### Commits

- add retry in 429 errors 77895dd

## v0.9.0 (2024-01-04)

### Commits

- add limit rate to the doit API http client a5aae75

## v0.8.0 (2024-01-03)

### Commits

- update documetation and leave optional values for targets in splits 18ffbd7

## v0.7.0 (2024-01-03)

### Commits

- Documentation with requiered and default values a2b0750
- adding default values for reports 85059f6

## v0.6.0 (2024-01-03)

### Commits

- add group limit to read and update f0e135d

## v0.5.0 (2023-12-05)

### Commits

- Add default values and send empty array when list are not provided 52e946c

## v0.4.0 (2023-11-30)

### Commits

- calling terraform provider doit instead of doit-console 05948d1
- updating documentation 811bb1f
- fix spelling error and update docs cf94a26
- fix spelling error and update docs 3d9870e
- adding new version 94c7427
- fixing issues 4,5,7,8,9,10,11 03dc849
- Fix docs bug with provider name including "provider-name" d1b1a37
- add report resource e69b12b
- Generating new documentation for attribution an attribution groups f0876ec
- adding new documentation 0dc31f4
- modify the provieder name and address for public repository bce6a0f
- Bump google.golang.org/grpc from 1.57.0 to 1.57.1 9dbcaf4
- Adding github action workflow cdaa443
- Bump golang.org/x/net from 0.13.0 to 0.17.0 32816cb
- saving the status in terraform 242fdd9
- fix attribution group update errors and update documentation c3f8971
- Add attribution group 5bbbdb0
- Adding an Attribution Group Resource 7269141
- add terraform example d4b7709
- first commit 4a78b4a
