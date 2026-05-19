resource "doit_custom_theme" "example" {
  name          = "Corporate Blue"
  primary_color = "#1A73E8"

  colors = {
    light = ["#1A73E8", "#34A853", "#FBBC04", "#EA4335", "#A142F4", "#24C1E0"]
    dark  = ["#8AB4F8", "#81C995", "#FDD663", "#F28B82", "#C58AF9", "#78D9EC"]
  }
}
