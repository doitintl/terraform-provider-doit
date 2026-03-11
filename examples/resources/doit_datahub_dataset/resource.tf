# Create a DataHub dataset
resource "doit_datahub_dataset" "example" {
  name        = "My Custom Dataset"
  description = "Dataset for tracking custom business metrics"
}

output "dataset_name" {
  value = doit_datahub_dataset.example.name
}

output "dataset_records" {
  value = doit_datahub_dataset.example.records
}
