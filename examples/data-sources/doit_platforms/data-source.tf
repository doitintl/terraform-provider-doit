# List all available cloud platforms
data "doit_platforms" "all" {}

# Output platform names
output "platform_names" {
  value = [for p in data.doit_platforms.all.platforms : p.display_name]
}
