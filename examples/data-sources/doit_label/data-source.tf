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
  title   = "Cost spike investigation"
  content = "Investigating unusual cost increase"
  scope   = "global"
  labels  = [data.doit_label.example.id]
}
