# List all DataHub datasets
data "doit_datahub_datasets" "all" {}

output "dataset_count" {
  value = length(data.doit_datahub_datasets.all.datasets)
}

output "dataset_names" {
  value = [for ds in data.doit_datahub_datasets.all.datasets : ds.name]
}

# ─────────────────────────────────────────────────────────────────────────────
# Look up a specific dataset from the list and output its details
# ─────────────────────────────────────────────────────────────────────────────

locals {
  datasets_by_name = {
    for ds in data.doit_datahub_datasets.all.datasets : ds.name => ds
  }
}

output "dataset_details" {
  description = "All datasets with their record counts"
  value = [for ds in data.doit_datahub_datasets.all.datasets : {
    name        = ds.name
    description = ds.description
    records     = ds.records
  }]
}
