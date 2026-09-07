# Create a DataHub dataset
resource "doit_datahub_dataset" "example" {
  name         = "My Custom Dataset"
  description  = "Dataset for tracking custom business metrics"
  display_name = "Business Metrics"
  logo_name    = "aws"
}

output "dataset_name" {
  value = doit_datahub_dataset.example.name
}

output "dataset_display_name" {
  value = doit_datahub_dataset.example.display_name
}

output "dataset_logo_name" {
  value = doit_datahub_dataset.example.logo_name
}

output "dataset_records" {
  value = doit_datahub_dataset.example.records
}
