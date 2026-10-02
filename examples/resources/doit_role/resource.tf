# Create a role without permissions
resource "doit_role" "placeholder" {
  name = "Placeholder"
}

# Create a role with a description and permissions.
# Permission IDs are opaque; look them up from an existing preset role.
data "doit_roles" "all" {}

locals {
  view_only_permissions = one([
    for r in data.doit_roles.all.roles : r.permissions
    if r.type == "preset" && r.name == "View Only"
  ])
}

resource "doit_role" "finops_analyst" {
  name        = "FinOps Analyst"
  description = "Read-only access to the DoiT Console"
  permissions = local.view_only_permissions
}

# Create a role that combines the permissions of two preset roles
locals {
  preset_permissions = {
    for r in data.doit_roles.all.roles : r.name => r.permissions if r.type == "preset"
  }
}

resource "doit_role" "finops_reviewer" {
  name        = "FinOps Reviewer"
  description = "View Only plus everything a Finance User can see"
  permissions = sort(distinct(concat(
    local.preset_permissions["View Only"],
    local.preset_permissions["Finance User"],
  )))
}
