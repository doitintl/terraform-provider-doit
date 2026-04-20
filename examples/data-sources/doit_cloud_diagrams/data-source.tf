# Look up Cloud Diagrams containing a specific BigQuery dataset
data "doit_cloud_diagrams" "example" {
  resources = [
    "//bigquery.googleapis.com/projects/my-project/datasets/my_dataset",
  ]
}

# Access the diagram URLs
output "diagram_urls" {
  value = [for d in data.doit_cloud_diagrams.example.cloud_diagrams : d.diagram_url]
}

output "image_urls" {
  value = [for d in data.doit_cloud_diagrams.example.cloud_diagrams : d.image_url]
}
