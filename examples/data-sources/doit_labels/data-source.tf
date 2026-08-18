# Retrieve all labels
data "doit_labels" "all" {}

# Filter labels by name substring
data "doit_labels" "environment" {
  name_contains = "env"
}

# Sort by name
data "doit_labels" "sorted" {
  sort_by    = "name"
  sort_order = "asc"
}

# Output label info
output "total_labels" {
  value = data.doit_labels.all.row_count
}

output "label_names" {
  value = [for l in data.doit_labels.all.labels : l.name]
}
