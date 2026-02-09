# Retrieve a label by its ID
data "doit_label" "example" {
  id = "your-label-id"
}

# Output label details
output "label_name" {
  value = data.doit_label.example.name
}

output "label_color" {
  value = data.doit_label.example.color
}

# Use a label in an annotation
resource "doit_annotation" "example" {
  content   = "Cost spike investigation - investigating unusual cost increase"
  timestamp = "2026-02-09T12:00:00Z"
  labels    = [data.doit_label.example.id]
}
