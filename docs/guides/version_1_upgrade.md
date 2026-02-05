---
page_title: "Terraform DoiT Provider 1.0 Upgrade Guide"
description: |-
  Upgrade guide for migrating from DoiT Provider v0.x to v1.0.0
---

# Terraform DoiT Provider 1.0 Upgrade Guide

Version 1.0.0 of the DoiT Provider for Terraform is a major rewrite that includes breaking changes. This guide covers the steps required to upgrade from v0.x to v1.0.0.

## Before You Upgrade

### Backup Your State

~> **Important:** Before upgrading, create a backup of your Terraform state file.

```shell
# For local state
cp terraform.tfstate terraform.tfstate.backup

# For remote state (S3 example)
terraform state pull > terraform.tfstate.backup
```

### Upgrade to the Latest v0.x Version

Before upgrading to v1.0.0, ensure you are on the latest v0.26.x version. Run `terraform plan` and confirm there are no pending changes or errors.

```hcl
terraform {
  required_providers {
    doit = {
      source  = "doitintl/doit"
      version = "~> 0.26"
    }
  }
}
```

After confirming a clean state:

```hcl
terraform {
  required_providers {
    doit = {
      source  = "doitintl/doit"
      version = "~> 1.0"
    }
  }
}
```

Run `terraform init -upgrade` to download the new provider version.

---

## Removed Resources

The following resources have been removed in v1.0.0:

### `doit_allocation_group` (Removed)

**Replacement:** Use `doit_allocation` with the `rules` attribute.

The allocation group functionality has been merged into the `doit_allocation` resource. Instead of creating a separate group resource, you now use the `rules` attribute to define multiple allocations within a single resource.

**Before (v0.x):**

```hcl
resource "doit_allocation" "japan" {
  name = "Japan in K8s project"
  rule = {
    formula = "A AND B"
    components = [
      { key = "country", mode = "is", type = "fixed", values = ["JP"] },
      { key = "project_id", mode = "is", type = "fixed", values = ["my-k8s-project"] }
    ]
  }
}

resource "doit_allocation" "germany" {
  name = "Germany in K8s project"
  rule = {
    formula = "A AND B"
    components = [
      { key = "country", mode = "is", type = "fixed", values = ["DE"] },
      { key = "project_id", mode = "is", type = "fixed", values = ["my-k8s-project"] }
    ]
  }
}

resource "doit_allocation_group" "k8s_regions" {
  name = "K8s Regional Allocations"
  rules = [
    { action = "select", id = doit_allocation.japan.id },
    { action = "select", id = doit_allocation.germany.id },
  ]
}
```

**After (v1.0.0):**

```hcl
resource "doit_allocation" "k8s_regions" {
  name              = "K8s Regional Allocations"
  description       = "Regional allocations for K8s project"
  unallocated_costs = "Other Regions"
  rules = [
    {
      action  = "select"
      id      = "existing-japan-allocation-id"
    },
    {
      action  = "select"
      id      = "existing-germany-allocation-id"
    },
  ]
}
```

**Migration Steps:**

1. Note the allocation group ID from the DoiT Console or your state:
   ```shell
   terraform state show doit_allocation_group.k8s_regions | grep id
   # Example output: id = "XWK9KCUoFNUO2UxfDCPK"
   ```
2. Check the current `unallocated_costs` value (if set) to match it in your new config
3. Remove `doit_allocation_group` resources from your state:
   ```shell
   terraform state rm doit_allocation_group.k8s_regions
   ```
4. Update your configuration to use `doit_allocation` with `rules`
5. Import the existing allocation group using the same ID:
   ```shell
   terraform import doit_allocation.k8s_regions XWK9KCUoFNUO2UxfDCPK
   ```
6. Run `terraform plan` to verify. Small diffs like `unallocated_costs` are expected if you set a new value.

-> **Note:** The backend uses the same resource type for both allocation groups and allocations with rules, so the import preserves the original resource ID.

### `doit_attribution` and `doit_attribution_group` (Removed)

These resources are no longer supported in the Terraform provider. Existing Attributions can be managed through the DoiT Console. You can use Allocations instead, which support the same functionality and more.

**Migration Steps:**

1. Export your current attribution configurations for reference
2. Remove these resources from your Terraform state:
   ```shell
   terraform state rm doit_attribution.my_attribution
   terraform state rm doit_attribution_group.my_group
   ```
3. Remove the resource blocks from your configuration

---

## Resource: `doit_budget`

### Deprecated Attributes

The following attributes are deprecated and will be removed in v2.0.0:

