resource "doit_user_notification" "anomaly" {
  user_id = "user_01H..."

  name = "Anomaly alerts"

  settings = {
    cost_anomalies = {
      min_severity = "warning"
      min_excess_cost = {
        amount   = "100.00"
        currency = "USD"
      }
      allocations = {
        mode = "include"
        ids  = ["alloc_abc", "alloc_def"]
      }
      # Omit `repeat` entirely when neither resend control is used.
      repeat = {
        every_hours = 24
        when_cost_grows = {
          # Required when both percent_increase and min_increase are set.
          match            = "or"
          percent_increase = 10
          min_increase = {
            amount   = "50.00"
            currency = "USD"
          }
          # Omit within_hours when the growth time window is off.
          within_hours = 12
        }
      }
    }
  }

  timeouts = {
    create = "5m"
    read   = "5m"
    update = "5m"
    delete = "5m"
  }
}
