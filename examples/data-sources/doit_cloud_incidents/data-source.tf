# List recent cloud incidents (max_results recommended to avoid long fetch times)
data "doit_cloud_incidents" "recent" {
  max_results = 100
}

# Filter cloud incidents by platform
data "doit_cloud_incidents" "gcp_only" {
  max_results = 50
  filter      = "platform:google-cloud"
}

# Paginate through cloud incidents
data "doit_cloud_incidents" "page1" {
  max_results = 25
}

# Use the page_token from the previous data source to get the next page
data "doit_cloud_incidents" "page2" {
  max_results = 25
  page_token  = data.doit_cloud_incidents.page1.page_token
}

output "incident_count" {
  value = data.doit_cloud_incidents.recent.row_count
}

# ─────────────────────────────────────────────────────────────────────────────
# Combining data sources: filter incidents and enrich with platform info
# ─────────────────────────────────────────────────────────────────────────────

# Use doit_platforms to discover valid platform identifiers for filtering
data "doit_platforms" "all" {}

output "available_platforms_for_incidents" {
  description = "Platform IDs that can be used in cloud incident filters (e.g., filter = \"platform:<id>\")"
  value       = [for p in data.doit_platforms.all.platforms : { id = p.id, name = p.display_name }]
}

# Use doit_products to cross-reference incident products with known services
data "doit_products" "gcp" {
  platform = "google_cloud_platform"
}

# Group GCP incidents by product and show which ones match known products
output "gcp_incidents_by_product" {
  description = "GCP incidents enriched with product matching"
  value = [for i in data.doit_cloud_incidents.gcp_only.incidents : {
    id      = i.id
    title   = i.title
    product = i.product
    status  = i.status
    # Check if this incident's product matches a known GCP product
    known_product = contains([for p in data.doit_products.gcp.products : p.display_name], i.product)
  }]
}
