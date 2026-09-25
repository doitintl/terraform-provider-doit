terraform {
  required_providers {
    doit = {
      source  = "doitintl/doit"
      version = "~> 1.0"
    }
    local = {
      source  = "hashicorp/local"
      version = "~> 2.5"
    }
  }
}

# List all available widgets in the catalogue
data "doit_widgets" "all" {}

# Output available widget catalogue
output "catalogue" {
  description = "List of all available widgets"
  value = [for w in data.doit_widgets.all.items : {
    id          = w.id
    alias       = w.alias
    name        = w.name
    description = w.description
    type        = w.type
    kind        = w.kind
  }]
}

# Export the widget catalogue to CSV format
resource "local_file" "widget_catalogue_csv" {
  filename = "${path.module}/widget_catalogue.csv"
  content = join("\n", concat(
    ["id,alias,name,description,type,kind"],
    [
      for w in data.doit_widgets.all.items :
      join(",", [
        w.id,
        w.alias,
        "\"${replace(w.name, "\"", "\"\"")}\"",
        "\"${replace(w.description, "\"", "\"\"")}\"",
        w.type,
        w.kind
      ])
    ]
  ))
}
