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
      repeat = {
        every_hours = 24
      }
    }
  }
}
