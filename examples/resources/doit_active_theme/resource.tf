resource "doit_custom_theme" "corporate" {
  name          = "Corporate Blue"
  primary_color = "#1A73E8"

  colors = {
    light = ["#1A73E8", "#34A853", "#FBBC04"]
    dark  = ["#8AB4F8", "#81C995", "#FDD663"]
  }
}

# Activate the custom theme for Cloud Analytics reports
resource "doit_active_theme" "this" {
  theme_id = doit_custom_theme.corporate.id
}