| Attribute | Replacement | Notes |
|-----------|-------------|-------|
| `scope` | `scopes` | The new `scopes` attribute provides more flexibility with filter modes |

**Before (v0.x):**

```hcl
resource "doit_budget" "my_budget" {
  name         = "My Budget"
  scope        = [doit_attribution.my_attr.id]
  # ...
}
```

**After (v1.0.0):**

```hcl
resource "doit_budget" "my_budget" {
  name = "My Budget"
  scopes = [
    {
      type   = "attribution"
      id     = "attribution"
      mode   = "is"
      values = ["your-attribution-id"]
    }
  ]
  # ...
}
```

### Schema Changes

| Attribute | v0.x | v1.0.0 |
|-----------|------|--------|
| `name` | Required | Optional |
| `currency` | Required | Optional |
| `type` | Required | Optional |
| `start_period` | Required | Optional |
| `collaborators` | Required | Optional |
| `recipients` | Required | Optional |
| `last_updated` | Computed | **Removed** (use `update_time`) |

### New Attributes

- `scopes` - List of scope filters with `type`, `id`, `mode`, and `values`
- `seasonal_amounts` - List of seasonal amounts for varying budget amounts per period
- `create_time` - Computed creation timestamp
- `update_time` - Computed last update timestamp
- `current_utilization` - Computed current budget utilization
- `forecasted_utilization` - Computed forecasted utilization

### Nested Schema Changes

**`alerts`:**
- New computed fields: `forecasted_date`, `triggered`

### State Migration

The provider includes an automatic state upgrader that migrates budget resources from the v0 schema to v1. New computed fields will be populated on the next `terraform refresh` or `terraform apply`.

No manual state manipulation is required.

---

## Resource: `doit_allocation`

### Schema Changes

| Attribute | v0.x | v1.0.0 |
|-----------|------|--------|
| `description` | Optional | **Required** |
| `rule` | Required | Optional (use `rule` for single allocations) |

### New Attributes

- `rules` - List of rules for group-type allocations (replaces `doit_allocation_group`)
- `unallocated_costs` - Label for costs that don't fit into the allocation (required for groups)
- `allocation_type` - Computed: "single" or "group"
- `create_time` - Computed creation timestamp
- `update_time` - Computed last update timestamp

---

## Resource: `doit_report`

### Schema Changes

Several nested attributes have changed from Required to Optional to improve flexibility. The most notable change is:

**`config.filters`:**
- New required attribute: `mode` (possible values: `is`, `starts_with`, `ends_with`, `contains`, `regexp`)

**Before (v0.x):**
```hcl
filters = [
  {
    id     = "project_id"
    type   = "fixed"
    values = ["my-project"]
  }
]
```

**After (v1.0.0):**
```hcl
filters = [
  {
    id     = "project_id"
    type   = "fixed"
    mode   = "is"
    values = ["my-project"]
  }
]
```

### New Attributes

- `config.custom_time_range` - For custom time range queries with `from` and `to` RFC3339 timestamps

### API Requirements

The report API may require `data_source` to be explicitly set. If you encounter a `dataSource: invalid datasource value` error, add:

```hcl
config = {
  data_source = "billing"  # or other valid data source
  # ... rest of config
}
```

---

## Common Migration Issues

### Report Filters Missing `mode`

If you see an error like `attribute "mode" is required`, add `mode = "is"` (or another valid mode) to each filter:

```diff
 filters = [
   {
     id     = "project_id"
     type   = "fixed"
+    mode   = "is"
     values = ["my-project"]
   }
 ]
```

### Allocation Description Now Required

If you see `attribute "description" is required`, add a description to each allocation:

```diff
 resource "doit_allocation" "my_allocation" {
   name        = "My Allocation"
+  description = "Description of this allocation"
   # ...
 }
```

## Upgrade Checklist

- [ ] Backup Terraform state
- [ ] Upgrade to latest v0.26.x and confirm clean state
- [ ] Remove `doit_allocation_group` resources (migrate to `doit_allocation` with `rules`)
- [ ] Remove `doit_attribution` and `doit_attribution_group` resources
- [ ] Update `doit_budget` resources:
  - [ ] Replace `scope` with `scopes`
  - [ ] Replace `last_updated` references with `update_time`
- [ ] Update `doit_allocation` resources:
  - [ ] Ensure `description` is set (now required)
- [ ] Update `doit_report` resources:
  - [ ] Add `mode` to all filter configurations
- [ ] Run `terraform init -upgrade`
- [ ] Run `terraform plan` to verify changes
- [ ] Run `terraform apply` to complete migration
