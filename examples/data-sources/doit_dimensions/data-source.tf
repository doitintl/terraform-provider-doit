# List all available analytics dimensions
data "doit_dimensions" "all" {}

# List dimensions with filtering
data "doit_dimensions" "gcp" {
  filter = "type:google-cloud"
}

# List dimensions with sorting
data "doit_dimensions" "sorted" {
  sort_by    = "id"
  sort_order = "desc"
}

# Output dimension IDs
output "all_dimension_ids" {
  value = [for d in data.doit_dimensions.all.dimensions : d.id]
}

# Output dimensions with labels and types
output "dimensions_detailed" {
  value = [for d in data.doit_dimensions.all.dimensions : {
    id    = d.id
    label = d.label
    type  = d.type
  }]
}

output "dimension_count" {
  value = data.doit_dimensions.all.row_count
}
