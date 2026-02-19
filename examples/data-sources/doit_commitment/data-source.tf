# Retrieve a commitment by its ID
data "doit_commitment" "example" {
  id = "your-commitment-id"
}

# Output commitment details
output "commitment_name" {
  value = data.doit_commitment.example.name
}

output "commitment_provider" {
  value = data.doit_commitment.example.cloud_provider
}

output "commitment_start_date" {
  value = data.doit_commitment.example.start_date
}

output "commitment_end_date" {
  value = data.doit_commitment.example.end_date
}

output "commitment_total_value" {
  value = data.doit_commitment.example.total_commitment_value
}
