# DoiT Cloud Intelligence Provider

[![Tests](https://github.com/doitintl/terraform-provider-doit/actions/workflows/tests.yml/badge.svg)](https://github.com/doitintl/terraform-provider-doit/actions/workflows/tests.yml)
[![Release](https://img.shields.io/github/v/release/doitintl/terraform-provider-doit)](https://github.com/doitintl/terraform-provider-doit/releases/latest)
[![Terraform Registry](https://img.shields.io/badge/terraform-registry-blueviolet?logo=terraform)](https://registry.terraform.io/providers/doitintl/doit/latest)
[![Go](https://img.shields.io/github/go-mod/go-version/doitintl/terraform-provider-doit)](https://go.dev/)
[![License](https://img.shields.io/github/license/doitintl/terraform-provider-doit)](LICENSE)

The DoiT Cloud Intelligence Terraform Provider lets you manage [DoiT Cloud Intelligence](https://www.doit.com/platform/) resources as code using the [DoiT API](https://developer.doit.com/reference). It covers FinOps workflows (budgets, allocations, alerts, reports), operational data (anomalies, incidents, commitments, assets), and organizational resources (labels, annotations, DataHub datasets).

> [!IMPORTANT]
> **Upgrading from v0.x?** Version 1.0.0 contains breaking changes. See the [v1.0.0 Upgrade Guide](docs/guides/version_1_upgrade.md) for migration instructions.

## Getting Started

### Prerequisites

- [Terraform](https://developer.hashicorp.com/terraform/downloads) >= 1.0
- A DoiT Cloud Intelligence account with an [API key](https://developer.doit.com/docs/start#api-keys)

### Authentication

The provider supports three configuration options, all of which can be set via environment variables:

| Attribute          | Environment Variable    | Required | Description                                           |
| ------------------ | ----------------------- | -------- | ----------------------------------------------------- |
| `api_token`        | `DOIT_API_TOKEN`        | Yes      | Your DoiT API key                                     |
| `host`             | `DOIT_HOST`             | No       | API host (defaults to `https://api.doit.com`)         |
| `customer_context` | `DOIT_CUSTOMER_CONTEXT` | No\*     | Customer context (_required for DoiT employees only_) |

### Provider Configuration

```terraform
terraform {
  required_providers {
    doit = {
      source  = "doitintl/doit"
      version = "~> 1.0"
    }
  }
}

provider "doit" {
  # Set DOIT_API_TOKEN in your environment before running terraform apply
}
```

Or configure explicitly:

```terraform
provider "doit" {
  api_token        = "your-api-key"
  customer_context = "your-customer-context" # DoiT employees only
}
```

## Available Resources

### Resources

| Resource                 | Description                                                       |
| ------------------------ | ----------------------------------------------------------------- |
| `doit_alert`             | Cost/usage alerts with threshold notifications                    |
| `doit_allocation`        | Cost allocation rules and groups                                  |
| `doit_annotation`        | Custom notes on cost data                                         |
| `doit_asset`             | Cloud assets (import-only; manage Google Workspace licenses)      |
| `doit_budget`            | Budget tracking with alerts and seasonal amounts                  |
| `doit_datahub_dataset`   | DataHub dataset management                                        |
| `doit_label`             | Labels for categorizing annotations                               |
| `doit_label_assignments` | Assign labels to resources                                        |
| `doit_report`            | Cloud Analytics reports with filters, metrics, and grouping       |
| `doit_sharing`           | Sharing permissions for reports, budgets, alerts, and allocations |

### Data Sources

<details>
<summary><strong>FinOps</strong> — budgets, allocations, alerts, reports</summary>

| Data Source                            | Description                         |
| -------------------------------------- | ----------------------------------- |
| `doit_alert` / `doit_alerts`           | Get or list cost/usage alerts       |
| `doit_allocation` / `doit_allocations` | Get or list allocation rules        |
| `doit_budget` / `doit_budgets`         | Get or list budgets                 |
| `doit_report` / `doit_reports`         | Get or list Cloud Analytics reports |
| `doit_report_query`                    | Run ad-hoc Cloud Analytics queries  |
| `doit_report_result`                   | Get results from an existing report |
| `doit_dimensions`                      | List available report dimensions    |

</details>

<details>
<summary><strong>Operations</strong> — anomalies, incidents, commitments, assets, invoices</summary>

| Data Source                                      | Description                          |
| ------------------------------------------------ | ------------------------------------ |
| `doit_anomaly` / `doit_anomalies`                | Get or list cost anomalies           |
| `doit_cloud_incident` / `doit_cloud_incidents`   | Get or list cloud provider incidents |
| `doit_commitment` / `doit_commitments`           | Get or list commitments              |
| `doit_asset` / `doit_assets`                     | Get or list cloud assets             |
| `doit_invoice` / `doit_invoices`                 | Get or list invoices                 |
| `doit_support_request` / `doit_support_requests` | Get or list support requests         |
| `doit_support_request_comments`                  | List comments on a support request   |

</details>

<details>
<summary><strong>Organization</strong> — users, labels, annotations, platforms, datasets</summary>

| Data Source                                      | Description                    |
| ------------------------------------------------ | ------------------------------ |
| `doit_annotation` / `doit_annotations`           | Get or list annotations        |
| `doit_label` / `doit_labels`                     | Get or list labels             |
| `doit_label_assignments`                         | List label assignments         |
| `doit_datahub_dataset` / `doit_datahub_datasets` | Get or list DataHub datasets   |
| `doit_current_user`                              | Get current authenticated user |
| `doit_users`                                     | List users                     |
| `doit_roles`                                     | List available roles           |
| `doit_account_team`                              | Get account team information   |
| `doit_organizations`                             | List organizations             |
| `doit_platforms`                                 | List available cloud platforms |
| `doit_products`                                  | List available cloud products  |
| `doit_ava`                                       | Query the Ava AI assistant     |

</details>

## Documentation

- **[Terraform Registry](https://registry.terraform.io/providers/doitintl/doit/latest/docs)** — Full schema documentation with examples
- **[Pagination Guide](docs/guides/pagination.md)** — Auto and manual pagination for list data sources
- **[v1.0.0 Upgrade Guide](docs/guides/version_1_upgrade.md)** — Migration instructions from v0.x
- **[Changelog](CHANGELOG.md)** — Release history
- **[DoiT API Reference](https://developer.doit.com/reference)** — Underlying API documentation

---

## Development

### Building the Provider

Requires [Go](https://golang.org/doc/install) >= 1.26.

```shell
git clone https://github.com/doitintl/terraform-provider-doit.git
cd terraform-provider-doit
go install
```

To generate or update documentation:

```shell
make docs
```

### Using Nix Flake (Optional)

If you have [Nix](https://nixos.org/download.html) with flakes enabled:

```shell
nix develop          # or: direnv allow (for automatic activation)
```

This provides Go, Terraform, and golangci-lint at the pinned versions. To skip flake activation with direnv, set `export DIRENV_SKIP_FLAKE=1` in your shell profile.

### Acceptance Tests

Acceptance tests create real resources in a DoiT account. Use the following make targets:

```shell
# Run all acceptance tests
make testacc

# Run specific tests
make testacc-run TEST=TestAccBudget
```

#### Required Environment Variables

Copy the example file and fill in your values:

```shell
cp .envrc.example .envrc.local
# Edit .envrc.local with your values
```

The `make testacc` targets automatically source `.envrc.local`. If you use [direnv](https://direnv.net/), you can also run `direnv allow` to load the variables into your interactive shell.

| Variable           | Description                                    |
| ------------------ | ---------------------------------------------- |
| `DOIT_API_TOKEN`   | Your DoiT API token                            |
| `DOIT_HOST`        | API host (e.g., `https://api.doit.com`)        |
| `TEST_USER`        | Email for test budget collaborators/recipients |
| `TEST_USER_2`      | Second user email for resource sharing tests   |
| `TEST_ATTRIBUTION` | Attribution ID for test budget scope           |
| `TEST_SLACK_CHAN`  | Slack channel ID for notification tests        |
| `TEST_PROJECT`     | Project ID for allocation rule tests           |
| `TEST_CUSTOMER_ID` | Customer ID for Slack channel recipient        |

<details>
<summary>Optional variables (specific data source tests are skipped if unset)</summary>

| Variable                      | Description                                            |
| ----------------------------- | ------------------------------------------------------ |
| `TEST_ATTRIBUTION_GROUP`      | Attribution group ID for report tests                  |
| `TEST_INVOICE_ID`             | Invoice ID for invoice data source tests               |
| `TEST_ANOMALY_ID`             | Anomaly ID for anomaly data source tests               |
| `TEST_CLOUD_INCIDENT_ID`      | Cloud incident ID for cloud incident data source tests |
| `TEST_COMMITMENT_ID`          | Commitment ID for commitment data source tests         |
| `TEST_ASSET_ID`               | Asset ID for asset data source tests (G Suite)         |
| `TEST_ASSET_ID_AWS`           | AWS asset ID for asset data source tests               |
| `TEST_CLOUD_DIAGRAM_RESOURCE` | Cloud resource ID for cloud diagrams data source tests |

</details>

> [!IMPORTANT]
> **DoiT employees only:** You must also set `DOIT_CUSTOMER_CONTEXT` to the same value as `TEST_CUSTOMER_ID`.

> [!WARNING]
> Acceptance tests create real resources and may incur costs.
