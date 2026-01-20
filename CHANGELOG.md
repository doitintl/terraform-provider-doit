# Changelog

## v1.0.0 (Unreleased)

This is a major release with breaking changes. Please see the [v1.0.0 Upgrade Guide](docs/guides/version_1_upgrade.md) for migration instructions.

### BREAKING CHANGES

* **resource/doit_allocation_group**: Resource has been removed. Use `doit_allocation` with the `rules` attribute instead.
* **resource/doit_attribution**: Resource has been removed. Use Allocations instead (via DoiT Console or `doit_allocation` resource).
* **resource/doit_attribution_group**: Resource has been removed. Use Allocations instead.
* **resource/doit_budget**: The `scope` attribute is deprecated and will be removed in v2.0.0. Use `scopes` instead.
* **resource/doit_budget**: The `last_updated` attribute has been removed. Use `update_time` instead.
* **resource/doit_allocation**: The `description` attribute is now required.
* **resource/doit_report**: The `mode` attribute is now required in filter configurations.

### FEATURES

* **resource/doit_allocation**: New `rules` attribute for creating allocation groups within a single resource
* **resource/doit_allocation**: New `unallocated_costs` attribute for group-type allocations
* **resource/doit_budget**: New `scopes` attribute with flexible filtering options (type, mode, values)
* **resource/doit_budget**: New `seasonal_amounts` attribute for varying budget amounts per period
* **resource/doit_report**: New `config.custom_time_range` attribute for custom time range queries

### ENHANCEMENTS

* **resource/doit_allocation**: New computed attributes: `allocation_type`, `create_time`, `update_time`
* **resource/doit_budget**: New computed attributes: `create_time`, `update_time`, `current_utilization`, `forecasted_utilization`
* **resource/doit_budget**: Alert thresholds now include computed `forecasted_date` and `triggered` fields
* Provider now uses generated OpenAPI client for improved API compatibility
* Improved error handling with automatic retry on rate limits and transient errors
* OAuth2 token validation for API authentication

### BUG FIXES

* Various schema validation improvements
* Fixed state inconsistencies after apply operations

### NOTES

* The provider includes automatic state migration for `doit_budget` resources from v0 to v1 schema
* Many previously required attributes are now optional for improved flexibility

## v0.26.0 (2025-12-04)

## What's Changed
* Branding and styling updates by @spark2ignite in https://github.com/doitintl/terraform-provider-doit/pull/15
* Cleanup by @hanneshayashi in https://github.com/doitintl/terraform-provider-doit/pull/27
* Bump golang.org/x/crypto from 0.41.0 to 0.45.0 by @dependabot[bot] in https://github.com/doitintl/terraform-provider-doit/pull/32
* feat: Add deprecation warnings to the allocation group resource schem… by @hanneshayashi in https://github.com/doitintl/terraform-provider-doit/pull/40
* add deprecation messages to attribution resources by @hanneshayashi in https://github.com/doitintl/terraform-provider-doit/pull/25
* Budget start period validation by @hanneshayashi in https://github.com/doitintl/terraform-provider-doit/pull/47
* More docs updates by @hanneshayashi in https://github.com/doitintl/terraform-provider-doit/pull/44
* Update doit provider version to 0.26.0 in documentation and examples. by @hanneshayashi in https://github.com/doitintl/terraform-provider-doit/pull/49

## New Contributors
* @spark2ignite made their first contribution in https://github.com/doitintl/terraform-provider-doit/pull/15

**Full Changelog**: https://github.com/doitintl/terraform-provider-doit/compare/v0.25.0...v0.26.0

## v0.25.0 (2025-09-10)

## What's Changed
* Support Allocations and Allocation Groups, using code generation from the DoiT OAS by @hanneshayashi in https://github.com/doitintl/terraform-provider-doit/pull/12
* Allocations and Allocation Groups user QoL and refactoring, added test suites for both state and plan lifecycles for all resources: by @oboukili in https://github.com/doitintl/terraform-provider-doit/pull/14

