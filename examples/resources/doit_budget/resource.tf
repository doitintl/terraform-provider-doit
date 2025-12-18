# Create a timestamp for the start period
resource "time_static" "now" {
  rfc3339 = "2025-12-01T00:00:00Z"
}

resource "doit_budget" "this" {
  name          = "My Budget"
  currency      = "AUD"
  type          = "recurring"
  amount        = 100
  time_interval = "month"
  start_period  = time_static.now.unix * 1000 # This is a UNIX timestamp in milliseconds
  # Instead of using a separate resource to get the unix timestamp, you can also use:
  # provider::time::rfc3339_parse("2025-12-01T00:00:00Z") * 1000
  alerts = [
    { percentage = 50 },
    { percentage = 80 },
    { percentage = 100 }
  ]
  collaborators = [
    {
      "email" : "me@company.com",
      "role" : "owner"
    },
  ]
  scopes = [
    {
      type   = "attribution"
      id     = "attribution"
      mode   = "is"
      values = ["ydDBFKVuz9kGlFDex8cN"]
    }
  ]
}
