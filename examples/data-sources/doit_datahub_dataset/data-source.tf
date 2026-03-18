# Retrieve a DataHub dataset by its name
data "doit_datahub_dataset" "example" {
  name = "My Custom Dataset"
}

# Output dataset details
output "dataset_description" {
  value = data.doit_datahub_dataset.example.description
}

output "dataset_records" {
  value = data.doit_datahub_dataset.example.records
}

output "dataset_updated_by" {
  value = data.doit_datahub_dataset.example.updated_by
}

output "dataset_last_updated" {
  value = data.doit_datahub_dataset.example.last_updated
}
