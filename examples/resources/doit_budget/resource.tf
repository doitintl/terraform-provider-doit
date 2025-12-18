
# Create a timestamp for the start period
resource "time_static" "now" {
  rfc3339 = "2025-12-01T16:52:23.000Z"
}

resource "doit_budget" "my_budget" {
  name          = "test budget terraform"
  currency      = "AUD"
  type          = "recurring"
  amount        = 100
  time_interval = "month"
  start_period  = time_static.now.unix * 1000 # This is a UNIX timestamp in milliseconds
  # Instead of using a separate resource to get the unix timestamp, you can also use:
  # provider::time::rfc3339_parse("2025-12-01T16:52:23.000Z") * 1000
  recipients = [
    "me@company.com"
  ]
  collaborators = [
    {
      "email" : "me@company.com",
      "role" : "owner"
    },
  ]
  scope = [
    "allocation-id"
  ]
}
