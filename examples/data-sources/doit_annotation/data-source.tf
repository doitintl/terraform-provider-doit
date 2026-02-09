# Retrieve an annotation by its ID
data "doit_annotation" "example" {
  id = "your-annotation-id"
}

# Output annotation details
output "annotation_content" {
  value = data.doit_annotation.example.content
}

output "annotation_timestamp" {
  value = data.doit_annotation.example.timestamp
}

output "annotation_labels" {
  value = data.doit_annotation.example.labels
}

output "annotation_reports" {
  value = data.doit_annotation.example.reports
}
