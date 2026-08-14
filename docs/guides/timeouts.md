---
page_title: "Timeouts Guide"
description: |-
  How to configure request and operation timeouts in the DoiT provider.
---

# Timeouts Guide

The DoiT provider supports configurable timeouts at two levels: a **global provider-level** HTTP request timeout and **per-resource/data-source operation** timeouts. This guide explains how to use both and how they interact.

## Provider-Level Request Timeout

The `request_timeout` attribute controls the timeout for each individual HTTP request to the DoiT API. It defaults to `150s` and can be set via the provider block or the `DOIT_REQUEST_TIMEOUT` environment variable.

```hcl
provider "doit" {
  request_timeout = "150s"  # Default value, shown for documentation
}
```

Or using the environment variable:

```shell
export DOIT_REQUEST_TIMEOUT="180s"
```

The value must be a Go duration string (e.g., `"150s"`, `"4m"`) greater than `120s`. The HCL attribute takes precedence over the environment variable when both are set.

-> This timeout applies to each individual HTTP request, including retries. If a single API call takes longer than this value, it will be cancelled and retried.

~> **Values at or below `120s` are rejected.** The DoiT API sits behind an edge proxy that answers requests still running after 120 seconds with a `524` status. The provider treats that response as a definitive failure and does not retry it. But if `request_timeout` is at or below 120 seconds, the request is cancelled locally *before* that response can arrive — and a local cancellation is indistinguishable from a network fault, so it *is* retried. The result is that a slow call fails with an opaque `context deadline exceeded` after repeatedly re-running an expensive query, instead of reporting the `524` immediately.

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
    read   = "5m"   # Default
    delete = "5m"   # Default
  }
}
```

### Default Resource Timeouts

| Operation | Default |
| --------- | ------- |
| Create    | 5 min   |
| Read      | 5 min   |
| Update    | 5 min   |
| Delete    | 5 min   |

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

The default read timeout for data sources is **5 minutes**.

## How Timeouts Interact

The provider has two timeout layers that work together:

1. **Request timeout** (provider-level `request_timeout`): Controls each individual HTTP request. If a single API call exceeds this duration, it is cancelled and retried.
2. **Operation timeout** (resource/data-source `timeouts {}`): Controls the entire Terraform operation, including all HTTP requests and retries.

The **operation timeout is the outer boundary**. Retries continue until the operation timeout's context deadline is reached, regardless of the `request_timeout` value.

| Layer | Controls | Example |
| ----- | -------- | ------- |
| **Operation timeout** (outer) | Total time for the Terraform operation, including all retries | `timeouts = { create = "10m" }` |
| **Request timeout** (inner) | Time for a single HTTP request to the API | `request_timeout = "150s"` |

Within a single operation (e.g., `create`), the flow is:

1. **Request 1**: The provider sends an HTTP request. If the API doesn't respond within `request_timeout` (e.g., 150s), the request is cancelled.
2. **Backoff**: The provider waits with exponential backoff before retrying. Backoff starts at 2 seconds and doubles, up to 60 seconds, with jitter.
3. **Request 2**: A retry is attempted. This cycle repeats.
4. **Deadline**: Once the operation timeout (e.g., 10m) is reached, all remaining retries are cancelled and Terraform reports the error.

### Keep the layers ordered

The defaults satisfy `operation timeout > request timeout > 120s`, and both bounds matter:

- **Operation timeout above request timeout.** Otherwise a single slow request consumes the entire operation budget with no room for retries. For example, `request_timeout = "300s"` with `timeouts = { create = "2m" }` means the first request could still be running when the operation deadline cancels it — leaving zero time for a retry.
- **Request timeout above 120s.** See the note under [Provider-Level Request Timeout](#provider-level-request-timeout): below that threshold the API's own `524` response cannot reach the provider, and slow calls degrade into retried, opaque deadline errors.

If you raise `request_timeout`, raise the `timeouts {}` blocks on the affected resources and data sources to match. The provider emits a warning when `request_timeout` is not below the default operation timeout, since it cannot see the per-resource blocks — they are resolved later, per operation.

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

### `status: 524`

A `524` comes from the DoiT API's edge proxy and means the API did not finish the request within its own 120-second limit. The provider reports it immediately and **does not retry** — the query already ran for the full edge timeout, so an identical retry would only re-run expensive work that is going to time out again.

Raising `request_timeout` will not help, because the limit is enforced upstream rather than locally. Reduce the amount of work in the request instead — for example, narrow a report's time range, or add filters to reduce the number of rows.

Seeing `context deadline exceeded` instead of `524` on a request you expect to be slow is a sign that `request_timeout` is at or below 120 seconds.

### Large Allocations

Allocation groups with many rules (100+) may require larger timeouts because the API processes all rules in a single request. Recommended settings:

```hcl
provider "doit" {
  request_timeout = "290s"  # just under 5 minutes per request
}

resource "doit_allocation" "large_group" {
  # ... many rules ...

  timeouts = {
    create = "15m"
    update = "15m"
  }
}
```

-> `request_timeout` is deliberately just under the 5-minute default operation timeout here. Raising it to `300s` or beyond triggers the ordering warning described above, because it would leave a default-timeout operation no room to retry.
