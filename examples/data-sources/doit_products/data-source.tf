# List all available products across all platforms
data "doit_products" "all" {}

# Output product names
output "product_names" {
  value = [for p in data.doit_products.all.products : p.display_name]
}

# List products for a specific platform
data "doit_products" "google_cloud" {
  platform = "google_cloud_platform"
}

# Output Google Cloud product names
output "google_cloud_products" {
  value = [for p in data.doit_products.google_cloud.products : p.display_name]
}
