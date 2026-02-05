# Retrieve an annotation by its ID
data "doit_annotation" "example" {
  id = "your-annotation-id"
}

# Output annotation details
output "annotation_title" {
  value = data.doit_annotation.example.title
}

output "annotation_content" {
  value = data.doit_annotation.example.content
}

output "annotation_scope" {
  value = data.doit_annotation.example.scope
}

output "annotation_labels" {
  value = data.doit_annotation.example.labels
}
