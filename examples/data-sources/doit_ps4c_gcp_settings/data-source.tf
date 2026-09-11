# Retrieve PS4C commitment settings across all GCP billing accounts
data "doit_ps4c_gcp_settings" "all" {}

output "settings_count" {
  value = data.doit_ps4c_gcp_settings.all.row_count
}

output "settings" {
  value = data.doit_ps4c_gcp_settings.all.items
}
