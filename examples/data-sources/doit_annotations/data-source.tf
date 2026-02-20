# Retrieve all annotations
data "doit_annotations" "all" {}

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
