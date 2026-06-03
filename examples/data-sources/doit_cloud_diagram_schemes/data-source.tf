# Retrieve all accessible diagrams (overview mode — no components).
data "doit_cloud_diagram_schemes" "all" {}

# Output diagram names and their layers.
output "diagrams" {
  value = {
    for key, diagram in data.doit_cloud_diagram_schemes.all.scheme :
    key => {
      name   = diagram.name
      type   = diagram.type
      layers = [for layer in diagram.statussheet : layer.account_name]
    }
  }
}

# Retrieve specific diagrams with full component data.
data "doit_cloud_diagram_schemes" "detailed" {
  scheme_ids = ["diagram-id-1", "diagram-id-2"]
  components = true
  skip_empty = true
}

# Retrieve specific layers only.
data "doit_cloud_diagram_schemes" "layers" {
  layer_ids  = ["layer-id-1"]
  components = true
  node       = true
  link       = true
}
