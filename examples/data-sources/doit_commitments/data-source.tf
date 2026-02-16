# List all commitments (auto-paginated)
data "doit_commitments" "all" {
}

# List commitments with a result limit
data "doit_commitments" "recent" {
  max_results = "10"
}

# Filter commitments by provider
data "doit_commitments" "gcp_only" {
  filter = "provider:[google-cloud]"
}

# Paginate through commitments
data "doit_commitments" "page1" {
  max_results = "5"
}

# Use the page_token from the previous data source to get the next page
data "doit_commitments" "page2" {
  max_results = "5"
  page_token  = data.doit_commitments.page1.page_token
}

output "commitment_count" {
  value = data.doit_commitments.all.row_count
}
