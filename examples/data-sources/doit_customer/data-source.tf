# Retrieve general settings and details for the customer.
# The customer_id can be found in the DoiT Console URL:
#   https://console.doit.com/customers/<customer-id>
# or in DoiT Console under Settings.
data "doit_customer" "current" {
  customer_id = "your-customer-id"
}

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
