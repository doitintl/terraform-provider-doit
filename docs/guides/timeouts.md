---
page_title: "Timeouts Guide"
description: |-
  How to configure request and operation timeouts in the DoiT provider.
---

# Timeouts Guide

The DoiT provider supports configurable timeouts at two levels: a **global provider-level** HTTP request timeout and **per-resource/data-source operation** timeouts. This guide explains how to use both and how they interact.

## Provider-Level Request Timeout

The `request_timeout` attribute controls the timeout for each individual HTTP request to the DoiT API. It defaults to `120s` and can be set via the provider block or the `DOIT_REQUEST_TIMEOUT` environment variable.

```hcl
provider "doit" {
  request_timeout = "120s"  # Default value, shown for documentation
}
```

Or using the environment variable:

```shell
export DOIT_REQUEST_TIMEOUT="180s"
```

The value must be a positive Go duration string (e.g., `"30s"`, `"2m"`, `"1h"`). The HCL attribute takes precedence over the environment variable when both are set.

-> This timeout applies to each individual HTTP request, including retries. If a single API call takes longer than this value, it will be cancelled and retried.

## Resource Timeouts

All DoiT resources support user-configurable timeouts for each CRUD operation via the `timeouts` attribute. These control how long Terraform will wait for the entire operation to complete, including any retries.

```hcl
resource "doit_allocation" "large_group" {
  name = "Large Allocation Group"
  type = "group"

  # ... rules and components ...

  timeouts = {
    create = "10m"  # Large allocations with many rules may need more time
    update = "10m"
    read   = "2m"   # Default
    delete = "2m"   # Default
  }
}
```

### Default Resource Timeouts

| Operation | Default |
| --------- | ------- |
| Create    | 5 min   |
| Read      | 2 min   |
| Update    | 5 min   |
| Delete    | 2 min   |

Only specify `timeouts` when you need to override the defaults — Terraform won't show any diff for resources without this attribute.

## Data Source Timeouts

All DoiT data sources support a configurable read timeout:

```hcl
data "doit_report_query" "heavy_report" {
  # ... config ...

  timeouts = {
    read = "5m"  # Complex reports may take longer to query
  }
}
```

The default read timeout for data sources is **2 minutes**.

## How Timeouts Interact

The provider has two timeout layers that work together:

1. **Request timeout** (provider-level `request_timeout`): Controls each individual HTTP request. If a single API call exceeds this duration, it is cancelled and retried.
2. **Operation timeout** (resource/data-source `timeouts {}`): Controls the entire Terraform operation, including all HTTP requests and retries.

The **operation timeout is the outer boundary**. Retries continue until the operation timeout's context deadline is reached, regardless of the `request_timeout` value.

| Layer | Controls | Example |
| ----- | -------- | ------- |
| **Operation timeout** (outer) | Total time for the Terraform operation, including all retries | `timeouts = { create = "10m" }` |
| **Request timeout** (inner) | Time for a single HTTP request to the API | `request_timeout = "120s"` |

Within a single operation (e.g., `create`), the flow is:

1. **Request 1**: The provider sends an HTTP request. If the API doesn't respond within `request_timeout` (e.g., 120s), the request is cancelled.
2. **Backoff**: The provider waits with exponential backoff before retrying.
3. **Request 2**: A retry is attempted. This cycle repeats.
4. **Deadline**: Once the operation timeout (e.g., 10m) is reached, all remaining retries are cancelled and Terraform reports the error.

~> **Important:** Ensure that your operation timeout is larger than your `request_timeout`. If `request_timeout` is larger than the operation timeout, a single slow request could consume the entire operation budget with no room for retries. For example, `request_timeout = "300s"` with `timeouts = { create = "2m" }` means the first request could take up to 2 minutes before being cancelled by the operation timeout — leaving zero time for retries.

## Troubleshooting

### `context deadline exceeded`

This error means a timeout was exceeded. Check which level caused it:

- **`Client.Timeout exceeded while awaiting headers`** — The per-request timeout was hit. Increase `request_timeout`:

  ```hcl
  provider "doit" {
    request_timeout = "300s"
  }
  ```

- **`context deadline exceeded` (without "Client.Timeout")** — The operation timeout was hit. Increase the relevant operation timeout:
  ```hcl
  resource "doit_allocation" "example" {
    # ...
    timeouts = {
      create = "15m"
    }
  }
  ```

### Large Allocations

Allocation groups with many rules (100+) may require larger timeouts because the API processes all rules in a single request. Recommended settings:

```hcl
provider "doit" {
  request_timeout = "300s"  # 5 minutes per request
}

resource "doit_allocation" "large_group" {
  # ... many rules ...

  timeouts = {
    create = "15m"
    update = "15m"
  }
}
```
