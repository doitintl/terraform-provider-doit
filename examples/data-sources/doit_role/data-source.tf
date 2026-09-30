# Create a custom role
resource "doit_role" "example" {
  name        = "Example Role"
  description = "Example role for data source lookup"
}

# Look up the role by ID using the data source
data "doit_role" "example" {
  id = doit_role.example.id
}

output "role_name" {
  value = data.doit_role.example.name
}

output "role_type" {
  value = data.doit_role.example.type
}

output "role_permissions" {
  value = data.doit_role.example.permissions
}
