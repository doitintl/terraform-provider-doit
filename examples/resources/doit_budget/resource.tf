resource "doit_budget" "my_budget" {
  name        = "test budget terraform"
  description = "hellogo test2"
  alerts = [
    {
      percentage = 50
    },
    {
      percentage = 85,
    },
    {
      percentage = 100,
    }
  ]
  recipients = [
    "diana@doit.com"
  ]
  collaborators = [
    {
      "email" : "diana@doit.com",
      "role" : "owner"
    },
  ]
  scope = [
    "Evct3J0DYcyXIVuAXORd"
  ]
  amount            = 200
  currency          = "AUD"
  growth_per_period = 10
  time_interval     = "month"
  type              = "recurring"
  use_prev_spend    = false
}