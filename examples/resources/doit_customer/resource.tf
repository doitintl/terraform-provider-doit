# First import the customer resource:
#   terraform import doit_customer.main <customer-id>
#
# Then manage customer general settings:
resource "doit_customer" "main" {
  url_slug = "my-company"

  settings = {
    currency               = "USD"
    allowed_invite_domains = ["mycompany.com"]
  }

  contact = {
    emails = ["admin@mycompany.com"]
  }
}
