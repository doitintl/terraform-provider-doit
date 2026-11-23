---
page_title: "Importing Existing Resources"
description: |-
  How to bring budgets, allocations, alerts, reports and other objects that already exist in DoiT under Terraform management.
---

# Importing Existing Resources

Much of a DoiT setup is often created in the console or through the API before anyone writes Terraform for it. You don't have to recreate it. Terraform can **import** an existing object into its state and write the matching configuration for you. Importing doesn't change anything in DoiT. This guide shows how to do that for one object, for many objects at once, and how to tidy the result afterwards.

This guide needs Terraform 1.7 or later: `import` blocks and `-generate-config-out` came with 1.5, and the `removed` block used at the end with 1.7.

## Import One Object

Add an `import` block with the address you want and the ID of the object:

```hcl
import {
  to = doit_report.monthly_costs
  id = "REPORT_ID"
}
```

Let Terraform write the configuration into a file that doesn't exist yet:

```shell
terraform plan -generate-config-out=generated.tf
```

Read `generated.tf`, move the resource into your configuration, and apply. Applying an import only writes Terraform state. It doesn't change the object in DoiT, as long as the plan shows no changes for it (see [When the Plan Shows Errors or Changes](#when-the-plan-shows-errors-or-changes)):

```shell
terraform apply
terraform plan   # No changes. Your infrastructure matches the configuration.
```

For objects that follow the current rules, the generated configuration is valid as it is, and the provider's acceptance tests check that it plans without changes. It also contains a few attributes that only repeat a default, such as `timeouts = null` or empty lists. You can delete them. Remove the `import` block once the import has been applied.

-> The `terraform import` command works as well and takes the same IDs, but it doesn't write any configuration for you.

### Import IDs

Most resources are imported by their ID. These are the exceptions:

| Resource | Import ID |
| --- | --- |
| `doit_user` | The user's email address |
| `doit_user_notification` | `user_id/notification_id` |
| `doit_service_account_token` | `service_account_id/token_id` |
| `doit_sharing` | `resource_type/resource_id`, for example `reports/abc123def456` |
| `doit_customer` | The customer ID |
| `doit_active_theme` | `active-theme` |
| `doit_cloudconnect_aws_account` | The AWS account ID |
| `doit_datahub_dataset` | The dataset name |
| `doit_label_assignments` | The ID of the label |
| `doit_support_request_tags` | The numeric ticket ID |

Each resource's documentation shows its import syntax at the bottom.

## Adopt Many Objects at Once

A tenant can hold hundreds of budgets, allocations and reports, and writing an `import` block for each one by hand isn't practical. An `import` block can use `for_each` (Terraform 1.7), but Terraform can't generate configuration for it: `-generate-config-out` fails with `The target resource does not use for_each`. So the work is split in two:

1. A small **discovery** configuration reads the objects with the list data sources and prints plain `import` blocks.
2. Your real configuration imports them and lets Terraform write the resources.

### Step 1: Discover

Put this in its own directory. It only reads from DoiT and creates nothing:

```hcl
terraform {
  required_providers {
    doit = {
      source = "doitintl/doit"
    }
  }
}

provider "doit" {}

variable "name_contains" {
  type        = string
  default     = null
  description = "Only adopt objects whose name contains this text (case-insensitive). Null adopts everything."
}

data "doit_allocations" "all" { name_contains = var.name_contains }
data "doit_budgets" "all" { name_contains = var.name_contains }
data "doit_alerts" "all" { name_contains = var.name_contains }
data "doit_reports" "all" { name_contains = var.name_contains }

locals {
  # Preset allocations and reports belong to DoiT, so only custom ones are
  # adopted.
  objects = concat(
    [for a in data.doit_allocations.all.allocations : { type = "doit_allocation", name = a.name, id = a.id } if a.type == "custom"],
    [for b in data.doit_budgets.all.budgets : { type = "doit_budget", name = b.budget_name, id = b.id }],
    [for a in data.doit_alerts.all.alerts : { type = "doit_alert", name = a.name, id = a.id }],
    [for r in data.doit_reports.all.reports : { type = "doit_report", name = r.report_name, id = r.id } if r.type == "custom"],
  )

  # Names aren't unique, so every address ends in the object's ID. The type
  # prefix keeps it a valid identifier even when a name starts with a digit.
  import_blocks = [
    for o in local.objects :
    "import {\n  to = ${o.type}.${trimprefix(o.type, "doit_")}_${substr(replace(lower(o.name), "/[^a-z0-9]+/", "_"), 0, 40)}_${o.id}\n  id = \"${o.id}\"\n}\n"
  ]
}

output "import_blocks" {
  value = join("\n", local.import_blocks)
}
```

Adopt one team or project at a time by filtering on the name:

```shell
terraform init
terraform apply -var name_contains=payments
terraform output -raw import_blocks > ../adopt/imports.tf
```

