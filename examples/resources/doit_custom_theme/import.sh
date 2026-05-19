terraform {
  required_providers {
    doit = {
      source = "registry.terraform.io/doitintl/doit"
    }
  }
}

provider "doit" {}

# Import an existing custom theme by ID
import {
  to = doit_custom_theme.existing
  id = "theme-abc123"
}

resource "doit_custom_theme" "existing" {
  name          = "Imported Theme"
  primary_color = "#FF5722"

  colors = {
    light = ["#FF5722", "#FF9800", "#FFC107"]
    dark  = ["#FF8A65", "#FFB74D", "#FFD54F"]
  }
}
