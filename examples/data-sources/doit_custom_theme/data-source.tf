data "doit_custom_theme" "example" {
  id = "theme-abc123"
}

output "theme_name" {
  value = data.doit_custom_theme.example.name
}
