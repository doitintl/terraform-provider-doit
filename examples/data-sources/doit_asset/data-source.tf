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

# Vendor support plan (AWS and Google Cloud assets only; null when unknown)
output "asset_support_model" {
  value = data.doit_asset.example.support.model
}

output "asset_support_tier" {
  value = data.doit_asset.example.support.tier
}

output "asset_url" {
  value = data.doit_asset.example.url
}
