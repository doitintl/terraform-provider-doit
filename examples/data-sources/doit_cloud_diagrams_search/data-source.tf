# Search for Cloud Diagram layers and components by name
data "doit_cloud_diagrams_search" "example" {
  query = "production"
}

# Access matching diagram layers (includes layer IDs for use with other data sources)
output "matching_layers" {
  value = [for s in data.doit_cloud_diagrams_search.example.scheme : {
    layer_id     = s.ss_id
    diagram_name = s.scheme
    account_name = s.account_name
  }]
}

# Access matching components
output "matching_components" {
  value = [for c in data.doit_cloud_diagrams_search.example.component : {
    name     = c.name
    type     = c.type
    layer_id = c.ss_id
  }]
}
