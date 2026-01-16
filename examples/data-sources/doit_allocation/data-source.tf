# Retrieve an allocation by its ID
data "doit_allocation" "example" {
  id = doit_allocation.my_allocation.id
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

# Use allocation data in another resource
resource "doit_report" "allocation_report" {
  name = "Costs by ${data.doit_allocation.example.name}"
  config = {
    metric     = "cost"
    time_range = { mode = "last", amount = 30, unit = "day" }
  }
}
