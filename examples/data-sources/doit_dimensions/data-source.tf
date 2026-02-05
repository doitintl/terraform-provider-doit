# List all available analytics dimensions
data "doit_dimensions" "all" {}

# List dimensions with filtering
data "doit_dimensions" "gcp" {
  filter = "type:google-cloud"
}

# Output all dimension keys
output "all_dimension_keys" {
  value = [for d in data.doit_dimensions.all.dimensions : d.get_id]
}

output "dimension_count" {
  value = data.doit_dimensions.all.row_count
}