## New Contributors
* @hanneshayashi made their first contribution in https://github.com/doitintl/terraform-provider-doit/pull/12
* @oboukili made their first contribution in https://github.com/doitintl/terraform-provider-doit/pull/13

**Full Changelog**: https://github.com/doitintl/terraform-provider-doit/compare/v0.24.0...v0.25.0

## v0.24.0 (2025-03-11)

### Commits
* Modified DoiT API auth endpoint 8e4010d

## v0.23.0 (2024-09-25)

### Commits
* Add latest version docs 9dac675

## v0.22.0 (2024-09-25)

### Commits
* Updated .goreleaser.yml b547147
* made customer_context optional 4c52381

## v0.20.0 (2024-05-21)

### Commits
* Add retry in 502,503 and 520 ba62018

## v0.19.0 (2024-05-10)

### Commits
* define RecipientsSlackChannels outside the cycle so it is updated when is empty 69dc4a8
* Spelling/grammar for README.md bf4d1dd

## v0.18.0 (2024-04-02)

### Commits
* fix documentation version and add clarification about proof of concept be77b3d

## v0.17.0 (2024-02-27)

### Commits
* fixes #6 by removing the arguments forecasted_date and triggered from the alert, fixes #4 by adding a budget example" d5e7421

## v0.16.0 (2024-02-27)

### Commits
* fixes #7; if a resource is not found the terraform read signal remove the resource from the state f4e1d27

## v0.15.0 (2024-02-07)

### Commits
* Fixing bugs to validate empty response and remove "/" to avoid redirect request 72fd38b
* Add budgets 462e554

## v0.14.0 (2024-01-29)

### Commits
* add plan modifier to avoid replacement when there is an update 2c5ec41
* Bump github.com/cloudflare/circl from 1.3.3 to 1.3.7 c6f09e1
* Bump golang.org/x/crypto from 0.14.0 to 0.17.0 ccdd75e

## v0.13.0 (2024-01-25)

### Commits
* Update terraform plugin references to work with terraform 1.6.x fbc405b

## v0.12.0 (2024-01-19)

### Commits
* Adding properties sort_groups and sort_dimensions 92e6df5

## v0.11.0 (2024-01-05)

### Commits
* fixing the rate 35ee031

## v0.10.0 (2024-01-04)

### Commits
* add retry in 429 errors 77895dd

## v0.9.0 (2024-01-04)

### Commits
* add limit rate to the doit API http client a5aae75

## v0.8.0 (2024-01-03)

### Commits
* update documetation and leave optional values for targets in splits 18ffbd7

## v0.7.0 (2024-01-03)

### Commits
* Documentation with requiered and default values a2b0750
* adding default values for reports 85059f6

## v0.6.0 (2024-01-03)

### Commits
* add group limit to read and update f0e135d

## v0.5.0 (2023-12-05)

### Commits
* Add default values and send empty array when list are not provided 52e946c

## v0.4.0 (2023-11-30)

### Commits
* calling terraform provider doit instead of doit-console 05948d1
* updating documentation 811bb1f
* fix spelling error and update docs cf94a26
* fix spelling error and update docs 3d9870e
* adding new version 94c7427
* fixing issues 4,5,7,8,9,10,11 03dc849
* Fix docs bug with provider name including "provider-name" d1b1a37
* add report resource e69b12b
* Generating new documentation for attribution an attribution groups f0876ec
* adding new documentation 0dc31f4
* modify the provieder name and address for public repository bce6a0f
* Bump google.golang.org/grpc from 1.57.0 to 1.57.1 9dbcaf4
* Adding github action workflow cdaa443
* Bump golang.org/x/net from 0.13.0 to 0.17.0 32816cb
* saving the status in terraform 242fdd9
* fix attribution group update errors and update documentation c3f8971
* Add attribution group 5bbbdb0
* Adding an Attribution Group Resource 7269141
* add terraform example d4b7709
* first commit 4a78b4a
