# List all commitments (auto-paginated)
data "doit_commitments" "all" {
}

# List commitments with a result limit
data "doit_commitments" "recent" {
  max_results = "10"
}

# Filter commitments by provider
data "doit_commitments" "gcp_only" {
  filter = "provider:google-cloud"
}

output "commitment_count" {
  value = data.doit_commitments.all.row_count
}
