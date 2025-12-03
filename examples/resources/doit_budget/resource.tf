# Create an attribution for the budget
resource "doit_attribution" "attribution" {
  name        = "My Attribution"
  description = "My Attribution Description"
  formula     = "A"
  components  = [{ type = "fixed", key = "project_id", values = ["847764956835"] }] # Note that 'project_id' is also used for AWS account IDs
}

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
    doit_attribution.attribution.id
  ]
}
