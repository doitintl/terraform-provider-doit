# List all assets
data "doit_assets" "all" {
}

# Filter assets by type
data "doit_assets" "gsuite" {
  filter = "type:g-suite"
}

output "asset_count" {
  value = data.doit_assets.all.row_count
}
