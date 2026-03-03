# Retrieve an asset by its ID
data "doit_asset" "example" {
  id = "your-asset-id"
}

# Output asset details
output "asset_name" {
  value = data.doit_asset.example.name
}

output "asset_type" {
  value = data.doit_asset.example.type
}

output "asset_url" {
  value = data.doit_asset.example.url
}
