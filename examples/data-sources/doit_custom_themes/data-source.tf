data "doit_custom_themes" "all" {}

output "theme_count" {
  value = data.doit_custom_themes.all.row_count
}

output "theme_names" {
  value = [for t in data.doit_custom_themes.all.themes : t.name]
}