`terraform apply` is needed here because outputs are only stored in state by an apply. Without the variable, the output lists every object, which is rarely what you want for a first import (see below).

### Step 2: Import

In the directory of your real configuration (a `terraform` block that requires the `doit` provider, and an empty `provider "doit" {}` block, are enough), import the objects and let Terraform write them:

```shell
terraform plan -generate-config-out=generated.tf
terraform apply
terraform plan   # No changes. Your infrastructure matches the configuration.
```

The generated resource addresses are long (`doit_budget.budget_payments_team_1a2B3c4D5e6F7g8H9i0J`). That is fine for a first import and easy to fix afterwards.

## When the Plan Shows Errors or Changes

The generated configuration goes through the same validation as configuration you write by hand. Objects that were created long ago, in the console, or before a rule existed can break today's rules, and one invalid resource fails the whole plan. The error names the resource. On a large test tenant, the errors were, for example:

- a group allocation without `unallocated_costs` (`rules` and `unallocated_costs` must be configured together)
- a budget without `scopes`, with a `start_period` of `0`, with empty `recipients`, or with two owners
- a report with `forecast_settings` while forecasting is switched off

Either leave the object where it is, or fix the generated resource. A fix is an edit of the object, so the next apply sends it to DoiT.

To leave an object out, delete its `import` block **and** its generated resource. With only the `import` block gone, the resource stays in the configuration, and Terraform plans to create a second copy of the object. If you haven't edited `generated.tf` yet, it's easier to delete the `import` block and `generated.tf` and run `terraform plan -generate-config-out=generated.tf` again, because Terraform doesn't overwrite an existing file.

This is also why you should adopt in batches: a batch of one team fails for one reason at a time, and a whole tenant fails for dozens.

The plan can also show `to change` for an imported object. On the same tenant, a few reports planned an in-place update that only filled in computed values, such as fields of `display_settings`. Read the diff of every such object before you apply, because an apply performs the update as well as the import.

## Tidy Up with `moved` Blocks

Generated configuration has one resource per object. If your objects follow a pattern, such as a budget and an alert for every team, you can collapse them into a `for_each` over a map, and a `moved` block tells Terraform that the new address is the same object. No object is recreated and nothing is sent to DoiT:

```hcl
locals {
  teams = {
    gcp = { cloud = "google-cloud" }
    aws = { cloud = "amazon-web-services" }
  }
}

resource "doit_allocation" "team" {
  for_each    = local.teams
  name        = "Team ${each.key}"
  description = "All ${each.value.cloud} costs"
  rule = {
    formula = "A"
    components = [{
      key    = "cloud_provider"
      mode   = "is"
      type   = "fixed"
      values = [each.value.cloud]
    }]
  }
}

moved {
  from = doit_allocation.allocation_team_gcp_gcpAllocId1
  to   = doit_allocation.team["gcp"]
}

moved {
  from = doit_allocation.allocation_team_aws_awsAllocId2
  to   = doit_allocation.team["aws"]
}
```

`terraform plan` shows `has moved to doit_allocation.team["gcp"]` for each block. If the plan also shows an update, the new configuration doesn't match the object yet: compare it with the generated resource and adjust it until the plan only reports the moves.

## Things to Know

- **Group allocations.** A group's members are ordinary single allocations in DoiT, however each one was created. The API doesn't return how a member was created, so an imported group always comes back with `action = "select"` rules that point at its members by ID. Import the member allocations as their own `doit_allocation` resources as well (the discovery configuration above includes every custom allocation), and then replace the literal IDs in the group's `rules` with references such as `doit_allocation.team["gcp"].id`.
- **Preset objects.** Preset allocations and reports belong to DoiT, so the discovery configuration only lists the custom ones.
- **Everything that exists gets listed.** Objects that nobody wrote by hand appear in the list data sources too. For example, `doit_customer_notifications` also returns the notifications for system events that the console set up. Decide for each type whether you want it in Terraform before you import it.
- **Secrets stay in DoiT.** An imported `doit_service_account_token` has no `access_token`: the API returns the secret only when the token is created, so it stays `null` in the state. Import adopts the token's lifecycle (name, expiry, state), but a pipeline that needs the secret needs a new token.
- **Sharing is a resource of its own.** The permissions of a report, budget or allocation belong to a `doit_sharing` resource for that object, which you import with `resource_type/resource_id`.

## Stop Managing an Object

If you imported something by mistake, or want to hand an object back to the console, remove it from Terraform without deleting it in DoiT. Replace its `resource` block with a `removed` block:

```hcl
removed {
  from = doit_budget.budget_payments_team_1a2B3c4D5e6F7g8H9i0J

  lifecycle {
    destroy = false
  }
}
```

`terraform apply` removes it from the state and leaves the budget in DoiT as it is. Delete the object's `import` block first if it is still there: an `import` block for an object that has no resource in the configuration is an error.
