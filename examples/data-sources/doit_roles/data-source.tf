# List all IAM roles in the organization
data "doit_roles" "all" {}

# Output role names
output "role_names" {
  value = [for r in data.doit_roles.all.roles : r.name]
}

# Find a specific role by name
output "admin_role" {
  value = [for r in data.doit_roles.all.roles : r if r.name == "Admin"][0]
}
