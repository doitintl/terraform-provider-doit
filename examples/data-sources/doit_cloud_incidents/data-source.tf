# List recent cloud incidents (max_results recommended to avoid long fetch times)
data "doit_cloud_incidents" "recent" {
  max_results = 100
}

# Filter cloud incidents by platform
data "doit_cloud_incidents" "gcp_only" {
  max_results = 50
  filter      = "platform:google-cloud"
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

locals {
  gcp_product_names = [for p in data.doit_products.gcp.products : p.display_name]
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
    known_product = contains(local.gcp_product_names, i.product)
  }]
}
