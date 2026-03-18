# Step 1: Use the doit_assets data source to discover your asset IDs.
# Apply this configuration first to find the asset you want to manage.
data "doit_assets" "gsuite" {
  filter = "type:g-suite"
}

output "gsuite_assets" {
  value = [for a in data.doit_assets.gsuite.assets : {
    id   = a.id
    name = a.name
    type = a.type
  }]
}

# Step 2: Use the doit_asset data source to inspect a specific asset's
# properties before importing it.
data "doit_asset" "example" {
  id = data.doit_assets.gsuite.assets[0].id
}

output "asset_details" {
  value = data.doit_asset.example
}

# Step 3: Add the resource block and import the asset into Terraform state:
#
#   terraform import doit_asset.licenses <asset-id>
#
# Then run `terraform apply` to manage the asset's license quantity.
resource "doit_asset" "licenses" {
  id       = data.doit_assets.gsuite.assets[0].id
  quantity = 10
}
