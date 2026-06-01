data "doit_active_theme" "current" {}

output "active_theme_id" {
  value = data.doit_active_theme.current.theme_id
}
