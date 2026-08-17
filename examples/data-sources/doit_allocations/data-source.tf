# Retrieve all allocations
data "doit_allocations" "all" {}

# Filter by type
data "doit_allocations" "groups" {
  filter = "allocationType:[group]"
}

# Filter by name substring
data "doit_allocations" "production" {
  name_contains = "production"
}

# Output allocation names
output "total_allocations" {
  value = data.doit_allocations.all.row_count
}

output "allocation_names" {
  value = [for a in data.doit_allocations.all.allocations : a.name]
}
