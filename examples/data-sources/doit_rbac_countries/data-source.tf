# List all available countries for geographic access policies
data "doit_rbac_countries" "all" {}

# Output list of country codes
output "country_codes" {
  value = [for c in data.doit_rbac_countries.all.countries : c.country_code]
}
