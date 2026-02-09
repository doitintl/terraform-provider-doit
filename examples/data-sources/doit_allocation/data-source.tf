# Retrieve an allocation by its ID
data "doit_allocation" "example" {
  id = "your-allocation-id"
}

# Output allocation details
output "allocation_name" {
  value = data.doit_allocation.example.name
}

output "allocation_type" {
  value = data.doit_allocation.example.allocation_type
}

output "allocation_description" {
  value = data.doit_allocation.example.description
}
