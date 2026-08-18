# Retrieve general settings and details for the authenticated customer
data "doit_customer" "current" {}

# Output customer details
output "customer_id" {
  value = data.doit_customer.current.id
}

output "customer_name" {
  value = data.doit_customer.current.name
}

output "customer_primary_domain" {
  value = data.doit_customer.current.primary_domain
}

output "customer_currency" {
  value = data.doit_customer.current.settings.currency
}
