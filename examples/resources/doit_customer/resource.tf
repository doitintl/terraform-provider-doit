# First import the customer resource:
#   terraform import doit_customer.main <customer-id>
#
# (Your customer ID can be found in the DoiT Console URL: https://console.doit.com/customers/<customer-id>
# or in DoiT Console under Settings)
#
# Then manage customer general settings:
resource "doit_customer" "main" {
  url_slug = "my-company"

  settings = {
    currency               = "USD"
    allowed_invite_domains = ["mycompany.com"]
    mfa_required           = true
  }

  contact = {
    emails = ["admin@mycompany.com"]
  }
}
