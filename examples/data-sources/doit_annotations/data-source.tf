# Retrieve all annotations
data "doit_annotations" "all" {}

# With pagination
data "doit_annotations" "paginated" {
  max_results = "10"
}

# Output annotation info
output "total_annotations" {
  value = data.doit_annotations.all.row_count
}

output "annotation_summary" {
  value = [for a in data.doit_annotations.all.annotations : {
    id      = a.id
    content = a.content
  }]
}
